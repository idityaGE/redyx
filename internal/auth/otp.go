package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"math/big"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// OTPManager generates and verifies 6-digit OTP codes stored in Redis.
type OTPManager struct {
	rdb *goredis.Client
	ttl time.Duration
}

// NewOTPManager creates an OTPManager backed by Redis with the given TTL.
func NewOTPManager(rdb *goredis.Client, ttl time.Duration) *OTPManager {
	return &OTPManager{
		rdb: rdb,
		ttl: ttl,
	}
}

// Generate creates a 6-digit OTP code using crypto/rand and stores it
// in Redis under the key "otp:{email}" with the configured TTL.
func (m *OTPManager) Generate(ctx context.Context, email string) (string, error) {
	// Generate cryptographically random 6-digit code
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("generate otp: %w", err)
	}
	code := fmt.Sprintf("%06d", n.Int64())

	key := fmt.Sprintf("otp:%s", email)
	if err := m.rdb.Set(ctx, key, code, m.ttl).Err(); err != nil {
		return "", fmt.Errorf("store otp: %w", err)
	}

	return code, nil
}

// Verify checks whether the provided code matches the stored OTP for the email.
// Uses constant-time comparison to prevent timing attacks.
// On success, the OTP is deleted (single-use).
func (m *OTPManager) Verify(ctx context.Context, email, code string) (bool, error) {
	key := fmt.Sprintf("otp:%s", email)

	stored, err := m.rdb.Get(ctx, key).Result()
	if err == goredis.Nil {
		return false, nil // OTP expired or doesn't exist
	}
	if err != nil {
		return false, fmt.Errorf("get otp: %w", err)
	}

	if subtle.ConstantTimeCompare([]byte(stored), []byte(code)) != 1 {
		return false, nil
	}

	// Delete on success — single use
	m.rdb.Del(ctx, key)

	return true, nil
}
