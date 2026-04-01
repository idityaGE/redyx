// Package main is the entry point for the skeleton gRPC service.
// It wires together all platform libraries and registers the health check service.
package main

import (
	"context"
	"log"
	"time"

	"go.uber.org/zap"

	healthv1 "github.com/redyx/redyx/gen/redyx/health/v1"
	"github.com/redyx/redyx/internal/platform/config"
	"github.com/redyx/redyx/internal/platform/database"
	"github.com/redyx/redyx/internal/platform/grpcserver"
	"github.com/redyx/redyx/internal/platform/middleware"
	"github.com/redyx/redyx/internal/platform/observability"
	platformredis "github.com/redyx/redyx/internal/platform/redis"
	"github.com/redyx/redyx/internal/skeleton"
)

func main() {
	// Initialize structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	// Load config from environment
	cfg := config.Load("skeleton")

	// Initialize metrics
	metrics, err := observability.InitMetrics(logger)
	if err != nil {
		logger.Fatal("failed to init metrics", zap.Error(err))
	}

	// Initialize tracing (optional - returns nil if env not set)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tracer, err := observability.InitTracing(ctx, logger)
	if err != nil {
		logger.Fatal("failed to init tracing", zap.Error(err))
	}
	if tracer != nil {
		defer tracer.Shutdown(ctx)
	}

	// Connect to PostgreSQL
	db, err := database.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	// Connect to Redis
	rdb, err := platformredis.NewClient(cfg.RedisURL)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer rdb.Close()

	// Create gRPC server with all middleware
	srv := grpcserver.New(cfg.GRPCPort, logger,
		grpcserver.WithServerOptions(observability.StatsHandler()),
		grpcserver.WithUnaryInterceptors(
			metrics.UnaryInterceptor(),
			middleware.Recovery(logger),
			middleware.Logging(logger),
			middleware.ErrorMapping(),
		),
		grpcserver.WithStreamInterceptors(
			metrics.StreamInterceptor(),
		),
	)

	// Register skeleton health service
	skeletonServer := skeleton.NewServer(db, rdb, logger)
	healthv1.RegisterHealthServiceServer(srv.Server(), skeletonServer)

	// Block until shutdown signal
	if err := srv.Run(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}
