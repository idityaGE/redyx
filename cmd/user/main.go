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
	"github.com/redyx/redyx/internal/vote"
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
	// Recovery → Logging → Auth → RateLimit → ErrorMapping
	// Auth runs before RateLimit so rate limiter can differentiate anonymous vs authenticated tiers
	srv := grpcserver.New(cfg.GRPCPort, logger,
		grpcserver.WithUnaryInterceptors(
			middleware.Recovery(logger),
			middleware.Logging(logger),
			auth.UnaryInterceptor(jwtValidator),
			ratelimit.UnaryInterceptor(limiter, cfg.RateLimitEnabled),
			middleware.ErrorMapping(),
		),
	)

	// Register UserService
	userServer := user.NewServer(db, logger)
	userv1.RegisterUserServiceServer(srv.Server(), userServer)

	// Connect to post shard databases for karma consumer author lookups
	var postShards []*pgxpool.Pool
	for _, dsn := range cfg.PostShardDSNs {
		shardCtx, shardCancel := context.WithTimeout(context.Background(), 10*time.Second)
		pool, err := database.NewPostgres(shardCtx, dsn)
		shardCancel()
		if err != nil {
			logger.Warn("failed to connect to post shard, karma author lookup may be limited", zap.Error(err))
			continue
		}
		postShards = append(postShards, pool)
	}
	defer func() {
		for _, pool := range postShards {
			pool.Close()
		}
	}()
	logger.Info("connected to post shards for karma lookups", zap.Int("shard_count", len(postShards)))

	// Start karma consumer goroutine if Kafka brokers configured
	var karmaConsumer *vote.KarmaConsumer
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()

	if cfg.KafkaBrokers != "" {
		brokers := strings.Split(cfg.KafkaBrokers, ",")
		kc, err := vote.NewKarmaConsumer(brokers, rdb, db, postShards, logger)
		if err != nil {
			logger.Warn("failed to create karma consumer, karma updates disabled", zap.Error(err))
		} else {
			karmaConsumer = kc
			go func() {
				if err := karmaConsumer.Run(consumerCtx); err != nil && consumerCtx.Err() == nil {
					logger.Error("karma consumer stopped unexpectedly", zap.Error(err))
				}
			}()
			logger.Info("karma consumer started",
				zap.String("brokers", cfg.KafkaBrokers),
				zap.Int("post_shards", len(postShards)),
			)
		}
	}

	logger.Info("user service starting",
		zap.Int("grpc_port", cfg.GRPCPort),
	)

	// Block until shutdown signal
	if err := srv.Run(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}

	// Cleanup karma consumer on shutdown
	consumerCancel()
	if karmaConsumer != nil {
		karmaConsumer.Close()
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
