package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/redyx/redyx/gen/redyx/events/v1"
)

const (
	// commentConsumerGroup is the Kafka consumer group for notification-service
	// consuming comment events. Unique per service, follows established pattern.
	commentConsumerGroup = "notification-service.redyx.comments.v1"
	// commentsTopic is the Kafka topic for comment events.
	commentsTopic = "redyx.comments.v1"
)

// Consumer processes CommentEvents from Kafka and creates notifications.
type Consumer struct {
	client       *kgo.Client
	store        *Store
	hub          *Hub
	postResolver *PostResolver
	logger       *zap.Logger
}

// NewConsumer creates a Kafka consumer connected to the given brokers.
func NewConsumer(brokers []string, store *Store, hub *Hub, postResolver *PostResolver, logger *zap.Logger) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(commentConsumerGroup),
		kgo.ConsumeTopics(commentsTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer: %w", err)
	}

	return &Consumer{
		client:       client,
		store:        store,
		hub:          hub,
		postResolver: postResolver,
		logger:       logger,
	}, nil
}

// Run starts the consumer loop, processing comment events until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	c.logger.Info("notification consumer started",
		zap.String("group", commentConsumerGroup),
		zap.String("topic", commentsTopic),
	)

	var processed, skipped int64
	lastLog := time.Now()

	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			c.logger.Info("notification consumer shutting down",
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
			pbEvent := &eventsv1.CommentEvent{}
			if err := proto.Unmarshal(record.Value, pbEvent); err != nil {
				c.logger.Error("failed to unmarshal comment event", zap.Error(err))
				return
			}

			// Convert proto event to internal CommentEvent
			event := &CommentEvent{
				EventID:               pbEvent.GetEventId(),
				CommentID:             pbEvent.GetCommentId(),
				PostID:                pbEvent.GetPostId(),
				AuthorID:              pbEvent.GetAuthorId(),
				AuthorUsername:        pbEvent.GetAuthorUsername(),
				ParentCommentID:       pbEvent.GetParentCommentId(),
				ParentCommentAuthorID: pbEvent.GetParentCommentAuthorId(),
				PostAuthorID:          pbEvent.GetPostAuthorId(),
				CommunityName:         pbEvent.GetCommunityName(),
				Body:                  pbEvent.GetBody(),
			}
			if ts := pbEvent.GetCreatedAt(); ts != nil {
				event.CreatedAt = ts.AsTime()
			}

			if err := c.processEvent(ctx, event); err != nil {
				c.logger.Error("failed to process comment event",
					zap.String("event_id", event.EventID),
					zap.String("comment_id", event.CommentID),
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
			c.logger.Info("notification consumer stats",
				zap.Int64("processed", processed),
				zap.Int64("skipped", skipped),
			)
			lastLog = time.Now()
		}
	}
}

