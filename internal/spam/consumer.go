package spam

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	commonv1 "github.com/redyx/redyx/gen/redyx/common/v1"
	modv1 "github.com/redyx/redyx/gen/redyx/moderation/v1"
)

const (
	// behaviorConsumerGroup is the Kafka consumer group for spam behavior analysis.
	behaviorConsumerGroup = "spam-service.redyx.posts.v1"
	// postsTopic is the existing Kafka topic for post events.
	postsTopic = "redyx.posts.v1"
	// behaviorTopic is a future-extensibility topic for behavior events.
	behaviorTopic = "redyx.behavior.v1"
	// behaviorTopicPartitions is the partition count for the behavior topic.
	behaviorTopicPartitions = 3

	// rapidPostThreshold is the maximum posts allowed in the rapid posting window.
	rapidPostThreshold = 10
	// rapidPostWindow is the sliding window for rapid posting detection (5 minutes).
	rapidPostWindow = 5 * time.Minute

	// linkSpamThreshold is the maximum link-containing posts in the link spam window.
	linkSpamThreshold = 5
	// linkSpamWindow is the sliding window for link spam detection (1 hour).
	linkSpamWindow = 1 * time.Hour
)

// BehaviorConsumer processes PostEvents from Kafka and detects abusive patterns:
// - Rapid posting: >10 posts in 5 minutes
// - Link spam: >5 posts with URLs in 1 hour
// Detected spam is reported to the moderation service for review.
type BehaviorConsumer struct {
	client           *kgo.Client
	rdb              *redis.Client
	moderationClient modv1.ModerationServiceClient
	logger           *zap.Logger
}

// NewBehaviorConsumer creates a Kafka consumer for post behavior analysis.
// It consumes from the existing posts topic and creates the behavior topic for future use.
func NewBehaviorConsumer(brokers []string, rdb *redis.Client, moderationClient modv1.ModerationServiceClient, logger *zap.Logger) (*BehaviorConsumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(behaviorConsumerGroup),
		kgo.ConsumeTopics(postsTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create behavior consumer: %w", err)
	}

	// Create behavior topic for future extensibility (idempotent)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ensureBehaviorTopic(ctx, client, logger)

	return &BehaviorConsumer{
		client:           client,
		rdb:              rdb,
		moderationClient: moderationClient,
		logger:           logger,
	}, nil
}

// ensureBehaviorTopic creates the behavior topic with kadm (idempotent).
func ensureBehaviorTopic(ctx context.Context, client *kgo.Client, logger *zap.Logger) {
	adm := kadm.NewClient(client)
	resp, err := adm.CreateTopics(ctx, behaviorTopicPartitions, 1, map[string]*string{
		"retention.ms": strPtr("604800000"), // 7 days
	}, behaviorTopic)
	if err != nil {
		logger.Warn("failed to create behavior topic (may not be fatal)",
			zap.Error(err),
		)
		return
	}
	for _, r := range resp.Sorted() {
		if r.Err != nil {
			if r.ErrMessage != "" && r.ErrMessage != "Topic with this name already exists" {
				logger.Warn("behavior topic creation issue",
					zap.String("topic", r.Topic),
					zap.String("error", r.ErrMessage),
				)
			}
		} else {
			logger.Info("created behavior topic",
				zap.String("topic", r.Topic),
				zap.Int32("partitions", int32(behaviorTopicPartitions)),
			)
		}
	}
}

// Run starts the consumer loop, processing post events until ctx is cancelled.
// It runs in the calling goroutine — callers should start it as `go consumer.Run(ctx)`.
func (c *BehaviorConsumer) Run(ctx context.Context) error {
	c.logger.Info("behavior consumer started",
		zap.String("group", behaviorConsumerGroup),
		zap.String("topic", postsTopic),
	)

	var processed, flagged int64
	lastLog := time.Now()

	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			c.logger.Info("behavior consumer shutting down",
				zap.Int64("total_processed", processed),
				zap.Int64("total_flagged", flagged),
			)
			return ctx.Err()
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				c.logger.Error("kafka fetch error",
					zap.String("topic", e.Topic),
					zap.Int32("partition", e.Partition),
					zap.Error(e.Err),
				)
			}
		}

		fetches.EachRecord(func(record *kgo.Record) {
			event := &commonv1.PostEvent{}
			if err := proto.Unmarshal(record.Value, event); err != nil {
				c.logger.Error("failed to unmarshal post event", zap.Error(err))
				return
			}

			wasFlag, err := c.analyzePostBehavior(ctx, event)
			if err != nil {
				c.logger.Error("failed to analyze post behavior",
					zap.String("post_id", event.GetPostId()),
					zap.String("author", event.GetAuthorUsername()),
					zap.Error(err),
				)
			} else {
				processed++
				if wasFlag {
					flagged++
				}
			}
		})

		// Commit offsets after processing the batch
		if err := c.client.CommitUncommittedOffsets(ctx); err != nil {
			c.logger.Error("failed to commit offsets", zap.Error(err))
		}

		// Log stats periodically (every 60s)
		if time.Since(lastLog) > 60*time.Second {
			c.logger.Info("behavior consumer stats",
				zap.Int64("processed", processed),
				zap.Int64("flagged", flagged),
			)
			lastLog = time.Now()
		}
	}
}

