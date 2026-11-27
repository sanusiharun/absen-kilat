package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/authz"
	"github.com/sanusi/absen-kilat/backend/internal/domain"
)

// ---- fakes ---------------------------------------------------------------

type stubUserFetcher struct {
	user *domain.User
	err  error
}

func (s stubUserFetcher) Get(context.Context, string) (*domain.User, error) {
	return s.user, s.err
}

type stubResolver struct {
	set authz.Set
	err error
}

func (s stubResolver) Effective(context.Context, string, string) (authz.Set, error) {
	return s.set, s.err
}

// ---- helpers -------------------------------------------------------------

// routerFor mounts one admin route behind RequireAuth(off)/Authorize so the
// middleware sees a matched pattern, exactly like production.
func routerFor(pattern, permission string, users userFetcher, res permissionResolver, called *bool) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler { // stand-in for RequireAuth
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					ctx := context.WithValue(req.Context(), ctxKeyUserID, "u1")
					next.ServeHTTP(w, req.WithContext(ctx))
				})
			})
			r.Use(Authorize(users, res))
			r.Get(pattern, func(w http.ResponseWriter, _ *http.Request) {
				if called != nil {
					*called = true
				}
				w.WriteHeader(http.StatusOK)
			})
		})
	})
	return r
}

func do(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// ---- tests ---------------------------------------------------------------

func TestAuthorizeAllowsGrantedPermission(t *testing.T) {
	called := false
	h := routerFor("/admin/users", "menu.pengguna.view",
		stubUserFetcher{user: &domain.User{ID: "u1", Role: domain.RoleHR, IsActive: true}},
		stubResolver{set: authz.Set{"menu.pengguna.view": true}}, &called)

	rec := do(h, "/api/v1/admin/users")
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("expected 200 and handler call, got %d called=%v", rec.Code, called)
	}
}

func TestAuthorizeDeniesMissingPermission(t *testing.T) {
	called := false
	h := routerFor("/admin/users", "menu.pengguna.view",
		stubUserFetcher{user: &domain.User{ID: "u1", Role: domain.RoleSupervisor, IsActive: true}},
		stubResolver{set: authz.Set{}}, &called)

	rec := do(h, "/api/v1/admin/users")
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("expected 403 without handler call, got %d called=%v", rec.Code, called)
	}
}

func TestAuthorizeDeniesUnmappedAdminRoute(t *testing.T) {
	called := false
	h := routerFor("/admin/secret-thing", "",
		stubUserFetcher{user: &domain.User{ID: "u1", Role: domain.RoleSuperAdmin, IsActive: true}},
		stubResolver{set: authz.Set{"menu.pengguna.view": true}}, &called)

	rec := do(h, "/api/v1/admin/secret-thing")
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("unmapped admin route must be denied, got %d called=%v", rec.Code, called)
	}
}

func TestAuthorizeDeniesInactiveUser(t *testing.T) {
	called := false
	h := routerFor("/admin/users", "menu.pengguna.view",
		stubUserFetcher{user: &domain.User{ID: "u1", Role: domain.RoleHR, IsActive: false}},
		stubResolver{set: authz.Set{"menu.pengguna.view": true}}, &called)

	rec := do(h, "/api/v1/admin/users")
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("inactive user must be denied, got %d called=%v", rec.Code, called)
	}
}

func TestAuthorizePassesNonAdminRoutesThrough(t *testing.T) {
	called := false
	h := routerFor("/me", "",
		stubUserFetcher{user: &domain.User{ID: "u1", Role: domain.RoleEmployee, IsActive: true}},
		stubResolver{set: authz.Set{}}, &called)

	rec := do(h, "/api/v1/me")
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("non-admin route must pass through, got %d called=%v", rec.Code, called)
	}
}
