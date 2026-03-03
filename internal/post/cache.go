package post

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Cache provides Redis-backed caching for the post service.
// Uses Redis DB 4 for post-service cache, and reads from Redis DB 5 (vote-service) for scores.
type Cache struct {
	rdb     *goredis.Client // Post-service cache (DB 4)
	voteRdb *goredis.Client // Vote-service Redis (DB 5, read-only)
	feedTTL time.Duration
	commTTL time.Duration
}

// NewCache creates a new post-service cache.
// rdb is the post-service Redis client (DB 4), voteRdb is vote-service Redis (DB 5, read-only).
func NewCache(rdb *goredis.Client, voteRdb *goredis.Client) *Cache {
	return &Cache{
		rdb:     rdb,
		voteRdb: voteRdb,
		feedTTL: 2 * time.Minute,
		commTTL: 5 * time.Minute,
	}
}

// --- Feed cache ---

// feedCacheKey generates a deterministic key for a feed page.
func feedCacheKey(userID, sort, timeRange, cursor string) string {
	h := sha256.Sum256([]byte(cursor))
	cursorHash := hex.EncodeToString(h[:8])
	return fmt.Sprintf("feed:%s:%s:%s:%s", userID, sort, timeRange, cursorHash)
}

// cachedFeedPage is serialized into Redis.
type cachedFeedPage struct {
	PostsJSON  []byte `json:"posts"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

// GetFeed retrieves a cached feed page.
func (c *Cache) GetFeed(ctx context.Context, userID, sort, timeRange, cursor string) (*cachedFeedPage, error) {
	key := feedCacheKey(userID, sort, timeRange, cursor)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cache get feed: %w", err)
	}

	var page cachedFeedPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("cache unmarshal feed: %w", err)
	}
	return &page, nil
}

// SetFeed stores a feed page in cache with 2min TTL.
func (c *Cache) SetFeed(ctx context.Context, userID, sort, timeRange, cursor string, page *cachedFeedPage) error {
	key := feedCacheKey(userID, sort, timeRange, cursor)
	data, err := json.Marshal(page)
	if err != nil {
		return fmt.Errorf("cache marshal feed: %w", err)
	}
	return c.rdb.Set(ctx, key, data, c.feedTTL).Err()
}

// --- Community membership cache ---

// cachedMembership stores a user's joined community IDs.
type cachedMembership struct {
	CommunityIDs []string `json:"community_ids"`
}

func membershipKey(userID string) string {
	return fmt.Sprintf("membership:%s", userID)
}

// GetMembership retrieves cached community IDs for a user.
func (c *Cache) GetMembership(ctx context.Context, userID string) ([]string, error) {
	data, err := c.rdb.Get(ctx, membershipKey(userID)).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cache get membership: %w", err)
	}

	var m cachedMembership
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("cache unmarshal membership: %w", err)
	}
	return m.CommunityIDs, nil
}

// SetMembership stores a user's community IDs with 5min TTL.
func (c *Cache) SetMembership(ctx context.Context, userID string, communityIDs []string) error {
	data, err := json.Marshal(&cachedMembership{CommunityIDs: communityIDs})
	if err != nil {
		return fmt.Errorf("cache marshal membership: %w", err)
	}
	return c.rdb.Set(ctx, membershipKey(userID), data, c.commTTL).Err()
}

// --- Vote state reads (from vote-service Redis DB 5) ---

// GetUserVote reads a user's vote state on a target from vote-service Redis.
// Returns 1 (upvote), -1 (downvote), or 0 (no vote).
func (c *Cache) GetUserVote(ctx context.Context, userID, targetID string) (int32, error) {
	if c.voteRdb == nil {
		return 0, nil
	}
	key := fmt.Sprintf("votes:state:%s:%s", userID, targetID)
	val, err := c.voteRdb.Get(ctx, key).Result()
	if err == goredis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, nil // Fail open — vote state is non-critical
	}
	switch val {
	case "up":
		return 1, nil
	case "down":
		return -1, nil
	default:
		return 0, nil
	}
}

// GetVoteScore reads the cached score for a target from vote-service Redis.
func (c *Cache) GetVoteScore(ctx context.Context, targetID string) (int32, bool, error) {
	if c.voteRdb == nil {
		return 0, false, nil
	}
	key := fmt.Sprintf("votes:score:%s", targetID)
	val, err := c.voteRdb.Get(ctx, key).Int()
	if err == goredis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, nil // Fail open
	}
	return int32(val), true, nil
}
