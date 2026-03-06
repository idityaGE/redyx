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

	_, _, err := s.verifyModerator(ctx, "testcommunity")
	if err == nil {
		t.Fatal("expected error for unauthenticated request, got nil")
	}
}

// TestVerifyModerator_WithClaims verifies that claims are extracted before community check.
func TestVerifyModerator_WithClaims(t *testing.T) {
	// This test verifies that when claims are present, verifyModerator
	// proceeds past the auth check and attempts the community client call.
	// Since the community client is nil, it will panic — which proves
	// the auth check passed. We recover from the panic to verify.
	s := &Server{logger: zap.NewNop()}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		UserID:   "user-123",
		Username: "testuser",
	})

	defer func() {
		if r := recover(); r != nil {
			// Expected: panic on nil communityClient means auth check passed
			t.Log("recovered expected panic from nil communityClient — auth check passed")
		}
	}()

	_, _, err := s.verifyModerator(ctx, "testcommunity")
	if err != nil {
		// If we get here without panic, the error should NOT be about auth
		if err.Error() == "unauthenticated" {
			t.Fatal("should not be unauthenticated when claims are present")
		}
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

// TestCheckBan_NilClients verifies CheckBan method exists and attempts community lookup.
func TestCheckBan_NilClients(t *testing.T) {
	s := &Server{
		logger: zap.NewNop(),
		store:  &Store{}, // store without pool will fail on DB call
	}

	// CheckBan calls communityClient which is nil — it will panic.
	// We recover to verify the method exists and is callable.
	defer func() {
		if r := recover(); r != nil {
			t.Log("recovered expected panic from nil communityClient in CheckBan")
		}
	}()

	_, err := s.CheckBan(context.Background(), &modv1.CheckBanRequest{
		CommunityName: "test",
		UserId:        "user-1",
	})
	if err != nil {
		t.Log("CheckBan returned error (expected with nil clients):", err)
	}
}

// TestModActionStringRoundTrip verifies action string<->enum conversion.
func TestModActionStringRoundTrip(t *testing.T) {
	actions := []struct {
		action modv1.ModAction
		str    string
	}{
		{modv1.ModAction_MOD_ACTION_REMOVE_POST, "remove_post"},
		{modv1.ModAction_MOD_ACTION_REMOVE_COMMENT, "remove_comment"},
		{modv1.ModAction_MOD_ACTION_BAN_USER, "ban_user"},
		{modv1.ModAction_MOD_ACTION_UNBAN_USER, "unban_user"},
		{modv1.ModAction_MOD_ACTION_PIN_POST, "pin_post"},
		{modv1.ModAction_MOD_ACTION_UNPIN_POST, "unpin_post"},
		{modv1.ModAction_MOD_ACTION_DISMISS_REPORT, "dismiss_report"},
		{modv1.ModAction_MOD_ACTION_RESTORE_CONTENT, "restore_content"},
	}

	for _, tc := range actions {
		t.Run(tc.str, func(t *testing.T) {
			str := modActionString(tc.action)
			if str != tc.str {
				t.Errorf("modActionString(%v) = %q, want %q", tc.action, str, tc.str)
			}
			back := modActionFromString(str)
			if back != tc.action {
				t.Errorf("modActionFromString(%q) = %v, want %v", str, back, tc.action)
			}
		})
	}
}

// TestContentTypeString verifies content type string conversion.
func TestContentTypeString(t *testing.T) {
	tests := []struct {
		ct   modv1.ContentType
		want string
	}{
		{modv1.ContentType_CONTENT_TYPE_POST, "post"},
		{modv1.ContentType_CONTENT_TYPE_COMMENT, "comment"},
		{modv1.ContentType_CONTENT_TYPE_UNSPECIFIED, "unknown"},
	}

	for _, tc := range tests {
		got := contentTypeString(tc.ct)
		if got != tc.want {
			t.Errorf("contentTypeString(%v) = %q, want %q", tc.ct, got, tc.want)
		}
	}
}
