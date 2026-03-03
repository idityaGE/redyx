package post

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/redyx/redyx/gen/redyx/common/v1"
	postv1 "github.com/redyx/redyx/gen/redyx/post/v1"
	"github.com/redyx/redyx/internal/platform/auth"
	perrors "github.com/redyx/redyx/internal/platform/errors"
	"github.com/redyx/redyx/internal/platform/pagination"
)

// Server implements the PostServiceServer gRPC interface.
type Server struct {
	postv1.UnimplementedPostServiceServer
	shards      *ShardRouter
	cache       *Cache
	communityDB *pgxpool.Pool
	logger      *zap.Logger
}

// NewServer creates a new post gRPC server.
func NewServer(shards *ShardRouter, cache *Cache, communityDB *pgxpool.Pool, logger *zap.Logger) *Server {
	return &Server{
		shards:      shards,
		cache:       cache,
		communityDB: communityDB,
		logger:      logger,
	}
}

// resolveCommunity looks up a community by name and returns its UUID.
func (s *Server) resolveCommunity(ctx context.Context, name string) (string, error) {
	if s.communityDB == nil {
		return "", fmt.Errorf("community database not configured")
	}
	var id string
	err := s.communityDB.QueryRow(ctx,
		`SELECT id FROM communities WHERE name = $1`,
		name,
	).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("community %q: %w", name, perrors.ErrNotFound)
		}
		return "", fmt.Errorf("resolve community: %w", err)
	}
	return id, nil
}

// CreatePost creates a new post in a community shard.
func (s *Server) CreatePost(ctx context.Context, req *postv1.CreatePostRequest) (*postv1.CreatePostResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("create post: %w", perrors.ErrUnauthenticated)
	}

	// Validate title
	title := strings.TrimSpace(req.GetTitle())
	if len(title) == 0 || len(title) > 300 {
		return nil, fmt.Errorf("title must be 1-300 characters: %w", perrors.ErrInvalidInput)
	}

	// Validate based on post type
	postType := req.GetPostType()
	if postType == postv1.PostType_POST_TYPE_UNSPECIFIED {
		postType = postv1.PostType_POST_TYPE_TEXT
	}

	body := req.GetBody()
	postURL := req.GetUrl()

	switch postType {
	case postv1.PostType_POST_TYPE_TEXT:
		if len(body) > 40000 {
			return nil, fmt.Errorf("body must be at most 40000 characters: %w", perrors.ErrInvalidInput)
		}
	case postv1.PostType_POST_TYPE_LINK:
		if postURL == "" {
			return nil, fmt.Errorf("link post requires a URL: %w", perrors.ErrInvalidInput)
		}
		if _, err := url.ParseRequestURI(postURL); err != nil {
			return nil, fmt.Errorf("invalid URL: %w", perrors.ErrInvalidInput)
		}
	case postv1.PostType_POST_TYPE_MEDIA:
		return nil, status.Error(codes.Unimplemented, "media posts not yet supported — coming in Phase 5")
	default:
		return nil, fmt.Errorf("unknown post type: %w", perrors.ErrInvalidInput)
	}

	// Resolve community name → UUID from community database
	communityName := req.GetCommunityName()
	if communityName == "" {
		return nil, fmt.Errorf("community name is required: %w", perrors.ErrInvalidInput)
	}

	communityID, err := s.resolveCommunity(ctx, communityName)
	if err != nil {
		return nil, err
	}

	// Use community name as shard routing key (all posts for a community on same shard)
	pool, _ := s.shards.GetPool(communityName)

	// Compute initial hot score
	now := time.Now()
	initialHot := HotScore(0, now)

	// Determine display values for anonymous posts
	authorID := claims.UserID
	authorUsername := claims.Username
	isAnonymous := req.GetIsAnonymous()

	var postID string
	var createdAt time.Time
	err = pool.QueryRow(ctx,
		`INSERT INTO posts (title, body, url, post_type, author_id, author_username,
		                    community_id, community_name, hot_score, is_anonymous, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, created_at`,
		title, body, postURL, int16(postType), authorID, authorUsername,
		communityID, communityName, initialHot, isAnonymous, now,
	).Scan(&postID, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("insert post: %w", err)
	}

	// Build response — mask anonymous author
	respAuthorID := authorID
	respAuthorUsername := authorUsername
	if isAnonymous {
		respAuthorID = ""
		respAuthorUsername = "[anonymous]"
	}

	return &postv1.CreatePostResponse{
		Post: &postv1.Post{
			PostId:         postID,
			Title:          title,
			Body:           body,
			Url:            postURL,
			PostType:       postType,
			AuthorId:       respAuthorID,
			AuthorUsername: respAuthorUsername,
			CommunityId:    communityID,
			CommunityName:  communityName,
			VoteScore:      0,
			CommentCount:   0,
			IsEdited:       false,
			IsDeleted:      false,
			IsPinned:       false,
			IsAnonymous:    isAnonymous,
			CreatedAt:      timestamppb.New(createdAt),
		},
	}, nil
}

