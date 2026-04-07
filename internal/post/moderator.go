package post

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	postv1 "github.com/idityaGE/redyx/gen/redyx/post/v1"
	perrors "github.com/idityaGE/redyx/internal/platform/errors"
)

// ModeratorRemovePost soft-deletes a post by ID within a community shard.
// Called by moderation-service via internal gRPC.
func (s *Server) ModeratorRemovePost(ctx context.Context, req *postv1.ModeratorRemovePostRequest) (*postv1.ModeratorRemovePostResponse, error) {
	postID := req.GetPostId()
	communityName := req.GetCommunityName()
	if postID == "" || communityName == "" {
		return nil, fmt.Errorf("post_id and community_name are required: %w", perrors.ErrInvalidInput)
	}

	pool, _ := s.shards.GetPool(communityName)
	tag, err := pool.Exec(ctx,
		`UPDATE posts SET is_deleted = true WHERE id = $1 AND community_name = $2`,
		postID, communityName,
	)
	if err != nil {
		return nil, fmt.Errorf("moderator remove post: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("post %q in community %q: %w", postID, communityName, perrors.ErrNotFound)
	}

	s.logger.Info("moderator removed post",
		zap.String("post_id", postID),
		zap.String("community", communityName),
	)

	return &postv1.ModeratorRemovePostResponse{}, nil
}

// ModeratorRestorePost restores a previously removed post.
// Called by moderation-service via internal gRPC.
func (s *Server) ModeratorRestorePost(ctx context.Context, req *postv1.ModeratorRestorePostRequest) (*postv1.ModeratorRestorePostResponse, error) {
	postID := req.GetPostId()
	communityName := req.GetCommunityName()
	if postID == "" || communityName == "" {
		return nil, fmt.Errorf("post_id and community_name are required: %w", perrors.ErrInvalidInput)
	}

	pool, _ := s.shards.GetPool(communityName)
	tag, err := pool.Exec(ctx,
		`UPDATE posts SET is_deleted = false WHERE id = $1 AND community_name = $2`,
		postID, communityName,
	)
	if err != nil {
		return nil, fmt.Errorf("moderator restore post: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("post %q in community %q: %w", postID, communityName, perrors.ErrNotFound)
	}

	s.logger.Info("moderator restored post",
		zap.String("post_id", postID),
		zap.String("community", communityName),
	)

	return &postv1.ModeratorRestorePostResponse{}, nil
}

// SetPostPinned pins or unpins a post in a community.
// Called by moderation-service via internal gRPC.
func (s *Server) SetPostPinned(ctx context.Context, req *postv1.SetPostPinnedRequest) (*postv1.SetPostPinnedResponse, error) {
	postID := req.GetPostId()
	communityName := req.GetCommunityName()
	if postID == "" || communityName == "" {
		return nil, fmt.Errorf("post_id and community_name are required: %w", perrors.ErrInvalidInput)
	}

	pool, _ := s.shards.GetPool(communityName)
	tag, err := pool.Exec(ctx,
		`UPDATE posts SET is_pinned = $1 WHERE id = $2 AND community_name = $3`,
		req.GetPinned(), postID, communityName,
	)
	if err != nil {
		return nil, fmt.Errorf("set post pinned: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("post %q in community %q: %w", postID, communityName, perrors.ErrNotFound)
	}

	s.logger.Info("set post pinned",
		zap.String("post_id", postID),
		zap.String("community", communityName),
		zap.Bool("pinned", req.GetPinned()),
	)

	return &postv1.SetPostPinnedResponse{}, nil
}

// CountPinnedPosts returns the count of pinned, non-deleted posts in a community.
// Called by moderation-service to enforce max 2 pins.
func (s *Server) CountPinnedPosts(ctx context.Context, req *postv1.CountPinnedPostsRequest) (*postv1.CountPinnedPostsResponse, error) {
	communityName := req.GetCommunityName()
	if communityName == "" {
		return nil, fmt.Errorf("community_name is required: %w", perrors.ErrInvalidInput)
	}

	pool, _ := s.shards.GetPool(communityName)
	var count int32
	err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts WHERE community_name = $1 AND is_pinned = true AND is_deleted = false`,
		communityName,
	).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("count pinned posts: %w", err)
	}

	return &postv1.CountPinnedPostsResponse{Count: count}, nil
}

// RemovePostsByUser soft-deletes all posts by a user in a community.
// Called by moderation-service when banning with content removal.
func (s *Server) RemovePostsByUser(ctx context.Context, req *postv1.RemovePostsByUserRequest) (*postv1.RemovePostsByUserResponse, error) {
	userID := req.GetUserId()
	communityName := req.GetCommunityName()
	if userID == "" || communityName == "" {
		return nil, fmt.Errorf("user_id and community_name are required: %w", perrors.ErrInvalidInput)
	}

	pool, _ := s.shards.GetPool(communityName)
	tag, err := pool.Exec(ctx,
		`UPDATE posts SET is_deleted = true WHERE author_id = $1 AND community_name = $2 AND is_deleted = false`,
		userID, communityName,
	)
	if err != nil {
		return nil, fmt.Errorf("remove posts by user: %w", err)
	}

	removedCount := int32(tag.RowsAffected())
	s.logger.Info("removed posts by user",
		zap.String("user_id", userID),
		zap.String("community", communityName),
		zap.Int32("removed_count", removedCount),
	)

	return &postv1.RemovePostsByUserResponse{RemovedCount: removedCount}, nil
}
