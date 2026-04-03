// Package middleware provides gRPC server interceptors for logging,
// panic recovery, and domain error mapping.
package middleware

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Logging returns a gRPC UnaryServerInterceptor that logs every RPC call
// with method name, duration, status code, and trace context (if present)
// using structured zap logging.
func Logging(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == "/grpc.health.v1.Health/Check" {
			return handler(ctx, req)
		}

		start := time.Now()

		// Extract trace context if present
		spanCtx := trace.SpanContextFromContext(ctx)
		fields := []zap.Field{
			zap.String("method", info.FullMethod),
		}
		if spanCtx.IsValid() {
			fields = append(fields,
				zap.String("trace_id", spanCtx.TraceID().String()),
				zap.String("span_id", spanCtx.SpanID().String()),
			)
		}

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		st, _ := status.FromError(err)
		code := st.Code()

		// Add duration and status to fields
		fields = append(fields,
			zap.Duration("duration", duration),
			zap.String("code", code.String()),
		)

		switch {
		case err == nil:
			logger.Info("rpc completed", fields...)
		case code >= 1 && code <= 11:
			// Client errors (codes 1-11 cover most client-side issues)
			logger.Warn("rpc client error", append(fields, zap.Error(err))...)
		default:
			logger.Error("rpc server error", append(fields, zap.Error(err))...)
		}

		return resp, err
	}
}