// GetPost returns a single post by ID, querying all shards in parallel.
func (s *Server) GetPost(ctx context.Context, req *postv1.GetPostRequest) (*postv1.GetPostResponse, error) {
	postID := req.GetPostId()
	if postID == "" {
		return nil, fmt.Errorf("post_id is required: %w", perrors.ErrInvalidInput)
	}

	claims := auth.ClaimsFromContext(ctx)

	// Query all shards in parallel — post_id is UUID, not shard-routable
	pools := s.shards.AllPools()
	type result struct {
		post *postv1.Post
		pool *pgxpool.Pool
		err  error
	}

	results := make(chan result, len(pools))
	for _, pool := range pools {
		go func(p *pgxpool.Pool) {
			post, err := s.getPostFromPool(ctx, p, postID)
			results <- result{post: post, pool: p, err: err}
		}(pool)
	}

	var foundPost *postv1.Post
	for range pools {
		r := <-results
		if r.err == nil && r.post != nil {
			foundPost = r.post
		}
	}

	if foundPost == nil {
		return nil, fmt.Errorf("post %q: %w", postID, perrors.ErrNotFound)
	}

	// Mask anonymous author if not moderator
	if foundPost.IsAnonymous {
		isMod := false
		if claims != nil {
			// Simple moderator check: for v1, we don't have a direct way to check
			// mod status without calling community-service. We'll check if the
			// requesting user is the author (they can see their own anonymous posts).
			if claims.UserID == foundPost.AuthorId {
				isMod = true
			}
		}
		if !isMod {
			foundPost.AuthorId = ""
			foundPost.AuthorUsername = "[anonymous]"
		}
	}

	// Get user_vote and is_saved for authenticated users
	var userVote int32
	var isSaved bool

	if claims != nil {
		userVote, _ = s.cache.GetUserVote(ctx, claims.UserID, postID)

		// Check saved_posts on shard_0 (centralized)
		pools := s.shards.AllPools()
		if len(pools) > 0 {
			var exists bool
			err := pools[0].QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM saved_posts WHERE user_id = $1 AND post_id = $2)`,
				claims.UserID, postID,
			).Scan(&exists)
			if err == nil {
				isSaved = exists
			}
		}
	}

	return &postv1.GetPostResponse{
		Post:     foundPost,
		UserVote: userVote,
		IsSaved:  isSaved,
	}, nil
}

// UpdatePost updates a post's content (author only).
func (s *Server) UpdatePost(ctx context.Context, req *postv1.UpdatePostRequest) (*postv1.UpdatePostResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("update post: %w", perrors.ErrUnauthenticated)
	}

	postID := req.GetPostId()
	if postID == "" {
		return nil, fmt.Errorf("post_id is required: %w", perrors.ErrInvalidInput)
	}

	// Find the post across shards
	pool, post, err := s.findPostAcrossShards(ctx, postID)
	if err != nil {
		return nil, err
	}

	// Only author can update
	if post.AuthorId != claims.UserID {
		return nil, fmt.Errorf("only the author can update a post: %w", perrors.ErrForbidden)
	}

	// Build update
	title := req.GetTitle()
	if title == "" {
		title = post.Title
	}
	if len(title) > 300 {
		return nil, fmt.Errorf("title must be at most 300 characters: %w", perrors.ErrInvalidInput)
	}

	body := req.GetBody()
	if body == "" {
		body = post.Body
	}

	postURL := req.GetUrl()
	if postURL == "" {
		postURL = post.Url
	}

	now := time.Now()
	_, err = pool.Exec(ctx,
		`UPDATE posts SET title = $1, body = $2, url = $3, is_edited = true, edited_at = $4
		 WHERE id = $5`,
		title, body, postURL, now, postID,
	)
	if err != nil {
		return nil, fmt.Errorf("update post: %w", err)
	}

	post.Title = title
	post.Body = body
	post.Url = postURL
	post.IsEdited = true
	post.EditedAt = timestamppb.New(now)

	return &postv1.UpdatePostResponse{Post: post}, nil
}

// DeletePost soft-deletes a post (author or moderator).
func (s *Server) DeletePost(ctx context.Context, req *postv1.DeletePostRequest) (*postv1.DeletePostResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("delete post: %w", perrors.ErrUnauthenticated)
	}

	postID := req.GetPostId()
	if postID == "" {
		return nil, fmt.Errorf("post_id is required: %w", perrors.ErrInvalidInput)
	}

	pool, post, err := s.findPostAcrossShards(ctx, postID)
	if err != nil {
		return nil, err
	}

	// Author or moderator can delete
	// For v1, we check author_id only — full moderator check requires
	// calling community-service. TODO: add moderator permission check.
	if post.AuthorId != claims.UserID {
		return nil, fmt.Errorf("only the author can delete a post: %w", perrors.ErrForbidden)
	}

	_, err = pool.Exec(ctx,
		`UPDATE posts SET is_deleted = true, title = '[deleted]', body = '[deleted]'
		 WHERE id = $1`,
		postID,
	)
	if err != nil {
		return nil, fmt.Errorf("delete post: %w", err)
	}

	return &postv1.DeletePostResponse{}, nil
}

// ListPosts returns paginated posts for a community feed (single-shard query).
func (s *Server) ListPosts(ctx context.Context, req *postv1.ListPostsRequest) (*postv1.ListPostsResponse, error) {
	communityName := req.GetCommunityName()
	if communityName == "" {
		return nil, fmt.Errorf("community_name is required: %w", perrors.ErrInvalidInput)
	}

	// Route to correct shard via community name
	pool, _ := s.shards.GetPool(communityName)

	pag := req.GetPagination()
	limit := pagination.DefaultLimit(pag.GetLimit(), 25, 100)
	fetchLimit := limit + 1

	sortOrder := req.GetSort()
	if sortOrder == postv1.SortOrder_SORT_ORDER_UNSPECIFIED {
		sortOrder = postv1.SortOrder_SORT_ORDER_HOT
	}

	// Build query based on sort order
	var query string
	var args []any
	argIdx := 1

	baseSelect := `SELECT id, title, body, url, post_type, author_id, author_username,
		community_id, community_name, vote_score, comment_count, hot_score,
		is_edited, is_deleted, is_pinned, is_anonymous, thumbnail_url, created_at, edited_at
		FROM posts WHERE community_name = $1 AND is_deleted = false`
	args = append(args, communityName)
	argIdx++

	// Time range filter for TOP sort
	if sortOrder == postv1.SortOrder_SORT_ORDER_TOP && req.GetTimeRange() != postv1.TimeRange_TIME_RANGE_ALL && req.GetTimeRange() != postv1.TimeRange_TIME_RANGE_UNSPECIFIED {
		interval := timeRangeToInterval(req.GetTimeRange())
		if interval != "" {
			baseSelect += fmt.Sprintf(" AND created_at > now() - interval '%s'", interval)
		}
	}

	// Cursor handling + ordering
	switch sortOrder {
	case postv1.SortOrder_SORT_ORDER_HOT:
		if pag.GetCursor() != "" {
			sortVal, cursorID, _, err := pagination.DecodeSortCursor(pag.GetCursor())
			if err != nil {
				return nil, fmt.Errorf("invalid cursor: %w", perrors.ErrInvalidInput)
			}
			baseSelect += fmt.Sprintf(" AND (hot_score, id) < ($%d, $%d)", argIdx, argIdx+1)
			args = append(args, sortVal, cursorID)
			argIdx += 2
		}
		query = baseSelect + " ORDER BY hot_score DESC, id DESC"

	case postv1.SortOrder_SORT_ORDER_NEW:
		if pag.GetCursor() != "" {
			_, cursorID, cursorTime, err := pagination.DecodeSortCursor(pag.GetCursor())
			if err != nil {
				return nil, fmt.Errorf("invalid cursor: %w", perrors.ErrInvalidInput)
			}
			baseSelect += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argIdx, argIdx+1)
			args = append(args, cursorTime, cursorID)
			argIdx += 2
		}
		query = baseSelect + " ORDER BY created_at DESC, id DESC"

	case postv1.SortOrder_SORT_ORDER_TOP:
		if pag.GetCursor() != "" {
			sortVal, cursorID, _, err := pagination.DecodeSortCursor(pag.GetCursor())
			if err != nil {
				return nil, fmt.Errorf("invalid cursor: %w", perrors.ErrInvalidInput)
			}
			baseSelect += fmt.Sprintf(" AND (vote_score, id) < ($%d, $%d)", argIdx, argIdx+1)
			args = append(args, int(sortVal), cursorID)
			argIdx += 2
		}
		query = baseSelect + " ORDER BY vote_score DESC, id DESC"

	case postv1.SortOrder_SORT_ORDER_RISING:
		// Rising: compute on-the-fly for recent posts (last 24h)
		baseSelect += " AND created_at > now() - interval '24 hours'"
		query = baseSelect + " ORDER BY (vote_score::float / GREATEST(1, EXTRACT(EPOCH FROM (now() - created_at)) / 3600.0)) DESC, id DESC"

	default:
		query = baseSelect + " ORDER BY hot_score DESC, id DESC"
	}

	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, fetchLimit)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	posts, err := scanPosts(rows)
	if err != nil {
		return nil, err
	}

	// Mask anonymous authors
	for _, p := range posts {
		if p.IsAnonymous {
			p.AuthorId = ""
			p.AuthorUsername = "[anonymous]"
		}
	}

	hasMore := len(posts) > int(limit)
	if hasMore {
		posts = posts[:limit]
	}

	var nextCursor string
	if hasMore && len(posts) > 0 {
		last := posts[len(posts)-1]
		switch sortOrder {
		case postv1.SortOrder_SORT_ORDER_HOT:
			nextCursor = pagination.EncodeSortCursor(last.hotScore, last.PostId, last.CreatedAt.AsTime())
		case postv1.SortOrder_SORT_ORDER_NEW:
			nextCursor = pagination.EncodeSortCursor(0, last.PostId, last.CreatedAt.AsTime())
		case postv1.SortOrder_SORT_ORDER_TOP:
			nextCursor = pagination.EncodeSortCursor(float64(last.VoteScore), last.PostId, last.CreatedAt.AsTime())
		case postv1.SortOrder_SORT_ORDER_RISING:
			risingVal := RisingScore(int(last.VoteScore), last.CreatedAt.AsTime())
			nextCursor = pagination.EncodeSortCursor(risingVal, last.PostId, last.CreatedAt.AsTime())
		}
	}

	protoPosts := make([]*postv1.Post, len(posts))
	for i, p := range posts {
		protoPosts[i] = p.Post
	}

	return &postv1.ListPostsResponse{
		Posts: protoPosts,
		Pagination: &commonv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}, nil
}

// ListHomeFeed returns posts from all communities the user has joined, across shards.
func (s *Server) ListHomeFeed(ctx context.Context, req *postv1.ListHomeFeedRequest) (*postv1.ListHomeFeedResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("list home feed: %w", perrors.ErrUnauthenticated)
	}

	pag := req.GetPagination()
	limit := pagination.DefaultLimit(pag.GetLimit(), 25, 100)

	sortOrder := req.GetSort()
	if sortOrder == postv1.SortOrder_SORT_ORDER_UNSPECIFIED {
		sortOrder = postv1.SortOrder_SORT_ORDER_HOT
	}

	// Check feed cache
	sortStr := sortOrder.String()
	timeRangeStr := req.GetTimeRange().String()
	cursor := pag.GetCursor()

	cached, err := s.cache.GetFeed(ctx, claims.UserID, sortStr, timeRangeStr, cursor)
	if err != nil {
		s.logger.Warn("feed cache get error", zap.Error(err))
	}
	if cached != nil {
		var posts []*postv1.Post
		if err := json.Unmarshal(cached.PostsJSON, &posts); err == nil {
			return &postv1.ListHomeFeedResponse{
				Posts: posts,
				Pagination: &commonv1.PaginationResponse{
					NextCursor: cached.NextCursor,
					HasMore:    cached.HasMore,
				},
			}, nil
		}
	}

	// Get user's joined community IDs
	communityIDs, err := s.getUserCommunityIDs(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user communities: %w", err)
	}

	if len(communityIDs) == 0 {
		return &postv1.ListHomeFeedResponse{
			Posts:      nil,
			Pagination: &commonv1.PaginationResponse{HasMore: false},
		}, nil
	}

	// Group community IDs by shard
	shardCommunities := make(map[string][]string)
	for _, cid := range communityIDs {
		_, shardName := s.shards.GetPool(cid)
		shardCommunities[shardName] = append(shardCommunities[shardName], cid)
	}

	// Query each shard in parallel
	fetchLimit := (limit + 1) * 2 // Overfetch to allow merge

	type shardResult struct {
		posts []*postWithScore
		err   error
	}

	var wg sync.WaitGroup
	resultsCh := make(chan shardResult, len(shardCommunities))

	for shardName, cids := range shardCommunities {
		wg.Add(1)
		go func(sName string, communityIDs []string) {
			defer wg.Done()
			pool, _ := s.shards.GetPool(communityIDs[0])
			posts, err := s.queryShardForFeed(ctx, pool, communityIDs, sortOrder, req.GetTimeRange(), cursor, fetchLimit)
			resultsCh <- shardResult{posts: posts, err: err}
		}(shardName, cids)
	}

	wg.Wait()
	close(resultsCh)

	// Merge results from all shards
	var allPosts []*postWithScore
	for r := range resultsCh {
		if r.err != nil {
			s.logger.Warn("shard query error", zap.Error(r.err))
			continue
		}
		allPosts = append(allPosts, r.posts...)
	}

	// Sort merged results
	sortPostsBy(allPosts, sortOrder)

	// Mask anonymous authors
	for _, p := range allPosts {
		if p.IsAnonymous {
			p.AuthorId = ""
			p.AuthorUsername = "[anonymous]"
		}
	}

	// Apply pagination to merged results
	hasMore := len(allPosts) > int(limit)
	if len(allPosts) > int(limit) {
		allPosts = allPosts[:limit]
	}

	var nextCursor string
	if hasMore && len(allPosts) > 0 {
		last := allPosts[len(allPosts)-1]
		switch sortOrder {
		case postv1.SortOrder_SORT_ORDER_HOT:
			nextCursor = pagination.EncodeSortCursor(last.hotScore, last.PostId, last.CreatedAt.AsTime())
		case postv1.SortOrder_SORT_ORDER_NEW:
			nextCursor = pagination.EncodeSortCursor(0, last.PostId, last.CreatedAt.AsTime())
		case postv1.SortOrder_SORT_ORDER_TOP:
			nextCursor = pagination.EncodeSortCursor(float64(last.VoteScore), last.PostId, last.CreatedAt.AsTime())
		case postv1.SortOrder_SORT_ORDER_RISING:
			risingVal := RisingScore(int(last.VoteScore), last.CreatedAt.AsTime())
			nextCursor = pagination.EncodeSortCursor(risingVal, last.PostId, last.CreatedAt.AsTime())
		}
	}

	protoPosts := make([]*postv1.Post, len(allPosts))
	for i, p := range allPosts {
		protoPosts[i] = p.Post
	}

	// Cache the result
	postsJSON, _ := json.Marshal(protoPosts)
	_ = s.cache.SetFeed(ctx, claims.UserID, sortStr, timeRangeStr, cursor, &cachedFeedPage{
		PostsJSON:  postsJSON,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	})

	return &postv1.ListHomeFeedResponse{
		Posts: protoPosts,
		Pagination: &commonv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}, nil
}

// SavePost saves or unsaves a post for the authenticated user.
func (s *Server) SavePost(ctx context.Context, req *postv1.SavePostRequest) (*postv1.SavePostResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("save post: %w", perrors.ErrUnauthenticated)
	}

	postID := req.GetPostId()
	if postID == "" {
		return nil, fmt.Errorf("post_id is required: %w", perrors.ErrInvalidInput)
	}

	// saved_posts table is centralized on shard_0
	pools := s.shards.AllPools()
	if len(pools) == 0 {
		return nil, fmt.Errorf("no shard pools available")
	}
	pool := pools[0]

	if req.GetSave() {
		_, err := pool.Exec(ctx,
			`INSERT INTO saved_posts (user_id, post_id) VALUES ($1, $2)
			 ON CONFLICT (user_id, post_id) DO NOTHING`,
			claims.UserID, postID,
		)
		if err != nil {
			return nil, fmt.Errorf("save post: %w", err)
		}
	} else {
		_, err := pool.Exec(ctx,
			`DELETE FROM saved_posts WHERE user_id = $1 AND post_id = $2`,
			claims.UserID, postID,
		)
		if err != nil {
			return nil, fmt.Errorf("unsave post: %w", err)
		}
	}

	return &postv1.SavePostResponse{}, nil
}

// ListSavedPosts returns the authenticated user's saved posts.
func (s *Server) ListSavedPosts(ctx context.Context, req *postv1.ListSavedPostsRequest) (*postv1.ListSavedPostsResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("list saved posts: %w", perrors.ErrUnauthenticated)
	}

	pag := req.GetPagination()
	limit := pagination.DefaultLimit(pag.GetLimit(), 25, 100)
	fetchLimit := limit + 1

	// Query saved_posts from shard_0
	pools := s.shards.AllPools()
	if len(pools) == 0 {
		return nil, fmt.Errorf("no shard pools available")
	}
	pool := pools[0]

	var args []any
	argIdx := 1
	query := `SELECT post_id, saved_at FROM saved_posts WHERE user_id = $1`
	args = append(args, claims.UserID)
	argIdx++

	if pag.GetCursor() != "" {
		_, _, cursorTime, err := pagination.DecodeSortCursor(pag.GetCursor())
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", perrors.ErrInvalidInput)
		}
		query += fmt.Sprintf(" AND saved_at < $%d", argIdx)
		args = append(args, cursorTime)
		argIdx++
	}

	query += " ORDER BY saved_at DESC"
	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, fetchLimit)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list saved posts: %w", err)
	}
	defer rows.Close()

	type savedEntry struct {
		postID  string
		savedAt time.Time
	}

	var entries []savedEntry
	for rows.Next() {
		var e savedEntry
		if err := rows.Scan(&e.postID, &e.savedAt); err != nil {
			return nil, fmt.Errorf("scan saved post: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("saved posts rows: %w", err)
	}

	hasMore := len(entries) > int(limit)
	if hasMore {
		entries = entries[:limit]
	}

	// Fetch actual post data for each saved post, batch by shard
	var posts []*postv1.Post
	for _, entry := range entries {
		// Query all shards for each post (could be optimized with batching)
		post, err := s.getPostByID(ctx, entry.postID)
		if err != nil {
			s.logger.Warn("saved post not found", zap.String("post_id", entry.postID), zap.Error(err))
			continue
		}
		// Mask anonymous authors
		if post.IsAnonymous {
			post.AuthorId = ""
			post.AuthorUsername = "[anonymous]"
		}
		posts = append(posts, post)
	}

	var nextCursor string
	if hasMore && len(entries) > 0 {
		last := entries[len(entries)-1]
		nextCursor = pagination.EncodeSortCursor(0, last.postID, last.savedAt)
	}

	return &postv1.ListSavedPostsResponse{
		Posts: posts,
		Pagination: &commonv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}, nil
}

// --- Internal helpers ---

// postWithScore wraps a Post proto with the raw hot_score for cursor encoding.
type postWithScore struct {
	*postv1.Post
	hotScore float64
}

// getPostFromPool retrieves a single post from a specific shard pool.
func (s *Server) getPostFromPool(ctx context.Context, pool *pgxpool.Pool, postID string) (*postv1.Post, error) {
	row := pool.QueryRow(ctx,
		`SELECT id, title, body, url, post_type, author_id, author_username,
		        community_id, community_name, vote_score, comment_count, hot_score,
		        is_edited, is_deleted, is_pinned, is_anonymous, thumbnail_url, created_at, edited_at
		 FROM posts WHERE id = $1`,
		postID,
	)
	return scanPostRow(row)
}

// findPostAcrossShards searches all shards for a post and returns the pool + post.
func (s *Server) findPostAcrossShards(ctx context.Context, postID string) (*pgxpool.Pool, *postv1.Post, error) {
	pools := s.shards.AllPools()
	type result struct {
		post *postv1.Post
		pool *pgxpool.Pool
		err  error
	}

	results := make(chan result, len(pools))
	for _, pool := range pools {
		go func(p *pgxpool.Pool) {
			post, err := s.getPostFromPool(ctx, p, postID)
			results <- result{post: post, pool: p, err: err}
		}(pool)
	}

	var foundPost *postv1.Post
	var foundPool *pgxpool.Pool
	for range pools {
		r := <-results
		if r.err == nil && r.post != nil {
			foundPost = r.post
			foundPool = r.pool
		}
	}

	if foundPost == nil {
		return nil, nil, fmt.Errorf("post %q: %w", postID, perrors.ErrNotFound)
	}
	return foundPool, foundPost, nil
}

// getPostByID is a convenience wrapper around findPostAcrossShards.
func (s *Server) getPostByID(ctx context.Context, postID string) (*postv1.Post, error) {
	_, post, err := s.findPostAcrossShards(ctx, postID)
	return post, err
}

// getUserCommunityIDs returns the community IDs the user has joined.
// First checks cache, then queries the community database directly.
func (s *Server) getUserCommunityIDs(ctx context.Context, userID string) ([]string, error) {
	// Check cache
	ids, err := s.cache.GetMembership(ctx, userID)
	if err != nil {
		s.logger.Warn("membership cache error", zap.Error(err))
	}
	if ids != nil {
		return ids, nil
	}

	// Query community database for user's joined communities by name
	if s.communityDB == nil {
		return nil, fmt.Errorf("community database not configured")
	}

	rows, err := s.communityDB.Query(ctx,
		`SELECT c.name FROM community_members cm
		 JOIN communities c ON c.id = cm.community_id
		 WHERE cm.user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query user communities: %w", err)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			result = append(result, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan user communities: %w", err)
	}

	// Cache for 5min
	if len(result) > 0 {
		_ = s.cache.SetMembership(ctx, userID, result)
	}

	return result, nil
}

