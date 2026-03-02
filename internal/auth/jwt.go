package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims are the custom JWT claims for token creation.
// This is separate from internal/platform/auth.Claims which is for context propagation.
type JWTClaims struct {
	jwt.RegisteredClaims
	UserID   string `json:"uid"`
	Username string `json:"username"`
}

// JWTManager creates JWT access tokens signed with HS256.
type JWTManager struct {
	secret    []byte
	accessTTL time.Duration
}

// NewJWTManager creates a JWTManager for signing tokens.
func NewJWTManager(secret string, accessTTL time.Duration) *JWTManager {
	return &JWTManager{
		secret:    []byte(secret),
		accessTTL: accessTTL,
	}
}

// Generate creates a signed JWT access token for the given user.
// Returns the token string and its expiration time.
func (m *JWTManager) Generate(userID, username string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.accessTTL)

	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "redyx",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		UserID:   userID,
		Username: username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}

	return signed, expiresAt, nil
}
