package transport

import (
	"context"
	"net/http"
	"strings"

	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// ctxKey is an unexported key type for request-scoped values.
type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
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
