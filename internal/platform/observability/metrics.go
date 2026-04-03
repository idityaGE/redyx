// Package observability provides Prometheus metrics and OpenTelemetry tracing
// initialization for gRPC services.
package observability

import (
	"net/http"
	"os"
	"strconv"
	"sync"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Metrics holds the gRPC Prometheus metrics.
type Metrics struct {
	serverMetrics *grpc_prometheus.ServerMetrics
	registry      *prometheus.Registry
}

var (
	metricsOnce sync.Once
	metricsInst *Metrics
	metricsErr  error
)

// InitMetrics creates Prometheus metrics for gRPC and starts the /metrics HTTP server.
// Returns metrics interceptors to add to the gRPC server.
// This function is safe to call multiple times - it will only initialize once.
func InitMetrics(logger *zap.Logger) (*Metrics, error) {
	metricsOnce.Do(func() {
		metricsInst, metricsErr = initMetricsInternal(logger)
	})
	return metricsInst, metricsErr
}

func initMetricsInternal(logger *zap.Logger) (*Metrics, error) {
	// Get metrics port from env (default 9090)
	metricsPort := 9090
	if p := os.Getenv("METRICS_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			metricsPort = parsed
		}
	}

	// Create a custom registry to avoid conflicts with default registry
	// which may have metrics auto-registered by go-grpc-prometheus init()
	registry := prometheus.NewRegistry()

	// Register standard Go metrics
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	// Create server metrics with histogram
	serverMetrics := grpc_prometheus.NewServerMetrics()
	serverMetrics.EnableHandlingTimeHistogram()

	// Register with our custom registry (not the default one)
	registry.MustRegister(serverMetrics)

	// Start HTTP server for Prometheus scraping
	mux := http.NewServeMux()
	// Use HandlerFor with our custom registry instead of the default Handler()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
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

	return &Metrics{
		serverMetrics: serverMetrics,
		registry:      registry,
	}, nil
}

// UnaryInterceptor returns the Prometheus unary server interceptor.
func (m *Metrics) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return m.serverMetrics.UnaryServerInterceptor()
}

// StreamInterceptor returns the Prometheus stream server interceptor.
func (m *Metrics) StreamInterceptor() grpc.StreamServerInterceptor {
	return m.serverMetrics.StreamServerInterceptor()
}