// queryShardForFeed queries a single shard for posts in the given communities.
func (s *Server) queryShardForFeed(ctx context.Context, pool *pgxpool.Pool, communityIDs []string, sortOrder postv1.SortOrder, timeRange postv1.TimeRange, cursor string, fetchLimit int32) ([]*postWithScore, error) {
	if len(communityIDs) == 0 {
		return nil, nil
	}

	var args []any
	argIdx := 1

	// Build community_name IN (...) clause
	placeholders := make([]string, len(communityIDs))
	for i, cid := range communityIDs {
		placeholders[i] = fmt.Sprintf("$%d", argIdx)
		args = append(args, cid)
		argIdx++
	}

	baseSelect := fmt.Sprintf(
		`SELECT id, title, body, url, post_type, author_id, author_username,
		        community_id, community_name, vote_score, comment_count, hot_score,
		        is_edited, is_deleted, is_pinned, is_anonymous, thumbnail_url, created_at, edited_at
		 FROM posts WHERE community_name IN (%s) AND is_deleted = false`,
		strings.Join(placeholders, ","),
	)

	// Time range filter for TOP
	if sortOrder == postv1.SortOrder_SORT_ORDER_TOP && timeRange != postv1.TimeRange_TIME_RANGE_ALL && timeRange != postv1.TimeRange_TIME_RANGE_UNSPECIFIED {
		interval := timeRangeToInterval(timeRange)
		if interval != "" {
			baseSelect += fmt.Sprintf(" AND created_at > now() - interval '%s'", interval)
		}
	}

	// Cursor for cross-shard pagination
	if cursor != "" {
		sortVal, cursorID, cursorTime, err := pagination.DecodeSortCursor(cursor)
		if err == nil {
			switch sortOrder {
			case postv1.SortOrder_SORT_ORDER_HOT:
				baseSelect += fmt.Sprintf(" AND (hot_score, id) < ($%d, $%d)", argIdx, argIdx+1)
				args = append(args, sortVal, cursorID)
				argIdx += 2
			case postv1.SortOrder_SORT_ORDER_NEW:
				baseSelect += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argIdx, argIdx+1)
				args = append(args, cursorTime, cursorID)
				argIdx += 2
			case postv1.SortOrder_SORT_ORDER_TOP:
				baseSelect += fmt.Sprintf(" AND (vote_score, id) < ($%d, $%d)", argIdx, argIdx+1)
				args = append(args, int(sortVal), cursorID)
				argIdx += 2
			}
		}
	}

	// ORDER BY
	var orderBy string
	switch sortOrder {
	case postv1.SortOrder_SORT_ORDER_HOT:
		orderBy = "ORDER BY hot_score DESC, id DESC"
	case postv1.SortOrder_SORT_ORDER_NEW:
		orderBy = "ORDER BY created_at DESC, id DESC"
	case postv1.SortOrder_SORT_ORDER_TOP:
		orderBy = "ORDER BY vote_score DESC, id DESC"
	case postv1.SortOrder_SORT_ORDER_RISING:
		orderBy = "ORDER BY (vote_score::float / GREATEST(1, EXTRACT(EPOCH FROM (now() - created_at)) / 3600.0)) DESC, id DESC"
	default:
		orderBy = "ORDER BY hot_score DESC, id DESC"
	}

	query := fmt.Sprintf("%s %s LIMIT $%d", baseSelect, orderBy, argIdx)
	args = append(args, fetchLimit)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query shard for feed: %w", err)
	}
	defer rows.Close()

	return scanPostsWithScore(rows)
}

