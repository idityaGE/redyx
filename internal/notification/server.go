package notification

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/redyx/redyx/gen/redyx/common/v1"
	notificationv1 "github.com/redyx/redyx/gen/redyx/notification/v1"
	"github.com/redyx/redyx/internal/platform/auth"
	perrors "github.com/redyx/redyx/internal/platform/errors"
)

// Server implements the NotificationServiceServer gRPC interface.
type Server struct {
	notificationv1.UnimplementedNotificationServiceServer
	store       *Store
	hub         *Hub
	redisClient *redis.Client
	logger      *zap.Logger
}

// NewServer creates a new notification gRPC server.
func NewServer(store *Store, hub *Hub, redisClient *redis.Client, logger *zap.Logger) *Server {
	return &Server{
		store:       store,
		hub:         hub,
		redisClient: redisClient,
		logger:      logger,
	}
}

// ListNotifications returns paginated notifications for the authenticated user.
func (s *Server) ListNotifications(ctx context.Context, req *notificationv1.ListNotificationsRequest) (*notificationv1.ListNotificationsResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("list notifications: %w", perrors.ErrUnauthenticated)
	}

	// Parse pagination — cursor-based with limit
	limit := int(req.GetPagination().GetLimit())
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	// For notifications, we use offset-based pagination internally
	// but expose cursor externally. Cursor encodes the offset.
	offset := 0
	if cursor := req.GetPagination().GetCursor(); cursor != "" {
		// cursor is the offset as a string
		if n, err := strconv.Atoi(cursor); err == nil && n >= 0 {
			offset = n
		}
	}

	notifications, unreadCount, err := s.store.ListByUser(ctx, claims.UserID, req.GetUnreadOnly(), limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	// Map to proto
	protoNotifications := make([]*notificationv1.Notification, 0, len(notifications))
	for _, n := range notifications {
		protoNotifications = append(protoNotifications, notificationToProto(&n))
	}

	// Build pagination response
	hasMore := len(notifications) == limit
	nextCursor := ""
	if hasMore {
		nextCursor = strconv.Itoa(offset + limit)
	}

	return &notificationv1.ListNotificationsResponse{
		Notifications: protoNotifications,
		UnreadCount:   int32(unreadCount),
		Pagination: &commonv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    hasMore,
			TotalCount: int32(unreadCount),
		},
	}, nil
}

// MarkRead marks a single notification as read.
func (s *Server) MarkRead(ctx context.Context, req *notificationv1.MarkReadRequest) (*notificationv1.MarkReadResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("mark read: %w", perrors.ErrUnauthenticated)
	}

	notificationID := req.GetNotificationId()
	if notificationID == "" {
		return nil, fmt.Errorf("notification_id is required: %w", perrors.ErrInvalidInput)
	}

	if err := s.store.MarkRead(ctx, notificationID, claims.UserID); err != nil {
		return nil, fmt.Errorf("mark read: %w", err)
	}

	// Invalidate Redis unread count cache for user
	s.invalidateUnreadCache(ctx, claims.UserID)

	return &notificationv1.MarkReadResponse{}, nil
}

// MarkAllRead marks all notifications as read for the authenticated user.
func (s *Server) MarkAllRead(ctx context.Context, _ *notificationv1.MarkAllReadRequest) (*notificationv1.MarkAllReadResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("mark all read: %w", perrors.ErrUnauthenticated)
	}

	markedCount, err := s.store.MarkAllRead(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("mark all read: %w", err)
	}

	// Invalidate Redis unread count cache for user
	s.invalidateUnreadCache(ctx, claims.UserID)

	return &notificationv1.MarkAllReadResponse{
		MarkedCount: int32(markedCount),
	}, nil
}

// GetPreferences returns notification preferences for the authenticated user.
func (s *Server) GetPreferences(ctx context.Context, _ *notificationv1.GetPreferencesRequest) (*notificationv1.GetPreferencesResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("get preferences: %w", perrors.ErrUnauthenticated)
	}

	prefs, err := s.store.GetPreferences(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("get preferences: %w", err)
	}

	return &notificationv1.GetPreferencesResponse{
		Preferences: prefsToProto(prefs),
	}, nil
}

// UpdatePreferences updates notification preferences.
func (s *Server) UpdatePreferences(ctx context.Context, req *notificationv1.UpdatePreferencesRequest) (*notificationv1.UpdatePreferencesResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("update preferences: %w", perrors.ErrUnauthenticated)
	}

	if req.GetPreferences() == nil {
		return nil, fmt.Errorf("preferences is required: %w", perrors.ErrInvalidInput)
	}

	prefs := &NotificationPreferences{
		PostReplies:      req.GetPreferences().GetPostReplies(),
		CommentReplies:   req.GetPreferences().GetCommentReplies(),
		Mentions:         req.GetPreferences().GetMentions(),
		MutedCommunities: req.GetPreferences().GetMutedCommunities(),
	}

	if prefs.MutedCommunities == nil {
		prefs.MutedCommunities = []string{}
	}

	updated, err := s.store.UpdatePreferences(ctx, claims.UserID, prefs)
	if err != nil {
		return nil, fmt.Errorf("update preferences: %w", err)
	}

	return &notificationv1.UpdatePreferencesResponse{
		Preferences: prefsToProto(updated),
	}, nil
}

// invalidateUnreadCache removes the cached unread count for a user from Redis.
func (s *Server) invalidateUnreadCache(ctx context.Context, userID string) {
	if s.redisClient == nil {
		return
	}
	key := fmt.Sprintf("notifications:unread:%s", userID)
	if err := s.redisClient.Del(ctx, key).Err(); err != nil {
		s.logger.Debug("failed to invalidate unread cache",
			zap.String("user_id", userID),
			zap.Error(err),
		)
	}
}

// notificationToProto converts an internal Notification to proto.
func notificationToProto(n *Notification) *notificationv1.Notification {
	var notifType notificationv1.NotificationType
	switch n.Type {
	case "post_reply":
		notifType = notificationv1.NotificationType_NOTIFICATION_TYPE_POST_REPLY
	case "comment_reply":
		notifType = notificationv1.NotificationType_NOTIFICATION_TYPE_COMMENT_REPLY
	case "mention":
		notifType = notificationv1.NotificationType_NOTIFICATION_TYPE_MENTION
	default:
		notifType = notificationv1.NotificationType_NOTIFICATION_TYPE_UNSPECIFIED
	}

	return &notificationv1.Notification{
		NotificationId: n.ID,
		Type:           notifType,
		ActorId:        n.ActorID,
		ActorUsername:  n.ActorUsername,
		TargetId:       n.TargetID,
		TargetType:     n.TargetType,
		PostId:         n.PostID,
		CommunityName:  n.CommunityName,
		Message:        n.Message,
		IsRead:         n.IsRead,
		CreatedAt:      timestamppb.New(n.CreatedAt),
	}
}

// prefsToProto converts internal NotificationPreferences to proto.
func prefsToProto(p *NotificationPreferences) *notificationv1.NotificationPreferences {
	return &notificationv1.NotificationPreferences{
		PostReplies:      p.PostReplies,
		CommentReplies:   p.CommentReplies,
		Mentions:         p.Mentions,
		MutedCommunities: p.MutedCommunities,
	}
}
