package comment

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	commentv1 "github.com/redyx/redyx/gen/redyx/comment/v1"
	commonv1 "github.com/redyx/redyx/gen/redyx/common/v1"
	"github.com/redyx/redyx/internal/platform/auth"
	perrors "github.com/redyx/redyx/internal/platform/errors"
)

// Server implements the CommentServiceServer gRPC interface.
type Server struct {
	commentv1.UnimplementedCommentServiceServer
	store     *Store
	voteRedis *redis.Client // vote-service Redis DB 5 (read-only for user_vote)
	logger    *zap.Logger
}

// NewServer creates a new comment gRPC server.
func NewServer(store *Store, voteRedis *redis.Client, logger *zap.Logger) *Server {
	return &Server{
		store:     store,
		voteRedis: voteRedis,
		logger:    logger,
	}
}

// CreateComment creates a new comment on a post (CMNT-01, CMNT-02).
func (s *Server) CreateComment(ctx context.Context, req *commentv1.CreateCommentRequest) (*commentv1.CreateCommentResponse, error) {
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		return nil, fmt.Errorf("create comment: %w", perrors.ErrUnauthenticated)
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

	comment, err := s.store.CreateComment(ctx, postID, parentID, claims.UserID, claims.Username, body)
	if err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
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