// sortPostsBy sorts a combined slice of posts from multiple shards.
func sortPostsBy(posts []*postWithScore, sortOrder postv1.SortOrder) {
	sort.Slice(posts, func(i, j int) bool {
		switch sortOrder {
		case postv1.SortOrder_SORT_ORDER_HOT:
			if posts[i].hotScore != posts[j].hotScore {
				return posts[i].hotScore > posts[j].hotScore
			}
			return posts[i].PostId > posts[j].PostId
		case postv1.SortOrder_SORT_ORDER_NEW:
			ti := posts[i].CreatedAt.AsTime()
			tj := posts[j].CreatedAt.AsTime()
			if !ti.Equal(tj) {
				return ti.After(tj)
			}
			return posts[i].PostId > posts[j].PostId
		case postv1.SortOrder_SORT_ORDER_TOP:
			if posts[i].VoteScore != posts[j].VoteScore {
				return posts[i].VoteScore > posts[j].VoteScore
			}
			return posts[i].PostId > posts[j].PostId
		case postv1.SortOrder_SORT_ORDER_RISING:
			ri := RisingScore(int(posts[i].VoteScore), posts[i].CreatedAt.AsTime())
			rj := RisingScore(int(posts[j].VoteScore), posts[j].CreatedAt.AsTime())
			if ri != rj {
				return ri > rj
			}
			return posts[i].PostId > posts[j].PostId
		default:
			return posts[i].hotScore > posts[j].hotScore
		}
	})
}

