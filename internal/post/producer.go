package post

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/idityaGE/redyx/gen/redyx/events/v1"
)

const (
	// TopicPosts is the Kafka topic for post events.
	TopicPosts = "redyx.posts.v1"
)

// PostProducer wraps a franz-go Kafka client for publishing post events.
type PostProducer struct {
	client *kgo.Client
	logger *zap.Logger
}

// NewPostProducer creates a Kafka producer connected to the given brokers.
func NewPostProducer(brokers []string, logger *zap.Logger) (*PostProducer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.DefaultProduceTopic(TopicPosts),
		kgo.ProducerBatchCompression(kgo.NoCompression()),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka post producer: %w", err)
	}
	return &PostProducer{client: client, logger: logger}, nil
}

// EnsureTopic creates the posts topic with the correct partition count.
// Safe to call on every startup — existing topics are ignored.
func (p *PostProducer) EnsureTopic(ctx context.Context) error {
	const partitions = 6
	adm := kadm.NewClient(p.client)

	resp, err := adm.CreateTopics(ctx, partitions, 1, map[string]*string{
		"retention.ms": strPtr("604800000"), // 7 days
	}, TopicPosts)
	if err != nil {
		return fmt.Errorf("create topic: %w", err)
	}

	for _, r := range resp.Sorted() {
		if r.Err != nil {
			if r.ErrMessage != "" && r.ErrMessage != "Topic with this name already exists" {
				p.logger.Warn("topic creation issue",
					zap.String("topic", r.Topic),
					zap.String("error", r.ErrMessage),
				)
			}
		} else {
			p.logger.Info("created topic",
				zap.String("topic", r.Topic),
				zap.Int32("partitions", int32(partitions)),
			)
		}
	}

	return nil
}

func strPtr(s string) *string {
	return &s
}

// Publish serializes and publishes a PostEvent to Kafka asynchronously.
// Uses context.Background() for fire-and-forget — the gRPC request context
// gets canceled when the RPC returns, which would cancel the pending produce.
func (p *PostProducer) Publish(_ context.Context, event *eventsv1.PostEvent) {
	data, err := proto.Marshal(event)
	if err != nil {
		p.logger.Error("failed to marshal post event", zap.Error(err))
		return
	}

	record := &kgo.Record{
		Key:   []byte(event.GetPostId()),
		Value: data,
		Topic: TopicPosts,
	}

	p.client.Produce(context.Background(), record, func(r *kgo.Record, err error) {
		if err != nil {
			p.logger.Error("failed to produce post event",
				zap.String("post_id", event.GetPostId()),
				zap.String("event_id", event.GetEventId()),
				zap.Error(err),
			)
		}
	})
}

// Close flushes pending produces and closes the Kafka client.
func (p *PostProducer) Close() {
	p.client.Flush(context.Background())
	p.client.Close()
}
