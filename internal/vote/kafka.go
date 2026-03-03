package vote

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	commonv1 "github.com/redyx/redyx/gen/redyx/common/v1"
)

const (
	// VotesTopic is the Kafka topic for vote events.
	VotesTopic = "redyx.votes.v1"
	// votesTopicPartitions is the number of partitions for the votes topic.
	votesTopicPartitions = 6
)

// Producer wraps a franz-go Kafka client for publishing vote events.
type Producer struct {
	client *kgo.Client
	logger *zap.Logger
}

// NewProducer creates a Kafka producer connected to the given brokers.
func NewProducer(brokers []string, logger *zap.Logger) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.DefaultProduceTopic(VotesTopic),
		kgo.ProducerBatchCompression(kgo.NoCompression()),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	return &Producer{client: client, logger: logger}, nil
}

// EnsureTopic creates the votes topic with the correct partition count and retention.
// It is safe to call on every startup — existing topics are ignored.
func (p *Producer) EnsureTopic(ctx context.Context) error {
	adm := kadm.NewClient(p.client)

	resp, err := adm.CreateTopics(ctx, votesTopicPartitions, 1, map[string]*string{
		"retention.ms": strPtr("604800000"), // 7 days
	}, VotesTopic)
	if err != nil {
		return fmt.Errorf("create topic: %w", err)
	}

	for _, r := range resp.Sorted() {
		if r.Err != nil {
			// "topic already exists" is fine
			if r.ErrMessage != "" && r.ErrMessage != "Topic with this name already exists" {
				p.logger.Warn("topic creation issue",
					zap.String("topic", r.Topic),
					zap.String("error", r.ErrMessage),
				)
			}
		} else {
			p.logger.Info("created topic",
				zap.String("topic", r.Topic),
				zap.Int32("partitions", int32(votesTopicPartitions)),
			)
		}
	}

	return nil
}

// PublishVoteEvent serializes and publishes a VoteEvent to Kafka asynchronously.
// The target_id is used as the partition key to ensure ordering per target.
// This method does not block the caller — errors are logged but not returned
// to keep the Vote RPC fast.
func (p *Producer) PublishVoteEvent(ctx context.Context, event *commonv1.VoteEvent) {
	data, err := proto.Marshal(event)
	if err != nil {
		p.logger.Error("failed to marshal vote event", zap.Error(err))
		return
	}

	record := &kgo.Record{
		Key:   []byte(event.GetTargetId()),
		Value: data,
		Topic: VotesTopic,
	}

	p.client.Produce(ctx, record, func(r *kgo.Record, err error) {
		if err != nil {
			p.logger.Error("failed to produce vote event",
				zap.String("target_id", event.GetTargetId()),
				zap.String("event_id", event.GetEventId()),
				zap.Error(err),
			)
		}
	})
}

// Close flushes pending produces and closes the Kafka client.
func (p *Producer) Close() {
	p.client.Flush(context.Background())
	p.client.Close()
}

func strPtr(s string) *string {
	return &s
}
