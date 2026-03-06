// Package main is the entry point for the post gRPC service.
// It creates shard database pools, Redis cache, runs migrations per shard,
// starts a background hot score refresh goroutine, and registers PostServiceServer.
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	communityv1 "github.com/redyx/redyx/gen/redyx/community/v1"
	mediav1 "github.com/redyx/redyx/gen/redyx/media/v1"
	modv1 "github.com/redyx/redyx/gen/redyx/moderation/v1"
	postv1 "github.com/redyx/redyx/gen/redyx/post/v1"
	spamv1 "github.com/redyx/redyx/gen/redyx/spam/v1"
	"github.com/redyx/redyx/internal/platform/auth"
	"github.com/redyx/redyx/internal/platform/config"
	"github.com/redyx/redyx/internal/platform/grpcserver"
	"github.com/redyx/redyx/internal/platform/middleware"
	"github.com/redyx/redyx/internal/platform/ratelimit"
	platformredis "github.com/redyx/redyx/internal/platform/redis"
	"github.com/redyx/redyx/internal/post"
)

func main() {
	// Initialize structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	// Load config from environment
	cfg := config.Load("post")

	// Create shard router from configured DSNs
	shardRouter, err := post.NewShardRouter(cfg.PostShardDSNs)
	if err != nil {
		logger.Fatal("failed to create shard router", zap.Error(err))
	}
	defer shardRouter.Close()

	// Run migrations on each shard
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pools := shardRouter.AllPools()
	for i, pool := range pools {
		migrationsDir := fmt.Sprintf("migrations/post_shard_%d", i)
		if err := runMigrations(ctx, pool, migrationsDir, logger); err != nil {
			logger.Fatal("failed to run migrations",
				zap.Int("shard", i),
				zap.Error(err),
			)
		}
	}

	// Connect to Redis DB 4 for post-service cache
	rdb, err := platformredis.NewClient(redisURL(cfg.RedisURL, 4))
	if err != nil {
		logger.Fatal("failed to connect to redis (DB 4)", zap.Error(err))
	}
	defer rdb.Close()

	// Connect to Redis DB 5 for vote-service reads (read-only)
	voteRdb, err := platformredis.NewClient(redisURL(cfg.RedisURL, 5))
	if err != nil {
		logger.Warn("failed to connect to vote redis (DB 5), vote state reads disabled", zap.Error(err))
		voteRdb = nil
	}
	if voteRdb != nil {
		defer voteRdb.Close()
	}

	// Connect to community-service via gRPC for name→UUID resolution and membership checks.
	communityServiceAddr := envStr("COMMUNITY_SERVICE_ADDR", "community-service:50054")
	communityConn, err := grpc.NewClient(communityServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal("failed to connect to community-service", zap.Error(err))
	}
	defer communityConn.Close()
	communityClient := communityv1.NewCommunityServiceClient(communityConn)
	logger.Info("connected to community-service gRPC", zap.String("addr", communityServiceAddr))

	// Connect to media-service via gRPC for resolving media IDs → URLs.
	mediaServiceAddr := envStr("MEDIA_SERVICE_ADDR", "media-service:50060")
	var mediaClient mediav1.MediaServiceClient
	mediaConn, err := grpc.NewClient(mediaServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Warn("failed to connect to media-service, media posts disabled", zap.Error(err))
	} else {
		defer mediaConn.Close()
		mediaClient = mediav1.NewMediaServiceClient(mediaConn)
		logger.Info("connected to media-service gRPC", zap.String("addr", mediaServiceAddr))
	}

	// Connect to spam-service via gRPC for content spam checks.
	spamServiceAddr := envStr("SPAM_SERVICE_ADDR", "spam-service:50062")
	var spamClient spamv1.SpamServiceClient
	spamConn, err := grpc.NewClient(spamServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Warn("failed to connect to spam-service, spam checks disabled", zap.Error(err))
	} else {
		defer spamConn.Close()
		spamClient = spamv1.NewSpamServiceClient(spamConn)
		logger.Info("connected to spam-service gRPC", zap.String("addr", spamServiceAddr))
	}

	// Connect to moderation-service via gRPC for ban checks.
	moderationServiceAddr := envStr("MODERATION_SERVICE_ADDR", "moderation-service:50061")
	var moderationClient modv1.ModerationServiceClient
	moderationConn, err := grpc.NewClient(moderationServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Warn("failed to connect to moderation-service, ban checks disabled", zap.Error(err))
	} else {
		defer moderationConn.Close()
		moderationClient = modv1.NewModerationServiceClient(moderationConn)
		logger.Info("connected to moderation-service gRPC", zap.String("addr", moderationServiceAddr))
	}

	// Create post cache
	cache := post.NewCache(rdb, voteRdb)

	// Create Kafka PostProducer for publishing post events
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	postProducer, err := post.NewPostProducer(brokers, logger)
	if err != nil {
		logger.Fatal("failed to create post producer", zap.Error(err))
	}
	defer postProducer.Close()

	// Ensure Kafka topics exist before producing
	if err := postProducer.EnsureTopic(context.Background()); err != nil {
		logger.Fatal("failed to ensure posts topic", zap.Error(err))
	}

	// Create rate limiter
	limiter := ratelimit.New(rdb)

	// Create JWT validator for auth interceptor
	jwtValidator := auth.NewJWTValidator(cfg.JWTSecret)

	// Create gRPC server with middleware chain:
	// Recovery → Logging → AuthInterceptor → RateLimit → ErrorMapping
	srv := grpcserver.New(cfg.GRPCPort, logger,
		grpcserver.WithUnaryInterceptors(
			middleware.Recovery(logger),
			middleware.Logging(logger),
			auth.UnaryInterceptor(jwtValidator),
			ratelimit.UnaryInterceptor(limiter, cfg.RateLimitEnabled),
			middleware.ErrorMapping(),
		),
	)

	// Create and register post service
	var postOpts []post.ServerOption
	if spamClient != nil {
		postOpts = append(postOpts, post.WithSpamClient(spamClient))
	}
	if moderationClient != nil {
		postOpts = append(postOpts, post.WithModerationClient(moderationClient))
	}
	postServer := post.NewServer(shardRouter, cache, postProducer, communityClient, mediaClient, logger, postOpts...)
	postv1.RegisterPostServiceServer(srv.Server(), postServer)

	// Start background hot score refresh goroutine
	refreshCtx, refreshCancel := context.WithCancel(context.Background())
	defer refreshCancel()
	go hotScoreRefreshLoop(refreshCtx, shardRouter, logger)

	// Start Kafka vote-event consumer (updates vote_score in PostgreSQL)
	if voteRdb != nil {
		scoreConsumer, err := post.NewScoreConsumer(brokers, shardRouter, voteRdb, logger)
		if err != nil {
			logger.Fatal("failed to create score consumer", zap.Error(err))
		}
		defer scoreConsumer.Close()
		go func() {
			if err := scoreConsumer.Run(refreshCtx); err != nil && err != context.Canceled {
				logger.Error("score consumer exited with error", zap.Error(err))
			}
		}()
		logger.Info("score consumer started", zap.String("brokers", cfg.KafkaBrokers))
	} else {
		logger.Warn("vote redis not available, score consumer disabled — vote scores will not update in DB")
	}

	logger.Info("post service starting",
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.Int("shard_count", shardRouter.ShardCount()),
	)

	// Block until shutdown signal
	if err := srv.Run(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

// hotScoreRefreshLoop periodically recalculates hot_score for recent posts on all shards.
// Runs every 15 minutes. Only updates posts from the last 48 hours.
func hotScoreRefreshLoop(ctx context.Context, shards *post.ShardRouter, logger *zap.Logger) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("hot score refresh loop stopped")
			return
		case <-ticker.C:
			start := time.Now()
			var totalRows int64

			for i, pool := range shards.AllPools() {
				tag, err := pool.Exec(ctx,
					`UPDATE posts SET hot_score = 10000.0 * ln(GREATEST(1, 3 + vote_score)) / POWER(EXTRACT(EPOCH FROM (now() - created_at)) / 3600.0 + 2, 1.8)
					 WHERE created_at > now() - interval '48 hours' AND is_deleted = false`,
				)
				if err != nil {
					logger.Error("hot score refresh failed",
						zap.Int("shard", i),
						zap.Error(err),
					)
					continue
				}
				totalRows += tag.RowsAffected()
			}

			logger.Info("hot score refresh completed",
				zap.Duration("duration", time.Since(start)),
				zap.Int64("rows_updated", totalRows),
			)
		}
	}
}

// redisURL adjusts a Redis URL to use a specific database number.
func redisURL(baseURL string, db int) string {
	// Parse the base URL and replace the DB number.
	// Base format: redis://localhost:6379/0
	if idx := strings.LastIndex(baseURL, "/"); idx != -1 {
		// Check if the part after the last / is a number (DB number)
		suffix := baseURL[idx+1:]
		if len(suffix) <= 2 && (suffix == "" || (suffix[0] >= '0' && suffix[0] <= '9')) {
			return fmt.Sprintf("%s/%d", baseURL[:idx], db)
		}
	}
	return fmt.Sprintf("%s/%d", baseURL, db)
}

// envStr returns the value of an environment variable or a default.
func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// runMigrations executes all .up.sql files from the given directory on the provided pool.
func runMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string, logger *zap.Logger) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Warn("migrations directory not found, skipping", zap.String("dir", migrationsDir))
			return nil
		}
		return err
	}

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

		logger.Info("running migration", zap.String("dir", migrationsDir), zap.String("file", entry.Name()))
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			logger.Warn("migration may already be applied",
				zap.String("file", entry.Name()),
				zap.Error(err),
			)
		}
	}

	return nil
}