// timeRangeToInterval converts a TimeRange enum to a PostgreSQL interval string.
func timeRangeToInterval(tr postv1.TimeRange) string {
	switch tr {
	case postv1.TimeRange_TIME_RANGE_HOUR:
		return "1 hour"
	case postv1.TimeRange_TIME_RANGE_DAY:
		return "1 day"
	case postv1.TimeRange_TIME_RANGE_WEEK:
		return "7 days"
	case postv1.TimeRange_TIME_RANGE_MONTH:
		return "30 days"
	case postv1.TimeRange_TIME_RANGE_YEAR:
		return "365 days"
	case postv1.TimeRange_TIME_RANGE_ALL:
		return ""
	default:
		return ""
	}
}

// --- Row scanning ---

// scanPostRow scans a single post from a pgx.Row.
func scanPostRow(row pgx.Row) (*postv1.Post, error) {
	var (
		id, title, body, postURL, authorID, authorUsername string
		communityID, communityName, thumbnailURL           string
		postType                                           int16
		voteScore, commentCount                            int32
		hotScore                                           float64
		isEdited, isDeleted, isPinned, isAnonymous         bool
		createdAt                                          time.Time
		editedAt                                           *time.Time
	)

	if err := row.Scan(
		&id, &title, &body, &postURL, &postType, &authorID, &authorUsername,
		&communityID, &communityName, &voteScore, &commentCount, &hotScore,
		&isEdited, &isDeleted, &isPinned, &isAnonymous, &thumbnailURL, &createdAt, &editedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("scan post: %w", err)
	}

	post := &postv1.Post{
		PostId:         id,
		Title:          title,
		Body:           body,
		Url:            postURL,
		PostType:       postv1.PostType(postType),
		AuthorId:       authorID,
		AuthorUsername: authorUsername,
		CommunityId:    communityID,
		CommunityName:  communityName,
		VoteScore:      voteScore,
		CommentCount:   commentCount,
		IsEdited:       isEdited,
		IsDeleted:      isDeleted,
		IsPinned:       isPinned,
		IsAnonymous:    isAnonymous,
		ThumbnailUrl:   thumbnailURL,
		CreatedAt:      timestamppb.New(createdAt),
	}

	if editedAt != nil {
		post.EditedAt = timestamppb.New(*editedAt)
	}

	return post, nil
}

