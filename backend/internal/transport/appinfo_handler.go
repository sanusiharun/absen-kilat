package transport

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// appInfoService is the subset of service.AppService used by handlers.
type appInfoService interface {
	PublicSettings(ctx context.Context) (*service.PublicSettings, error)
	ActiveLocations(ctx context.Context) ([]domain.Location, error)
}

// AppInfoHandler wires the authenticated mobile endpoints for system info
// (settings subset + active location list for the "absen dimana" picker).
type AppInfoHandler struct {
	svc appInfoService
}

// NewAppInfoHandler creates an AppInfoHandler.
func NewAppInfoHandler(svc appInfoService) *AppInfoHandler {
	return &AppInfoHandler{svc: svc}
}

// RegisterRoutes mounts mobile info routes under an authenticated router.
func (h *AppInfoHandler) RegisterRoutes(r chi.Router) {
	r.Get("/settings/public", h.publicSettings)
	r.Get("/locations", h.locations)
}

func (h *AppInfoHandler) publicSettings(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.PublicSettings(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"settings": st})
}

func (h *AppInfoHandler) locations(w http.ResponseWriter, r *http.Request) {
	locs, err := h.svc.ActiveLocations(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	resp := make([]map[string]any, 0, len(locs))
	for i := range locs {
		resp = append(resp, map[string]any{
			"id":   locs[i].ID,
			"name": locs[i].Name,
			"type": locs[i].Type,
		})
	}
	httpx.WriteOK(w, map[string]any{"locations": resp})
}
