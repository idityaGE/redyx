package spam

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// dedupTTL is how long a content hash is remembered (24 hours).
	dedupTTL = 24 * time.Hour
)

// whitespaceRegex collapses multiple whitespace characters into a single space.
var whitespaceRegex = regexp.MustCompile(`\s+`)

// DedupChecker performs SHA-256 based content deduplication using Redis.
// Each user+content_hash combination is stored with a 24h TTL.
type DedupChecker struct {
	rdb *redis.Client
}

// NewDedupChecker creates a DedupChecker backed by the given Redis client.
func NewDedupChecker(rdb *redis.Client) *DedupChecker {
	return &DedupChecker{rdb: rdb}
}

// Check tests whether content from this user has been seen before.
// Content is normalized (lowercase, trimmed, whitespace collapsed) before hashing.
// Returns the SHA-256 hash, whether it's a duplicate, and any error.
//
// Uses Redis SET NX (set if not exists) with a 24h TTL.
// If the key already exists, the content is a duplicate.
func (d *DedupChecker) Check(ctx context.Context, userID, content string) (string, bool, error) {
	// Normalize content: lowercase, trim, collapse whitespace
	normalized := strings.ToLower(strings.TrimSpace(content))
	normalized = whitespaceRegex.ReplaceAllString(normalized, " ")

	// SHA-256 hash of normalized content
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(normalized)))

	// Redis key: dedup:{userID}:{hash}
	key := fmt.Sprintf("dedup:%s:%s", userID, hash)

	// SET NX with TTL — returns true if key was set (new content), false if already exists
	set, err := d.rdb.SetNX(ctx, key, "1", dedupTTL).Result()
	if err != nil {
		return hash, false, fmt.Errorf("dedup setnx: %w", err)
	}

	// If SetNX returned false, the key already existed → duplicate
	isDuplicate := !set
	return hash, isDuplicate, nil
}
