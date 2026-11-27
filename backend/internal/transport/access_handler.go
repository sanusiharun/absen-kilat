package transport

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/authz"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// AccessHandler exposes the caller's effective permissions (so the admin shell
// can render navigation from the server) and the per-user editor used by the
// owner to hand out menus.
type AccessHandler struct {
	resolver *authz.Resolver
	users    userFetcher
	svc      *service.AccessService
}

// NewAccessHandler wires the handler.
func NewAccessHandler(resolver *authz.Resolver, users userFetcher, svc *service.AccessService) *AccessHandler {
	return &AccessHandler{resolver: resolver, users: users, svc: svc}
}

// RegisterRoutes mounts the endpoints available to every authenticated user.
func (h *AccessHandler) RegisterRoutes(r chi.Router) {
	r.Get("/me/permissions", h.myPermissions)
}

// RegisterAdminRoutes mounts the per-user access editor. The caller is expected
// to guard them with RequireAuth + RequireRole + Authorize (see authz routes).
func (h *AccessHandler) RegisterAdminRoutes(r chi.Router) {
	r.Get("/admin/users/{id}/permissions", h.getUserAccess)
	r.Patch("/admin/users/{id}/permissions", h.patchUserAccess)
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

// getUserAccess returns every registry permission with its source for one user.
func (h *AccessHandler) getUserAccess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	access, err := h.svc.ForUser(r.Context(), id)
	if err != nil {
		httpx.WriteHTTPError(w, httpx.ErrNotFound)
		return
	}
	httpx.WriteOK(w, map[string]any{"access": access})
}

// patchUserAccess applies overrides. Body:
//
//	{"changes": [{"permission": "menu.laporan.export", "effect": "allow"}]}
//
// effect "" removes the override (back to inheriting the role).
func (h *AccessHandler) patchUserAccess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var body struct {
		Changes []service.PermissionChange `json:"changes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "body tidak valid"))
		return
	}
	if len(body.Changes) == 0 {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "changes kosong"))
		return
	}

	actorID := UserIDFromContext(r.Context())
	if err := h.svc.Update(r.Context(), actorID, id, body.Changes); err != nil {
		h.ttpxErr(w, err)
		return
	}

	access, err := h.svc.ForUser(r.Context(), id)
	if err != nil {
		httpx.WriteHTTPError(w, httpx.ErrNotFound)
		return
	}
	httpx.WriteOK(w, map[string]any{"access": access})
}

// ttpxErr maps a service error to the API error envelope.
func (h *AccessHandler) ttpxErr(w http.ResponseWriter, err error) {
	httpx.WriteError(w, err)
}
