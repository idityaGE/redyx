// Package main is the entry point for the notification gRPC + WebSocket service.
// It wires together PostgreSQL (notification storage), Redis (unread count cache),
// Kafka (comment event consumer), WebSocket hub (real-time delivery), auth interceptor,
// rate limiter, and registers NotificationServiceServer. The service runs both a
// gRPC server (port 50059) and an HTTP server (port 8081) for WebSocket connections.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	notificationv1 "github.com/idityaGE/redyx/gen/redyx/notification/v1"
	"github.com/idityaGE/redyx/internal/notification"
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

	// Load config from environment
	cfg := config.Load("notification")

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

	// Connect to PostgreSQL (notifications database)
	db, err := database.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	// Run migrations
	if err := runMigrations(ctx, db, logger); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	// Connect to Redis DB 8 (notification-service's reserved DB)
	notifRedis, err := platformredis.NewClient(redisURL(cfg.RedisURL, 8))
	if err != nil {
		logger.Fatal("failed to connect to redis (DB 8)", zap.Error(err))
	}
	defer notifRedis.Close()

	// Create JWT validator for auth interceptor + WebSocket validation
	jwtValidator := auth.NewJWTValidator(cfg.JWTSecret)

	// Create rate limiter with notification Redis client
	limiter := ratelimit.New(notifRedis)

	// Create Store, Hub, Server
	store := notification.NewStore(db, logger)
	hub := notification.NewHub(store, jwtValidator, logger)
	server := notification.NewServer(store, hub, notifRedis, logger)

	// Create gRPC server with middleware chain on port 50059:
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

	// Register NotificationServiceServer
	notificationv1.RegisterNotificationServiceServer(srv.Server(), server)

	// Start Kafka Consumer in goroutine
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()

	// Create PostResolver for enriching notifications with post author/community info
	postServiceAddr := os.Getenv("POST_SERVICE_ADDR")
	if postServiceAddr == "" {
		postServiceAddr = "post-service:50055"
	}
	postResolver, err := notification.NewPostResolver(postServiceAddr, logger)
	if err != nil {
		logger.Fatal("failed to create post resolver", zap.Error(err))
	}
	defer postResolver.Close()

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	consumer, err := notification.NewConsumer(brokers, store, hub, postResolver, logger)
	if err != nil {
		logger.Fatal("failed to create kafka consumer", zap.Error(err))
	}
	defer consumer.Close()

	go func() {
		if err := consumer.Run(consumerCtx); err != nil && err != context.Canceled {
			logger.Error("notification consumer exited with error", zap.Error(err))
		}
	}()
	logger.Info("notification consumer started", zap.String("brokers", cfg.KafkaBrokers))

	// Start HTTP server on WebSocketPort (8081) for WebSocket endpoint
	mux := http.NewServeMux()
	hub.ServeHTTP(mux)
	go func() {
		addr := fmt.Sprintf(":%d", cfg.WebSocketPort)
		logger.Info("websocket server starting", zap.String("addr", addr))
		if err := http.ListenAndServe(addr, mux); err != nil {
			logger.Fatal("websocket server failed", zap.Error(err))
		}
	}()

	logger.Info("notification service starting",
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.Int("websocket_port", cfg.WebSocketPort),
		zap.String("database_url", "***"),
	)

	// Block on gRPC srv.Run()
	if err := srv.Run(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

// runMigrations executes all .up.sql files from migrations/notification/ in sorted order.
func runMigrations(ctx context.Context, db *pgxpool.Pool, logger *zap.Logger) error {
	migrationsDir := "migrations/notification"

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
