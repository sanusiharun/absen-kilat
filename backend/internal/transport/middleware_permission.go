package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/authz"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// permissionResolver resolves a user's effective permission set.
type permissionResolver interface {
	Effective(ctx context.Context, userID, role string) (authz.Set, error)
}

// Authorize enforces the route -> permission table (authz.RouteRule).
//
// It must be mounted inside the same route group as the admin routes so chi has
// already matched a pattern when it runs. An admin route WITHOUT a mapping is
// denied: forgetting to register a permission must never open an endpoint.
func Authorize(fetch userFetcher, resolver permissionResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := UserIDFromContext(r.Context())
			if userID == "" {
				httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
				return
			}

			pattern := chi.RouteContext(r.Context()).RoutePattern()
			permission, mapped := authz.PermissionFor(r.Method, pattern)
			if !mapped {
				if authz.IsAdminPattern(pattern) {
					slog.Warn("authz: unmapped admin route denied", "method", r.Method, "pattern", pattern)
					httpx.WriteHTTPError(w, httpx.ErrForbidden)
					return
				}
				// Non-admin routes are guarded by RequireAuth/RequireRole only.
				next.ServeHTTP(w, r)
				return
			}

			u, err := fetch.Get(r.Context(), userID)
			if err != nil || u == nil || !u.IsActive {
				httpx.WriteHTTPError(w, httpx.ErrForbidden)
				return
			}

			set, err := resolver.Effective(r.Context(), userID, string(u.Role))
			if err != nil {
				slog.Error("authz: resolve permissions failed", "error", err, "user_id", userID)
				httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusInternalServerError, "INTERNAL", "internal error"))
				return
			}
			if !set.Has(permission) {
				httpx.WriteHTTPError(w, httpx.ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
