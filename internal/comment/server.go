package comment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	commentv1 "github.com/idityaGE/redyx/gen/redyx/comment/v1"
	commonv1 "github.com/idityaGE/redyx/gen/redyx/common/v1"
	eventsv1 "github.com/idityaGE/redyx/gen/redyx/events/v1"
	modv1 "github.com/idityaGE/redyx/gen/redyx/moderation/v1"
	postv1 "github.com/idityaGE/redyx/gen/redyx/post/v1"
	spamv1 "github.com/idityaGE/redyx/gen/redyx/spam/v1"
	"github.com/idityaGE/redyx/internal/platform/auth"
	perrors "github.com/idityaGE/redyx/internal/platform/errors"
	"github.com/idityaGE/redyx/internal/platform/pagination"
	"github.com/idityaGE/redyx/internal/platform/ratelimit"
)

// Server implements the CommentServiceServer gRPC interface.
type Server struct {
	commentv1.UnimplementedCommentServiceServer
	store            *Store
	producer         *CommentProducer
	postClient       postv1.PostServiceClient      // gRPC client for post-service (comment enrichment)
	spamClient       spamv1.SpamServiceClient      // gRPC client for spam-service (content checks)
	moderationClient modv1.ModerationServiceClient // gRPC client for moderation-service (ban checks)
	limiter          *ratelimit.Limiter            // action-specific rate limiter
	voteRedis        *redis.Client                 // vote-service Redis DB 5 (read-only for user_vote)
	logger           *zap.Logger
}

// NewServer creates a new comment gRPC server.
func NewServer(store *Store, producer *CommentProducer, voteRedis *redis.Client, logger *zap.Logger, opts ...ServerOption) *Server {
	s := &Server{
		store:     store,
		producer:  producer,
		voteRedis: voteRedis,
		logger:    logger,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ServerOption configures optional dependencies for the comment server.
type ServerOption func(*Server)

// WithPostClient configures the post-service gRPC client for comment enrichment.
func WithPostClient(client postv1.PostServiceClient) ServerOption {
	return func(s *Server) {
		s.postClient = client
	}
}

// WithSpamClient configures the spam-service gRPC client for content checks.
func WithSpamClient(client spamv1.SpamServiceClient) ServerOption {
	return func(s *Server) {
		s.spamClient = client
	}
}

// WithModerationClient configures the moderation-service gRPC client for ban checks.
func WithModerationClient(client modv1.ModerationServiceClient) ServerOption {
	return func(s *Server) {
		s.moderationClient = client
	}
}

// WithLimiter configures the rate limiter for action-specific rate limiting.
func WithLimiter(limiter *ratelimit.Limiter) ServerOption {
	return func(s *Server) {
		s.limiter = limiter
	}
}

// CreateComment creates a new comment on a post (CMNT-01, CMNT-02).
func (s *Server) CreateComment(ctx context.Context, req *commentv1.CreateCommentRequest) (*commentv1.CreateCommentResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("create comment: %w", perrors.ErrUnauthenticated)
	}

	// Action-specific rate limit: 30 comments per hour
	if s.limiter != nil {
		key := fmt.Sprintf("action:comment:%s", claims.UserID)
		cfg := ratelimit.ActionLimits["comment"]
		result, err := s.limiter.Check(ctx, key, cfg.Limit, cfg.WindowSec)
		if err != nil {
			s.logger.Warn("rate limit check failed, allowing request (fail-open)",
				zap.String("user_id", claims.UserID),
				zap.Error(err),
			)
		} else if !result.Allowed {
			return nil, fmt.Errorf("rate limit exceeded: you can create %d comments per hour, retry after %v: %w",
				cfg.Limit, result.RetryAfter.Round(time.Second), perrors.ErrRateLimited)
		}
	}

	// Validate post_id
	postID := strings.TrimSpace(req.GetPostId())
	if postID == "" {
		return nil, fmt.Errorf("post_id is required: %w", perrors.ErrInvalidInput)
	}

	// Validate body (1-10,000 chars)
	body := req.GetBody()
	if len(body) == 0 || len(body) > 10000 {
		return nil, fmt.Errorf("body must be 1-10000 characters: %w", perrors.ErrInvalidInput)
	}

	// If parent_id is set, validate it exists
	parentID := strings.TrimSpace(req.GetParentId())
	if parentID != "" {
		if _, err := s.store.GetComment(ctx, parentID); err != nil {
			return nil, fmt.Errorf("parent comment %q: %w", parentID, perrors.ErrNotFound)
		}
	}

	// Resolve community name from post (needed for ban check).
	// If post-service is unavailable, skip ban/spam checks (fail-open).
	var communityName string
	if s.postClient != nil {
		postResp, err := s.postClient.GetPost(ctx, &postv1.GetPostRequest{PostId: postID})
		if err != nil {
			s.logger.Warn("failed to resolve post for ban/spam check, allowing comment through (fail-open)",
				zap.String("post_id", postID),
				zap.Error(err),
			)
		} else if postResp.GetPost() != nil {
			communityName = postResp.GetPost().GetCommunityName()
		}
	}

	// Ban check: verify user is not banned from this community (fail-open on service error)
	if s.moderationClient != nil && communityName != "" {
		banResp, err := s.moderationClient.CheckBan(ctx, &modv1.CheckBanRequest{
			CommunityName: communityName,
			UserId:        claims.UserID,
		})
		if err != nil {
			s.logger.Warn("ban check failed, allowing comment through (fail-open)",
				zap.String("community", communityName),
				zap.String("user_id", claims.UserID),
				zap.Error(err),
			)
		} else if banResp.GetIsBanned() {
			return nil, fmt.Errorf("you are banned from this community: %w", perrors.ErrForbidden)
		}
	}

	// Spam check: evaluate content before persisting (fail-open on service error)
	if s.spamClient != nil {
		spamResp, err := s.spamClient.CheckContent(ctx, &spamv1.CheckContentRequest{
			UserId:      claims.UserID,
			ContentType: "comment",
			Content:     body,
		})
		if err != nil {
			s.logger.Warn("spam check failed, allowing comment through (fail-open)",
				zap.String("user_id", claims.UserID),
				zap.Error(err),
			)
		} else {
			if spamResp.GetResult() == spamv1.SpamCheckResult_SPAM_CHECK_RESULT_SPAM {
				return nil, fmt.Errorf("your comment couldn't be published — it may contain restricted content: %w", perrors.ErrInvalidInput)
			}
			if spamResp.GetIsDuplicate() {
				return nil, fmt.Errorf("duplicate content detected: %w", perrors.ErrAlreadyExists)
			}
		}
	}

	comment, err := s.store.CreateComment(ctx, postID, parentID, claims.UserID, claims.Username, body)
	if err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}

	// Publish CommentEvent to Kafka (fire-and-forget for notification-service).
	if s.producer != nil {
		var parentCommentAuthorID string
		if parentID != "" {
			parent, err := s.store.GetComment(ctx, parentID)
			if err == nil {
				parentCommentAuthorID = parent.AuthorID.String()
			}
		}

		s.producer.Publish(ctx, &eventsv1.CommentEvent{
			EventId:               uuid.New().String(),
			CommentId:             comment.CommentID.String(),
			PostId:                comment.PostID.String(),
			AuthorId:              claims.UserID,
			AuthorUsername:        claims.Username,
			ParentCommentId:       parentID,
			ParentCommentAuthorId: parentCommentAuthorID,
			PostAuthorId:          "", // Not available in comment-service; notification-service resolves via post-service
			CommunityName:         "", // Not available in comment-service; notification-service resolves via post
			Body:                  body,
			CreatedAt:             timestamppb.New(comment.CreatedAt),
		})
	}

	return &commentv1.CreateCommentResponse{
		Comment: commentToProto(comment),
	}, nil
}

