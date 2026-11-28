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
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/sanusi/absen-kilat/backend/internal/authz"
	"github.com/sanusi/absen-kilat/backend/internal/config"
	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/domain"
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

	// C1b: setelan identity divalidasi saat start; nilai tak valid menolak start.
	if verr := cfg.ValidateIdentity(); verr != nil {
		logger.Error("invalid identity configuration", "error", verr)
		os.Exit(1)
	}

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
	qrTokenRepo := repo.NewQrTokenRepository(pool)
	settingsSvc := service.NewSettingsService(settingsRepo)
	refreshRepo := repo.NewRefreshTokenRepository(pool)
	// Jendela grace pemakaian ulang token (C1), sudah divalidasi di config.
	refreshRepo.ReuseGrace = cfg.ReuseGrace
	authSvc := service.NewAuthServiceWithTokens(userRepo, cfg.JWTSecret, refreshRepo)
	authSvc.LoadLegacyRefreshCutoff(os.Getenv)
	attendanceSvc := service.NewAttendanceService(attendanceRepo, locationRepo, settingsSvc, cfg.CutoffHour, service.OfficeConfig{
		Lat:     cfg.OfficeLat,
		Lng:     cfg.OfficeLng,
		RadiusM: cfg.AbsenRadiusM,
	})
	profileSvc := service.NewProfileService(userRepo)
	notificationSvc := service.NewNotificationService(notificationRepo)
	adminSvc := service.NewAdminService(userRepo, attendanceRepo, locationRepo, settingsSvc, cfg.AdminEmail)
	qrSvc := service.NewQrService(qrTokenRepo, locationRepo, userRepo, attendanceRepo)
	appSvc := service.NewAppService(settingsSvc, locationRepo)
	geocodeSvc := service.NewGeocodeService()

	// Leave (cuti) module. Workdays come from settings (["mon".."sun"]).
	leaveRepo := repo.NewLeaveRepository(pool)
	leaveTypeRepo := repo.NewLeaveTypeRepository(pool)
	leaveWorkdays := parseWorkdays(settingsSvc, ctx)
	leaveSvc := service.NewLeaveService(leaveRepo, leaveTypeRepo, userRepo, leaveWorkdays)
	leaveHandler := transport.NewLeaveHandler(leaveSvc, profileSvc, userRepo, settingsRepo)

	// Overtime (lembur) module: records approved time only; payroll converts it.
	overtimeRepo := repo.NewOvertimeRepository(pool)
	overtimeSvc := service.NewOvertimeService(overtimeRepo)
	overtimeHandler := transport.NewOvertimeHandler(overtimeSvc)

	// Mitra (partner crew) + Proyek/Kapal + Penugasan master data.
	partnerRepo := repo.NewPartnerRepository(pool)
	projectRepo := repo.NewProjectRepository(pool)
	assignmentRepo := repo.NewAssignmentRepository(pool)
	projectSvc := service.NewProjectService(partnerRepo, projectRepo, assignmentRepo, userRepo)
	projectHandler := transport.NewProjectHandler(projectSvc)

	// Access control: menu registry lives in code (internal/authz), assignments
	// in the database (role template + per-user overrides).
	accessRepo := repo.NewAccessRepository(pool)
	accessResolver := authz.NewResolver(accessRepo)
	if seeded, err := accessRepo.SeedDefaults(ctx); err != nil {
		logger.Error("access control seed failed", "error", err)
	} else if seeded {
		logger.Info("access control templates seeded")
	}

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
	qrHandler := transport.NewQrHandler(qrSvc, photoStore)
	appInfoHandler := transport.NewAppInfoHandler(appSvc)
	geocodeHandler := transport.NewGeocodeHandler(geocodeSvc)
	accessSvc := service.NewAccessService(accessRepo, userRepo, accessResolver)
	accessHandler := transport.NewAccessHandler(accessResolver, profileSvc, accessSvc)

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
		// Public QR attendance: no auth (token + rate limit + dedup guard
		// the endpoint). Must stay OUTSIDE the RequireAuth group.
		qrHandler.RegisterPublicRoutes(r)

		r.Group(func(r chi.Router) {
			r.Use(transport.RequireAuth(authSvc))
			attendanceHandler.RegisterRoutes(r)
			profileHandler.RegisterRoutes(r)
			notificationHandler.RegisterRoutes(r)
			geocodeHandler.RegisterRoutes(r)
			appInfoHandler.RegisterRoutes(r)
			leaveHandler.RegisterRoutes(r)
			overtimeHandler.RegisterRoutes(r)
			projectHandler.RegisterRoutes(r)
			accessHandler.RegisterRoutes(r)

			// Admin dashboard: auth + role check (super_admin | hr), then the
			// fine-grained menu permission for each route.
			r.Group(func(r chi.Router) {
				r.Use(transport.RequireRole(profileSvc, domain.RoleSuperAdmin, domain.RoleHR))
				r.Use(transport.Authorize(profileSvc, accessResolver))
				adminHandler.RegisterRoutes(r)
				adminHandler.RegisterApprovalRoutes(r)
				qrHandler.RegisterRoutes(r)
				leaveHandler.RegisterAdminRoutes(r)
				overtimeHandler.RegisterAdminRoutes(r)
				projectHandler.RegisterAdminRoutes(r)
				accessHandler.RegisterAdminRoutes(r)
			})

			// Approval queue: supervisors see/decide their own team;
			// super_admin and hr see everything.
			r.Group(func(r chi.Router) {
				r.Use(transport.RequireRole(profileSvc, domain.RoleSupervisor, domain.RoleSuperAdmin, domain.RoleHR))
				r.Use(transport.Authorize(profileSvc, accessResolver))
				adminHandler.RegisterApprovalRoutes(r)
				overtimeHandler.RegisterAdminRoutes(r)
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

// parseWorkdays reads working weekdays from settings (["mon".."sun"]) and maps
// them to time.Weekday values. Falls back to Mon-Fri when unset/invalid.
func parseWorkdays(s *service.SettingsService, ctx context.Context) []time.Weekday {
	def := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	st, err := s.Get(ctx)
	if err != nil || len(st.Workdays) == 0 {
		return def
	}
	lookup := map[string]time.Weekday{
		"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
		"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
	}
	out := make([]time.Weekday, 0, len(st.Workdays))
	for _, w := range st.Workdays {
		if d, ok := lookup[strings.ToLower(strings.TrimSpace(w))]; ok {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}
