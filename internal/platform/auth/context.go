// Package auth provides JWT validation and gRPC interceptor for authentication.
// Claims are injected into context via WithClaims and extracted with ClaimsFromContext.
package auth

import "context"

// Claims holds the user identity extracted from a validated JWT.
type Claims struct {
	UserID   string
	Username string
}

// contextKey is an unexported type to prevent collisions with keys in other packages.
type contextKey struct{}

// WithClaims returns a new context carrying the given claims.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, contextKey{}, claims)
}

// ClaimsFromContext extracts the claims from the context.
// Returns nil if the request is anonymous (no valid JWT).
func ClaimsFromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(contextKey{}).(*Claims)
	return c
}

// UserIDFromContext is a convenience helper that returns the user ID
// from context, or an empty string if the request is anonymous.
func UserIDFromContext(ctx context.Context) string {
	if c := ClaimsFromContext(ctx); c != nil {
		return c.UserID
	}
	return ""
}