// GetComment returns a single comment by ID (CMNT-03).
// Public method — auth optional for user_vote state.
func (s *Server) GetComment(ctx context.Context, req *commentv1.GetCommentRequest) (*commentv1.GetCommentResponse, error) {
	commentID := strings.TrimSpace(req.GetCommentId())
	if commentID == "" {
		return nil, fmt.Errorf("comment_id is required: %w", perrors.ErrInvalidInput)
	}

	comment, err := s.store.GetComment(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("comment %q: %w", commentID, perrors.ErrNotFound)
	}

	// Check user vote state if authenticated
	var userVote int32
	claims := auth.ClaimsFromContext(ctx)
	if claims != nil && s.voteRedis != nil {
		userVote = s.getUserVote(ctx, claims.UserID, commentID)
	}

	return &commentv1.GetCommentResponse{
		Comment:  commentToProto(comment),
		UserVote: userVote,
	}, nil
}

// UpdateComment updates an existing comment (author only).
func (s *Server) UpdateComment(ctx context.Context, req *commentv1.UpdateCommentRequest) (*commentv1.UpdateCommentResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("update comment: %w", perrors.ErrUnauthenticated)
	}

	commentID := strings.TrimSpace(req.GetCommentId())
	if commentID == "" {
		return nil, fmt.Errorf("comment_id is required: %w", perrors.ErrInvalidInput)
	}

	// Validate body (1-10,000 chars)
	body := req.GetBody()
	if len(body) == 0 || len(body) > 10000 {
		return nil, fmt.Errorf("body must be 1-10000 characters: %w", perrors.ErrInvalidInput)
	}

	// Verify author ownership
	existing, err := s.store.GetComment(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("comment %q: %w", commentID, perrors.ErrNotFound)
	}
	if existing.AuthorID.String() != claims.UserID {
		return nil, fmt.Errorf("only the author can update a comment: %w", perrors.ErrForbidden)
	}

	updated, err := s.store.UpdateComment(ctx, commentID, body)
	if err != nil {
		return nil, fmt.Errorf("update comment: %w", err)
	}

	return &commentv1.UpdateCommentResponse{
		Comment: commentToProto(updated),
	}, nil
}

