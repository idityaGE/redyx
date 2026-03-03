package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// jwtClaims is the internal JWT claims struct used for parsing.
type jwtClaims struct {
	jwt.RegisteredClaims
	UserID   string `json:"uid"`
	Username string `json:"username"`
}

// JWTValidator validates JWT tokens using HS256 and a shared secret.
type JWTValidator struct {
	parser *jwt.Parser
	secret []byte
}

// NewJWTValidator creates a validator that checks HS256 tokens with
// issuer "redyx" and requires expiration.
func NewJWTValidator(secret string) *JWTValidator {
	return &JWTValidator{
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithIssuer("redyx"),
			jwt.WithExpirationRequired(),
		),
		secret: []byte(secret),
	}
}

// Validate parses and validates a JWT token string, returning the extracted claims.
func (v *JWTValidator) Validate(tokenString string) (*Claims, error) {
	token, err := v.parser.ParseWithClaims(tokenString, &jwtClaims{}, func(token *jwt.Token) (any, error) {
		return v.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return &Claims{
		UserID:   claims.UserID,
		Username: claims.Username,
	}, nil
}

// publicMethods lists gRPC full method names that allow anonymous access.
// Format: /package.ServiceName/MethodName
var publicMethods = map[string]bool{
	// Auth service — unauthenticated flows
	"/redyx.auth.v1.AuthService/Register":      true,
	"/redyx.auth.v1.AuthService/Login":         true,
	"/redyx.auth.v1.AuthService/VerifyOTP":     true,
	"/redyx.auth.v1.AuthService/ResetPassword": true,
	"/redyx.auth.v1.AuthService/GoogleOAuth":   true,
	"/redyx.auth.v1.AuthService/RefreshToken":  true,
	// User service — public profile viewing
	"/redyx.user.v1.UserService/GetProfile":      true,
	"/redyx.user.v1.UserService/GetUserPosts":    true,
	"/redyx.user.v1.UserService/GetUserComments": true,
	// Community service — public browsing
	"/redyx.community.v1.CommunityService/GetCommunity":    true,
	"/redyx.community.v1.CommunityService/ListCommunities": true,
	"/redyx.community.v1.CommunityService/ListMembers":     true,
	// Post service — public browsing
	"/redyx.post.v1.PostService/GetPost":   true,
	"/redyx.post.v1.PostService/ListPosts": true,
	// Health check
	"/grpc.health.v1.Health/Check": true,
}

// UnaryInterceptor returns a gRPC unary interceptor that validates JWT tokens
// from the "authorization" metadata header. Public methods proceed without auth.
// Protected methods without a valid token return codes.Unauthenticated.
func UnaryInterceptor(validator *JWTValidator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Public methods allow anonymous access
		if publicMethods[info.FullMethod] {
			// Still try to extract claims for optional auth (e.g., rate limit tier)
			if token := extractToken(ctx); token != "" {
				if claims, err := validator.Validate(token); err == nil {
					ctx = WithClaims(ctx, claims)
				}
			}
			return handler(ctx, req)
		}

		// Protected method — require valid auth
		token := extractToken(ctx)
		if token == "" {
			return nil, status.Error(codes.Unauthenticated, "missing authorization token")
		}

		claims, err := validator.Validate(token)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid authorization token: %v", err)
		}

		ctx = WithClaims(ctx, claims)
		return handler(ctx, req)
	}
}

// extractToken gets the bearer token from gRPC metadata "authorization" header.
func extractToken(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return ""
	}

	auth := values[0]
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return auth
}
