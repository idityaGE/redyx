// Package grpcserver provides a shared gRPC server bootstrap with health checking,
// reflection, interceptor chaining, and graceful shutdown on SIGTERM.
// Every service's cmd/*/main.go calls New() to create the server.
package grpcserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Server wraps a grpc.Server with health checking and graceful shutdown.
type Server struct {
	srv    *grpc.Server
	health *health.Server
	port   int
	logger *zap.Logger
}

type serverConfig struct {
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	serverOptions      []grpc.ServerOption
}

// Option configures the gRPC server.
type Option func(*serverConfig)

// WithUnaryInterceptors appends unary interceptors to the chain.
func WithUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) Option {
	return func(c *serverConfig) {
		c.unaryInterceptors = append(c.unaryInterceptors, interceptors...)
	}
}

// WithStreamInterceptors appends stream interceptors to the chain.
func WithStreamInterceptors(interceptors ...grpc.StreamServerInterceptor) Option {
	return func(c *serverConfig) {
		c.streamInterceptors = append(c.streamInterceptors, interceptors...)
	}
}

// WithServerOptions appends raw gRPC server options (e.g., stats handler for tracing).
func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(c *serverConfig) {
		c.serverOptions = append(c.serverOptions, opts...)
	}
}

// New creates a gRPC server with health checking, reflection, and the provided
// interceptors. Initial health status is set to SERVING.
func New(port int, logger *zap.Logger, opts ...Option) *Server {
	cfg := &serverConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	var serverOpts []grpc.ServerOption
	// Add custom server options (like stats handler for tracing)
	serverOpts = append(serverOpts, cfg.serverOptions...)
	if len(cfg.unaryInterceptors) > 0 {
		serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(cfg.unaryInterceptors...))
	}
	if len(cfg.streamInterceptors) > 0 {
		serverOpts = append(serverOpts, grpc.ChainStreamInterceptor(cfg.streamInterceptors...))
	}

	srv := grpc.NewServer(serverOpts...)

	hs := health.NewServer()
	healthpb.RegisterHealthServer(srv, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	reflection.Register(srv)

	return &Server{srv: srv, health: hs, port: port, logger: logger}
}

// Server returns the underlying grpc.Server for service registration.
func (s *Server) Server() *grpc.Server { return s.srv }

// SetServingStatus sets the health status for a specific service name.
func (s *Server) SetServingStatus(service string, status healthpb.HealthCheckResponse_ServingStatus) {
	s.health.SetServingStatus(service, status)
}

// Run starts the gRPC server and blocks until SIGTERM or SIGINT is received.
// On shutdown, it sets health to NOT_SERVING and calls GracefulStop.
func (s *Server) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		s.logger.Info("shutting down gRPC server")
		s.health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		s.srv.GracefulStop()
	}()

	s.logger.Info("starting gRPC server", zap.Int("port", s.port))
	return s.srv.Serve(lis)
}
