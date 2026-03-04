package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Notification represents a notification record in PostgreSQL.
type Notification struct {
	ID            string
	UserID        string
	Type          string // "post_reply", "comment_reply", "mention"
	ActorID       string
	ActorUsername string
	TargetID      string
	TargetType    string // "post" or "comment"
	PostID        string
	CommunityName string
	Message       string
	IsRead        bool
	CreatedAt     time.Time
}

// NotificationPreferences holds per-user notification settings.
type NotificationPreferences struct {
	UserID           string
	PostReplies      bool
	CommentReplies   bool
	Mentions         bool
	MutedCommunities []string
	UpdatedAt        time.Time
}

// Store provides PostgreSQL notification + preferences storage.
type Store struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewStore creates a new notification store backed by PostgreSQL.
func NewStore(pool *pgxpool.Pool, logger *zap.Logger) *Store {
	return &Store{pool: pool, logger: logger}
}

// Create inserts a new notification and returns its ID.
func (s *Store) Create(ctx context.Context, n *Notification) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO notifications (user_id, type, actor_id, actor_username, target_id, target_type, post_id, community_name, message)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id`,
		n.UserID, n.Type, n.ActorID, n.ActorUsername,
		n.TargetID, n.TargetType, n.PostID, n.CommunityName, n.Message,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert notification: %w", err)
	}
	return id, nil
}

// ListByUser returns paginated notifications for a user, optionally filtered to unread only.
// Also returns the total unread count for badge display.
func (s *Store) ListByUser(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]Notification, int, error) {
	// Get total unread count
	var unreadCount int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false`,
		userID,
	).Scan(&unreadCount)
	if err != nil {
		return nil, 0, fmt.Errorf("count unread: %w", err)
	}

	// Build query with optional filter
	query := `SELECT id, user_id, type, actor_id, actor_username, target_id, target_type, post_id, community_name, message, is_read, created_at
		FROM notifications WHERE user_id = $1`
	args := []any{userID}

	if unreadOnly {
		query += ` AND is_read = false`
	}

	query += ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.ActorID, &n.ActorUsername,
			&n.TargetID, &n.TargetType, &n.PostID, &n.CommunityName,
			&n.Message, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		notifications = append(notifications, n)
	}

	return notifications, unreadCount, nil
}

// MarkRead marks a single notification as read for the given user.
func (s *Store) MarkRead(ctx context.Context, notificationID, userID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE notifications SET is_read = true WHERE id = $1 AND user_id = $2`,
		notificationID, userID,
	)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

// MarkAllRead marks all unread notifications as read for the given user.
// Returns the number of notifications that were marked.
func (s *Store) MarkAllRead(ctx context.Context, userID string) (int, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE notifications SET is_read = true WHERE user_id = $1 AND is_read = false`,
		userID,
	)
	if err != nil {
		return 0, fmt.Errorf("mark all read: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// GetUnreadCount returns the count of unread notifications for a user.
func (s *Store) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get unread count: %w", err)
	}
	return count, nil
}

// GetPreferences returns notification preferences for a user.
// Returns default preferences (all enabled, no muted communities) if no row exists.
func (s *Store) GetPreferences(ctx context.Context, userID string) (*NotificationPreferences, error) {
	prefs := &NotificationPreferences{
		UserID:           userID,
		PostReplies:      true,
		CommentReplies:   true,
		Mentions:         true,
		MutedCommunities: []string{},
	}

	err := s.pool.QueryRow(ctx,
		`SELECT post_replies, comment_replies, mentions, muted_communities, updated_at
		 FROM notification_preferences WHERE user_id = $1`,
		userID,
	).Scan(&prefs.PostReplies, &prefs.CommentReplies, &prefs.Mentions, &prefs.MutedCommunities, &prefs.UpdatedAt)

	if err == pgx.ErrNoRows {
		return prefs, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get preferences: %w", err)
	}
	return prefs, nil
}

// UpdatePreferences upserts notification preferences for a user.
func (s *Store) UpdatePreferences(ctx context.Context, userID string, prefs *NotificationPreferences) (*NotificationPreferences, error) {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO notification_preferences (user_id, post_replies, comment_replies, mentions, muted_communities, updated_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET
		   post_replies = EXCLUDED.post_replies,
		   comment_replies = EXCLUDED.comment_replies,
		   mentions = EXCLUDED.mentions,
		   muted_communities = EXCLUDED.muted_communities,
		   updated_at = NOW()
		 RETURNING post_replies, comment_replies, mentions, muted_communities, updated_at`,
		userID, prefs.PostReplies, prefs.CommentReplies, prefs.Mentions, prefs.MutedCommunities,
	).Scan(&prefs.PostReplies, &prefs.CommentReplies, &prefs.Mentions, &prefs.MutedCommunities, &prefs.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("upsert preferences: %w", err)
	}

	prefs.UserID = userID
	return prefs, nil
}

// GetUndeliveredSince returns notifications created after the given time for a user.
// Used to deliver offline notifications when a WebSocket connection is established.
func (s *Store) GetUndeliveredSince(ctx context.Context, userID string, since time.Time) ([]Notification, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, type, actor_id, actor_username, target_id, target_type, post_id, community_name, message, is_read, created_at
		 FROM notifications
		 WHERE user_id = $1 AND created_at > $2 AND is_read = false
		 ORDER BY created_at ASC`,
		userID, since,
	)
	if err != nil {
		return nil, fmt.Errorf("get undelivered: %w", err)
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.ActorID, &n.ActorUsername,
			&n.TargetID, &n.TargetType, &n.PostID, &n.CommunityName,
			&n.Message, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan undelivered: %w", err)
		}
		notifications = append(notifications, n)
	}

	return notifications, nil
}
