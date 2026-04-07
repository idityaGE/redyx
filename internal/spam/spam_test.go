package spam

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	spamv1 "github.com/idityaGE/redyx/gen/redyx/spam/v1"
)

// ---------- Blocklist Tests ----------

func TestLoadBlocklist(t *testing.T) {
	path := filepath.Join("data", "blocklist.json")
	bl, err := LoadBlocklist(path)
	if err != nil {
		t.Fatalf("LoadBlocklist(%q) error: %v", path, err)
	}
	if bl == nil {
		t.Fatal("LoadBlocklist returned nil")
	}
	if len(bl.keywords) == 0 {
		t.Error("expected keywords to be loaded")
	}
	if len(bl.domains) == 0 {
		t.Error("expected domains to be loaded")
	}
}

func TestCheckKeywords_Clean(t *testing.T) {
	bl := testBlocklist(t)
	matched, _ := bl.CheckKeywords("This is a normal post about programming")
	if matched {
		t.Error("expected clean content to not match any keywords")
	}
}

func TestCheckKeywords_Match(t *testing.T) {
	bl := testBlocklist(t)
	matched, _ := bl.CheckKeywords("Hey everyone, BUY NOW before it's too late!")
	if !matched {
		t.Error("expected 'buy now' keyword to be matched (case-insensitive)")
	}
}

func TestCheckKeywords_CaseInsensitive(t *testing.T) {
	bl := testBlocklist(t)
	matched, _ := bl.CheckKeywords("CLICK HERE for amazing deals")
	if !matched {
		t.Error("expected case-insensitive matching for 'click here'")
	}
}

func TestCheckURLs_Clean(t *testing.T) {
	bl := testBlocklist(t)
	matched, _ := bl.CheckURLs([]string{"https://golang.org", "https://github.com"})
	if matched {
		t.Error("expected clean URLs to not match any blocked domains")
	}
}

func TestCheckURLs_BlockedDomain(t *testing.T) {
	bl := testBlocklist(t)
	matched, _ := bl.CheckURLs([]string{"https://malware-site.com/payload"})
	if !matched {
		t.Error("expected blocked domain to be matched")
	}
}

func TestCheckURLs_BitLy(t *testing.T) {
	bl := testBlocklist(t)
	matched, _ := bl.CheckURLs([]string{"https://bit.ly/abc123"})
	if !matched {
		t.Error("expected bit.ly shortened URL to be matched")
	}
}

func TestExtractURLs_BareURLs(t *testing.T) {
	urls := ExtractURLs("Check out https://example.com and http://test.org/page")
	if len(urls) != 2 {
		t.Errorf("expected 2 URLs, got %d: %v", len(urls), urls)
	}
}

func TestExtractURLs_MarkdownLinks(t *testing.T) {
	urls := ExtractURLs("Visit [my site](https://example.com) for more")
	if len(urls) < 1 {
		t.Errorf("expected at least 1 URL from markdown link, got %d: %v", len(urls), urls)
	}
}

func TestExtractURLs_NoURLs(t *testing.T) {
	urls := ExtractURLs("This is a plain text post with no links")
	if len(urls) != 0 {
		t.Errorf("expected 0 URLs, got %d: %v", len(urls), urls)
	}
}

// ---------- Dedup Tests ----------

func TestCheckDuplicate_FirstTime(t *testing.T) {
	rdb := testRedis(t)
	dc := NewDedupChecker(rdb)

	hash, isDup, err := dc.Check(context.Background(), "user1", "Hello world")
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if isDup {
		t.Error("expected first submission to not be duplicate")
	}
	if hash == "" {
		t.Error("expected non-empty content hash")
	}
}

func TestCheckDuplicate_SecondTime(t *testing.T) {
	rdb := testRedis(t)
	dc := NewDedupChecker(rdb)
	ctx := context.Background()

	_, _, err := dc.Check(ctx, "user1", "Hello world")
	if err != nil {
		t.Fatalf("first Check error: %v", err)
	}

	_, isDup, err := dc.Check(ctx, "user1", "Hello world")
	if err != nil {
		t.Fatalf("second Check error: %v", err)
	}
	if !isDup {
		t.Error("expected second identical submission to be duplicate")
	}
}

func TestCheckDuplicate_DifferentUser(t *testing.T) {
	rdb := testRedis(t)
	dc := NewDedupChecker(rdb)
	ctx := context.Background()

	_, _, err := dc.Check(ctx, "user1", "Hello world")
	if err != nil {
		t.Fatalf("user1 Check error: %v", err)
	}

	_, isDup, err := dc.Check(ctx, "user2", "Hello world")
	if err != nil {
		t.Fatalf("user2 Check error: %v", err)
	}
	if isDup {
		t.Error("expected different user with same content to not be duplicate")
	}
}

func TestCheckDuplicate_NormalizesContent(t *testing.T) {
	rdb := testRedis(t)
	dc := NewDedupChecker(rdb)
	ctx := context.Background()

	hash1, _, _ := dc.Check(ctx, "user1", "  Hello   World  ")
	hash2, isDup, _ := dc.Check(ctx, "user1", "hello world")
	if !isDup {
		t.Error("expected normalized content to be treated as duplicate")
	}
	if hash1 != hash2 {
		t.Errorf("expected same hash after normalization, got %s and %s", hash1, hash2)
	}
}

