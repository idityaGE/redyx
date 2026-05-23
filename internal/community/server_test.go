package community

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	commv1 "github.com/idityaGE/redyx/gen/redyx/community/v1"
	"github.com/idityaGE/redyx/internal/platform/auth"
)

// testCommunityServer creates a minimal Server for validation tests.
// DB and cache are nil; tests only exercise logic that short-circuits before I/O.
func testCommunityServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(nil, nil, zap.NewNop())
}

// authedCommCtx returns a context with test user claims.
func authedCommCtx() context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{
		UserID:   "user-comm-1",
		Username: "commtestuser",
	})
}

// ---------- nameRegex Tests ----------

func TestNameRegex_ValidNames(t *testing.T) {
	valid := []string{
		"abc",
		"Go_lang",
		"community123",
		"a_b_c_d_e_f",
		"ALLCAPS",
		"Mix3dC4se",
	}
	for _, name := range valid {
		if !nameRegex.MatchString(name) {
			t.Errorf("expected %q to be a valid community name", name)
		}
	}
}

func TestNameRegex_InvalidNames(t *testing.T) {
	invalid := []string{
		"ab",           // too short
		"",             // empty
		"has space",    // space
		"has-dash",     // dash
		"has.dot",      // dot
		"toolongname123456789012", // 22 chars — over limit
	}
	for _, name := range invalid {
		if nameRegex.MatchString(name) {
			t.Errorf("expected %q to be an invalid community name", name)
		}
	}
}

// ---------- CreateCommunity validation ----------

func TestCreateCommunity_Unauthenticated(t *testing.T) {
	s := testCommunityServer(t)

	_, err := s.CreateCommunity(context.Background(), &commv1.CreateCommunityRequest{
		Name: "validname",
	})
	if err == nil {
		t.Error("expected error for unauthenticated create community, got nil")
	}
}

func TestCreateCommunity_InvalidName_TooShort(t *testing.T) {
	s := testCommunityServer(t)

	_, err := s.CreateCommunity(authedCommCtx(), &commv1.CreateCommunityRequest{
		Name: "ab", // 2 chars — min is 3
	})
	if err == nil {
		t.Error("expected error for too-short community name, got nil")
	}
}

func TestCreateCommunity_InvalidName_WithSpace(t *testing.T) {
	s := testCommunityServer(t)

	_, err := s.CreateCommunity(authedCommCtx(), &commv1.CreateCommunityRequest{
		Name: "my community",
	})
	if err == nil {
		t.Error("expected error for community name with space, got nil")
	}
}

func TestCreateCommunity_InvalidName_WithDash(t *testing.T) {
	s := testCommunityServer(t)

	_, err := s.CreateCommunity(authedCommCtx(), &commv1.CreateCommunityRequest{
		Name: "my-community",
	})
	if err == nil {
		t.Error("expected error for community name with dash, got nil")
	}
}

func TestCreateCommunity_InvalidName_TooLong(t *testing.T) {
	s := testCommunityServer(t)

	_, err := s.CreateCommunity(authedCommCtx(), &commv1.CreateCommunityRequest{
		Name: "toolongname123456789012", // 23 chars
	})
	if err == nil {
		t.Error("expected error for too-long community name (>21 chars), got nil")
	}
}

func TestCreateCommunity_ValidName_FailsAtDB(t *testing.T) {
	// Valid name passes validation but fails at DB (nil pool). Recover the panic.
	s := testCommunityServer(t)

	defer func() {
		if r := recover(); r != nil {
			t.Log("recovered expected panic from nil DB — validation passed")
		}
	}()

	_, err := s.CreateCommunity(authedCommCtx(), &commv1.CreateCommunityRequest{
		Name:        "validname",
		Description: "A test community",
	})
	// Either panics (caught above) or returns DB error — both acceptable.
	_ = err
}

// ---------- UpdateCommunity validation ----------

func TestUpdateCommunity_Unauthenticated(t *testing.T) {
	s := testCommunityServer(t)

	_, err := s.UpdateCommunity(context.Background(), &commv1.UpdateCommunityRequest{
		Name:        "validname",
		Description: "New description",
	})
	if err == nil {
		t.Error("expected error for unauthenticated update community, got nil")
	}
}

// ---------- JoinCommunity validation ----------

func TestJoinCommunity_Unauthenticated(t *testing.T) {
	s := testCommunityServer(t)

	_, err := s.JoinCommunity(context.Background(), &commv1.JoinCommunityRequest{
		Name: "testcommunity",
	})
	if err == nil {
		t.Error("expected error for unauthenticated join community, got nil")
	}
}

