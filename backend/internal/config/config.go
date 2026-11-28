// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultReuseGrace adalah jendela grace default untuk pemakaian ulang
	// refresh token (C1, identity-refresh-corrective).
	DefaultReuseGrace = 10 * time.Second
	// MaxReuseGrace adalah batas atas grace yang disetujui (C1b).
	MaxReuseGrace = 60 * time.Second
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
	// AdminEmail/AdminPassword seed the initial admin account on startup if
	// no admin exists yet.
	AdminEmail    string
	AdminPassword string
	// OfficeLat/OfficeLng are the office coordinates used for geofencing.
	OfficeLat float64
	// OfficeLng is the office longitude.
	OfficeLng float64
	// ReuseGrace adalah jendela toleransi pemakaian ulang refresh token yang
	// sudah dikonsumsi (0 = ketat, tanpa grace). Dimuat dari REFRESH_REUSE_GRACE.
	ReuseGrace time.Duration
	// reuseGraceErr menyimpan kegagalan parse env grace agar start bisa
	// ditolak eksplisit, bukan diam-diam memakai default (C1b).
	reuseGraceErr error
	// AbsenRadiusM is the maximum distance (meters) from the office at which
	// an attendance is accepted. 0 disables geofencing.
	AbsenRadiusM int
}

// Load reads configuration from environment variables, applying defaults
// suitable for local development.
func Load() Config {
	cfg := Config{
		Env:           getenv("APP_ENV", "development"),
		Port:          getenv("PORT", "8090"),
		DBPath:        getenv("DB_PATH", "data/absen-kilat.db"),
		CutoffHour:    getenvInt("CUTOFF_HOUR", 9),
		JWTSecret:     getenv("JWT_SECRET", "dev-secret-change-me"),
		UploadDir:     getenv("UPLOAD_DIR", "uploads"),
		PublicBaseURL: getenv("PUBLIC_BASE_URL", "http://localhost:8090"),
		AdminEmail:    getenv("ADMIN_EMAIL", "admin@absenkilat.dev"),
		AdminPassword: getenv("ADMIN_PASSWORD", ""),
		OfficeLat:     getenvFloat("OFFICE_LAT", -6.2088),  // Jakarta Selatan
		OfficeLng:     getenvFloat("OFFICE_LNG", 106.8456), // Jakarta Selatan
		AbsenRadiusM:  getenvInt("ABSEN_RADIUS_M", 500),
	}
	cfg.ReuseGrace, cfg.reuseGraceErr = loadReuseGrace()
	return cfg
}

// loadReuseGrace membaca REFRESH_REUSE_GRACE (durasi, mis. "10s"). Alias lama
// REFRESH_REUSE_GRACE_SECONDS (detik) masih diterima agar deploy yang sudah
// memakainya tidak diam-diam kembali ke default; variabel baru menang.
func loadReuseGrace() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv("REFRESH_REUSE_GRACE"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("REFRESH_REUSE_GRACE_SECONDS"))
		if raw == "" {
			return DefaultReuseGrace, nil
		}
		secs, err := strconv.Atoi(raw)
		if err != nil {
			return 0, fmt.Errorf("REFRESH_REUSE_GRACE_SECONDS: nilai %q bukan bilangan bulat detik", raw)
		}
		return time.Duration(secs) * time.Second, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("REFRESH_REUSE_GRACE: nilai %q bukan durasi valid (mis. 10s)", raw)
	}
	return d, nil
}

// ValidateIdentity memvalidasi setelan identity yang harus fail-closed saat
// start (C1b): grace negatif, tak bisa diparse, atau di atas batas disetujui
// membuat proses menolak start, bukan memakai nilai diam-diam.
func (c Config) ValidateIdentity() error {
	if c.reuseGraceErr != nil {
		return c.reuseGraceErr
	}
	if c.ReuseGrace < 0 || c.ReuseGrace > MaxReuseGrace {
		return fmt.Errorf("REFRESH_REUSE_GRACE: %s di luar batas 0s..%s", c.ReuseGrace, MaxReuseGrace)
	}
	return nil
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

func getenvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}
