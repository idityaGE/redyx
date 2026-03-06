// Package main is the entry point for the moderation gRPC service.
// It wires together PostgreSQL (reports, bans, mod_log), Redis (ban cache),
// cross-service gRPC clients (community, post, comment), auth interceptor,
// rate limiter, and registers ModerationServiceServer.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	commentv1 "github.com/redyx/redyx/gen/redyx/comment/v1"
	commv1 "github.com/redyx/redyx/gen/redyx/community/v1"
	modv1 "github.com/redyx/redyx/gen/redyx/moderation/v1"
	postv1 "github.com/redyx/redyx/gen/redyx/post/v1"
	"github.com/redyx/redyx/internal/moderation"
	"github.com/redyx/redyx/internal/platform/auth"
	"github.com/redyx/redyx/internal/platform/config"
	"github.com/redyx/redyx/internal/platform/database"
	"github.com/redyx/redyx/internal/platform/grpcserver"
	"github.com/redyx/redyx/internal/platform/middleware"
	"github.com/redyx/redyx/internal/platform/ratelimit"
	platformredis "github.com/redyx/redyx/internal/platform/redis"
)

func main() {
	// Initialize structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	// Load config from environment
	cfg := config.Load("moderation")

	// Connect to PostgreSQL (moderation database)
	ctx := context.Background()
	db, err := database.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	// Run migrations
	if err := runMigrations(ctx, db, logger); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	// Connect to Redis DB 10 (moderation-service's reserved DB)
	modRedis, err := platformredis.NewClient(redisURL(cfg.RedisURL, 10))
	if err != nil {
		logger.Fatal("failed to connect to redis (DB 10)", zap.Error(err))
	}
	defer modRedis.Close()

	// Create JWT validator for auth interceptor
	jwtValidator := auth.NewJWTValidator(cfg.JWTSecret)

	// Create rate limiter
	limiter := ratelimit.New(modRedis)

	// Connect to community-service (for role verification)
	communityAddr := envStr("COMMUNITY_SERVICE_ADDR", "community-service:50054")
	communityConn, err := grpc.NewClient(communityAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal("failed to connect to community-service", zap.Error(err), zap.String("addr", communityAddr))
	}
	defer communityConn.Close()
	communityClient := commv1.NewCommunityServiceClient(communityConn)

	// Connect to post-service (for pin/remove operations)
	postAddr := envStr("POST_SERVICE_ADDR", "post-service:50055")
	postConn, err := grpc.NewClient(postAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal("failed to connect to post-service", zap.Error(err), zap.String("addr", postAddr))
	}
	defer postConn.Close()
	postClient := postv1.NewPostServiceClient(postConn)

	// Connect to comment-service (for remove operations)
	commentAddr := envStr("COMMENT_SERVICE_ADDR", "comment-service:50057")
	commentConn, err := grpc.NewClient(commentAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal("failed to connect to comment-service", zap.Error(err), zap.String("addr", commentAddr))
	}
	defer commentConn.Close()
	commentClient := commentv1.NewCommentServiceClient(commentConn)

	// Create Store and Server
	store := moderation.NewStore(db)
	server := moderation.NewServer(store, logger, communityClient, postClient, commentClient, modRedis)

	// Create gRPC server with middleware chain on configured port:
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

	// Register ModerationServiceServer
	modv1.RegisterModerationServiceServer(srv.Server(), server)

	logger.Info("moderation service starting",
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.String("community_addr", communityAddr),
		zap.String("post_addr", postAddr),
		zap.String("comment_addr", commentAddr),
	)

	// Block on gRPC srv.Run()
	if err := srv.Run(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

// runMigrations executes all .up.sql files from migrations/moderation/ in sorted order.
func runMigrations(ctx context.Context, db *pgxpool.Pool, logger *zap.Logger) error {
	migrationsDir := "migrations/moderation"

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Warn("migrations directory not found, skipping", zap.String("dir", migrationsDir))
			return nil
		}
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// Collect and sort .up.sql files
	var upFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upFiles = append(upFiles, entry.Name())
		}
	}
	sort.Strings(upFiles)

	for _, filename := range upFiles {
		path := filepath.Join(migrationsDir, filename)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}

		sql := string(sqlBytes)
		if strings.TrimSpace(sql) == "" {
			continue
		}

		_, err = db.Exec(ctx, sql)
		if err != nil {
			// Ignore "already exists" errors for idempotent migrations
			if strings.Contains(err.Error(), "already exists") {
				logger.Debug("migration already applied", zap.String("file", filename))
				continue
			}
			return fmt.Errorf("execute migration %s: %w", filename, err)
		}

		logger.Info("applied migration", zap.String("file", filename))
	}

	return nil
}

// envStr returns the environment variable or fallback value.
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
