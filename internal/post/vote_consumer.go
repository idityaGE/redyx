// Package post implements the PostService gRPC server with shard-aware
// storage, ranking algorithms, and feed aggregation.
package post

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
	// scoreConsumerGroup is the Kafka consumer group for post score updates.
	scoreConsumerGroup = "post-service.redyx.votes.v1"
	// votesTopic is the Kafka topic for vote events.
	votesTopic = "redyx.votes.v1"
)

// ScoreConsumer processes vote events from Kafka and updates post vote_score,
// upvotes, downvotes, and hot_score on the correct shard.
// It reads authoritative vote counts from vote-service Redis (DB 5) using
// set cardinality (SCARD) — this is naturally idempotent.
type ScoreConsumer struct {
	shards      *ShardRouter
	voteRdb     *redis.Client // Vote-service Redis (DB 5, read-only)
	logger      *zap.Logger
	kafkaClient *kgo.Client
}

// NewScoreConsumer creates a Kafka consumer for post score updates.
// voteRdb should be connected to vote-service Redis (DB 5) for reading
// authoritative vote counts via SCARD.
func NewScoreConsumer(brokers []string, shards *ShardRouter, voteRdb *redis.Client, logger *zap.Logger) (*ScoreConsumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(scoreConsumerGroup),
		kgo.ConsumeTopics(votesTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create score consumer: %w", err)
	}

	return &ScoreConsumer{
		shards:      shards,
		voteRdb:     voteRdb,
		logger:      logger,
		kafkaClient: client,
	}, nil
}

// Run starts the consumer loop, processing vote events until ctx is cancelled.
// It runs in the calling goroutine — callers should start it as `go consumer.Run(ctx)`.
func (c *ScoreConsumer) Run(ctx context.Context) error {
	c.logger.Info("score consumer started",
		zap.String("group", scoreConsumerGroup),
		zap.String("topic", votesTopic),
	)

	var processed, skipped int64
	lastLog := time.Now()

	for {
		fetches := c.kafkaClient.PollFetches(ctx)
		if ctx.Err() != nil {
			c.logger.Info("score consumer shutting down",
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

			// Only process post votes — comment votes are Phase 4
			if event.GetTargetType() != "post" {
				skipped++
				return
			}

			if err := c.processEvent(ctx, event); err != nil {
				c.logger.Error("failed to process score event",
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
			c.logger.Info("score consumer stats",
				zap.Int64("processed", processed),
				zap.Int64("skipped", skipped),
			)
			lastLog = time.Now()
		}
	}
}

// processEvent reads authoritative vote counts from vote-service Redis (DB 5)
// and updates the post's scores on the correct shard.
// This is naturally idempotent — SCARD always returns current set size.
func (c *ScoreConsumer) processEvent(ctx context.Context, event *commonv1.VoteEvent) error {
	targetID := event.GetTargetId()
	communityID := event.GetCommunityId()

	// Read authoritative vote counts from vote-service Redis sets
	upvotes, err := c.voteRdb.SCard(ctx, fmt.Sprintf("votes:up:%s", targetID)).Result()
	if err != nil {
		return fmt.Errorf("scard upvotes: %w", err)
	}
	downvotes, err := c.voteRdb.SCard(ctx, fmt.Sprintf("votes:down:%s", targetID)).Result()
	if err != nil {
		return fmt.Errorf("scard downvotes: %w", err)
	}

	newScore := int(upvotes) - int(downvotes)

	// Route to the correct shard
	var pool *pgxpool.Pool

	if communityID != "" {
		// Direct shard routing via community_id
		pool, _ = c.shards.GetPool(communityID)
	} else {
		// Fallback: query all shards to find the post
		pool = c.findPostShard(ctx, targetID)
	}

	if pool == nil {
		return fmt.Errorf("post %s not found on any shard", targetID)
	}

	// Update post scores with hot_score computed in SQL
	// hot_score = 10000.0 * ln(GREATEST(1, 3 + vote_score)) / POWER(hours_age + 2, 1.8)
	_, err = pool.Exec(ctx,
		`UPDATE posts SET
			vote_score = $1,
			upvotes = $2,
			downvotes = $3,
			hot_score = 10000.0 * ln(GREATEST(1, 3 + $1)) / POWER(EXTRACT(EPOCH FROM (now() - created_at)) / 3600.0 + 2, 1.8)
		WHERE id = $4 AND is_deleted = false`,
		newScore, int(upvotes), int(downvotes), targetID,
	)
	if err != nil {
		return fmt.Errorf("update post scores: %w", err)
	}

	return nil
}

// findPostShard queries all shards to find which one contains the given post.
// This is the fallback when community_id is not available in the event.
func (c *ScoreConsumer) findPostShard(ctx context.Context, postID string) *pgxpool.Pool {
	for _, pool := range c.shards.AllPools() {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1)`,
			postID,
		).Scan(&exists)
		if err != nil {
			c.logger.Warn("error checking shard for post",
				zap.String("post_id", postID),
				zap.Error(err),
			)
			continue
		}
		if exists {
			return pool
		}
	}
	return nil
}

// Close gracefully shuts down the consumer.
func (c *ScoreConsumer) Close() {
	c.kafkaClient.Close()
}
