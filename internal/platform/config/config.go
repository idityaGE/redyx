// Package config provides environment-based configuration loading.
// Each service calls Load with its service name; environment variables
// override sensible defaults for local development.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Config holds the common configuration for every service.
type Config struct {
	GRPCPort    int
	DatabaseURL string
	RedisURL    string
	ServiceName string

	// Auth fields (Phase 2)
	JWTSecret          string
	JWTAccessTTL       time.Duration
	JWTRefreshTTL      time.Duration
	OTPTTLMinutes      int
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// Rate limit fields
	RateLimitEnabled bool
}

// Load reads configuration from environment variables with sensible defaults.
// serviceName is used to derive the default database name.
func Load(serviceName string) *Config {
	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:errcheck

	jwtSecret := envStr("JWT_SECRET", "dev-secret-change-in-production")
	if jwtSecret == "dev-secret-change-in-production" {
		logger.Warn("using default JWT_SECRET — set JWT_SECRET env var in production")
	}

	cfg := &Config{
		GRPCPort:    envInt("GRPC_PORT", 50051),
		DatabaseURL: envStr("DATABASE_URL", fmt.Sprintf("postgres://redyx:dev@localhost:5432/%s?sslmode=disable", serviceName)),
		RedisURL:    envStr("REDIS_URL", "redis://localhost:6379/0"),
		ServiceName: serviceName,

		// Auth
		JWTSecret:          jwtSecret,
		JWTAccessTTL:       envDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:      envDuration("JWT_REFRESH_TTL", 168*time.Hour),
		OTPTTLMinutes:      envInt("OTP_TTL_MINUTES", 5),
		GoogleClientID:     envStr("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: envStr("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  envStr("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback"),

		// Rate limit
		RateLimitEnabled: envBool("RATE_LIMIT_ENABLED", true),
	}

	logger.Info("loaded config",
		zap.String("service", cfg.ServiceName),
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.String("database_url", redactURL(cfg.DatabaseURL)),
		zap.String("redis_url", cfg.RedisURL),
		zap.Duration("jwt_access_ttl", cfg.JWTAccessTTL),
		zap.Duration("jwt_refresh_ttl", cfg.JWTRefreshTTL),
		zap.Bool("rate_limit_enabled", cfg.RateLimitEnabled),
	)

	return cfg
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(v) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

// redactURL replaces the password portion of a database URL for safe logging.
func redactURL(url string) string {
	// Pattern: postgres://user:password@host
	if idx := strings.Index(url, "://"); idx != -1 {
		rest := url[idx+3:]
		if atIdx := strings.Index(rest, "@"); atIdx != -1 {
			if colonIdx := strings.Index(rest[:atIdx], ":"); colonIdx != -1 {
				return url[:idx+3] + rest[:colonIdx] + ":****@" + rest[atIdx+1:]
			}
		}
	}
	return url
}
