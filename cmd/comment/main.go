// Package main is the entry point for the comment gRPC service.
// It wires together ScyllaDB (comment storage), Redis (caching + vote state reads),
// Kafka (vote event consumer), auth interceptor, rate limiter, and registers
// CommentServiceServer. ScyllaDB is a new data store for this project.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"go.uber.org/zap"

	commentv1 "github.com/redyx/redyx/gen/redyx/comment/v1"
	"github.com/redyx/redyx/internal/comment"
	"github.com/redyx/redyx/internal/platform/auth"
	"github.com/redyx/redyx/internal/platform/config"
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
	cfg := config.Load("comment")

	// Connect to ScyllaDB with retry loop (ScyllaDB container takes 30-60s to start)
	session, err := connectScyllaDB(cfg.ScyllaDBHosts, cfg.ScyllaDBKeyspace, logger)
	if err != nil {
		logger.Fatal("failed to connect to scylladb", zap.Error(err))
	}
	defer session.Close()

	// Run ScyllaDB migrations (CREATE IF NOT EXISTS is idempotent)
	if err := comment.RunMigrations(session, "migrations/comment"); err != nil {
		logger.Fatal("failed to run scylladb migrations", zap.Error(err))
	}
	logger.Info("scylladb migrations applied")

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
		grpcserver.WithUnaryInterceptors(
			middleware.Recovery(logger),
			middleware.Logging(logger),
			auth.UnaryInterceptor(jwtValidator),
			ratelimit.UnaryInterceptor(limiter, cfg.RateLimitEnabled),
			middleware.ErrorMapping(),
		),
	)

	// Create comment store with ScyllaDB session
	store := comment.NewStore(session, logger)

	// Create and register comment service
	commentServer := comment.NewServer(store, voteRedis, logger)
	commentv1.RegisterCommentServiceServer(srv.Server(), commentServer)

	// Start Kafka VoteConsumer goroutine
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()

	if voteRedis != nil {
		brokers := strings.Split(cfg.KafkaBrokers, ",")
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

// connectScyllaDB connects to ScyllaDB with a retry loop.
// ScyllaDB container can take 30-60s to become ready, so we retry up to 30 times (2s apart = 60s total).
func connectScyllaDB(hosts, keyspace string, logger *zap.Logger) (*gocql.Session, error) {
	hostList := strings.Split(hosts, ",")
	for i := range hostList {
		hostList[i] = strings.TrimSpace(hostList[i])
	}

	cluster := gocql.NewCluster(hostList...)
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	// First connect without keyspace to run migrations that create the keyspace
	var session *gocql.Session
	var err error

	for attempt := 1; attempt <= 30; attempt++ {
		session, err = cluster.CreateSession()
		if err == nil {
			session.Close()
			break
		}
		logger.Warn("scylladb connection attempt failed, retrying...",
			zap.Int("attempt", attempt),
			zap.Error(err),
		)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("scylladb connect failed after 30 retries: %w", err)
	}

	// Now connect with keyspace
	cluster.Keyspace = keyspace
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