// ---------- Server / CheckContent Tests ----------

func TestCheckContent_Clean(t *testing.T) {
	srv := testServer(t)
	resp, err := srv.CheckContent(context.Background(), &spamv1.CheckContentRequest{
		UserId:      "user1",
		ContentType: "post_body",
		Content:     "This is a normal discussion about Go programming",
	})
	if err != nil {
		t.Fatalf("CheckContent error: %v", err)
	}
	if resp.GetResult() != spamv1.SpamCheckResult_SPAM_CHECK_RESULT_CLEAN {
		t.Errorf("expected CLEAN result, got %v", resp.GetResult())
	}
	if len(resp.GetReasons()) != 0 {
		t.Errorf("expected no reasons for clean content, got %v", resp.GetReasons())
	}
}

func TestCheckContent_BlockedKeyword(t *testing.T) {
	srv := testServer(t)
	resp, err := srv.CheckContent(context.Background(), &spamv1.CheckContentRequest{
		UserId:      "user1",
		ContentType: "post_body",
		Content:     "Hey! Buy now before it's too late!",
	})
	if err != nil {
		t.Fatalf("CheckContent error: %v", err)
	}
	if resp.GetResult() != spamv1.SpamCheckResult_SPAM_CHECK_RESULT_SPAM {
		t.Errorf("expected SPAM result for blocked keyword, got %v", resp.GetResult())
	}
	if len(resp.GetReasons()) == 0 {
		t.Error("expected at least one reason for blocked keyword")
	}
}

func TestCheckContent_BlockedURL(t *testing.T) {
	srv := testServer(t)
	resp, err := srv.CheckContent(context.Background(), &spamv1.CheckContentRequest{
		UserId:      "user1",
		ContentType: "post_body",
		Content:     "Check out this link",
		Urls:        []string{"https://malware-site.com/payload"},
	})
	if err != nil {
		t.Fatalf("CheckContent error: %v", err)
	}
	if resp.GetResult() != spamv1.SpamCheckResult_SPAM_CHECK_RESULT_SPAM {
		t.Errorf("expected SPAM result for blocked URL, got %v", resp.GetResult())
	}
}

func TestCheckContent_BlockedURL_InContent(t *testing.T) {
	srv := testServer(t)
	resp, err := srv.CheckContent(context.Background(), &spamv1.CheckContentRequest{
		UserId:      "user1",
		ContentType: "post_body",
		Content:     "Visit https://bit.ly/scam for free stuff",
	})
	if err != nil {
		t.Fatalf("CheckContent error: %v", err)
	}
	if resp.GetResult() != spamv1.SpamCheckResult_SPAM_CHECK_RESULT_SPAM {
		t.Errorf("expected SPAM result for blocked URL in content, got %v", resp.GetResult())
	}
}

func TestCheckContent_Duplicate(t *testing.T) {
	srv := testServer(t)
	ctx := context.Background()

	req := &spamv1.CheckContentRequest{
		UserId:      "user1",
		ContentType: "post_body",
		Content:     "Some unique post content here",
	}

	resp1, err := srv.CheckContent(ctx, req)
	if err != nil {
		t.Fatalf("first CheckContent error: %v", err)
	}
	if resp1.GetIsDuplicate() {
		t.Error("expected first submission to not be duplicate")
	}
	if resp1.GetContentHash() == "" {
		t.Error("expected content hash to be set")
	}

	resp2, err := srv.CheckContent(ctx, req)
	if err != nil {
		t.Fatalf("second CheckContent error: %v", err)
	}
	if !resp2.GetIsDuplicate() {
		t.Error("expected second identical submission to be duplicate")
	}
}

func TestCheckContent_VagueReasons(t *testing.T) {
	srv := testServer(t)
	resp, err := srv.CheckContent(context.Background(), &spamv1.CheckContentRequest{
		UserId:      "user1",
		ContentType: "post_body",
		Content:     "Buy now and get free money!",
	})
	if err != nil {
		t.Fatalf("CheckContent error: %v", err)
	}
	for _, reason := range resp.GetReasons() {
		if reason == "buy now" || reason == "free money" {
			t.Errorf("reason should be vague, not contain specific keyword: %s", reason)
		}
	}
}

// ---------- Test helpers ----------

func testBlocklist(t *testing.T) *Blocklist {
	t.Helper()
	path := filepath.Join("data", "blocklist.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("blocklist.json not found at %s", path)
	}
	bl, err := LoadBlocklist(path)
	if err != nil {
		t.Fatalf("LoadBlocklist error: %v", err)
	}
	return bl
}

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func testServer(t *testing.T) *Server {
	t.Helper()
	bl := testBlocklist(t)
	rdb := testRedis(t)
	dc := NewDedupChecker(rdb)
	logger := zap.NewNop()
	return NewServer(bl, dc, logger)
}
