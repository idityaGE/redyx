// Package main is the entry point for the spam gRPC service.
// It wires together Redis (dedup + behavior tracking), Kafka (behavior consumer),
// blocklist from JSON seed, and registers SpamServiceServer.
// The spam-service does NOT connect to PostgreSQL — all state is in Redis
// with Kafka for event consumption.
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	modv1 "github.com/idityaGE/redyx/gen/redyx/moderation/v1"
	spamv1 "github.com/idityaGE/redyx/gen/redyx/spam/v1"
	"github.com/idityaGE/redyx/internal/platform/auth"
	"github.com/idityaGE/redyx/internal/platform/config"
	"github.com/idityaGE/redyx/internal/platform/grpcserver"
	"github.com/idityaGE/redyx/internal/platform/middleware"
	"github.com/idityaGE/redyx/internal/platform/observability"
	"github.com/idityaGE/redyx/internal/platform/ratelimit"
	platformredis "github.com/idityaGE/redyx/internal/platform/redis"
	"github.com/idityaGE/redyx/internal/spam"
)

func main() {
	// Initialize structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	// Load config from environment — spam-service uses Redis DB 11, no PostgreSQL
	cfg := config.Load("spam")

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

	// Connect to Redis DB 11 (spam-service's reserved DB)
	redisURL := cfg.RedisURL
	if !strings.Contains(redisURL, "/11") {
		if idx := strings.LastIndex(redisURL, "/"); idx != -1 && idx > len("redis://") {
			redisURL = redisURL[:idx] + "/11"
		} else {
			redisURL = redisURL + "/11"
		}
	}

	rdb, err := platformredis.NewClient(redisURL)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer rdb.Close()

	// Load blocklist from JSON seed file
	blocklistPath := findBlocklistPath()
	blocklist, err := spam.LoadBlocklist(blocklistPath)
	if err != nil {
		logger.Fatal("failed to load blocklist",
			zap.String("path", blocklistPath),
			zap.Error(err),
		)
	}
	logger.Info("blocklist loaded", zap.String("path", blocklistPath))

	// Create dedup checker backed by Redis
	dedup := spam.NewDedupChecker(rdb)

	// Create spam server
	spamServer := spam.NewServer(blocklist, dedup, logger)

	// Connect to moderation service for behavior flagging
	moderationAddr := envStr("MODERATION_SERVICE_ADDR", "moderation-service:50061")
	var moderationClient modv1.ModerationServiceClient

	modConn, err := grpc.NewClient(moderationAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Warn("failed to connect to moderation service (behavior flagging will be logged only)",
			zap.String("addr", moderationAddr),
			zap.Error(err),
		)
	} else {
		moderationClient = modv1.NewModerationServiceClient(modConn)
		defer modConn.Close()
		logger.Info("connected to moderation service", zap.String("addr", moderationAddr))
	}

	// Create Kafka behavior consumer
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	consumer, err := spam.NewBehaviorConsumer(brokers, rdb, moderationClient, logger)
	if err != nil {
		logger.Warn("failed to create behavior consumer (behavior analysis disabled)",
			zap.Error(err),
		)
	} else {
		// Start consumer in background goroutine
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			if err := consumer.Run(ctx); err != nil && err != context.Canceled {
				logger.Error("behavior consumer error", zap.Error(err))
			}
		}()
		defer consumer.Close()
		logger.Info("behavior consumer started",
			zap.String("brokers", cfg.KafkaBrokers),
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

	// Register spam service
	spamv1.RegisterSpamServiceServer(srv.Server(), spamServer)

	logger.Info("spam service starting",
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.String("redis_url", redisURL),
		zap.String("kafka_brokers", cfg.KafkaBrokers),
		zap.String("moderation_addr", moderationAddr),
	)

	// Block until shutdown signal
	if err := srv.Run(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

// findBlocklistPath locates the blocklist JSON seed file.
// Tries common paths relative to the binary and working directory.
func findBlocklistPath() string {
	candidates := []string{
		"internal/spam/data/blocklist.json",
		filepath.Join("data", "blocklist.json"),
		"/app/internal/spam/data/blocklist.json",
	}

	// Check if running from repo root or from cmd/spam
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Default to repo-root relative path
	return "internal/spam/data/blocklist.json"
}

// envStr reads an environment variable with a fallback default.
func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Give the Kafka consumer a moment to start before proceeding
func init() {
	_ = time.Millisecond // referenced to avoid unused import
}
