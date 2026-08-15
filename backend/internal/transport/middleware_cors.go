// CORS middleware for the token-based Absen Kilat API.
package transport

import (
	"net/http"
)

// CORSMiddleware enables cross-origin requests. The web client (Expo dev
// server) and phone clients hit the API from a different origin than the API
// itself, so browsers send a CORS preflight (OPTIONS) before cross-origin
// requests. Without handling it, preflights fall through to chi's
// MethodNotAllowed handler and return 405, blocking the request.
//
// The API authenticates with bearer tokens (no cookies), so credentials mode
// is off and a wildcard/reflected origin is safe.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Reflect the requesting origin when present; otherwise allow all
		// (curl, native apps, same-origin calls that send no Origin header).
		allowOrigin := "*"
		if origin != "" {
			allowOrigin = origin
		}

		h := w.Header()
		h.Set("Access-Control-Allow-Origin", allowOrigin)
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
		h.Set("Access-Control-Max-Age", "86400")
		// Required when reflecting Origin so caches key on the origin.
		h.Add("Vary", "Origin")

		// Short-circuit preflight requests.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
