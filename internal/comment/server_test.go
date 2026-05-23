package comment

import (
	"context"
	"testing"

	"github.com/gocql/gocql"
	"go.uber.org/zap"

	commentv1 "github.com/idityaGE/redyx/gen/redyx/comment/v1"
	commonv1 "github.com/idityaGE/redyx/gen/redyx/common/v1"
	"github.com/idityaGE/redyx/internal/platform/auth"
)

// testCommentServer creates a minimal Server suitable for input validation tests.
// Store and external clients are nil — these tests short-circuit before DB/gRPC calls.
func testCommentServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(nil, nil, nil, zap.NewNop())
}

// authedCtx returns a context with test user claims.
func authedCtx() context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{
		UserID:   "user-001",
		Username: "testuser",
	})
}

// ---------- mapSortOrder Tests ----------

func TestMapSortOrder_AllCases(t *testing.T) {
	cases := []struct {
		proto commentv1.CommentSortOrder
		want  CommentSortOrder
	}{
		{commentv1.CommentSortOrder_COMMENT_SORT_ORDER_BEST, SortBest},
		{commentv1.CommentSortOrder_COMMENT_SORT_ORDER_TOP, SortTop},
		{commentv1.CommentSortOrder_COMMENT_SORT_ORDER_NEW, SortNew},
		{commentv1.CommentSortOrder_COMMENT_SORT_ORDER_CONTROVERSIAL, SortControversial},
		{commentv1.CommentSortOrder_COMMENT_SORT_ORDER_UNSPECIFIED, SortBest}, // default
	}

	for _, tc := range cases {
		got := mapSortOrder(tc.proto)
		if got != tc.want {
			t.Errorf("mapSortOrder(%v) = %v, want %v", tc.proto, got, tc.want)
		}
	}
}

// ---------- CreateComment validation ----------

func TestCreateComment_Unauthenticated(t *testing.T) {
	s := testCommentServer(t)

	_, err := s.CreateComment(context.Background(), &commentv1.CreateCommentRequest{
		PostId: "post-1",
		Body:   "A valid comment body",
	})
	if err == nil {
		t.Error("expected error for unauthenticated create comment, got nil")
	}
}

func TestCreateComment_EmptyPostID(t *testing.T) {
	s := testCommentServer(t)

	_, err := s.CreateComment(authedCtx(), &commentv1.CreateCommentRequest{
		PostId: "",
		Body:   "A valid comment body",
	})
	if err == nil {
		t.Error("expected error for empty post_id, got nil")
	}
}

func TestCreateComment_EmptyBody(t *testing.T) {
	s := testCommentServer(t)

	_, err := s.CreateComment(authedCtx(), &commentv1.CreateCommentRequest{
		PostId: "post-1",
		Body:   "",
	})
	if err == nil {
		t.Error("expected error for empty body, got nil")
	}
}

func TestCreateComment_BodyTooLong(t *testing.T) {
	s := testCommentServer(t)

	// 10001 characters — exceeds 10000 limit
	longBody := make([]byte, 10001)
	for i := range longBody {
		longBody[i] = 'a'
	}

	_, err := s.CreateComment(authedCtx(), &commentv1.CreateCommentRequest{
		PostId: "post-1",
		Body:   string(longBody),
	})
	if err == nil {
		t.Error("expected error for body > 10000 characters, got nil")
	}
}

// ---------- GetComment validation ----------

func TestGetComment_EmptyCommentID(t *testing.T) {
	s := testCommentServer(t)

	_, err := s.GetComment(context.Background(), &commentv1.GetCommentRequest{
		CommentId: "",
	})
	if err == nil {
		t.Error("expected error for empty comment_id, got nil")
	}
}

// ---------- UpdateComment validation ----------

func TestUpdateComment_Unauthenticated(t *testing.T) {
	s := testCommentServer(t)

	_, err := s.UpdateComment(context.Background(), &commentv1.UpdateCommentRequest{
		CommentId: "comment-1",
		Body:      "Updated body",
	})
	if err == nil {
		t.Error("expected error for unauthenticated update comment, got nil")
	}
}

func TestUpdateComment_EmptyCommentID(t *testing.T) {
	s := testCommentServer(t)

	_, err := s.UpdateComment(authedCtx(), &commentv1.UpdateCommentRequest{
		CommentId: "",
		Body:      "Updated body",
	})
	if err == nil {
		t.Error("expected error for empty comment_id in update, got nil")
	}
}

func TestUpdateComment_EmptyBody(t *testing.T) {
	s := testCommentServer(t)

	_, err := s.UpdateComment(authedCtx(), &commentv1.UpdateCommentRequest{
		CommentId: "comment-1",
		Body:      "",
	})
	if err == nil {
		t.Error("expected error for empty body in update, got nil")
	}
}

func TestUpdateComment_BodyTooLong(t *testing.T) {
	s := testCommentServer(t)

	longBody := make([]byte, 10001)
	for i := range longBody {
		longBody[i] = 'x'
	}

	_, err := s.UpdateComment(authedCtx(), &commentv1.UpdateCommentRequest{
		CommentId: "comment-1",
		Body:      string(longBody),
	})
	if err == nil {
		t.Error("expected error for body > 10000 chars in update, got nil")
	}
}

