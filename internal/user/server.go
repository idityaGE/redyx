// Package user implements the UserService gRPC server for managing user profiles.
// It handles profile viewing (public), profile updates (authenticated),
// account deletion (soft-delete), and user posts/comments listing.
package user

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/gocql/gocql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/redyx/redyx/gen/redyx/common/v1"
	userv1 "github.com/redyx/redyx/gen/redyx/user/v1"
	"github.com/redyx/redyx/internal/platform/auth"
	perrors "github.com/redyx/redyx/internal/platform/errors"
	"github.com/redyx/redyx/internal/platform/pagination"
)

// Server implements the UserServiceServer gRPC interface.
type Server struct {
	userv1.UnimplementedUserServiceServer
	db         *pgxpool.Pool
	postShards []*pgxpool.Pool // post shard databases for GetUserPosts
	scyllaDB   *gocql.Session  // ScyllaDB session for GetUserComments
	logger     *zap.Logger
}

// NewServer creates a new user service server with database and logger dependencies.
func NewServer(db *pgxpool.Pool, logger *zap.Logger, opts ...ServerOption) *Server {
	s := &Server{
		db:     db,
		logger: logger,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ServerOption configures optional dependencies for the user server.
type ServerOption func(*Server)

// WithPostShards configures the post shard database pools for GetUserPosts.
func WithPostShards(pools []*pgxpool.Pool) ServerOption {
	return func(s *Server) {
		s.postShards = pools
	}
}

// WithScyllaDB configures the ScyllaDB session for GetUserComments.
func WithScyllaDB(session *gocql.Session) ServerOption {
	return func(s *Server) {
		s.scyllaDB = session
	}
}

// profile is an internal representation of a user profile row.
type profile struct {
	UserID      string
	Username    string
	DisplayName string
	Bio         string
	AvatarURL   string
	Karma       int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   sql.NullTime
}

// profileToProto converts an internal profile to the proto User message.
// If the profile is soft-deleted, it returns a sanitized "[deleted]" version.
func profileToProto(p *profile) *userv1.User {
	if p.DeletedAt.Valid {
		return &userv1.User{
			UserId:      p.UserID,
			Username:    "[deleted]",
			DisplayName: "",
			Bio:         "",
			AvatarUrl:   "",
			Karma:       0,
			CreatedAt:   timestamppb.New(p.CreatedAt),
		}
	}
	return &userv1.User{
		UserId:      p.UserID,
		Username:    p.Username,
		DisplayName: p.DisplayName,
		Bio:         p.Bio,
		AvatarUrl:   p.AvatarURL,
		Karma:       p.Karma,
		CreatedAt:   timestamppb.New(p.CreatedAt),
	}
}

// createProfileIfOwner creates a default profile for the authenticated user
// if they are viewing their own profile that doesn't exist yet.
// This handles the gap between auth-service creating the user and the profile existing.
func (s *Server) createProfileIfOwner(ctx context.Context, username string) (*profile, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil || claims.Username != username {
		return nil, nil // not the owner or not authenticated
	}

	now := time.Now()
	p := &profile{
		UserID:    claims.UserID,
		Username:  claims.Username,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO profiles (user_id, username, display_name, bio, avatar_url, karma, created_at, updated_at)
		 VALUES ($1, $2, '', '', '', 0, $3, $4)
		 ON CONFLICT (user_id) DO NOTHING`,
		p.UserID, p.Username, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create profile: %w", err)
	}

	// Re-read to get the actual row (in case ON CONFLICT triggered)
	return s.getProfileByUsername(ctx, username)
}

// getProfileByUsername queries the profiles table by username.
func (s *Server) getProfileByUsername(ctx context.Context, username string) (*profile, error) {
	p := &profile{}
	err := s.db.QueryRow(ctx,
		`SELECT user_id, username, display_name, bio, avatar_url, karma, created_at, updated_at, deleted_at
		 FROM profiles WHERE username = $1`,
		username,
	).Scan(&p.UserID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL, &p.Karma, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// getProfileByUserID queries the profiles table by user_id.
func (s *Server) getProfileByUserID(ctx context.Context, userID string) (*profile, error) {
	p := &profile{}
	err := s.db.QueryRow(ctx,
		`SELECT user_id, username, display_name, bio, avatar_url, karma, created_at, updated_at, deleted_at
		 FROM profiles WHERE user_id = $1`,
		userID,
	).Scan(&p.UserID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL, &p.Karma, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetProfile returns a user's public profile by username.
// If the profile doesn't exist and the requester is the owner, it creates one (create-on-first-access).
func (s *Server) GetProfile(ctx context.Context, req *userv1.GetProfileRequest) (*userv1.GetProfileResponse, error) {
	if req.GetUsername() == "" {
		return nil, fmt.Errorf("username is required: %w", perrors.ErrInvalidInput)
	}

	p, err := s.getProfileByUsername(ctx, req.GetUsername())
	if err != nil {
		if err == pgx.ErrNoRows {
			// Try create-on-first-access if the requester is the owner
			p, err = s.createProfileIfOwner(ctx, req.GetUsername())
			if err != nil {
				s.logger.Error("failed to create profile on first access", zap.Error(err))
				return nil, fmt.Errorf("create profile: %w", err)
			}
			if p == nil {
				return nil, fmt.Errorf("user %q: %w", req.GetUsername(), perrors.ErrNotFound)
			}
		} else {
			s.logger.Error("failed to get profile", zap.Error(err))
			return nil, fmt.Errorf("get profile: %w", err)
		}
	}

	return &userv1.GetProfileResponse{
		User: profileToProto(p),
	}, nil
}

// UpdateProfile updates the authenticated user's profile.
// Only non-empty fields are applied (partial update pattern).
// Validates bio max 500 chars and display_name max 50 chars.
func (s *Server) UpdateProfile(ctx context.Context, req *userv1.UpdateProfileRequest) (*userv1.UpdateProfileResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("authentication required: %w", perrors.ErrUnauthenticated)
	}

	// Validate input constraints
	if len(req.GetBio()) > 500 {
		return nil, fmt.Errorf("bio must be 500 characters or fewer: %w", perrors.ErrInvalidInput)
	}
	if len(req.GetDisplayName()) > 50 {
		return nil, fmt.Errorf("display name must be 50 characters or fewer: %w", perrors.ErrInvalidInput)
	}

	// Get or create profile (create-on-first-access)
	p, err := s.getProfileByUserID(ctx, claims.UserID)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Create profile first
			now := time.Now()
			_, execErr := s.db.Exec(ctx,
				`INSERT INTO profiles (user_id, username, display_name, bio, avatar_url, karma, created_at, updated_at)
				 VALUES ($1, $2, '', '', '', 0, $3, $4)`,
				claims.UserID, claims.Username, now, now,
			)
			if execErr != nil {
				s.logger.Error("failed to create profile for update", zap.Error(execErr))
				return nil, fmt.Errorf("create profile: %w", execErr)
			}
			p, err = s.getProfileByUserID(ctx, claims.UserID)
			if err != nil {
				return nil, fmt.Errorf("get profile after create: %w", err)
			}
		} else {
			s.logger.Error("failed to get profile for update", zap.Error(err))
			return nil, fmt.Errorf("get profile: %w", err)
		}
	}

	// Check if soft-deleted
	if p.DeletedAt.Valid {
		return nil, fmt.Errorf("account has been deleted: %w", perrors.ErrNotFound)
	}

	// Build partial update — only update non-empty fields
	displayName := p.DisplayName
	bio := p.Bio
	avatarURL := p.AvatarURL

	if req.GetDisplayName() != "" {
		displayName = req.GetDisplayName()
	}
	if req.GetBio() != "" {
		bio = req.GetBio()
	}
	if req.GetAvatarUrl() != "" {
		avatarURL = req.GetAvatarUrl()
	}

	now := time.Now()
	_, err = s.db.Exec(ctx,
		`UPDATE profiles SET display_name = $1, bio = $2, avatar_url = $3, updated_at = $4
		 WHERE user_id = $5`,
		displayName, bio, avatarURL, now, claims.UserID,
	)
	if err != nil {
		s.logger.Error("failed to update profile", zap.Error(err))
		return nil, fmt.Errorf("update profile: %w", err)
	}

	// Re-read updated profile
	updated, err := s.getProfileByUserID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("get updated profile: %w", err)
	}

	return &userv1.UpdateProfileResponse{
		User: profileToProto(updated),
	}, nil
}

// DeleteAccount soft-deletes the authenticated user's account by wiping PII
// and setting deleted_at. The username stays in DB for uniqueness but
// responses show "[deleted]".
func (s *Server) DeleteAccount(ctx context.Context, req *userv1.DeleteAccountRequest) (*userv1.DeleteAccountResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("authentication required: %w", perrors.ErrUnauthenticated)
	}

	result, err := s.db.Exec(ctx,
		`UPDATE profiles SET display_name = '', bio = '', avatar_url = '', deleted_at = NOW(), updated_at = NOW()
		 WHERE user_id = $1 AND deleted_at IS NULL`,
		claims.UserID,
	)
	if err != nil {
		s.logger.Error("failed to delete account", zap.Error(err))
		return nil, fmt.Errorf("delete account: %w", err)
	}

	if result.RowsAffected() == 0 {
		// Either profile doesn't exist or already deleted
		return nil, fmt.Errorf("account not found or already deleted: %w", perrors.ErrNotFound)
	}

	s.logger.Info("account soft-deleted",
		zap.String("user_id", claims.UserID),
		zap.String("username", claims.Username),
	)

	return &userv1.DeleteAccountResponse{}, nil
}

// GetUserPosts returns paginated posts authored by the given user.
// Queries all post shard databases in parallel and merges results by created_at DESC.
func (s *Server) GetUserPosts(ctx context.Context, req *userv1.GetUserPostsRequest) (*userv1.GetUserPostsResponse, error) {
	username := req.GetUsername()
	if username == "" {
		return nil, fmt.Errorf("username is required: %w", perrors.ErrInvalidInput)
	}

	if len(s.postShards) == 0 {
		s.logger.Warn("GetUserPosts: no post shard pools configured, returning empty")
		return &userv1.GetUserPostsResponse{
			Posts:      []*userv1.PostSummary{},
			Pagination: &commonv1.PaginationResponse{},
		}, nil
	}

	pag := req.GetPagination()
	limit := pagination.DefaultLimit(pag.GetLimit(), 25, 100)
	fetchLimit := limit + 1

	// Decode cursor if provided
	var cursorTime time.Time
	var cursorID string
	if pag.GetCursor() != "" {
		var err error
		cursorID, cursorTime, err = pagination.DecodeCursor(pag.GetCursor())
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", perrors.ErrInvalidInput)
		}
	}

	// Query all shards in parallel
	type shardResult struct {
		posts []*userv1.PostSummary
		err   error
	}

	var wg sync.WaitGroup
	resultsCh := make(chan shardResult, len(s.postShards))

	for _, pool := range s.postShards {
		wg.Add(1)
		go func(p *pgxpool.Pool) {
			defer wg.Done()
			var args []any
			argIdx := 1

			query := `SELECT id, title, community_name, vote_score, comment_count, created_at
				FROM posts WHERE author_username = $1 AND is_deleted = false`
			args = append(args, username)
			argIdx++

			if !cursorTime.IsZero() {
				query += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argIdx, argIdx+1)
				args = append(args, cursorTime, cursorID)
				argIdx += 2
			}

			query += " ORDER BY created_at DESC, id DESC"
			query += fmt.Sprintf(" LIMIT $%d", argIdx)
			args = append(args, fetchLimit)

			rows, err := p.Query(ctx, query, args...)
			if err != nil {
				resultsCh <- shardResult{err: err}
				return
			}
			defer rows.Close()

			var posts []*userv1.PostSummary
			for rows.Next() {
				var postID, title, communityName string
				var voteScore, commentCount int32
				var createdAt time.Time
				if err := rows.Scan(&postID, &title, &communityName, &voteScore, &commentCount, &createdAt); err != nil {
					resultsCh <- shardResult{err: err}
					return
				}
				posts = append(posts, &userv1.PostSummary{
					PostId:        postID,
					Title:         title,
					CommunityName: communityName,
					VoteScore:     voteScore,
					CommentCount:  commentCount,
					CreatedAt:     timestamppb.New(createdAt),
				})
			}
			resultsCh <- shardResult{posts: posts}
		}(pool)
	}

	wg.Wait()
	close(resultsCh)

	// Merge results from all shards
	var allPosts []*userv1.PostSummary
	for r := range resultsCh {
		if r.err != nil {
			s.logger.Warn("shard query error in GetUserPosts", zap.Error(r.err))
			continue
		}
		allPosts = append(allPosts, r.posts...)
	}

	// Sort merged results by created_at DESC, then by post_id DESC
	sortPostSummaries(allPosts)

	// Count total (approximate — from merged shards before pagination)
	totalCount := int32(len(allPosts))

	// Apply pagination
	hasMore := len(allPosts) > int(limit)
	if len(allPosts) > int(limit) {
		allPosts = allPosts[:limit]
	}

	var nextCursor string
	if hasMore && len(allPosts) > 0 {
		last := allPosts[len(allPosts)-1]
		nextCursor = pagination.EncodeCursor(last.PostId, last.CreatedAt.AsTime())
	}

	return &userv1.GetUserPostsResponse{
		Posts: allPosts,
		Pagination: &commonv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    hasMore,
			TotalCount: totalCount,
		},
	}, nil
}

// sortPostSummaries sorts post summaries by created_at DESC, post_id DESC.
func sortPostSummaries(posts []*userv1.PostSummary) {
	for i := 1; i < len(posts); i++ {
		for j := i; j > 0; j-- {
			ti := posts[j].CreatedAt.AsTime()
			tj := posts[j-1].CreatedAt.AsTime()
			if ti.After(tj) || (ti.Equal(tj) && posts[j].PostId > posts[j-1].PostId) {
				posts[j], posts[j-1] = posts[j-1], posts[j]
			} else {
				break
			}
		}
	}
}

// GetUserComments returns paginated comments authored by the given user.
// Queries the comments_by_author ScyllaDB table.
func (s *Server) GetUserComments(ctx context.Context, req *userv1.GetUserCommentsRequest) (*userv1.GetUserCommentsResponse, error) {
	username := req.GetUsername()
	if username == "" {
		return nil, fmt.Errorf("username is required: %w", perrors.ErrInvalidInput)
	}

	if s.scyllaDB == nil {
		s.logger.Warn("GetUserComments: no ScyllaDB session configured, returning empty")
		return &userv1.GetUserCommentsResponse{
			Comments:   []*userv1.CommentSummary{},
			Pagination: &commonv1.PaginationResponse{},
		}, nil
	}

	pag := req.GetPagination()
	limit := pagination.DefaultLimit(pag.GetLimit(), 25, 100)

	// Decode cursor if provided
	var cursorTime time.Time
	if pag.GetCursor() != "" {
		_, ct, err := pagination.DecodeCursor(pag.GetCursor())
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", perrors.ErrInvalidInput)
		}
		cursorTime = ct
	}

	// Query comments_by_author table
	var iter *gocql.Iter
	if cursorTime.IsZero() {
		iter = s.scyllaDB.Query(
			`SELECT comment_id, post_id, body, vote_score, is_deleted, created_at
			 FROM redyx_comments.comments_by_author
			 WHERE author_username = ?
			 LIMIT ?`,
			username, limit+1,
		).WithContext(ctx).Iter()
	} else {
		iter = s.scyllaDB.Query(
			`SELECT comment_id, post_id, body, vote_score, is_deleted, created_at
			 FROM redyx_comments.comments_by_author
			 WHERE author_username = ? AND created_at < ?
			 LIMIT ?`,
			username, cursorTime, limit+1,
		).WithContext(ctx).Iter()
	}

	var comments []*userv1.CommentSummary
	var commentID, postID gocql.UUID
	var body string
	var voteScore int
	var isDeleted bool
	var createdAt time.Time

	for iter.Scan(&commentID, &postID, &body, &voteScore, &isDeleted, &createdAt) {
		if isDeleted {
			continue
		}
		comments = append(comments, &userv1.CommentSummary{
			CommentId:     commentID.String(),
			PostId:        postID.String(),
			CommunityName: "", // Not stored in comments_by_author; frontend can resolve if needed
			Body:          body,
			VoteScore:     int32(voteScore),
			CreatedAt:     timestamppb.New(createdAt),
		})
	}
	if err := iter.Close(); err != nil {
		s.logger.Error("failed to query comments_by_author", zap.Error(err))
		return nil, fmt.Errorf("list user comments: %w", err)
	}

	totalCount := int32(len(comments))
	hasMore := len(comments) > int(limit)
	if hasMore {
		comments = comments[:limit]
	}

	var nextCursor string
	if hasMore && len(comments) > 0 {
		last := comments[len(comments)-1]
		nextCursor = pagination.EncodeCursor(last.CommentId, last.CreatedAt.AsTime())
	}

	return &userv1.GetUserCommentsResponse{
		Comments: comments,
		Pagination: &commonv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    hasMore,
			TotalCount: totalCount,
		},
	}, nil
}
