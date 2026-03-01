// Package skeleton implements the skeleton service, a minimal gRPC service
// that proves the platform libraries work together via a health check endpoint.
package skeleton

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	healthv1 "github.com/redyx/redyx/gen/redyx/health/v1"
)

// Server implements the HealthServiceServer interface from the generated proto code.
type Server struct {
	healthv1.UnimplementedHealthServiceServer
	db      *pgxpool.Pool
	rdb     *goredis.Client
	logger  *zap.Logger
	startAt time.Time
}

// NewServer creates a new skeleton server with database and Redis clients.
func NewServer(db *pgxpool.Pool, rdb *goredis.Client, logger *zap.Logger) *Server {
	return &Server{
		db:      db,
		rdb:     rdb,
		logger:  logger,
		startAt: time.Now(),
	}
}

// Check returns the health status of the skeleton service by pinging
// PostgreSQL and Redis. If both are reachable, status is SERVING.
func (s *Server) Check(ctx context.Context, req *healthv1.CheckRequest) (*healthv1.CheckResponse, error) {
	resp := &healthv1.CheckResponse{
		Version: "0.1.0",
	}

	// Check PostgreSQL connectivity
	if err := s.db.Ping(ctx); err != nil {
		s.logger.Warn("postgres health check failed", zap.Error(err))
		resp.Status = healthv1.CheckResponse_SERVING_STATUS_NOT_SERVING
		return resp, nil
	}

	// Check Redis connectivity
	if err := s.rdb.Ping(ctx).Err(); err != nil {
		s.logger.Warn("redis health check failed", zap.Error(err))
		resp.Status = healthv1.CheckResponse_SERVING_STATUS_NOT_SERVING
		return resp, nil
	}

	resp.Status = healthv1.CheckResponse_SERVING_STATUS_SERVING
	return resp, nil
}