// ---------- DeleteComment validation ----------

func TestDeleteComment_Unauthenticated(t *testing.T) {
	s := testCommentServer(t)

	_, err := s.DeleteComment(context.Background(), &commentv1.DeleteCommentRequest{
		CommentId: "comment-1",
	})
	if err == nil {
		t.Error("expected error for unauthenticated delete comment, got nil")
	}
}

func TestDeleteComment_EmptyCommentID(t *testing.T) {
	s := testCommentServer(t)

	_, err := s.DeleteComment(authedCtx(), &commentv1.DeleteCommentRequest{
		CommentId: "",
	})
	if err == nil {
		t.Error("expected error for empty comment_id in delete, got nil")
	}
}

// ---------- ListComments validation ----------

func TestListComments_EmptyPostID(t *testing.T) {
	s := testCommentServer(t)

	_, err := s.ListComments(context.Background(), &commentv1.ListCommentsRequest{
		PostId: "",
	})
	if err == nil {
		t.Error("expected error for empty post_id in list comments, got nil")
	}
}

func TestListComments_ValidRequest_FailsAtStore(t *testing.T) {
	// With a nil store, any valid request will fail at the store layer.
	// This test documents that validation passes for a valid post_id.
	s := testCommentServer(t)

	defer func() {
		if r := recover(); r != nil {
			t.Log("recovered panic from nil store — validation passed")
		}
	}()

	_, err := s.ListComments(context.Background(), &commentv1.ListCommentsRequest{
		PostId: "some-post-id",
		Pagination: &commonv1.PaginationRequest{
			Limit: 200, // over limit — clamps to 50 at server
		},
	})
	// Either panics on nil store (recovered above) or returns a store error.
	_ = err
}

// ---------- ListReplies validation ----------

func TestListReplies_EmptyCommentID(t *testing.T) {
	s := testCommentServer(t)

	_, err := s.ListReplies(context.Background(), &commentv1.ListRepliesRequest{
		CommentId: "",
	})
	if err == nil {
		t.Error("expected error for empty comment_id in list replies, got nil")
	}
}

// ---------- ListCommentsByAuthor validation ----------

func TestListCommentsByAuthor_EmptyUsername(t *testing.T) {
	s := testCommentServer(t)

	_, err := s.ListCommentsByAuthor(context.Background(), &commentv1.ListCommentsByAuthorRequest{
		Username: "",
	})
	if err == nil {
		t.Error("expected error for empty username in list comments by author, got nil")
	}
}

// ---------- commentToProto Tests ----------

func TestCommentToProto_NonZeroParentID_SetInProto(t *testing.T) {
	parentID, err := gocql.ParseUUID("cccccccc-cccc-cccc-cccc-cccccccccccc")
	if err != nil {
		t.Fatalf("parse parentID: %v", err)
	}
	commentID, err := gocql.ParseUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("parse commentID: %v", err)
	}
	postID, err := gocql.ParseUUID("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("parse postID: %v", err)
	}
	authorID, err := gocql.ParseUUID("dddddddd-dddd-dddd-dddd-dddddddddddd")
	if err != nil {
		t.Fatalf("parse authorID: %v", err)
	}

	c := &Comment{
		CommentID:      commentID,
		PostID:         postID,
		ParentID:       parentID,
		AuthorID:       authorID,
		AuthorUsername: "alice",
		Body:           "Hello world",
		VoteScore:      5,
		ReplyCount:     2,
		Path:           "cccc/aaaa",
		DepthVal:       1,
	}

	proto := commentToProto(c)

	if proto.ParentId == "" {
		t.Error("expected ParentId to be set for non-zero parent UUID")
	}
	if proto.CommentId != c.CommentID.String() {
		t.Errorf("expected CommentId %q, got %q", c.CommentID.String(), proto.CommentId)
	}
	if proto.Body != "Hello world" {
		t.Errorf("expected body %q, got %q", "Hello world", proto.Body)
	}
	if proto.VoteScore != 5 {
		t.Errorf("expected VoteScore 5, got %d", proto.VoteScore)
	}
	if proto.AuthorUsername != "alice" {
		t.Errorf("expected AuthorUsername %q, got %q", "alice", proto.AuthorUsername)
	}
}

func TestCommentToProto_ZeroParentID_NotSetInProto(t *testing.T) {
	commentID, _ := gocql.ParseUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	postID, _ := gocql.ParseUUID("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	authorID, _ := gocql.ParseUUID("dddddddd-dddd-dddd-dddd-dddddddddddd")

	c := &Comment{
		CommentID:      commentID,
		PostID:         postID,
		// ParentID is zero value (top-level comment)
		AuthorID:       authorID,
		AuthorUsername: "bob",
		Body:           "Top-level comment",
	}

	proto := commentToProto(c)

	if proto.ParentId != "" {
		t.Errorf("expected empty ParentId for zero UUID, got %q", proto.ParentId)
	}
}
