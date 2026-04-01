// Package observability provides Prometheus metrics and OpenTelemetry tracing
// initialization for gRPC services.
package observability

import (
	"net/http"
	"os"
	"strconv"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Metrics holds the gRPC Prometheus metrics.
type Metrics struct {
	serverMetrics *grpc_prometheus.ServerMetrics
}

// InitMetrics creates Prometheus metrics for gRPC and starts the /metrics HTTP server.
// Returns metrics interceptors to add to the gRPC server.
func InitMetrics(logger *zap.Logger) (*Metrics, error) {
	// Get metrics port from env (default 9090)
	metricsPort := 9090
	if p := os.Getenv("METRICS_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			metricsPort = parsed
		}
	}

	// Create server metrics with histogram
	serverMetrics := grpc_prometheus.NewServerMetrics()
	serverMetrics.EnableHandlingTimeHistogram()

	// Register with default Prometheus registry
	prometheus.MustRegister(serverMetrics)

	// Start HTTP server for Prometheus scraping
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	go func() {
		addr := ":" + strconv.Itoa(metricsPort)
		logger.Info("starting metrics server", zap.String("addr", addr))
		if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", zap.Error(err))
		}
	}()

	return &Metrics{serverMetrics: serverMetrics}, nil
}

// UnaryInterceptor returns the Prometheus unary server interceptor.
func (m *Metrics) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return m.serverMetrics.UnaryServerInterceptor()
}

// StreamInterceptor returns the Prometheus stream server interceptor.
func (m *Metrics) StreamInterceptor() grpc.StreamServerInterceptor {
	return m.serverMetrics.StreamServerInterceptor()
}
