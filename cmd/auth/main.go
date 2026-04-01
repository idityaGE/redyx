// Package main is the entry point for the auth gRPC service.
// It wires together platform libraries, auth modules, and registers
// the AuthServiceServer with the gRPC server.
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

	authv1 "github.com/redyx/redyx/gen/redyx/auth/v1"
	authsvc "github.com/redyx/redyx/internal/auth"
	platformauth "github.com/redyx/redyx/internal/platform/auth"
	"github.com/redyx/redyx/internal/platform/config"
	"github.com/redyx/redyx/internal/platform/database"
	"github.com/redyx/redyx/internal/platform/grpcserver"
	"github.com/redyx/redyx/internal/platform/middleware"
	"github.com/redyx/redyx/internal/platform/observability"
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
	cfg := config.Load("auth")

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

	// Connect to PostgreSQL
	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.NewPostgres(dbCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	// Run database migrations
	if err := runMigrations(dbCtx, db, logger); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	// Connect to Redis
	rdb, err := platformredis.NewClient(cfg.RedisURL)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer rdb.Close()

	// Create auth helper modules
	jwtMgr := authsvc.NewJWTManager(cfg.JWTSecret, cfg.JWTAccessTTL)
	otpMgr := authsvc.NewOTPManager(rdb, time.Duration(cfg.OTPTTLMinutes)*time.Minute)
	oauthMgr := authsvc.NewOAuthManager(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	emailer := authsvc.NewLogSender(logger)

	// Create auth interceptor and rate limiter
	jwtValidator := platformauth.NewJWTValidator(cfg.JWTSecret)
	limiter := ratelimit.New(rdb)

	// Create gRPC server with full interceptor chain:
	// Recovery → Logging → AuthInterceptor → RateLimit → ErrorMapping
	// Auth runs before RateLimit so rate limiter can differentiate anonymous vs authenticated tiers
	srv := grpcserver.New(cfg.GRPCPort, logger,
		grpcserver.WithServerOptions(observability.StatsHandler()),
		grpcserver.WithUnaryInterceptors(
			metrics.UnaryInterceptor(),
			middleware.Recovery(logger),
			middleware.Logging(logger),
			platformauth.UnaryInterceptor(jwtValidator),
			ratelimit.UnaryInterceptor(limiter, cfg.RateLimitEnabled),
			middleware.ErrorMapping(),
		),
		grpcserver.WithStreamInterceptors(
			metrics.StreamInterceptor(),
		),
	)

	// Register AuthServiceServer
	authServer := authsvc.NewServer(db, rdb, jwtMgr, otpMgr, oauthMgr, emailer, logger, cfg.JWTRefreshTTL)
	authv1.RegisterAuthServiceServer(srv.Server(), authServer)

	logger.Info("auth service starting",
		zap.Int("port", cfg.GRPCPort),
		zap.Bool("google_oauth_enabled", oauthMgr != nil),
	)

	// Block until shutdown signal
	if err := srv.Run(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

// runMigrations reads and executes .up.sql files from migrations/auth/ directory
// in sorted order. Simple sequential execution for development.
func runMigrations(ctx context.Context, db *pgxpool.Pool, logger *zap.Logger) error {
	migrationsDir := "migrations/auth"

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
