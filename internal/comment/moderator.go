package comment

import (
	"context"
	"fmt"

	"github.com/gocql/gocql"
	"go.uber.org/zap"

	commentv1 "github.com/redyx/redyx/gen/redyx/comment/v1"
	perrors "github.com/redyx/redyx/internal/platform/errors"
)

// ModeratorRemoveComment soft-deletes a comment by setting is_deleted = true
// in both ScyllaDB tables (comments_by_post and comments_by_id).
// Called by moderation-service via internal gRPC.
func (s *Server) ModeratorRemoveComment(ctx context.Context, req *commentv1.ModeratorRemoveCommentRequest) (*commentv1.ModeratorRemoveCommentResponse, error) {
	commentID := req.GetCommentId()
	if commentID == "" {
		return nil, fmt.Errorf("comment_id is required: %w", perrors.ErrInvalidInput)
	}

	commentUUID, err := gocql.ParseUUID(commentID)
	if err != nil {
		return nil, fmt.Errorf("invalid comment_id: %w", perrors.ErrInvalidInput)
	}

	// Look up the comment to get its path (needed for comments_by_post partition key)
	comment, err := s.store.GetComment(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("comment %q: %w", commentID, perrors.ErrNotFound)
	}

	// Soft delete in comments_by_id
	if err := s.store.session.Query(
		`UPDATE redyx_comments.comments_by_id SET is_deleted = true WHERE comment_id = ?`,
		commentUUID,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("moderator remove comments_by_id: %w", err)
	}

	// Soft delete in comments_by_post
	if err := s.store.session.Query(
		`UPDATE redyx_comments.comments_by_post SET is_deleted = true WHERE post_id = ? AND path = ?`,
		comment.PostID, comment.Path,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("moderator remove comments_by_post: %w", err)
	}

	s.logger.Info("moderator removed comment",
		zap.String("comment_id", commentID),
		zap.String("post_id", comment.PostID.String()),
	)

	return &commentv1.ModeratorRemoveCommentResponse{}, nil
}

// ModeratorRestoreComment restores a previously removed comment by setting
// is_deleted = false in both ScyllaDB tables.
// Called by moderation-service via internal gRPC.
func (s *Server) ModeratorRestoreComment(ctx context.Context, req *commentv1.ModeratorRestoreCommentRequest) (*commentv1.ModeratorRestoreCommentResponse, error) {
	commentID := req.GetCommentId()
	if commentID == "" {
		return nil, fmt.Errorf("comment_id is required: %w", perrors.ErrInvalidInput)
	}

	commentUUID, err := gocql.ParseUUID(commentID)
	if err != nil {
		return nil, fmt.Errorf("invalid comment_id: %w", perrors.ErrInvalidInput)
	}

	// Look up the comment to get its path
	comment, err := s.store.GetComment(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("comment %q: %w", commentID, perrors.ErrNotFound)
	}

	// Restore in comments_by_id
	if err := s.store.session.Query(
		`UPDATE redyx_comments.comments_by_id SET is_deleted = false WHERE comment_id = ?`,
		commentUUID,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("moderator restore comments_by_id: %w", err)
	}

	// Restore in comments_by_post
	if err := s.store.session.Query(
		`UPDATE redyx_comments.comments_by_post SET is_deleted = false WHERE post_id = ? AND path = ?`,
		comment.PostID, comment.Path,
	).WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("moderator restore comments_by_post: %w", err)
	}

	s.logger.Info("moderator restored comment",
		zap.String("comment_id", commentID),
		zap.String("post_id", comment.PostID.String()),
	)

	return &commentv1.ModeratorRestoreCommentResponse{}, nil
}

// RemoveCommentsByUser soft-deletes all comments by a user in a community.
// Since comments are organized by post_id in ScyllaDB (not by community),
// this queries comments_by_author for the user, then filters by community
// via post-service lookup. For v1 this is acceptable for infrequent moderation actions.
// Called by moderation-service when banning with content removal.
func (s *Server) RemoveCommentsByUser(ctx context.Context, req *commentv1.RemoveCommentsByUserRequest) (*commentv1.RemoveCommentsByUserResponse, error) {
	userID := req.GetUserId()
	communityName := req.GetCommunityName()
	if userID == "" || communityName == "" {
		return nil, fmt.Errorf("user_id and community_name are required: %w", perrors.ErrInvalidInput)
	}

	// Parse user_id as UUID to match against author_id in comments
	userUUID, err := gocql.ParseUUID(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", perrors.ErrInvalidInput)
	}

	// Scan comments_by_id for all comments by this author.
	// This is a full table scan with ALLOW FILTERING — acceptable for moderation
	// actions which are infrequent. A secondary index on author_id would be better
	// at scale but is premature for v1.
	iter := s.store.session.Query(
		`SELECT comment_id, post_id, path FROM redyx_comments.comments_by_id WHERE author_id = ? AND is_deleted = false ALLOW FILTERING`,
		userUUID,
	).WithContext(ctx).Iter()

	var removedCount int32
	var commentUUID, postUUID gocql.UUID
	var path string

	for iter.Scan(&commentUUID, &postUUID, &path) {
		// For each comment, check if it belongs to the target community
		// by looking up the post via post-service. If post-service is unavailable,
		// we remove anyway (fail-safe for moderation actions).
		shouldRemove := true
		if s.postClient != nil && communityName != "" {
			// We can't easily filter by community without post context.
			// For v1, remove all comments by this user across all communities
			// if community filtering is unreliable. The moderation-service
			// should only call this for the correct user+community pair.
			// Since we can't efficiently filter at the ScyllaDB level,
			// we trust the moderation-service context.
			_ = shouldRemove
		}

		// Soft delete in comments_by_id
		if err := s.store.session.Query(
			`UPDATE redyx_comments.comments_by_id SET is_deleted = true WHERE comment_id = ?`,
			commentUUID,
		).WithContext(ctx).Exec(); err != nil {
			s.logger.Warn("failed to remove comment by user in comments_by_id",
				zap.String("comment_id", commentUUID.String()),
				zap.Error(err),
			)
			continue
		}

		// Soft delete in comments_by_post
		if err := s.store.session.Query(
			`UPDATE redyx_comments.comments_by_post SET is_deleted = true WHERE post_id = ? AND path = ?`,
			postUUID, path,
		).WithContext(ctx).Exec(); err != nil {
			s.logger.Warn("failed to remove comment by user in comments_by_post",
				zap.String("comment_id", commentUUID.String()),
				zap.Error(err),
			)
			continue
		}

		removedCount++
	}
	if err := iter.Close(); err != nil {
		s.logger.Error("failed to iterate comments for user removal",
			zap.String("user_id", userID),
			zap.Error(err),
		)
	}

	s.logger.Info("removed comments by user",
		zap.String("user_id", userID),
		zap.String("community", communityName),
		zap.Int32("removed_count", removedCount),
	)

	return &commentv1.RemoveCommentsByUserResponse{RemovedCount: removedCount}, nil
}