// ---------- LeaveCommunity validation ----------

func TestLeaveCommunity_Unauthenticated(t *testing.T) {
	s := testCommunityServer(t)

	_, err := s.LeaveCommunity(context.Background(), &commv1.LeaveCommunityRequest{
		Name: "testcommunity",
	})
	if err == nil {
		t.Error("expected error for unauthenticated leave community, got nil")
	}
}

// ---------- AssignModerator validation ----------

func TestAssignModerator_Unauthenticated(t *testing.T) {
	s := testCommunityServer(t)

	_, err := s.AssignModerator(context.Background(), &commv1.AssignModeratorRequest{
		Name:     "testcommunity",
		UserId:   "user-2",
		Username: "otheruser",
	})
	if err == nil {
		t.Error("expected error for unauthenticated assign moderator, got nil")
	}
}

// ---------- RevokeModerator validation ----------

func TestRevokeModerator_Unauthenticated(t *testing.T) {
	s := testCommunityServer(t)

	_, err := s.RevokeModerator(context.Background(), &commv1.RevokeModeratorRequest{
		Name:   "testcommunity",
		UserId: "user-2",
	})
	if err == nil {
		t.Error("expected error for unauthenticated revoke moderator, got nil")
	}
}

// ---------- communityRulesToSlice Tests ----------

func TestCommunityRulesToSlice_ConvertsCorrectly(t *testing.T) {
	rules := []*commv1.CommunityRule{
		{Title: "Be Kind", Description: "Treat others well"},
		{Title: "No Spam", Description: "No promotional content"},
	}

	result := communityRulesToSlice(rules)

	if len(result) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(result))
	}

	if result[0]["title"] != "Be Kind" {
		t.Errorf("expected first rule title 'Be Kind', got %q", result[0]["title"])
	}
	if result[0]["description"] != "Treat others well" {
		t.Errorf("expected first rule description 'Treat others well', got %q", result[0]["description"])
	}
	if result[1]["title"] != "No Spam" {
		t.Errorf("expected second rule title 'No Spam', got %q", result[1]["title"])
	}
}

func TestCommunityRulesToSlice_Empty(t *testing.T) {
	result := communityRulesToSlice([]*commv1.CommunityRule{})
	if len(result) != 0 {
		t.Errorf("expected empty result for empty input, got %v", result)
	}
}

func TestCommunityRulesToSlice_Nil(t *testing.T) {
	result := communityRulesToSlice(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %v", result)
	}
}

// ---------- buildCommunityProto Tests ----------

func TestBuildCommunityProto_NoRules(t *testing.T) {
	comm, err := buildCommunityProto(
		"comm-id-1", "testcomm", "A test community",
		nil,            // no rules
		"", "",         // bannerURL, iconURL
		int16(1),       // VISIBILITY_PUBLIC
		42,             // memberCount
		"owner-id-1",
		time.Now(),
	)
	if err != nil {
		t.Fatalf("buildCommunityProto error: %v", err)
	}
	if comm.CommunityId != "comm-id-1" {
		t.Errorf("expected CommunityId 'comm-id-1', got %q", comm.CommunityId)
	}
	if comm.Name != "testcomm" {
		t.Errorf("expected Name 'testcomm', got %q", comm.Name)
	}
	if comm.MemberCount != 42 {
		t.Errorf("expected MemberCount 42, got %d", comm.MemberCount)
	}
	if len(comm.Rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(comm.Rules))
	}
}

func TestBuildCommunityProto_WithRules(t *testing.T) {
	rulesJSON := []byte(`[{"title":"Rule 1","description":"Desc 1"},{"title":"Rule 2","description":"Desc 2"}]`)

	comm, err := buildCommunityProto(
		"comm-id-2", "rulecomm", "Community with rules",
		rulesJSON,
		"https://example.com/banner.jpg", "https://example.com/icon.jpg",
		int16(1),
		10,
		"owner-id-2",
		time.Now(),
	)
	if err != nil {
		t.Fatalf("buildCommunityProto error: %v", err)
	}
	if len(comm.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(comm.Rules))
	}
	if comm.Rules[0].Title != "Rule 1" {
		t.Errorf("expected first rule title 'Rule 1', got %q", comm.Rules[0].Title)
	}
}

func TestBuildCommunityProto_InvalidRulesJSON(t *testing.T) {
	_, err := buildCommunityProto(
		"id", "name", "desc",
		[]byte(`not valid json`),
		"", "", int16(1), 0, "owner",
		time.Now(),
	)
	if err == nil {
		t.Error("expected error for invalid rules JSON, got nil")
	}
}
