// Package post implements the PostService gRPC server with shard-aware
// storage, ranking algorithms, and feed aggregation.
package post

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serialx/hashring"
)

// ShardRouter routes community IDs to the correct database shard using
// consistent hashing with virtual nodes. Each shard has its own pgxpool.Pool.
type ShardRouter struct {
	ring     *hashring.HashRing
	pools    map[string]*pgxpool.Pool
	shardIDs []string
}

// NewShardRouter creates a ShardRouter from a list of DSN strings.
// Each DSN gets a named shard (shard_0, shard_1, ...) and a pgxpool.Pool.
// The hash ring uses 40 virtual nodes per shard for even distribution.
func NewShardRouter(dsns []string) (*ShardRouter, error) {
	if len(dsns) == 0 {
		return nil, fmt.Errorf("no shard DSNs provided")
	}

	pools := make(map[string]*pgxpool.Pool, len(dsns))
	shardIDs := make([]string, 0, len(dsns))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i, dsn := range dsns {
		name := fmt.Sprintf("shard_%d", i)
		shardIDs = append(shardIDs, name)

		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			// Close any pools already created
			for _, p := range pools {
				p.Close()
			}
			return nil, fmt.Errorf("create pool for %s: %w", name, err)
		}

		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			for _, p := range pools {
				p.Close()
			}
			return nil, fmt.Errorf("ping %s: %w", name, err)
		}

		pools[name] = pool
	}

	// Build hash ring with virtual nodes for better distribution.
	// hashring.New uses 40 virtual nodes by default per node.
	ring := hashring.New(shardIDs)

	return &ShardRouter{
		ring:     ring,
		pools:    pools,
		shardIDs: shardIDs,
	}, nil
}

// GetPool returns the pool and shard name for a given community ID.
// The community ID is hashed to determine the shard.
func (r *ShardRouter) GetPool(communityID string) (*pgxpool.Pool, string) {
	node, _ := r.ring.GetNode(communityID)
	return r.pools[node], node
}

// AllPools returns all shard pools for cross-shard queries (e.g., home feed).
func (r *ShardRouter) AllPools() []*pgxpool.Pool {
	pools := make([]*pgxpool.Pool, 0, len(r.pools))
	for _, id := range r.shardIDs {
		pools = append(pools, r.pools[id])
	}
	return pools
}

// ShardCount returns the number of shards.
func (r *ShardRouter) ShardCount() int {
	return len(r.shardIDs)
}

// Close closes all shard database pools.
func (r *ShardRouter) Close() {
	for _, pool := range r.pools {
		pool.Close()
	}
}
