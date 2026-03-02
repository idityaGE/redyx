// Package main is the entry point for the user gRPC service.
// It connects to the user_profiles PostgreSQL database, runs migrations,
// and serves the UserService over gRPC.
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	userv1 "github.com/redyx/redyx/gen/redyx/user/v1"
	"github.com/redyx/redyx/internal/platform/auth"
	"github.com/redyx/redyx/internal/platform/config"
	"github.com/redyx/redyx/internal/platform/database"
	"github.com/redyx/redyx/internal/platform/grpcserver"
	"github.com/redyx/redyx/internal/platform/middleware"
	"github.com/redyx/redyx/internal/platform/ratelimit"
	platformredis "github.com/redyx/redyx/internal/platform/redis"
	"github.com/redyx/redyx/internal/user"
)

func main() {
	// Initialize structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	// Load config — database URL defaults to user_profiles
	cfg := config.Load("user_profiles")

	// Connect to PostgreSQL (user_profiles database)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	// Run migrations
	if err := runMigrations(ctx, db, logger); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	// Connect to Redis
	rdb, err := platformredis.NewClient(cfg.RedisURL)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer rdb.Close()

	// Create auth validator and rate limiter
	jwtValidator := auth.NewJWTValidator(cfg.JWTSecret)
	limiter := ratelimit.New(rdb)

	// Create gRPC server with middleware chain:
	// Recovery → Logging → RateLimit → Auth → ErrorMapping
	srv := grpcserver.New(cfg.GRPCPort, logger,
		grpcserver.WithUnaryInterceptors(
			middleware.Recovery(logger),
			middleware.Logging(logger),
			ratelimit.UnaryInterceptor(limiter, cfg.RateLimitEnabled),
			auth.UnaryInterceptor(jwtValidator),
			middleware.ErrorMapping(),
		),
	)

	// Register UserService
	userServer := user.NewServer(db, logger)
	userv1.RegisterUserServiceServer(srv.Server(), userServer)

	logger.Info("user service starting",
		zap.Int("grpc_port", cfg.GRPCPort),
	)

	// Block until shutdown signal
	if err := srv.Run(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

// runMigrations executes all *.up.sql files from migrations/user/ in sorted order.
func runMigrations(ctx context.Context, db *pgxpool.Pool, logger *zap.Logger) error {
	migrationDir := "migrations/user"

	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Warn("migrations directory not found, skipping", zap.String("dir", migrationDir))
			return nil
		}
		return err
	}

	// Collect and sort .up.sql files
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		path := filepath.Join(migrationDir, file)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if _, err := db.Exec(ctx, string(sqlBytes)); err != nil {
			// Ignore errors from already-applied migrations (e.g., table already exists)
			logger.Warn("migration may have been already applied",
				zap.String("file", file),
				zap.Error(err),
			)
			continue
		}

		logger.Info("applied migration", zap.String("file", file))
	}

	return nil
}
