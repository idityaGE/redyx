package post

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	postv1 "github.com/idityaGE/redyx/gen/redyx/post/v1"
	"github.com/idityaGE/redyx/internal/platform/auth"
)

// ---------- HotScore / RisingScore Tests ----------

func TestHotScore_NewPost_HighScore(t *testing.T) {
	// Brand-new post with a positive score should have a positive hot score
	score := HotScore(10, time.Now())
	if score <= 0 {
		t.Errorf("expected positive hot score for new post with votes, got %f", score)
	}
}

func TestHotScore_OlderPost_LowerScore(t *testing.T) {
	now := time.Now()
	recent := HotScore(10, now)
	old := HotScore(10, now.Add(-24*time.Hour))
	if recent <= old {
		t.Errorf("expected recent post to have higher hot score: recent=%f, old=%f", recent, old)
	}
}

func TestHotScore_ZeroScore_StillPositive(t *testing.T) {
	score := HotScore(0, time.Now())
	if score <= 0 {
		t.Errorf("expected positive hot score for 0-score post, got %f", score)
	}
}

func TestRisingScore_PositiveVotes(t *testing.T) {
	score := RisingScore(100, time.Now().Add(-1*time.Hour))
	if score <= 0 {
		t.Errorf("expected positive rising score, got %f", score)
	}
}

func TestRisingScore_ZeroVotes(t *testing.T) {
	score := RisingScore(0, time.Now())
	if score != 0 {
		t.Errorf("expected zero rising score for zero votes, got %f", score)
	}
}

// ---------- Server Input Validation Tests ----------

// testPostServer creates a minimal Server for validation-only tests.
// ShardRouter is constructed as a stub with no real pools — tests only
// cover pure validation logic that short-circuits before any DB access.
func testPostServer(t *testing.T) *Server {
	t.Helper()
	// Bypass NewShardRouter (requires live DSNs) by building the struct directly.
	shards := &ShardRouter{}
	return NewServer(shards, nil, nil, nil, nil, zap.NewNop())
}

// authenticatedCtx returns a context carrying test user claims.
func authenticatedCtx() context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{
		UserID:   "user-test-1",
		Username: "testuser",
	})
}

// ---------- CreatePost validation ----------

func TestCreatePost_Unauthenticated(t *testing.T) {
	s := testPostServer(t)

	_, err := s.CreatePost(context.Background(), &postv1.CreatePostRequest{
		Title:         "Valid Title",
		CommunityName: "testcomm",
	})
	if err == nil {
		t.Error("expected error for unauthenticated create post, got nil")
	}
}

func TestCreatePost_EmptyTitle(t *testing.T) {
	s := testPostServer(t)

	_, err := s.CreatePost(authenticatedCtx(), &postv1.CreatePostRequest{
		Title:         "",
		CommunityName: "testcomm",
	})
	if err == nil {
		t.Error("expected error for empty title, got nil")
	}
}

func TestCreatePost_TitleTooLong(t *testing.T) {
	s := testPostServer(t)

	// Generate a 301-character title
	longTitle := make([]byte, 301)
	for i := range longTitle {
		longTitle[i] = 'a'
	}

	_, err := s.CreatePost(authenticatedCtx(), &postv1.CreatePostRequest{
		Title:         string(longTitle),
		CommunityName: "testcomm",
	})
	if err == nil {
		t.Error("expected error for title > 300 characters, got nil")
	}
}

func TestCreatePost_EmptyCommunityName(t *testing.T) {
	s := testPostServer(t)

	_, err := s.CreatePost(authenticatedCtx(), &postv1.CreatePostRequest{
		Title:         "Valid Title",
		CommunityName: "",
	})
	if err == nil {
		t.Error("expected error for empty community_name, got nil")
	}
}

func TestCreatePost_LinkType_MissingURL(t *testing.T) {
	s := testPostServer(t)

	_, err := s.CreatePost(authenticatedCtx(), &postv1.CreatePostRequest{
		Title:         "A Link Post",
		CommunityName: "testcomm",
		PostType:      postv1.PostType_POST_TYPE_LINK,
		Url:           "", // missing
	})
	if err == nil {
		t.Error("expected error for link post without URL, got nil")
	}
}

