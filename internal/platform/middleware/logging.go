// Package middleware provides gRPC server interceptors for logging,
// panic recovery, and domain error mapping.
package middleware

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Logging returns a gRPC UnaryServerInterceptor that logs every RPC call
// with method name, duration, and status code using structured zap logging.
func Logging(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		st, _ := status.FromError(err)
		code := st.Code()

		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.Duration("duration", duration),
			zap.String("code", code.String()),
		}

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
