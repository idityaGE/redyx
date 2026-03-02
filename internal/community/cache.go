// Package community implements the CommunityService gRPC server with all
// community management, membership, and moderator assignment RPCs.
package community

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// cachedCommunity is a lightweight struct for Redis caching.
// We avoid caching the full proto message to keep serialization simple.
type cachedCommunity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Rules       []struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"rules"`
	BannerURL   string `json:"banner_url"`
	IconURL     string `json:"icon_url"`
	Visibility  int32  `json:"visibility"`
	MemberCount int32  `json:"member_count"`
	OwnerID     string `json:"owner_id"`
	CreatedAt   int64  `json:"created_at"` // Unix seconds
}

// Cache provides Redis-backed caching for community metadata.
type Cache struct {
	rdb *goredis.Client
	ttl time.Duration
}

// NewCache creates a new community cache with a 1-hour TTL.
func NewCache(rdb *goredis.Client) *Cache {
	return &Cache{
		rdb: rdb,
		ttl: 1 * time.Hour,
	}
}

func cacheKey(name string) string {
	return fmt.Sprintf("community:%s", name)
}

// Get retrieves a cached community by name. Returns nil, nil on cache miss.
func (c *Cache) Get(ctx context.Context, name string) (*cachedCommunity, error) {
	data, err := c.rdb.Get(ctx, cacheKey(name)).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var comm cachedCommunity
	if err := json.Unmarshal(data, &comm); err != nil {
		return nil, fmt.Errorf("cache unmarshal: %w", err)
	}

	return &comm, nil
}

// Set stores a community in the cache with 1hr TTL.
func (c *Cache) Set(ctx context.Context, name string, comm *cachedCommunity) error {
	data, err := json.Marshal(comm)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.rdb.Set(ctx, cacheKey(name), data, c.ttl).Err(); err != nil {
		return fmt.Errorf("cache set: %w", err)
	}

	return nil
}

// Invalidate removes a community from the cache.
func (c *Cache) Invalidate(ctx context.Context, name string) error {
	if err := c.rdb.Del(ctx, cacheKey(name)).Err(); err != nil {
		return fmt.Errorf("cache invalidate: %w", err)
	}
	return nil
}
