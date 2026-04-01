// Package main is the entry point for the vote gRPC service.
// It wires together Redis (vote state), Kafka (event publishing),
// auth interceptor, rate limiter, and registers VoteServiceServer.
// The vote-service does NOT connect to PostgreSQL — all state is in Redis
// with Kafka for durability via the event stream.
package main

import (
	"context"
	"log"
	"strings"
	"time"

	"go.uber.org/zap"

	votev1 "github.com/redyx/redyx/gen/redyx/vote/v1"
	"github.com/redyx/redyx/internal/platform/auth"
	"github.com/redyx/redyx/internal/platform/config"
	"github.com/redyx/redyx/internal/platform/grpcserver"
	"github.com/redyx/redyx/internal/platform/middleware"
	"github.com/redyx/redyx/internal/platform/observability"
	"github.com/redyx/redyx/internal/platform/ratelimit"
	platformredis "github.com/redyx/redyx/internal/platform/redis"
	"github.com/redyx/redyx/internal/vote"
)

func main() {
	// Initialize structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	// Load config from environment — vote-service uses Redis DB 5, no PostgreSQL
	cfg := config.Load("vote")

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

	// Connect to Redis DB 5 (vote-service's reserved DB)
	// Override the default Redis URL to use DB 5
	redisURL := cfg.RedisURL
	if !strings.Contains(redisURL, "/5") {
		// Replace the default DB number with 5
		if strings.HasSuffix(redisURL, "/0") {
			redisURL = redisURL[:len(redisURL)-2] + "/5"
		} else {
			redisURL = redisURL + "/5"
		}
	}

	rdb, err := platformredis.NewClient(redisURL)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer rdb.Close()

	// Create vote store backed by Redis
	voteStore := vote.NewVoteStore(rdb, logger)

	// Create Kafka producer
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	producer, err := vote.NewProducer(brokers, logger)
	if err != nil {
		logger.Fatal("failed to create kafka producer", zap.Error(err))
	}
	defer producer.Close()

	// Ensure the votes topic exists with correct partition count (6 partitions)
	// This handles Pitfall 1 and 5 from research — explicit topic creation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := producer.EnsureTopic(ctx); err != nil {
		logger.Warn("failed to ensure kafka topic (may not be fatal if topic exists)",
			zap.Error(err),
		)
	}

	// Create rate limiter for Redis
	limiter := ratelimit.New(rdb)

	// Create JWT validator for auth interceptor
	jwtValidator := auth.NewJWTValidator(cfg.JWTSecret)

	// Create gRPC server with middleware chain:
	// Recovery → Logging → AuthInterceptor → RateLimit → ErrorMapping
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

	// Create and register vote service
	voteServer := vote.NewServer(voteStore, producer, logger, vote.WithLimiter(limiter))
	votev1.RegisterVoteServiceServer(srv.Server(), voteServer)

	logger.Info("vote service starting",
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.String("redis_url", redisURL),
		zap.String("kafka_brokers", cfg.KafkaBrokers),
	)

	// Block until shutdown signal
	if err := srv.Run(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}
