package transport

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// geocoder is the subset of service.GeocodeService used by handlers.
type geocoder interface {
	Reverse(ctx context.Context, lat, lng float64) (*service.GeocodeResult, error)
}

// GeocodeHandler wires the reverse-geocoding endpoint onto a router.
type GeocodeHandler struct {
	svc geocoder
}

// NewGeocodeHandler creates a GeocodeHandler.
func NewGeocodeHandler(svc geocoder) *GeocodeHandler {
	return &GeocodeHandler{svc: svc}
}

// RegisterRoutes mounts geocode routes under an authenticated router.
func (h *GeocodeHandler) RegisterRoutes(r chi.Router) {
	r.Get("/geocode", h.reverse)
}

func (h *GeocodeHandler) reverse(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat, err := strconv.ParseFloat(q.Get("lat"), 64)
	if err != nil || lat < -90 || lat > 90 {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "lat must be a valid latitude"))
		return
	}
	lng, err := strconv.ParseFloat(q.Get("lng"), 64)
	if err != nil || lng < -180 || lng > 180 {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "lng must be a valid longitude"))
		return
	}

	res, err := h.svc.Reverse(r.Context(), lat, lng)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]string{
		"address":      res.Address,
		"short_address": res.ShortAddress,
	})
}