// scanPosts scans multiple posts from pgx.Rows, returning postWithScore for cursor encoding.
func scanPosts(rows pgx.Rows) ([]*postWithScore, error) {
	var posts []*postWithScore
	for rows.Next() {
		var (
			id, title, body, postURL, authorID, authorUsername string
			communityID, communityName, thumbnailURL           string
			postType                                           int16
			voteScore, commentCount                            int32
			hotScore                                           float64
			isEdited, isDeleted, isPinned, isAnonymous         bool
			createdAt                                          time.Time
			editedAt                                           *time.Time
		)

		if err := rows.Scan(
			&id, &title, &body, &postURL, &postType, &authorID, &authorUsername,
			&communityID, &communityName, &voteScore, &commentCount, &hotScore,
			&isEdited, &isDeleted, &isPinned, &isAnonymous, &thumbnailURL, &createdAt, &editedAt,
		); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}

		post := &postv1.Post{
			PostId:         id,
			Title:          title,
			Body:           body,
			Url:            postURL,
			PostType:       postv1.PostType(postType),
			AuthorId:       authorID,
			AuthorUsername: authorUsername,
			CommunityId:    communityID,
			CommunityName:  communityName,
			VoteScore:      voteScore,
			CommentCount:   commentCount,
			IsEdited:       isEdited,
			IsDeleted:      isDeleted,
			IsPinned:       isPinned,
			IsAnonymous:    isAnonymous,
			ThumbnailUrl:   thumbnailURL,
			CreatedAt:      timestamppb.New(createdAt),
		}
		if editedAt != nil {
			post.EditedAt = timestamppb.New(*editedAt)
		}

		posts = append(posts, &postWithScore{Post: post, hotScore: hotScore})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan posts rows: %w", err)
	}
	return posts, nil
}

// scanPostsWithScore is an alias for scanPosts for clarity.
func scanPostsWithScore(rows pgx.Rows) ([]*postWithScore, error) {
	return scanPosts(rows)
}

// Ensure we import encoding/json for the home feed cache serialization
var _ = json.Marshal
var _ = math.Max
var _ = sync.WaitGroup{}
