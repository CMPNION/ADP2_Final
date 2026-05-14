package app

import (
	"fmt"
	"os"
)

type Config struct {
	HTTPAddr           string
	MetricsAddr        string
	InventoryGRPCAddr  string
	AuthToken          string
	JWTSecret          string
	RateLimitPerMinute int
}

func LoadConfig() Config {
	return Config{
		HTTPAddr:           envOr("HTTP_ADDR", ":8080"),
		MetricsAddr:        envOr("METRICS_ADDR", ":9095"),
		InventoryGRPCAddr:  envOr("INVENTORY_GRPC_ADDR", "127.0.0.1:50051"),
		AuthToken:          os.Getenv("AUTH_TOKEN"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		RateLimitPerMinute: envIntOr("RATE_LIMIT_PER_MINUTE", 120),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
