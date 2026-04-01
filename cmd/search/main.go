// Package main is the entry point for the search gRPC service.
// It wires together Meilisearch (full-text search), Redis (community autocomplete cache),
// Kafka (post event consumer for indexing), auth interceptor, rate limiter, and registers
// SearchServiceServer. Uses Redis DB 7 per the search-service reservation.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	commonv1 "github.com/redyx/redyx/gen/redyx/common/v1"
	communityv1 "github.com/redyx/redyx/gen/redyx/community/v1"
	searchv1 "github.com/redyx/redyx/gen/redyx/search/v1"
	"github.com/redyx/redyx/internal/platform/auth"
	"github.com/redyx/redyx/internal/platform/config"
	"github.com/redyx/redyx/internal/platform/grpcserver"
	"github.com/redyx/redyx/internal/platform/middleware"
	"github.com/redyx/redyx/internal/platform/observability"
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

	// Initialize metrics
	metrics, err := observability.InitMetrics(logger)
	if err != nil {
		logger.Fatal("failed to init metrics", zap.Error(err))
	}

	// Initialize tracing (optional - returns nil if env not set)
	ctx := context.Background()
	tracer, err := observability.InitTracing(ctx, logger)
	if err != nil {
		logger.Fatal("failed to init tracing", zap.Error(err))
	}
	if tracer != nil {
		defer tracer.Shutdown(ctx)
	}

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

	// Connect to community-service via gRPC for seeding community autocomplete.
	communityServiceAddr := envStr("COMMUNITY_SERVICE_ADDR", "community-service:50054")
	communityConn, err := grpc.NewClient(communityServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Warn("failed to connect to community-service, community autocomplete seeding skipped", zap.Error(err))
	} else {
		communityClient := communityv1.NewCommunityServiceClient(communityConn)
		defer communityConn.Close()
		logger.Info("connected to community-service gRPC for autocomplete seeding", zap.String("addr", communityServiceAddr))

		// Seed community autocomplete via community-service gRPC.
		go seedCommunityAutocomplete(communityClient, meili, searchRedis, logger)
	}

	// NOTE: seedPostsFromShards removed — the Kafka indexer handles post indexing.
	// Posts created before the indexer existed won't be searchable (acceptable for v1).

	// Create rate limiter with search Redis client.
	limiter := ratelimit.New(searchRedis)

	// Create JWT validator for auth interceptor.
	jwtValidator := auth.NewJWTValidator(cfg.JWTSecret)

	// Create gRPC server with middleware chain:
	// Recovery -> Logging -> Auth -> RateLimit -> ErrorMapping
	srv := grpcserver.New(cfg.GRPCPort, logger,
		grpcserver.WithServerOptions(observability.StatsHandler()),
		grpcserver.WithUnaryInterceptors(
			metrics.UnaryInterceptor(),
			middleware.Recovery(logger),
			middleware.Logging(logger),
			auth.UnaryInterceptor(jwtValidator),
			ratelimit.UnaryInterceptor(limiter, cfg.RateLimitEnabled),
			middleware.ErrorMapping(),
		),
		grpcserver.WithStreamInterceptors(
			metrics.StreamInterceptor(),
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
// index from community-service via gRPC. Runs once on startup in a goroutine.
func seedCommunityAutocomplete(communityClient communityv1.CommunityServiceClient, meili *search.MeiliClient, redisClient *goredis.Client, logger *zap.Logger) {
	ctx := context.Background()

	var seeded int
	var cursor string

	for {
		resp, err := communityClient.ListCommunities(ctx, &communityv1.ListCommunitiesRequest{
			Pagination: &commonv1.PaginationRequest{
				Cursor: cursor,
				Limit:  100,
			},
		})
		if err != nil {
			logger.Warn("failed to list communities for seeding", zap.Error(err))
			break
		}

		for _, comm := range resp.GetCommunities() {
			name := comm.GetName()
			memberCount := comm.GetMemberCount()

			// Add to Redis sorted set for ZRANGEBYLEX prefix matching.
			if err := redisClient.ZAdd(ctx, "communities:autocomplete", goredis.Z{
				Score:  0,
				Member: strings.ToLower(name),
			}).Err(); err != nil {
				logger.Warn("failed to add community to redis autocomplete",
					zap.String("name", name), zap.Error(err))
			}

			// Add to Meilisearch communities index.
			if err := meili.IndexCommunity(name, comm.GetIconUrl(), memberCount); err != nil {
				logger.Warn("failed to index community in meilisearch",
					zap.String("name", name), zap.Error(err))
			}

			seeded++
		}

		pag := resp.GetPagination()
		if !pag.GetHasMore() || pag.GetNextCursor() == "" {
			break
		}
		cursor = pag.GetNextCursor()
	}

	logger.Info("seeded community autocomplete via gRPC", zap.Int("count", seeded))
}

// envStr returns the value of an environment variable or a default.
func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