// analyzePostBehavior checks a post event for rapid posting and link spam patterns.
// Only processes POST_EVENT_TYPE_CREATED events.
// Returns true if the event was flagged, false otherwise.
func (c *BehaviorConsumer) analyzePostBehavior(ctx context.Context, event *commonv1.PostEvent) (bool, error) {
	// Only analyze new posts
	if event.GetEventType() != commonv1.PostEventType_POST_EVENT_TYPE_CREATED {
		return false, nil
	}

	author := event.GetAuthorUsername()
	if author == "" {
		return false, nil
	}

	flagged := false

	// 1. Rapid posting detection: sliding window with INCR + EXPIRE
	rapidKey := fmt.Sprintf("behavior:rapid:%s", author)
	count, err := c.rdb.Incr(ctx, rapidKey).Result()
	if err != nil {
		return false, fmt.Errorf("rapid posting incr: %w", err)
	}
	// Set expiry only on first increment (count == 1)
	if count == 1 {
		c.rdb.Expire(ctx, rapidKey, rapidPostWindow)
	}
	if count > int64(rapidPostThreshold) {
		c.logger.Warn("rapid posting detected",
			zap.String("author", author),
			zap.Int64("count", count),
			zap.String("post_id", event.GetPostId()),
		)
		c.submitFlag(ctx, event, "rapid_posting")
		flagged = true
	}

	// 2. Link spam detection: count posts with URLs in last hour
	hasLinks := strings.Contains(event.GetBody(), "http://") ||
		strings.Contains(event.GetBody(), "https://") ||
		strings.Contains(event.GetTitle(), "http://") ||
		strings.Contains(event.GetTitle(), "https://")

	if hasLinks {
		linkKey := fmt.Sprintf("behavior:links:%s", author)
		linkCount, err := c.rdb.Incr(ctx, linkKey).Result()
		if err != nil {
			return flagged, fmt.Errorf("link spam incr: %w", err)
		}
		if linkCount == 1 {
			c.rdb.Expire(ctx, linkKey, linkSpamWindow)
		}
		if linkCount > int64(linkSpamThreshold) {
			c.logger.Warn("link spam detected",
				zap.String("author", author),
				zap.Int64("link_count", linkCount),
				zap.String("post_id", event.GetPostId()),
			)
			c.submitFlag(ctx, event, "link_spam")
			flagged = true
		}
	}

	return flagged, nil
}

// submitFlag reports detected spam behavior to the moderation service.
// If the moderation client is nil (moderation service not yet available),
// it logs the detection for manual review.
func (c *BehaviorConsumer) submitFlag(ctx context.Context, event *commonv1.PostEvent, reason string) {
	if c.moderationClient == nil {
		c.logger.Info("spam behavior flagged (moderation service not connected)",
			zap.String("post_id", event.GetPostId()),
			zap.String("author", event.GetAuthorUsername()),
			zap.String("community", event.GetCommunityName()),
			zap.String("reason", reason),
			zap.String("source", "spam-detection"),
		)
		return
	}

	// Call moderation service ListReportQueue is not the right RPC.
	// The SubmitReport RPC will be added by Plan 06-01.
	// For now, log the detection — the consumer is ready to call SubmitReport
	// once the moderation service proto is extended.
	c.logger.Info("spam behavior flagged for moderation review",
		zap.String("post_id", event.GetPostId()),
		zap.String("author", event.GetAuthorUsername()),
		zap.String("community", event.GetCommunityName()),
		zap.String("reason", reason),
		zap.String("source", "spam-detection"),
	)
}

// Close gracefully shuts down the consumer.
func (c *BehaviorConsumer) Close() {
	c.client.Close()
}

func strPtr(s string) *string {
	return &s
}