func TestCreatePost_LinkType_InvalidURL(t *testing.T) {
	s := testPostServer(t)

	_, err := s.CreatePost(authenticatedCtx(), &postv1.CreatePostRequest{
		Title:         "A Link Post",
		CommunityName: "testcomm",
		PostType:      postv1.PostType_POST_TYPE_LINK,
		Url:           "not a valid url",
	})
	if err == nil {
		t.Error("expected error for link post with invalid URL, got nil")
	}
}

func TestCreatePost_MediaType_NoMediaIDs(t *testing.T) {
	s := testPostServer(t)

	_, err := s.CreatePost(authenticatedCtx(), &postv1.CreatePostRequest{
		Title:         "A Media Post",
		CommunityName: "testcomm",
		PostType:      postv1.PostType_POST_TYPE_MEDIA,
		MediaIds:      nil,
	})
	if err == nil {
		t.Error("expected error for media post without media_ids, got nil")
	}
}

func TestCreatePost_TextBody_TooLong(t *testing.T) {
	s := testPostServer(t)

	// Generate a 40001-character body
	longBody := make([]byte, 40001)
	for i := range longBody {
		longBody[i] = 'x'
	}

	_, err := s.CreatePost(authenticatedCtx(), &postv1.CreatePostRequest{
		Title:         "Valid Title",
		CommunityName: "testcomm",
		PostType:      postv1.PostType_POST_TYPE_TEXT,
		Body:          string(longBody),
	})
	if err == nil {
		t.Error("expected error for text body > 40000 characters, got nil")
	}
}

// ---------- GetPost validation ----------

func TestGetPost_EmptyPostID(t *testing.T) {
	s := testPostServer(t)

	_, err := s.GetPost(context.Background(), &postv1.GetPostRequest{
		PostId: "",
	})
	if err == nil {
		t.Error("expected error for empty post_id, got nil")
	}
}

// ---------- UpdatePost validation ----------

func TestUpdatePost_Unauthenticated(t *testing.T) {
	s := testPostServer(t)

	_, err := s.UpdatePost(context.Background(), &postv1.UpdatePostRequest{
		PostId: "post-123",
		Title:  "New Title",
	})
	if err == nil {
		t.Error("expected error for unauthenticated update post, got nil")
	}
}

func TestUpdatePost_EmptyPostID(t *testing.T) {
	s := testPostServer(t)

	_, err := s.UpdatePost(authenticatedCtx(), &postv1.UpdatePostRequest{
		PostId: "",
		Title:  "New Title",
	})
	if err == nil {
		t.Error("expected error for empty post_id in update, got nil")
	}
}

func TestUpdatePost_TitleTooLong(t *testing.T) {
	s := testPostServer(t)

	longTitle := make([]byte, 301)
	for i := range longTitle {
		longTitle[i] = 'a'
	}

	_, err := s.UpdatePost(authenticatedCtx(), &postv1.UpdatePostRequest{
		PostId: "some-id",
		Title:  string(longTitle),
	})
	if err == nil {
		t.Error("expected error for title > 300 chars in update, got nil")
	}
}

// ---------- DeletePost validation ----------

func TestDeletePost_Unauthenticated(t *testing.T) {
	s := testPostServer(t)

	_, err := s.DeletePost(context.Background(), &postv1.DeletePostRequest{
		PostId: "post-123",
	})
	if err == nil {
		t.Error("expected error for unauthenticated delete post, got nil")
	}
}

func TestDeletePost_EmptyPostID(t *testing.T) {
	s := testPostServer(t)

	_, err := s.DeletePost(authenticatedCtx(), &postv1.DeletePostRequest{
		PostId: "",
	})
	if err == nil {
		t.Error("expected error for empty post_id in delete, got nil")
	}
}

// ---------- ListPosts validation ----------

func TestListPosts_EmptyCommunityName(t *testing.T) {
	s := testPostServer(t)

	_, err := s.ListPosts(context.Background(), &postv1.ListPostsRequest{
		CommunityName: "",
	})
	if err == nil {
		t.Error("expected error for empty community_name in list posts, got nil")
	}
}
