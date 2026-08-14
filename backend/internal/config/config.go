// Package config loads application configuration from environment variables.
package config

import (
	"os"
	"strconv"
)

// Config holds runtime configuration for the API server.
type Config struct {
	// Env is the application environment: "development" or "production".
	Env string
	// Port is the TCP port the HTTP server listens on.
	Port string
	// DBPath is the filesystem path to the SQLite database file.
	DBPath string
	// CutoffHour is the local hour (0-23) after which check-ins are "late".
	// Interpreted in the server's local timezone.
	CutoffHour int
	// JWTSecret is the secret used to sign and verify access tokens.
	JWTSecret string
	// UploadDir is the directory where uploaded photos are stored.
	UploadDir string
	// PublicBaseURL is used to build absolute URLs for uploaded files.
	PublicBaseURL string
}

// Load reads configuration from environment variables, applying defaults
// suitable for local development.
func Load() Config {
	return Config{
		Env:           getenv("APP_ENV", "development"),
		Port:          getenv("PORT", "8090"),
		DBPath:        getenv("DB_PATH", "data/absen-kilat.db"),
		CutoffHour:    getenvInt("CUTOFF_HOUR", 9),
		JWTSecret:     getenv("JWT_SECRET", "dev-secret-change-me"),
		UploadDir:     getenv("UPLOAD_DIR", "uploads"),
		PublicBaseURL: getenv("PUBLIC_BASE_URL", "http://localhost:8090"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
