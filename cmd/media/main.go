// Package main is the entry point for the media gRPC service.
// It wires together PostgreSQL (media metadata), MinIO/S3 (object storage),
// Redis (rate limiting), auth interceptor, and registers MediaServiceServer.
// Media-service generates presigned URLs for direct client-to-S3 uploads,
// validates file types/sizes, generates image thumbnails, and tracks upload lifecycle.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	mediav1 "github.com/idityaGE/redyx/gen/redyx/media/v1"
	"github.com/idityaGE/redyx/internal/media"
	"github.com/idityaGE/redyx/internal/platform/auth"
	"github.com/idityaGE/redyx/internal/platform/config"
	"github.com/idityaGE/redyx/internal/platform/database"
	"github.com/idityaGE/redyx/internal/platform/grpcserver"
	"github.com/idityaGE/redyx/internal/platform/middleware"
	"github.com/idityaGE/redyx/internal/platform/observability"
	"github.com/idityaGE/redyx/internal/platform/ratelimit"
	platformredis "github.com/idityaGE/redyx/internal/platform/redis"
)

func main() {
	// Initialize structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	// Load config for "media" service
	cfg := config.Load("media")

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

	// Connect to PostgreSQL (media database)
	db, err := database.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	// Run migrations from migrations/media/
	if err := runMigrations(ctx, db, logger); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	// Connect to Redis DB 9 (media-service's reserved DB) for rate limiting
	mediaRedis, err := platformredis.NewClient(redisURL(cfg.RedisURL, 9))
	if err != nil {
		logger.Fatal("failed to connect to redis (DB 9)", zap.Error(err))
	}
	defer mediaRedis.Close()

	// Create S3/MinIO client
	// publicEndpoint is what the browser uses for presigned uploads (e.g. http://localhost:9000)
	// endpoint is what the server uses internally (e.g. http://minio:9000)
	s3Client, err := media.NewS3Client(
		cfg.MinIOEndpoint,
		cfg.MinIOPublicEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		"us-east-1",
		cfg.MinIOBucket,
	)
	if err != nil {
		logger.Fatal("failed to create S3 client", zap.Error(err))
	}

	// Create rate limiter with media Redis client
	limiter := ratelimit.New(mediaRedis)

	// Create JWT validator for auth interceptor
	jwtValidator := auth.NewJWTValidator(cfg.JWTSecret)

	// Create gRPC server with middleware chain on port 50060:
	// Recovery → Logging → Auth → RateLimit → ErrorMapping
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

	// Create Store and Server
	store := media.NewStore(db, logger)
	mediaServer := media.NewServer(store, s3Client, logger)

	// Register MediaServiceServer
	mediav1.RegisterMediaServiceServer(srv.Server(), mediaServer)

	logger.Info("media service starting",
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.String("minio_endpoint", cfg.MinIOEndpoint),
		zap.String("minio_bucket", cfg.MinIOBucket),
	)

	// Block until shutdown signal
	if err := srv.Run(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

// runMigrations executes all .up.sql files from the migrations/media directory.
func runMigrations(ctx context.Context, db *pgxpool.Pool, logger *zap.Logger) error {
	migrationsDir := "migrations/media"

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		// If migrations directory doesn't exist, skip
		if os.IsNotExist(err) {
			logger.Warn("migrations directory not found, skipping", zap.String("dir", migrationsDir))
			return nil
		}
		return err
	}

	// Sort entries to ensure order
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		path := filepath.Join(migrationsDir, entry.Name())
		sql, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		logger.Info("running migration", zap.String("file", entry.Name()))
		if _, err := db.Exec(ctx, string(sql)); err != nil {
			// Log but continue — migration may already be applied (IF NOT EXISTS)
			logger.Warn("migration may already be applied",
				zap.String("file", entry.Name()),
				zap.Error(err),
			)
		}
	}

	return nil
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
