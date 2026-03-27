package middleware

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	perrors "github.com/redyx/redyx/internal/platform/errors"
)

// ErrorMapping returns a gRPC UnaryServerInterceptor that maps domain errors
// to proper gRPC status codes. Envoy then transcodes these to HTTP status codes:
//   - ErrNotFound       → codes.NotFound       (HTTP 404)
//   - ErrAlreadyExists  → codes.AlreadyExists  (HTTP 409)
//   - ErrForbidden      → codes.PermissionDenied (HTTP 403)
//   - ErrInvalidInput   → codes.InvalidArgument (HTTP 400)
//   - ErrUnauthenticated→ codes.Unauthenticated (HTTP 401)
//   - ErrRateLimited    → codes.ResourceExhausted (HTTP 429)
//   - Any other error   → codes.Internal with sanitized message (HTTP 500)
//
// If the error is already a gRPC status, it is passed through unchanged.
func ErrorMapping() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		// Already a gRPC status — pass through
		if _, ok := status.FromError(err); ok {
			return resp, err
		}

		// Map domain errors to gRPC codes
		switch {
		case errors.Is(err, perrors.ErrNotFound):
			return nil, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, perrors.ErrAlreadyExists):
			return nil, status.Error(codes.AlreadyExists, err.Error())
		case errors.Is(err, perrors.ErrForbidden):
			return nil, status.Error(codes.PermissionDenied, err.Error())
		case errors.Is(err, perrors.ErrInvalidInput):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, perrors.ErrUnauthenticated):
			return nil, status.Error(codes.Unauthenticated, err.Error())
		case errors.Is(err, perrors.ErrRateLimited):
			return nil, status.Error(codes.ResourceExhausted, err.Error())
		default:
			// Never leak raw errors to clients (Pitfall 11)
			return nil, status.Error(codes.Internal, "internal error")
		}
	}
}
