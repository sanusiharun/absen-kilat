package transport

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/authz"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// AccessHandler exposes the caller's effective permissions so the admin shell
// can render its navigation from the server instead of a hardcoded list.
type AccessHandler struct {
	resolver *authz.Resolver
	users    userFetcher
}

// NewAccessHandler wires the handler.
func NewAccessHandler(resolver *authz.Resolver, users userFetcher) *AccessHandler {
	return &AccessHandler{resolver: resolver, users: users}
}

// RegisterRoutes mounts the endpoints available to every authenticated user.
func (h *AccessHandler) RegisterRoutes(r chi.Router) {
	r.Get("/me/permissions", h.myPermissions)
}

type menuAccess struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Href    string   `json:"href"`
	Actions []string `json:"actions"`
}

func (h *AccessHandler) myPermissions(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
		return
	}

	u, err := h.users.Get(r.Context(), userID)
	if err != nil || u == nil || !u.IsActive {
		httpx.WriteHTTPError(w, httpx.ErrForbidden)
		return
	}

	set, err := h.resolver.Effective(r.Context(), userID, string(u.Role))
	if err != nil {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusInternalServerError, "INTERNAL", "internal error"))
		return
	}

	menus := make([]menuAccess, 0, len(authz.Menus()))
	for _, m := range authz.Menus() {
		if !set.Has(authz.Permission(m.Key, authz.ActionView)) {
			continue
		}
		actions := make([]string, 0, len(m.Actions))
		for _, a := range m.Actions {
			if set.Has(authz.Permission(m.Key, a)) {
				actions = append(actions, string(a))
			}
		}
		menus = append(menus, menuAccess{Key: m.Key, Label: m.Label, Href: m.Href, Actions: actions})
	}

	httpx.WriteOK(w, map[string]any{
		"role":        string(u.Role),
		"permissions": set.Sorted(),
		"menus":       menus,
	})
}
