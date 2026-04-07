package search

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	commonv1 "github.com/idityaGE/redyx/gen/redyx/common/v1"
)

const (
	// postsTopic is the Kafka topic for post events.
	postsTopic = "redyx.posts.v1"
	// indexerConsumerGroup is the consumer group for the search indexer.
	// Unique per-service per-topic, following the established pattern.
	indexerConsumerGroup = "search-service.redyx.posts.v1"
)

// Indexer consumes PostEvent messages from Kafka and indexes them into Meilisearch.
type Indexer struct {
	kafkaClient *kgo.Client
	meili       *MeiliClient
	logger      *zap.Logger
}

// NewIndexer creates a Kafka consumer that indexes post events into Meilisearch.
func NewIndexer(brokers []string, meili *MeiliClient, logger *zap.Logger) (*Indexer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(indexerConsumerGroup),
		kgo.ConsumeTopics(postsTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer: %w", err)
	}

	return &Indexer{
		kafkaClient: client,
		meili:       meili,
		logger:      logger,
	}, nil
}

// Run starts the consumer loop, processing PostEvents until ctx is cancelled.
// Callers should start it as `go indexer.Run(ctx)`.
func (ix *Indexer) Run(ctx context.Context) error {
	ix.logger.Info("search indexer started",
		zap.String("group", indexerConsumerGroup),
		zap.String("topic", postsTopic),
	)

	var processed, errored int64
	lastLog := time.Now()

	for {
		fetches := ix.kafkaClient.PollFetches(ctx)
		if ctx.Err() != nil {
			ix.logger.Info("search indexer shutting down",
				zap.Int64("total_processed", processed),
				zap.Int64("total_errored", errored),
			)
			return ctx.Err()
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				ix.logger.Error("kafka fetch error",
					zap.String("topic", e.Topic),
					zap.Int32("partition", e.Partition),
					zap.Error(e.Err),
				)
			}
		}

		fetches.EachRecord(func(record *kgo.Record) {
			event := &commonv1.PostEvent{}
			if err := proto.Unmarshal(record.Value, event); err != nil {
				ix.logger.Error("failed to unmarshal post event",
					zap.Error(err),
					zap.Int64("offset", record.Offset),
				)
				errored++
				return
			}

			if err := ix.processEvent(event); err != nil {
				ix.logger.Error("failed to process post event",
					zap.String("event_id", event.GetEventId()),
					zap.String("post_id", event.GetPostId()),
					zap.Error(err),
				)
				errored++
			} else {
				processed++
			}
		})

		// Commit offsets after processing the batch.
		if err := ix.kafkaClient.CommitUncommittedOffsets(ctx); err != nil {
			ix.logger.Error("failed to commit offsets", zap.Error(err))
		}

		// Log stats periodically (every 60s).
		if time.Since(lastLog) > 60*time.Second {
			ix.logger.Info("search indexer stats",
				zap.Int64("processed", processed),
				zap.Int64("errored", errored),
			)
			lastLog = time.Now()
		}
	}
}

// processEvent routes a PostEvent to the appropriate Meilisearch operation.
func (ix *Indexer) processEvent(event *commonv1.PostEvent) error {
	switch event.GetEventType() {
	case commonv1.PostEventType_POST_EVENT_TYPE_CREATED,
		commonv1.PostEventType_POST_EVENT_TYPE_UPDATED:
		var createdAt int64
		if ts := event.GetCreatedAt(); ts != nil {
			createdAt = ts.GetSeconds()
		}
		return ix.meili.IndexPost(
			event.GetPostId(),
			event.GetTitle(),
			event.GetBody(),
			event.GetAuthorUsername(),
			event.GetCommunityName(),
			event.GetVoteScore(),
			event.GetCommentCount(),
			createdAt,
		)

	case commonv1.PostEventType_POST_EVENT_TYPE_DELETED:
		return ix.meili.DeletePost(event.GetPostId())

	default:
		ix.logger.Warn("unknown post event type",
			zap.String("event_id", event.GetEventId()),
			zap.Int32("event_type", int32(event.GetEventType())),
		)
		return nil
	}
}

// Close gracefully shuts down the consumer.
func (ix *Indexer) Close() {
	ix.kafkaClient.Close()
}
