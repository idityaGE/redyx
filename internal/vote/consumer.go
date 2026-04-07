package vote

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	commonv1 "github.com/idityaGE/redyx/gen/redyx/common/v1"
)

const (
	// karmaConsumerGroup is the consumer group for karma updates in user-service.
	karmaConsumerGroup = "user-service.redyx.votes.v1"
	// processedSetTTL is the TTL for the deduplication set per author.
	processedSetTTL = 24 * time.Hour
)

// KarmaConsumer processes vote events from Kafka and updates user karma in PostgreSQL.
// Designed to be embedded in the user-service process.
type KarmaConsumer struct {
	client *kgo.Client
	rdb    *redis.Client
	db     *pgxpool.Pool // user_profiles DB
	logger *zap.Logger
}

// NewKarmaConsumer creates a Kafka consumer for vote events targeting user karma updates.
// The vote-service now populates author_id in VoteEvents (provided by the frontend),
// so post shard database pools are no longer needed here.
func NewKarmaConsumer(brokers []string, rdb *redis.Client, db *pgxpool.Pool, logger *zap.Logger) (*KarmaConsumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(karmaConsumerGroup),
		kgo.ConsumeTopics(VotesTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create karma consumer: %w", err)
	}

	return &KarmaConsumer{
		client: client,
		rdb:    rdb,
		db:     db,
		logger: logger,
	}, nil
}

// Run starts the consumer loop, processing vote events until ctx is cancelled.
// It runs in the calling goroutine — callers should start it as `go consumer.Run(ctx)`.
func (c *KarmaConsumer) Run(ctx context.Context) error {
	c.logger.Info("karma consumer started",
		zap.String("group", karmaConsumerGroup),
		zap.String("topic", VotesTopic),
	)

	var processed, skipped int64
	lastLog := time.Now()

	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			c.logger.Info("karma consumer shutting down",
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

			if err := c.processEvent(ctx, event); err != nil {
				c.logger.Error("failed to process karma event",
					zap.String("event_id", event.GetEventId()),
					zap.String("author_id", event.GetAuthorId()),
					zap.Error(err),
				)
			} else {
				processed++
			}
		})

		// Commit offsets after processing the batch
		if err := c.client.CommitUncommittedOffsets(ctx); err != nil {
			c.logger.Error("failed to commit offsets", zap.Error(err))
		}

		// Log stats periodically (every 60s)
		if time.Since(lastLog) > 60*time.Second {
			c.logger.Info("karma consumer stats",
				zap.Int64("processed", processed),
				zap.Int64("skipped", skipped),
			)
			lastLog = time.Now()
		}
	}
}

// processEvent applies a single vote event to the author's karma with deduplication.
// Deduplication uses Redis SADD on karma:processed:{author_id} with 24h TTL.
func (c *KarmaConsumer) processEvent(ctx context.Context, event *commonv1.VoteEvent) error {
	authorID := event.GetAuthorId()
	eventID := event.GetEventId()
	delta := event.GetScoreDelta()

	// author_id is now populated by vote-service (provided by the frontend).
	// Skip events with empty author_id — these are legacy/malformed events.
	if authorID == "" {
		c.logger.Debug("skipping vote event with empty author_id",
			zap.String("event_id", eventID),
			zap.String("target_id", event.GetTargetId()),
		)
		return nil
	}

	if delta == 0 {
		return nil // No karma change
	}

	// Deduplication: SADD returns 1 if the member was added (new event), 0 if already exists
	dedupeKey := fmt.Sprintf("karma:processed:%s", authorID)
	added, err := c.rdb.SAdd(ctx, dedupeKey, eventID).Result()
	if err != nil {
		return fmt.Errorf("dedup sadd: %w", err)
	}

	// Set TTL on the deduplication set (refreshed on each new event for this author)
	if err := c.rdb.Expire(ctx, dedupeKey, processedSetTTL).Err(); err != nil {
		c.logger.Warn("failed to set dedupe TTL", zap.Error(err))
	}

	if added == 0 {
		// Duplicate event — already processed
		return nil
	}

	// Apply karma delta to user profile
	_, err = c.db.Exec(ctx,
		`UPDATE profiles SET karma = karma + $1 WHERE user_id = $2`,
		delta, authorID,
	)
	if err != nil {
		return fmt.Errorf("update karma: %w", err)
	}

	return nil
}

// Close gracefully shuts down the consumer.
func (c *KarmaConsumer) Close() {
	c.client.Close()
}
