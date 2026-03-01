// Package config provides environment-based configuration loading.
// Each service calls Load with its service name; environment variables
// override sensible defaults for local development.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// Config holds the common configuration for every service.
type Config struct {
	GRPCPort    int
	DatabaseURL string
	RedisURL    string
	ServiceName string
}

// Load reads configuration from environment variables with sensible defaults.
// serviceName is used to derive the default database name.
func Load(serviceName string) *Config {
	cfg := &Config{
		GRPCPort:    envInt("GRPC_PORT", 50051),
		DatabaseURL: envStr("DATABASE_URL", fmt.Sprintf("postgres://redyx:dev@localhost:5432/%s?sslmode=disable", serviceName)),
		RedisURL:    envStr("REDIS_URL", "redis://localhost:6379/0"),
		ServiceName: serviceName,
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:errcheck

	logger.Info("loaded config",
		zap.String("service", cfg.ServiceName),
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.String("database_url", redactURL(cfg.DatabaseURL)),
		zap.String("redis_url", cfg.RedisURL),
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
