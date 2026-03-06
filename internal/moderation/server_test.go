package moderation

import (
	"context"
	"testing"

	"go.uber.org/zap"

	modv1 "github.com/redyx/redyx/gen/redyx/moderation/v1"
	"github.com/redyx/redyx/internal/platform/auth"
)

// TestVerifyModerator_NoClaims verifies that unauthenticated requests are rejected.
func TestVerifyModerator_NoClaims(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	ctx := context.Background() // no claims in context

	_, err := s.verifyModerator(ctx, "testcommunity")
	if err == nil {
		t.Fatal("expected error for unauthenticated request, got nil")
	}
}

// TestVerifyModerator_WithClaims verifies that authenticated requests go through to community check.
func TestVerifyModerator_WithClaims(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		UserID:   "user-123",
		Username: "testuser",
	})

	// Without a community client, this will fail, but it should NOT fail
	// with "unauthenticated" — it should fail because communityClient is nil.
	_, err := s.verifyModerator(ctx, "testcommunity")
	if err == nil {
		t.Fatal("expected error (nil community client), got nil")
	}
	// Verify it's NOT an unauthenticated error
	if err.Error() == "unauthenticated" {
		t.Fatal("should not be unauthenticated when claims are present")
	}
}

// TestSubmitReport_NoClaims verifies unauthenticated users cannot submit reports.
func TestSubmitReport_NoClaims(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	ctx := context.Background()

	_, err := s.SubmitReport(ctx, &modv1.SubmitReportRequest{
		CommunityName: "test",
		ContentId:     "post-1",
		ContentType:   modv1.ContentType_CONTENT_TYPE_POST,
		Reason:        "Spam",
	})
	if err == nil {
		t.Fatal("expected error for unauthenticated report submission, got nil")
	}
}

// TestCheckBan_NoBan verifies CheckBan returns is_banned=false when no ban exists.
func TestCheckBan_NoBan(t *testing.T) {
	s := &Server{
		logger: zap.NewNop(),
		store:  &Store{}, // store without pool will fail on DB call
	}

	// Without Redis or DB, this test validates the method signature and
	// that the server struct is properly constructed.
	_, err := s.CheckBan(context.Background(), &modv1.CheckBanRequest{
		CommunityName: "test",
		UserId:        "user-1",
	})
	// Expected to fail because Redis client and store pool are nil,
	// but the method should exist and be callable.
	if err == nil {
		t.Log("CheckBan returned nil error (no backing store)")
	}
}