// DeleteComment soft-deletes a comment (author only, CMNT-05).
// Moderator check deferred to Phase 6.
func (s *Server) DeleteComment(ctx context.Context, req *commentv1.DeleteCommentRequest) (*commentv1.DeleteCommentResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("delete comment: %w", perrors.ErrUnauthenticated)
	}

	commentID := strings.TrimSpace(req.GetCommentId())
	if commentID == "" {
		return nil, fmt.Errorf("comment_id is required: %w", perrors.ErrInvalidInput)
	}

	// Verify author ownership
	existing, err := s.store.GetComment(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("comment %q: %w", commentID, perrors.ErrNotFound)
	}
	if existing.AuthorID.String() != claims.UserID {
		return nil, fmt.Errorf("only the author can delete a comment: %w", perrors.ErrForbidden)
	}

	if err := s.store.DeleteComment(ctx, commentID); err != nil {
		return nil, fmt.Errorf("delete comment: %w", err)
	}

	return &commentv1.DeleteCommentResponse{}, nil
}

// ListComments returns top-level comments for a post with sorting (CMNT-03, CMNT-04).
// Public method — auth optional.
func (s *Server) ListComments(ctx context.Context, req *commentv1.ListCommentsRequest) (*commentv1.ListCommentsResponse, error) {
	postID := strings.TrimSpace(req.GetPostId())
	if postID == "" {
		return nil, fmt.Errorf("post_id is required: %w", perrors.ErrInvalidInput)
	}

	// Map proto sort order to internal type
	sort := mapSortOrder(req.GetSort())

	// Pagination
	pag := req.GetPagination()
	limit := int(pag.GetLimit())
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	cursor := pag.GetCursor()

	comments, nextCursor, totalCount, err := s.store.ListComments(ctx, postID, sort, limit, cursor)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}

	protoComments := make([]*commentv1.Comment, len(comments))
	for i, c := range comments {
		protoComments[i] = commentToProto(c)
	}

	return &commentv1.ListCommentsResponse{
		Comments: protoComments,
		Pagination: &commonv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    nextCursor != "",
			TotalCount: int32(totalCount),
		},
	}, nil
}

// ListReplies returns replies to a specific comment (CMNT-06).
// Public method.
func (s *Server) ListReplies(ctx context.Context, req *commentv1.ListRepliesRequest) (*commentv1.ListRepliesResponse, error) {
	commentID := strings.TrimSpace(req.GetCommentId())
	if commentID == "" {
		return nil, fmt.Errorf("comment_id is required: %w", perrors.ErrInvalidInput)
	}

	// Pagination
	pag := req.GetPagination()
	limit := int(pag.GetLimit())
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	cursor := pag.GetCursor()

	replies, nextCursor, err := s.store.ListReplies(ctx, commentID, limit, cursor)
	if err != nil {
		return nil, fmt.Errorf("list replies: %w", err)
	}

	protoReplies := make([]*commentv1.Comment, len(replies))
	for i, c := range replies {
		protoReplies[i] = commentToProto(c)
	}

	return &commentv1.ListRepliesResponse{
		Replies: protoReplies,
		Pagination: &commonv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    nextCursor != "",
		},
	}, nil
}

// --- Helpers ---

// commentToProto converts an internal Comment to a proto Comment.
func commentToProto(c *Comment) *commentv1.Comment {
	pc := &commentv1.Comment{
		CommentId:      c.CommentID.String(),
		PostId:         c.PostID.String(),
		AuthorId:       c.AuthorID.String(),
		AuthorUsername: c.AuthorUsername,
		Body:           c.Body,
		VoteScore:      int32(c.VoteScore),
		ReplyCount:     int32(c.ReplyCount),
		Path:           c.Path,
		Depth:          int32(c.DepthVal),
		IsEdited:       c.IsEdited,
		IsDeleted:      c.IsDeleted,
		CreatedAt:      timestamppb.New(c.CreatedAt),
	}

	// Only set parent_id if non-zero UUID
	emptyUUID := [16]byte{}
	if c.ParentID != emptyUUID {
		pc.ParentId = c.ParentID.String()
	}

	if !c.EditedAt.IsZero() {
		pc.EditedAt = timestamppb.New(c.EditedAt)
	}

	return pc
}

