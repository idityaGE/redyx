// Package main is the entry point for the comment gRPC service.
// It wires together ScyllaDB (comment storage), Redis (caching + vote state reads),
// Kafka (vote event consumer), auth interceptor, rate limiter, and registers
// CommentServiceServer. ScyllaDB is a new data store for this project.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	commentv1 "github.com/idityaGE/redyx/gen/redyx/comment/v1"
	modv1 "github.com/idityaGE/redyx/gen/redyx/moderation/v1"
	postv1 "github.com/idityaGE/redyx/gen/redyx/post/v1"
	spamv1 "github.com/idityaGE/redyx/gen/redyx/spam/v1"
	"github.com/idityaGE/redyx/internal/comment"
	"github.com/idityaGE/redyx/internal/platform/auth"
	"github.com/idityaGE/redyx/internal/platform/config"
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
	cfg := config.Load("comment")

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

	// Connect to ScyllaDB with retry loop (ScyllaDB container takes 30-60s to start)
	// Phase 1: connect WITHOUT keyspace to run migrations that create it
	migrationSession, err := connectScyllaDBNoKeyspace(cfg.ScyllaDBHosts, logger)
	if err != nil {
		logger.Fatal("failed to connect to scylladb for migrations", zap.Error(err))
	}

	// Run ScyllaDB migrations (CREATE IF NOT EXISTS is idempotent — creates keyspace + tables)
	if err := comment.RunMigrations(migrationSession, "migrations/comment"); err != nil {
		migrationSession.Close()
		logger.Fatal("failed to run scylladb migrations", zap.Error(err))
	}
	migrationSession.Close()
	logger.Info("scylladb migrations applied")

	// Phase 2: reconnect WITH keyspace now that it exists
	session, err := connectScyllaDBWithKeyspace(cfg.ScyllaDBHosts, cfg.ScyllaDBKeyspace, logger)
	if err != nil {
		logger.Fatal("failed to connect to scylladb with keyspace", zap.Error(err))
	}
	defer session.Close()

	// Connect to Redis DB 6 (comment-service's reserved DB) for rate limiting
	commentRedis, err := platformredis.NewClient(redisURL(cfg.RedisURL, 6))
	if err != nil {
		logger.Fatal("failed to connect to redis (DB 6)", zap.Error(err))
	}
	defer commentRedis.Close()

	// Connect to vote-service Redis DB 5 (read-only, for user_vote state + SCARD counts)
	voteRedis, err := platformredis.NewClient(redisURL(cfg.RedisURL, 5))
	if err != nil {
		logger.Warn("failed to connect to vote redis (DB 5), vote state reads disabled", zap.Error(err))
		voteRedis = nil
	}
	if voteRedis != nil {
		defer voteRedis.Close()
	}

	// Create rate limiter with comment Redis client
	limiter := ratelimit.New(commentRedis)

	// Create JWT validator for auth interceptor
	jwtValidator := auth.NewJWTValidator(cfg.JWTSecret)

	// Create gRPC server with middleware chain:
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

	// Create comment store with ScyllaDB session
	store := comment.NewStore(session, logger)

	// Backfill comments_by_author table from existing comments_by_post data.
	// This is idempotent (INSERT IF NOT EXISTS) and handles the case where
	// comments_by_author was created after comments already existed.
	backfillCtx, backfillCancel := context.WithTimeout(context.Background(), 60*time.Second)
	backfillCount, err := store.BackfillCommentsByAuthor(backfillCtx)
	backfillCancel()
	if err != nil {
		logger.Warn("comments_by_author backfill completed with errors", zap.Int("inserted", backfillCount), zap.Error(err))
	} else if backfillCount > 0 {
		logger.Info("comments_by_author backfill complete", zap.Int("inserted", backfillCount))
	}

	// Create Kafka CommentProducer for publishing comment events
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	commentProducer, err := comment.NewCommentProducer(brokers, logger)
	if err != nil {
		logger.Fatal("failed to create comment producer", zap.Error(err))
	}
	defer commentProducer.Close()

	// Ensure Kafka topics exist before producing
	if err := commentProducer.EnsureTopic(context.Background()); err != nil {
		logger.Fatal("failed to ensure comments topic", zap.Error(err))
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

	// Connect to post-service via gRPC for comment enrichment (post_title, community_name).
	postServiceAddr := envStr("POST_SERVICE_ADDR", "post-service:50055")
	var postClient postv1.PostServiceClient
	postConn, err := grpc.NewClient(postServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Warn("failed to connect to post-service, comment enrichment disabled", zap.Error(err))
	} else {
		defer postConn.Close()
		postClient = postv1.NewPostServiceClient(postConn)
		logger.Info("connected to post-service gRPC", zap.String("addr", postServiceAddr))
	}

	// Create and register comment service
	var serverOpts []comment.ServerOption
	if postClient != nil {
		serverOpts = append(serverOpts, comment.WithPostClient(postClient))
	}
	if spamClient != nil {
		serverOpts = append(serverOpts, comment.WithSpamClient(spamClient))
	}
	if moderationClient != nil {
		serverOpts = append(serverOpts, comment.WithModerationClient(moderationClient))
	}
	serverOpts = append(serverOpts, comment.WithLimiter(limiter))
	commentServer := comment.NewServer(store, commentProducer, voteRedis, logger, serverOpts...)
	commentv1.RegisterCommentServiceServer(srv.Server(), commentServer)

	// Start Kafka VoteConsumer goroutine
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()

	if voteRedis != nil {
		voteConsumer, err := comment.NewVoteConsumer(brokers, store, voteRedis, logger)
		if err != nil {
			logger.Fatal("failed to create vote consumer", zap.Error(err))
		}
		defer voteConsumer.Close()
		go func() {
			if err := voteConsumer.Run(consumerCtx); err != nil && err != context.Canceled {
				logger.Error("vote consumer exited with error", zap.Error(err))
			}
		}()
		logger.Info("comment vote consumer started", zap.String("brokers", cfg.KafkaBrokers))
	} else {
		logger.Warn("vote redis not available, vote consumer disabled — vote scores will not update")
	}

	logger.Info("comment service starting",
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.String("scylladb_hosts", cfg.ScyllaDBHosts),
		zap.String("scylladb_keyspace", cfg.ScyllaDBKeyspace),
	)

	// Block until shutdown signal
	if err := srv.Run(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

// connectScyllaDBNoKeyspace connects to ScyllaDB without a keyspace (for running migrations).
// Retries up to 30 times (2s apart = 60s total) to handle slow ScyllaDB startup.
func connectScyllaDBNoKeyspace(hosts string, logger *zap.Logger) (*gocql.Session, error) {
	hostList := strings.Split(hosts, ",")
	for i := range hostList {
		hostList[i] = strings.TrimSpace(hostList[i])
	}

	cluster := gocql.NewCluster(hostList...)
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	var session *gocql.Session
	var err error

	for attempt := 1; attempt <= 30; attempt++ {
		session, err = cluster.CreateSession()
		if err == nil {
			logger.Info("connected to scylladb (no keyspace, for migrations)",
				zap.Strings("hosts", hostList),
			)
			return session, nil
		}
		logger.Warn("scylladb connection attempt failed, retrying...",
			zap.Int("attempt", attempt),
			zap.Error(err),
		)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("scylladb connect failed after 30 retries: %w", err)
}

// connectScyllaDBWithKeyspace connects to ScyllaDB with a specific keyspace.
// Retries up to 30 times (2s apart = 60s total).
func connectScyllaDBWithKeyspace(hosts, keyspace string, logger *zap.Logger) (*gocql.Session, error) {
	hostList := strings.Split(hosts, ",")
	for i := range hostList {
		hostList[i] = strings.TrimSpace(hostList[i])
	}

	cluster := gocql.NewCluster(hostList...)
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	cluster.Keyspace = keyspace

	var session *gocql.Session
	var err error

	for attempt := 1; attempt <= 30; attempt++ {
		session, err = cluster.CreateSession()
		if err == nil {
			logger.Info("connected to scylladb",
				zap.Strings("hosts", hostList),
				zap.String("keyspace", keyspace),
			)
			return session, nil
		}
		logger.Warn("scylladb keyspace connection attempt failed, retrying...",
			zap.Int("attempt", attempt),
			zap.String("keyspace", keyspace),
			zap.Error(err),
		)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("scylladb keyspace connect failed after 30 retries: %w", err)
}

// envStr returns the value of an environment variable or a default.
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
