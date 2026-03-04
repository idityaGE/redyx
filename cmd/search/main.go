// Package main is the entry point for the search gRPC service.
// It wires together Meilisearch (full-text search), Redis (community autocomplete cache),
// Kafka (post event consumer for indexing), auth interceptor, rate limiter, and registers
// SearchServiceServer. Uses Redis DB 7 per the search-service reservation.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	searchv1 "github.com/redyx/redyx/gen/redyx/search/v1"
	"github.com/redyx/redyx/internal/platform/auth"
	"github.com/redyx/redyx/internal/platform/config"
	"github.com/redyx/redyx/internal/platform/grpcserver"
	"github.com/redyx/redyx/internal/platform/middleware"
	"github.com/redyx/redyx/internal/platform/ratelimit"
	platformredis "github.com/redyx/redyx/internal/platform/redis"
	"github.com/redyx/redyx/internal/search"
)

func main() {
	// Initialize structured logger.
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	// Load config from environment.
	cfg := config.Load("search")

	// Create Meilisearch client and configure indexes.
	meili, err := search.NewMeiliClient(cfg.MeilisearchURL, cfg.MeilisearchAPIKey, logger)
	if err != nil {
		logger.Fatal("failed to create meilisearch client", zap.Error(err))
	}

	// Configure communities index for autocomplete.
	if err := meili.ConfigureCommunitiesIndex(); err != nil {
		logger.Warn("failed to configure communities index", zap.Error(err))
	}

	// Connect to Redis DB 7 (search-service's reserved DB) for rate limiting + autocomplete cache.
	searchRedis, err := platformredis.NewClient(redisURL(cfg.RedisURL, 7))
	if err != nil {
		logger.Fatal("failed to connect to redis (DB 7)", zap.Error(err))
	}
	defer searchRedis.Close()

	// Seed community autocomplete from community database.
	go seedCommunityAutocomplete(cfg, meili, searchRedis, logger)

	// Create rate limiter with search Redis client.
	limiter := ratelimit.New(searchRedis)

	// Create JWT validator for auth interceptor.
	jwtValidator := auth.NewJWTValidator(cfg.JWTSecret)

	// Create gRPC server with middleware chain:
	// Recovery -> Logging -> Auth -> RateLimit -> ErrorMapping
	srv := grpcserver.New(cfg.GRPCPort, logger,
		grpcserver.WithUnaryInterceptors(
			middleware.Recovery(logger),
			middleware.Logging(logger),
			auth.UnaryInterceptor(jwtValidator),
			ratelimit.UnaryInterceptor(limiter, cfg.RateLimitEnabled),
			middleware.ErrorMapping(),
		),
	)

	// Create and register search service.
	searchServer := search.NewServer(meili, searchRedis, logger)
	searchv1.RegisterSearchServiceServer(srv.Server(), searchServer)

	// Start Kafka Indexer goroutine.
	indexerCtx, indexerCancel := context.WithCancel(context.Background())
	defer indexerCancel()

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	indexer, err := search.NewIndexer(brokers, meili, logger)
	if err != nil {
		logger.Fatal("failed to create search indexer", zap.Error(err))
	}
	defer indexer.Close()
	go func() {
		if err := indexer.Run(indexerCtx); err != nil && err != context.Canceled {
			logger.Error("search indexer exited with error", zap.Error(err))
		}
	}()
	logger.Info("search indexer started", zap.String("brokers", cfg.KafkaBrokers))

	logger.Info("search service starting",
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.String("meilisearch_url", cfg.MeilisearchURL),
	)

	// Block until shutdown signal.
	if err := srv.Run(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

// seedCommunityAutocomplete populates the Redis sorted set and Meilisearch communities
// index from the community database. Runs once on startup in a goroutine.
func seedCommunityAutocomplete(cfg *config.Config, meili *search.MeiliClient, redisClient *goredis.Client, logger *zap.Logger) {
	ctx := context.Background()

	// Connect to community database (read-only).
	pool, err := pgxpool.New(ctx, cfg.CommunityDatabaseURL)
	if err != nil {
		logger.Warn("failed to connect to community database for seeding", zap.Error(err))
		return
	}
	defer pool.Close()

	// Query community names and member counts.
	rows, err := pool.Query(ctx, "SELECT name, member_count FROM communities ORDER BY member_count DESC")
	if err != nil {
		logger.Warn("failed to query communities for seeding", zap.Error(err))
		return
	}
	defer rows.Close()

	var seeded int
	for rows.Next() {
		var name string
		var memberCount int32
		if err := rows.Scan(&name, &memberCount); err != nil {
			logger.Warn("failed to scan community row", zap.Error(err))
			continue
		}

		// Add to Redis sorted set for ZRANGEBYLEX prefix matching.
		// Score is 0 for lexicographic ordering.
		if err := redisClient.ZAdd(ctx, "communities:autocomplete", goredis.Z{
			Score:  0,
			Member: strings.ToLower(name),
		}).Err(); err != nil {
			logger.Warn("failed to add community to redis autocomplete",
				zap.String("name", name), zap.Error(err))
		}

		// Add to Meilisearch communities index.
		if err := meili.IndexCommunity(name, "", memberCount); err != nil {
			logger.Warn("failed to index community in meilisearch",
				zap.String("name", name), zap.Error(err))
		}

		seeded++
	}

	if err := rows.Err(); err != nil {
		logger.Warn("community rows iteration error", zap.Error(err))
	}

	logger.Info("seeded community autocomplete", zap.Int("count", seeded))
}

// redisURL adjusts a Redis URL to use a specific database number.
func redisURL(baseURL string, db int) string {
	if idx := strings.LastIndex(baseURL, "/"); idx != -1 {
		suffix := baseURL[idx+1:]
		if len(suffix) <= 2 && (suffix == "" || (suffix[0] >= '0' && suffix[0] <= '9')) {
			return fmt.Sprintf("%s/%d", baseURL[:idx], db)
		}
	}
	return fmt.Sprintf("%s/%d", baseURL, db)
}