// mapSortOrder converts a proto CommentSortOrder to internal type.
func mapSortOrder(sort commentv1.CommentSortOrder) CommentSortOrder {
	switch sort {
	case commentv1.CommentSortOrder_COMMENT_SORT_ORDER_BEST:
		return SortBest
	case commentv1.CommentSortOrder_COMMENT_SORT_ORDER_TOP:
		return SortTop
	case commentv1.CommentSortOrder_COMMENT_SORT_ORDER_NEW:
		return SortNew
	case commentv1.CommentSortOrder_COMMENT_SORT_ORDER_CONTROVERSIAL:
		return SortControversial
	default:
		return SortBest // Default to Best
	}
}

// ListCommentsByAuthor returns paginated comments authored by a given user.
// Queries the comments_by_author ScyllaDB table and enriches with post data.
func (s *Server) ListCommentsByAuthor(ctx context.Context, req *commentv1.ListCommentsByAuthorRequest) (*commentv1.ListCommentsByAuthorResponse, error) {
	username := strings.TrimSpace(req.GetUsername())
	if username == "" {
		return nil, fmt.Errorf("username is required: %w", perrors.ErrInvalidInput)
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
		iter = s.store.session.Query(
			`SELECT comment_id, post_id, body, vote_score, is_deleted, created_at
			 FROM redyx_comments.comments_by_author
			 WHERE author_username = ?
			 LIMIT ?`,
			username, limit+1,
		).WithContext(ctx).Iter()
	} else {
		iter = s.store.session.Query(
			`SELECT comment_id, post_id, body, vote_score, is_deleted, created_at
			 FROM redyx_comments.comments_by_author
			 WHERE author_username = ? AND created_at < ?
			 LIMIT ?`,
			username, cursorTime, limit+1,
		).WithContext(ctx).Iter()
	}

	var comments []*commentv1.CommentSummary
	var commentID, postID gocql.UUID
	var body string
	var voteScore int
	var isDeleted bool
	var createdAt time.Time

	for iter.Scan(&commentID, &postID, &body, &voteScore, &isDeleted, &createdAt) {
		if isDeleted {
			continue
		}
		comments = append(comments, &commentv1.CommentSummary{
			CommentId: commentID.String(),
			PostId:    postID.String(),
			Body:      body,
			VoteScore: int32(voteScore),
			CreatedAt: timestamppb.New(createdAt),
		})
	}
	if err := iter.Close(); err != nil {
		s.logger.Error("failed to query comments_by_author", zap.Error(err))
		return nil, fmt.Errorf("list comments by author: %w", err)
	}

	hasMore := len(comments) > int(limit)
	if hasMore {
		comments = comments[:limit]
	}

	// Enrich comments with post title + community name from post-service
	s.enrichCommentSummaries(ctx, comments)

	var nextCursor string
	if hasMore && len(comments) > 0 {
		last := comments[len(comments)-1]
		nextCursor = pagination.EncodeCursor(last.CommentId, last.CreatedAt.AsTime())
	}

	return &commentv1.ListCommentsByAuthorResponse{
		Comments: comments,
		Pagination: &commonv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}, nil
}

// enrichCommentSummaries batch-looks up post titles and community names
// via the post-service gRPC client and fills in the CommentSummary fields.
func (s *Server) enrichCommentSummaries(ctx context.Context, comments []*commentv1.CommentSummary) {
	if s.postClient == nil || len(comments) == 0 {
		return
	}

	// Collect unique post IDs
	postIDs := make(map[string]struct{})
	for _, c := range comments {
		if c.PostId != "" {
			postIDs[c.PostId] = struct{}{}
		}
	}

	type postInfo struct {
		title         string
		communityName string
	}
	postMap := make(map[string]postInfo)

	// Call GetPost for each unique post ID
	for pid := range postIDs {
		resp, err := s.postClient.GetPost(ctx, &postv1.GetPostRequest{PostId: pid})
		if err != nil {
			s.logger.Debug("failed to enrich comment with post data", zap.String("post_id", pid), zap.Error(err))
			continue
		}
		if p := resp.GetPost(); p != nil {
			postMap[pid] = postInfo{title: p.Title, communityName: p.CommunityName}
		}
	}

	// Fill in the comment summaries
	for _, c := range comments {
		if info, ok := postMap[c.PostId]; ok {
			c.PostTitle = info.title
			c.CommunityName = info.communityName
		}
	}
}

// getUserVote reads the user's vote state from vote-service Redis.
// Returns -1 (downvote), 0 (no vote), or 1 (upvote).
func (s *Server) getUserVote(ctx context.Context, userID, commentID string) int32 {
	state, err := s.voteRedis.Get(ctx, fmt.Sprintf("votes:state:%s:%s", userID, commentID)).Result()
	if err != nil {
		return 0
	}
	switch state {
	case "up":
		return 1
	case "down":
		return -1
	default:
		return 0
	}
}
