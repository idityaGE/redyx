// Package media implements the media-service backend for file uploads via presigned S3/MinIO URLs
// with thumbnail generation and PostgreSQL metadata tracking.
package media

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// MediaItem represents a media record in the database.
type MediaItem struct {
	ID           string
	UserID       string
	Filename     string
	ContentType  string
	SizeBytes    int64
	MediaType    string // "image", "video", "gif"
	Status       string // "pending", "processing", "ready", "failed"
	S3Key        string
	URL          string
	ThumbnailURL string
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Store provides PostgreSQL media metadata CRUD operations.
type Store struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewStore creates a new media metadata store backed by PostgreSQL.
func NewStore(pool *pgxpool.Pool, logger *zap.Logger) *Store {
	return &Store{pool: pool, logger: logger}
}

// Create inserts a new media item and returns its ID.
func (s *Store) Create(ctx context.Context, item MediaItem) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO media_items (user_id, filename, content_type, size_bytes, media_type, status, s3_key)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		item.UserID, item.Filename, item.ContentType, item.SizeBytes, item.MediaType, item.Status, item.S3Key,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert media item: %w", err)
	}
	s.logger.Debug("created media item", zap.String("id", id), zap.String("user_id", item.UserID))
	return id, nil
}

// Get retrieves a media item by ID.
func (s *Store) Get(ctx context.Context, mediaID string) (*MediaItem, error) {
	item := &MediaItem{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, filename, content_type, size_bytes, media_type, status, s3_key,
		        COALESCE(url, ''), COALESCE(thumbnail_url, ''), COALESCE(error_message, ''),
		        created_at, updated_at
		 FROM media_items WHERE id = $1`,
		mediaID,
	).Scan(
		&item.ID, &item.UserID, &item.Filename, &item.ContentType, &item.SizeBytes,
		&item.MediaType, &item.Status, &item.S3Key,
		&item.URL, &item.ThumbnailURL, &item.ErrorMessage,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get media item %s: %w", mediaID, err)
	}
	return item, nil
}

// UpdateStatus updates the status, URLs, and error message for a media item.
func (s *Store) UpdateStatus(ctx context.Context, mediaID, status, url, thumbnailURL, errorMsg string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE media_items
		 SET status = $1, url = $2, thumbnail_url = $3, error_message = $4, updated_at = NOW()
		 WHERE id = $5`,
		status, url, thumbnailURL, errorMsg, mediaID,
	)
	if err != nil {
		return fmt.Errorf("update media item %s status: %w", mediaID, err)
	}
	s.logger.Debug("updated media status", zap.String("id", mediaID), zap.String("status", status))
	return nil
}