// processEvent handles a single CommentEvent, creating notifications for:
// 1. Post replies (top-level comment → notify post author)
// 2. Comment replies (nested comment → notify parent comment author)
// 3. Mentions (u/username in body → notify mentioned users)
func (c *Consumer) processEvent(ctx context.Context, event *CommentEvent) error {
	// Resolve missing post info (PostAuthorID, CommunityName) via post-service gRPC
	if event.PostAuthorID == "" || event.CommunityName == "" {
		if c.postResolver != nil {
			info, err := c.postResolver.Resolve(ctx, event.PostID)
			if err != nil {
				c.logger.Warn("failed to resolve post info for notification",
					zap.String("post_id", event.PostID),
					zap.Error(err),
				)
			} else {
				if event.PostAuthorID == "" {
					event.PostAuthorID = info.AuthorID
				}
				if event.CommunityName == "" {
					event.CommunityName = info.CommunityName
				}
			}
		}
	}

	// Track who we've already notified to avoid duplicates
	notified := make(map[string]bool)

	// 1. Post reply notification (top-level comment, no parent)
	if event.ParentCommentID == "" && event.PostAuthorID != "" {
		if event.AuthorID != event.PostAuthorID { // don't notify yourself
			if err := c.maybeCreateNotification(ctx, event, event.PostAuthorID, "post_reply", notified); err != nil {
				c.logger.Error("failed to create post reply notification",
					zap.String("target_user", event.PostAuthorID),
					zap.Error(err),
				)
			}
		}
	}

	// 2. Comment reply notification (nested comment)
	if event.ParentCommentID != "" && event.ParentCommentAuthorID != "" {
		if event.AuthorID != event.ParentCommentAuthorID { // don't notify yourself
			if err := c.maybeCreateNotification(ctx, event, event.ParentCommentAuthorID, "comment_reply", notified); err != nil {
				c.logger.Error("failed to create comment reply notification",
					zap.String("target_user", event.ParentCommentAuthorID),
					zap.Error(err),
				)
			}
		}
	}

	// 3. Mention notifications
	mentions := ExtractMentions(event.Body)
	for _, username := range mentions {
		// For v1, we store username-based mentions. The notification targets
		// the mentioned username. Since we don't have a user-lookup service
		// connection, we use actor_username as a key. The frontend resolves
		// the user via the username field.
		// Skip self-mentions
		if username == event.AuthorUsername {
			continue
		}

		// For mentions, we don't have the user_id — store with username as target.
		// The frontend or a future user-lookup integration can resolve this.
		n := &Notification{
			UserID:        username, // username as placeholder — resolve via user-service later
			Type:          "mention",
			ActorID:       event.AuthorID,
			ActorUsername: event.AuthorUsername,
			TargetID:      event.CommentID,
			TargetType:    "comment",
			PostID:        event.PostID,
			CommunityName: event.CommunityName,
			Message:       fmt.Sprintf("u/%s mentioned you in a comment", event.AuthorUsername),
		}

		// Check mention preferences (use username as user_id for v1)
		prefs, err := c.store.GetPreferences(ctx, username)
		if err != nil {
			c.logger.Error("failed to get preferences for mention target",
				zap.String("username", username),
				zap.Error(err),
			)
			continue
		}

		if !prefs.Mentions {
			continue
		}
		if isCommunityMuted(prefs.MutedCommunities, event.CommunityName) {
			continue
		}

		id, err := c.store.Create(ctx, n)
		if err != nil {
			c.logger.Error("failed to create mention notification",
				zap.String("username", username),
				zap.Error(err),
			)
			continue
		}

		n.ID = id
		// Push to WebSocket hub for real-time delivery
		if err := c.hub.Send(username, n); err != nil {
			c.logger.Debug("websocket push failed (user likely offline)",
				zap.String("username", username),
				zap.Error(err),
			)
		}
	}

	return nil
}

// maybeCreateNotification checks preferences and creates a notification if allowed.
func (c *Consumer) maybeCreateNotification(ctx context.Context, event *CommentEvent, targetUserID, notifType string, notified map[string]bool) error {
	// Avoid duplicate notifications to the same user
	key := targetUserID + ":" + notifType
	if notified[key] {
		return nil
	}

	// Check preferences
	prefs, err := c.store.GetPreferences(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("get preferences: %w", err)
	}

	// Check notification type preference
	switch notifType {
	case "post_reply":
		if !prefs.PostReplies {
			return nil
		}
	case "comment_reply":
		if !prefs.CommentReplies {
			return nil
		}
	}

	// Check community mute
	if isCommunityMuted(prefs.MutedCommunities, event.CommunityName) {
		return nil
	}

	// Build notification message
	var message string
	switch notifType {
	case "post_reply":
		message = fmt.Sprintf("u/%s commented on your post", event.AuthorUsername)
	case "comment_reply":
		message = fmt.Sprintf("u/%s replied to your comment", event.AuthorUsername)
	default:
		message = fmt.Sprintf("u/%s sent you a notification", event.AuthorUsername)
	}

	// Determine target type and ID
	targetID := event.CommentID
	targetType := "comment"

	n := &Notification{
		UserID:        targetUserID,
		Type:          notifType,
		ActorID:       event.AuthorID,
		ActorUsername: event.AuthorUsername,
		TargetID:      targetID,
		TargetType:    targetType,
		PostID:        event.PostID,
		CommunityName: event.CommunityName,
		Message:       message,
	}

	id, err := c.store.Create(ctx, n)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	n.ID = id
	notified[key] = true

	// Push to WebSocket hub for real-time delivery
	if err := c.hub.Send(targetUserID, n); err != nil {
		c.logger.Debug("websocket push failed (user likely offline)",
			zap.String("target_user", targetUserID),
			zap.Error(err),
		)
	}

	return nil
}

// isCommunityMuted checks if a community is in the user's muted list.
func isCommunityMuted(mutedCommunities []string, communityName string) bool {
	for _, muted := range mutedCommunities {
		if muted == communityName {
			return true
		}
	}
	return false
}

// Close gracefully shuts down the consumer.
func (c *Consumer) Close() {
	c.client.Close()
}
