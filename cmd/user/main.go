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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	commentv1 "github.com/idityaGE/redyx/gen/redyx/comment/v1"
	communityv1 "github.com/idityaGE/redyx/gen/redyx/community/v1"
	postv1 "github.com/idityaGE/redyx/gen/redyx/post/v1"
	userv1 "github.com/idityaGE/redyx/gen/redyx/user/v1"
	"github.com/idityaGE/redyx/internal/platform/auth"
	"github.com/idityaGE/redyx/internal/platform/config"
	"github.com/idityaGE/redyx/internal/platform/database"
	"github.com/idityaGE/redyx/internal/platform/grpcserver"
	"github.com/idityaGE/redyx/internal/platform/middleware"
	"github.com/idityaGE/redyx/internal/platform/observability"
	"github.com/idityaGE/redyx/internal/platform/ratelimit"
	platformredis "github.com/idityaGE/redyx/internal/platform/redis"
	"github.com/idityaGE/redyx/internal/user"
	"github.com/idityaGE/redyx/internal/vote"
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

	// Connect to PostgreSQL (user_profiles database)
	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.NewPostgres(dbCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	// Run migrations
	if err := runMigrations(dbCtx, db, logger); err != nil {
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

	// Connect to post-service via gRPC for GetUserPosts and comment enrichment.
	// This replaces direct post shard DB access — clean service boundaries.
	postServiceAddr := envStr("POST_SERVICE_ADDR", "post-service:50055")
	postConn, err := grpc.NewClient(postServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Warn("failed to connect to post-service, user posts will be unavailable", zap.Error(err))
	}
	if postConn != nil {
		defer postConn.Close()
	}
	var postClient postv1.PostServiceClient
	if postConn != nil {
		postClient = postv1.NewPostServiceClient(postConn)
		logger.Info("connected to post-service gRPC", zap.String("addr", postServiceAddr))
	}

	// Connect to comment-service via gRPC for GetUserComments.
	commentServiceAddr := envStr("COMMENT_SERVICE_ADDR", "comment-service:50057")
	var commentClient commentv1.CommentServiceClient
	commentConn, err := grpc.NewClient(commentServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Warn("failed to connect to comment-service, GetUserComments will return empty", zap.Error(err))
	} else {
		defer commentConn.Close()
		commentClient = commentv1.NewCommentServiceClient(commentConn)
		logger.Info("connected to comment-service gRPC", zap.String("addr", commentServiceAddr))
	}

	// Connect to community-service via gRPC for GetUserCommunities.
	communityServiceAddr := envStr("COMMUNITY_SERVICE_ADDR", "community-service:50054")
	var communityClient communityv1.CommunityServiceClient
	communityConn, err := grpc.NewClient(communityServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Warn("failed to connect to community-service, GetUserCommunities will return empty", zap.Error(err))
	} else {
		defer communityConn.Close()
		communityClient = communityv1.NewCommunityServiceClient(communityConn)
		logger.Info("connected to community-service gRPC", zap.String("addr", communityServiceAddr))
	}

	// Register UserService with post/comment/community gRPC clients
	var serverOpts []user.ServerOption
	if postClient != nil {
		serverOpts = append(serverOpts, user.WithPostClient(postClient))
	}
	if commentClient != nil {
		serverOpts = append(serverOpts, user.WithCommentClient(commentClient))
	}
	if communityClient != nil {
		serverOpts = append(serverOpts, user.WithCommunityClient(communityClient))
	}
	userServer := user.NewServer(db, logger, serverOpts...)
	userv1.RegisterUserServiceServer(srv.Server(), userServer)

	// Start karma consumer goroutine if Kafka brokers configured
	var karmaConsumer *vote.KarmaConsumer
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()

	if cfg.KafkaBrokers != "" {
		brokers := strings.Split(cfg.KafkaBrokers, ",")
		kc, err := vote.NewKarmaConsumer(brokers, rdb, db, logger)
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

// envStr returns the value of an environment variable or a default.
func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
