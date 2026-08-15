// Command api runs the Absen Kilat HTTP API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/sanusi/absen-kilat/backend/internal/config"
	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/internal/storage"
	"github.com/sanusi/absen-kilat/backend/internal/transport"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

func main() {
	cfg := config.Load()

	logger := newLogger(cfg.Env)
	slog.SetDefault(logger)

	// Open database (applies migrations on first run).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		logger.Error("database init failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("database ready", "path", cfg.DBPath)

	// GORM handle for ORM-backed repositories (e.g. notifications).
	gdb, err := db.OpenGORM(cfg.DBPath)
	if err != nil {
		logger.Error("gorm init failed", "error", err)
		os.Exit(1)
	}

	// Wire repositories and services.
	userRepo := repo.NewUserRepository(pool)
	attendanceRepo := repo.NewAttendanceRepository(pool)
	locationRepo := repo.NewLocationRepository(pool)
	settingsRepo := repo.NewSettingsRepository(pool)
	notificationRepo := repo.NewNotificationRepository(gdb)
	settingsSvc := service.NewSettingsService(settingsRepo)
	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret)
	attendanceSvc := service.NewAttendanceService(attendanceRepo, locationRepo, settingsSvc, cfg.CutoffHour, service.OfficeConfig{
		Lat:     cfg.OfficeLat,
		Lng:     cfg.OfficeLng,
		RadiusM: cfg.AbsenRadiusM,
	})
	profileSvc := service.NewProfileService(userRepo)
	notificationSvc := service.NewNotificationService(notificationRepo)
	adminSvc := service.NewAdminService(userRepo, attendanceRepo, locationRepo, settingsSvc, cfg.AdminEmail)
	appSvc := service.NewAppService(settingsSvc, locationRepo)
	geocodeSvc := service.NewGeocodeService()

	// Seed the initial admin account when configured.
	if cfg.AdminPassword != "" {
		if err := adminSvc.SeedAdmin(ctx, cfg.AdminPassword); err != nil {
			logger.Error("seed admin failed", "error", err)
		} else {
			logger.Info("admin account ensured", "email", cfg.AdminEmail)
		}
	}

	// photoStore adapts storage.Store to transport.photoStorer.
	photoStore := &photoStoreAdapter{Dir: cfg.UploadDir}

	authHandler := transport.NewAuthHandler(authSvc)
	attendanceHandler := transport.NewAttendanceHandler(attendanceSvc, photoStore)
	profileHandler := transport.NewProfileHandler(profileSvc, photoStore, authSvc)
	notificationHandler := transport.NewNotificationHandler(notificationSvc)
	adminHandler := transport.NewAdminHandler(adminSvc)
	appInfoHandler := transport.NewAppInfoHandler(appSvc)
	geocodeHandler := transport.NewGeocodeHandler(geocodeSvc)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(transport.RequestLogger(logger))
	r.Use(transport.CORSMiddleware)

	// Unknown routes respond with the contract error envelope.
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteHTTPError(w, httpx.ErrNotFound)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed"))
	})

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteOK(w, map[string]string{"status": "ok"})
	})

	// Serve uploaded photos (avatars & attendance photos) as static files.
	// Files are stored under cfg.UploadDir as "<year>/<month>/<uuid>.<ext>"
	// and are exposed publicly at /uploads/<path>.
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadDir))))

	r.Route("/api/v1", func(r chi.Router) {
		authHandler.RegisterRoutes(r)
		r.Group(func(r chi.Router) {
			r.Use(transport.RequireAuth(authSvc))
			attendanceHandler.RegisterRoutes(r)
			profileHandler.RegisterRoutes(r)
			notificationHandler.RegisterRoutes(r)
			geocodeHandler.RegisterRoutes(r)
			appInfoHandler.RegisterRoutes(r)

			// Admin-only routes: auth + role check.
			r.Group(func(r chi.Router) {
				r.Use(transport.RequireAdmin(profileSvc))
				adminHandler.RegisterRoutes(r)
			})
		})
	})

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		logger.Info("server listening", "addr", addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

// photoStoreAdapter adapts storage.Store to transport.photoStorer.
type photoStoreAdapter struct {
	Dir string
}

// Store saves an uploaded photo and returns its relative path.
func (a *photoStoreAdapter) Store(file multipart.File, header *multipart.FileHeader) (string, error) {
	return storage.Store(a.Dir, file, header)
}

// newLogger builds a structured slog logger: JSON in production, text in development.
func newLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "development" {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}
	if env == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
