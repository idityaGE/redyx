package ratelimit

import (
	"context"
	"fmt"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/redyx/redyx/internal/platform/auth"
)

// UnaryInterceptor returns a gRPC unary interceptor that enforces
// global request rate limits based on user tier. If enabled is false,
// the interceptor is a no-op passthrough.
//
// Tier determination: nil claims = "anonymous", otherwise "authenticated".
// Trusted tier is deferred to a future implementation.
//
// Action-specific limits (post/comment/vote) should be called from
// individual service handlers using Limiter.Check directly.
func UnaryInterceptor(limiter *Limiter, enabled bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !enabled {
			return handler(ctx, req)
		}

		// Determine tier and identifier from auth claims
		tier := "anonymous"
		identifier := clientIP(ctx)

		if claims := auth.ClaimsFromContext(ctx); claims != nil {
			tier = "authenticated"
			identifier = claims.UserID
		}

		// Look up tier limits
		tc, ok := TierLimits[tier]
		if !ok {
			// Unknown tier — allow through
			return handler(ctx, req)
		}

		// Build rate limit key
		key := fmt.Sprintf("rl:%s:%s", tier, identifier)

		result, err := limiter.Check(ctx, key, tc.Limit, tc.WindowSec)
		if err != nil {
			// On Redis error, allow through (fail open) to avoid blocking all requests
			return handler(ctx, req)
		}

		if !result.Allowed {
			retryAfterSec := int(result.RetryAfter.Seconds())
			if retryAfterSec < 1 {
				retryAfterSec = 1
			}

			// Set retry-after as trailing metadata
			trailer := metadata.Pairs("retry-after", strconv.Itoa(retryAfterSec))
			if err := grpc.SetTrailer(ctx, trailer); err != nil {
				// Best effort — still return rate limit error
				_ = err
			}

			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}

		return handler(ctx, req)
	}
}

// clientIP extracts the client IP address from gRPC peer info.
func clientIP(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return p.Addr.String()
	}
	return "unknown"
}
