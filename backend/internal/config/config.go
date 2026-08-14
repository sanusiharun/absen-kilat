// Package config loads application configuration from environment variables.
package config

import (
	"os"
)

// Config holds runtime configuration for the API server.
type Config struct {
	// Env is the application environment: "development" or "production".
	Env string
	// Port is the TCP port the HTTP server listens on.
	Port string
}

// Load reads configuration from environment variables, applying defaults
// suitable for local development.
func Load() Config {
	return Config{
		Env:  getenv("APP_ENV", "development"),
		Port: getenv("PORT", "8090"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
