package user

import (
	"context"
	"testing"

	"go.uber.org/zap"

	commonv1 "github.com/idityaGE/redyx/gen/redyx/common/v1"
	userv1 "github.com/idityaGE/redyx/gen/redyx/user/v1"
	"github.com/idityaGE/redyx/internal/platform/auth"
)

// testUserServer creates a minimal Server for validation tests.
// DB is nil; tests only cover logic that short-circuits before DB calls.
func testUserServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(nil, zap.NewNop())
}

// authedUserCtx returns a context with test user claims.
func authedUserCtx() context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{
		UserID:   "user-profile-1",
		Username: "profiletestuser",
	})
}

// ---------- profileToProto Tests ----------

func TestProfileToProto_ActiveUser(t *testing.T) {
	p := &profile{
		UserID:      "user-abc",
		Username:    "alice",
		DisplayName: "Alice Wonderland",
		Bio:         "Curiouser and curiouser",
		AvatarURL:   "https://example.com/avatar.jpg",
		Karma:       1234,
	}

	proto := profileToProto(p)

	if proto.UserId != "user-abc" {
		t.Errorf("expected UserId 'user-abc', got %q", proto.UserId)
	}
	if proto.Username != "alice" {
		t.Errorf("expected Username 'alice', got %q", proto.Username)
	}
	if proto.DisplayName != "Alice Wonderland" {
		t.Errorf("expected DisplayName 'Alice Wonderland', got %q", proto.DisplayName)
	}
	if proto.Bio != "Curiouser and curiouser" {
		t.Errorf("expected Bio, got %q", proto.Bio)
	}
	if proto.Karma != 1234 {
		t.Errorf("expected Karma 1234, got %d", proto.Karma)
	}
}

func TestProfileToProto_DeletedUser_Sanitized(t *testing.T) {
	p := &profile{
		UserID:      "user-deleted",
		Username:    "deleted_user",
		DisplayName: "Secret Name",
		Bio:         "Private bio",
		Karma:       9999,
	}
	// Set DeletedAt to simulate soft-deleted account
	p.DeletedAt.Valid = true

	proto := profileToProto(p)

	// Deleted users should show "[deleted]" username and no PII
	if proto.Username != "[deleted]" {
		t.Errorf("expected Username '[deleted]' for deleted user, got %q", proto.Username)
	}
	if proto.DisplayName != "" {
		t.Errorf("expected empty DisplayName for deleted user, got %q", proto.DisplayName)
	}
	if proto.Bio != "" {
		t.Errorf("expected empty Bio for deleted user, got %q", proto.Bio)
	}
	if proto.Karma != 0 {
		t.Errorf("expected zero Karma for deleted user, got %d", proto.Karma)
	}
}

// ---------- GetProfile validation ----------

func TestGetProfile_EmptyUsername(t *testing.T) {
	s := testUserServer(t)

	_, err := s.GetProfile(context.Background(), &userv1.GetProfileRequest{
		Username: "",
	})
	if err == nil {
		t.Error("expected error for empty username in GetProfile, got nil")
	}
}

// ---------- UpdateProfile validation ----------

func TestUpdateProfile_Unauthenticated(t *testing.T) {
	s := testUserServer(t)

	_, err := s.UpdateProfile(context.Background(), &userv1.UpdateProfileRequest{
		DisplayName: "New Name",
	})
	if err == nil {
		t.Error("expected error for unauthenticated UpdateProfile, got nil")
	}
}

func TestUpdateProfile_BioTooLong(t *testing.T) {
	s := testUserServer(t)

	// 501 characters — exceeds 500 limit
	longBio := make([]byte, 501)
	for i := range longBio {
		longBio[i] = 'a'
	}

	_, err := s.UpdateProfile(authedUserCtx(), &userv1.UpdateProfileRequest{
		Bio: string(longBio),
	})
	if err == nil {
		t.Error("expected error for bio > 500 characters, got nil")
	}
}

func TestUpdateProfile_DisplayNameTooLong(t *testing.T) {
	s := testUserServer(t)

	// 51 characters — exceeds 50 limit
	longName := make([]byte, 51)
	for i := range longName {
		longName[i] = 'a'
	}

	_, err := s.UpdateProfile(authedUserCtx(), &userv1.UpdateProfileRequest{
		DisplayName: string(longName),
	})
	if err == nil {
		t.Error("expected error for display_name > 50 characters, got nil")
	}
}

// ---------- DeleteAccount validation ----------

func TestDeleteAccount_Unauthenticated(t *testing.T) {
	s := testUserServer(t)

	_, err := s.DeleteAccount(context.Background(), &userv1.DeleteAccountRequest{})
	if err == nil {
		t.Error("expected error for unauthenticated DeleteAccount, got nil")
	}
}

// ---------- GetUserPosts validation ----------

func TestGetUserPosts_EmptyUsername(t *testing.T) {
	s := testUserServer(t)

	_, err := s.GetUserPosts(context.Background(), &userv1.GetUserPostsRequest{
		Username: "",
	})
	if err == nil {
		t.Error("expected error for empty username in GetUserPosts, got nil")
	}
}

func TestGetUserPosts_NoPostClient_ReturnsEmpty(t *testing.T) {
	// No post client configured — server returns empty list gracefully
	s := testUserServer(t)

	resp, err := s.GetUserPosts(context.Background(), &userv1.GetUserPostsRequest{
		Username: "someuser",
	})
	if err != nil {
		t.Fatalf("expected no error when post client is nil, got: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.GetPosts()) != 0 {
		t.Errorf("expected 0 posts when no post client, got %d", len(resp.GetPosts()))
	}
}

// ---------- GetUserComments validation ----------

func TestGetUserComments_EmptyUsername(t *testing.T) {
	s := testUserServer(t)

	_, err := s.GetUserComments(context.Background(), &userv1.GetUserCommentsRequest{
		Username: "",
	})
	if err == nil {
		t.Error("expected error for empty username in GetUserComments, got nil")
	}
}

func TestGetUserComments_NoCommentClient_ReturnsEmpty(t *testing.T) {
	// No comment client — server returns empty list gracefully
	s := testUserServer(t)

	resp, err := s.GetUserComments(context.Background(), &userv1.GetUserCommentsRequest{
		Username: "someuser",
	})
	if err != nil {
		t.Fatalf("expected no error when comment client is nil, got: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.GetComments()) != 0 {
		t.Errorf("expected 0 comments when no comment client, got %d", len(resp.GetComments()))
	}
}

// ---------- GetUserCommunities validation ----------

func TestGetUserCommunities_EmptyUserID(t *testing.T) {
	s := testUserServer(t)

	_, err := s.GetUserCommunities(context.Background(), &userv1.GetUserCommunitiesRequest{
		UserId: "",
	})
	if err == nil {
		t.Error("expected error for empty user_id in GetUserCommunities, got nil")
	}
}

func TestGetUserCommunities_NoCommunityClient_ReturnsEmpty(t *testing.T) {
	// No community client — server returns empty list gracefully
	s := testUserServer(t)

	resp, err := s.GetUserCommunities(context.Background(), &userv1.GetUserCommunitiesRequest{
		UserId: "user-123",
		Pagination: &commonv1.PaginationRequest{
			Limit: 10,
		},
	})
	if err != nil {
		t.Fatalf("expected no error when community client is nil, got: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.GetCommunities()) != 0 {
		t.Errorf("expected 0 communities when no community client, got %d", len(resp.GetCommunities()))
	}
}
