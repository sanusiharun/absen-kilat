package transport

import (
	"context"
	"net/http"
	"strings"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// ctxKey is an unexported key type for request-scoped values.
type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyUserRole
)

// UserIDFromContext returns the authenticated user id stored by RequireAuth,
// or "" when absent.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyUserID).(string)
	return v
}

// tokenParser extracts the subject user id from a bearer token.
type tokenParser interface {
	ParseAccessToken(token string) (string, error)
}

// RequireAuth returns middleware that validates the Authorization bearer
// token and stores the user id in the request context.
func RequireAuth(parser tokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
				return
			}
			token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			if token == "" {
				httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
				return
			}

			userID, err := parser.ParseAccessToken(token)
			if err != nil {
				httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// userFetcher loads a full user (with role) from the datastore.
type userFetcher interface {
	Get(ctx context.Context, userID string) (*domain.User, error)
}

// RequireAdmin returns middleware that runs after RequireAuth and only lets
// active admin accounts through. The role is read from the datastore (not the
// token) so that promotion/revocation takes effect immediately.
func RequireAdmin(fetch userFetcher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := UserIDFromContext(r.Context())
			if userID == "" {
				httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
				return
			}
			u, err := fetch.Get(r.Context(), userID)
			if err != nil || u.Role != domain.RoleAdmin || !u.IsActive {
				httpx.WriteHTTPError(w, httpx.ErrForbidden)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyUserRole, string(domain.RoleAdmin))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserRoleFromContext returns the role stored by RequireAdmin, or "".
func UserRoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyUserRole).(string)
	return v
}
