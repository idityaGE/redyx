package comment

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	commonv1 "github.com/redyx/redyx/gen/redyx/common/v1"
)

const (
	// voteConsumerGroup is the Kafka consumer group for comment vote updates.
	// Separate from post-service's group so both consume the same topic independently.
	voteConsumerGroup = "comment-service.redyx.votes.v1"
	// votesTopic is the Kafka topic for vote events.
	votesTopic = "redyx.votes.v1"
)

// VoteConsumer processes vote events from Kafka and updates comment vote scores.
// It reads authoritative vote counts from vote-service Redis (DB 5) using
// set cardinality (SCARD) — this is naturally idempotent.
type VoteConsumer struct {
	store       *Store
	voteRedis   *redis.Client // Vote-service Redis (DB 5, read-only)
	logger      *zap.Logger
	kafkaClient *kgo.Client
}

// NewVoteConsumer creates a Kafka consumer for comment vote score updates.
// voteRedis should be connected to vote-service Redis (DB 5) for reading
// authoritative vote counts via SCARD.
func NewVoteConsumer(brokers []string, store *Store, voteRedis *redis.Client, logger *zap.Logger) (*VoteConsumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(voteConsumerGroup),
		kgo.ConsumeTopics(votesTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create vote consumer: %w", err)
	}

	return &VoteConsumer{
		store:       store,
		voteRedis:   voteRedis,
		logger:      logger,
		kafkaClient: client,
	}, nil
}

// Run starts the consumer loop, processing vote events until ctx is cancelled.
// It runs in the calling goroutine — callers should start it as `go consumer.Run(ctx)`.
func (c *VoteConsumer) Run(ctx context.Context) error {
	c.logger.Info("comment vote consumer started",
		zap.String("group", voteConsumerGroup),
		zap.String("topic", votesTopic),
	)

	var processed, skipped int64
	lastLog := time.Now()

	for {
		fetches := c.kafkaClient.PollFetches(ctx)
		if ctx.Err() != nil {
			c.logger.Info("comment vote consumer shutting down",
				zap.Int64("total_processed", processed),
				zap.Int64("total_skipped", skipped),
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
			event := &commonv1.VoteEvent{}
			if err := proto.Unmarshal(record.Value, event); err != nil {
				c.logger.Error("failed to unmarshal vote event", zap.Error(err))
				return
			}

			// Only process comment votes — post votes are handled by post-service
			if event.GetTargetType() != "comment" {
				skipped++
				return
			}

			if err := c.processEvent(ctx, event); err != nil {
				c.logger.Error("failed to process comment vote event",
					zap.String("event_id", event.GetEventId()),
					zap.String("target_id", event.GetTargetId()),
					zap.Error(err),
				)
			} else {
				processed++
			}
		})

		// Commit offsets after processing the batch
		if err := c.kafkaClient.CommitUncommittedOffsets(ctx); err != nil {
			c.logger.Error("failed to commit offsets", zap.Error(err))
		}

		// Log stats periodically (every 60s)
		if time.Since(lastLog) > 60*time.Second {
			c.logger.Info("comment vote consumer stats",
				zap.Int64("processed", processed),
				zap.Int64("skipped", skipped),
			)
			lastLog = time.Now()
		}
	}
}

// processEvent reads authoritative vote counts from vote-service Redis (DB 5)
// and updates the comment's scores in ScyllaDB.
func (c *VoteConsumer) processEvent(ctx context.Context, event *commonv1.VoteEvent) error {
	targetID := event.GetTargetId()

	// Read authoritative vote counts from vote-service Redis sets
	upvotes, err := c.voteRedis.SCard(ctx, fmt.Sprintf("votes:up:%s", targetID)).Result()
	if err != nil {
		return fmt.Errorf("scard upvotes: %w", err)
	}
	downvotes, err := c.voteRedis.SCard(ctx, fmt.Sprintf("votes:down:%s", targetID)).Result()
	if err != nil {
		return fmt.Errorf("scard downvotes: %w", err)
	}

	voteScore := int(upvotes) - int(downvotes)

	if err := c.store.UpdateVoteScore(ctx, targetID, voteScore, int(upvotes), int(downvotes)); err != nil {
		return fmt.Errorf("update vote score: %w", err)
	}

	return nil
}

// Close gracefully shuts down the consumer.
func (c *VoteConsumer) Close() {
	c.kafkaClient.Close()
}
