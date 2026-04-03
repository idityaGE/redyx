package observability

import (
	"context"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Tracer holds the OpenTelemetry tracer provider.
type Tracer struct {
	tp *sdktrace.TracerProvider
}

// InitTracing initializes OpenTelemetry tracing with OTLP gRPC exporter.
// Reads OTEL_EXPORTER_OTLP_ENDPOINT and OTEL_SERVICE_NAME from environment.
// Returns nil without error if OTEL_EXPORTER_OTLP_ENDPOINT is not set (optional tracing).
func InitTracing(ctx context.Context, logger *zap.Logger) (*Tracer, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		logger.Info("OTEL_EXPORTER_OTLP_ENDPOINT not set, tracing disabled")
		return nil, nil
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "unknown-service"
	}

	// Create OTLP exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(), // Local dev, no TLS
	)
	if err != nil {
		return nil, err
	}

	// Create resource with service name
	// Use NewWithAttributes without SchemaURL to avoid version conflicts with resource.Default()
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			attribute.String("service.name", serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Info("tracing initialized", zap.String("endpoint", endpoint), zap.String("service", serviceName))
	return &Tracer{tp: tp}, nil
}

// Shutdown flushes and shuts down the tracer provider.
func (t *Tracer) Shutdown(ctx context.Context) error {
	if t == nil || t.tp == nil {
		return nil
	}
	return t.tp.Shutdown(ctx)
}

// StatsHandler returns the OTEL gRPC stats handler for tracing.
func StatsHandler() grpc.ServerOption {
	return grpc.StatsHandler(otelgrpc.NewServerHandler())
}
