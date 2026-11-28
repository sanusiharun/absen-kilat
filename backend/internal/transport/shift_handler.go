package transport

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// shiftService is the subset of service.ShiftService used by the handler.
type shiftService interface {
	ListShifts(ctx context.Context, all bool) ([]domain.Shift, error)
	CreateShift(ctx context.Context, in service.ShiftInput) (*domain.Shift, error)
	UpdateShift(ctx context.Context, id string, p service.ShiftPatch) (*domain.Shift, error)
}

// ShiftHandler exposes the admin shift master-data endpoints.
type ShiftHandler struct {
	svc shiftService
}

// NewShiftHandler creates a ShiftHandler.
func NewShiftHandler(svc shiftService) *ShiftHandler { return &ShiftHandler{svc: svc} }

// RegisterRoutes mounts the shift routes. The caller is expected to guard them
// with RequireAuth + RequireRole + the menu permission middleware.
func (h *ShiftHandler) RegisterRoutes(r chi.Router) {
	r.Get("/admin/shifts", h.list)
	r.Post("/admin/shifts", h.create)
	r.Patch("/admin/shifts/{id}", h.update)
}

// shiftResponse is the explicit API projection (never the domain struct).
type shiftResponse struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	CrossesMidnight bool   `json:"crosses_midnight"`
	GraceMinutes    int    `json:"grace_minutes"`
	BreakMinutes    int    `json:"break_minutes"`
	LocationID      string `json:"location_id"`
	Active          bool   `json:"active"`
}

func toShiftResponse(s *domain.Shift) shiftResponse {
	return shiftResponse{
		ID:              s.ID,
		Code:            s.Code,
		Name:            s.Name,
		StartTime:       s.StartTime,
		EndTime:         s.EndTime,
		CrossesMidnight: s.CrossesMidnight,
		GraceMinutes:    s.GraceMinutes,
		BreakMinutes:    s.BreakMinutes,
		LocationID:      s.LocationID,
		Active:          s.Active,
	}
}

func (h *ShiftHandler) list(w http.ResponseWriter, r *http.Request) {
	all := false
	if v := r.URL.Query().Get("all"); v != "" {
		switch v {
		case "true":
			all = true
		case "false":
			all = false
		default:
			httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "all must be true or false"))
			return
		}
	}
	shifts, err := h.svc.ListShifts(r.Context(), all)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]shiftResponse, 0, len(shifts))
	for i := range shifts {
		out = append(out, toShiftResponse(&shifts[i]))
	}
	httpx.WriteOK(w, map[string]any{"shifts": out})
}

func (h *ShiftHandler) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code         string  `json:"code"`
		Name         string  `json:"name"`
		StartTime    string  `json:"start_time"`
		EndTime      string  `json:"end_time"`
		GraceMinutes *int    `json:"grace_minutes"`
		BreakMinutes *int    `json:"break_minutes"`
		LocationID   *string `json:"location_id"`
		Active       *bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}
	sh, err := h.svc.CreateShift(r.Context(), service.ShiftInput{
		Code:         body.Code,
		Name:         body.Name,
		StartTime:    body.StartTime,
		EndTime:      body.EndTime,
		GraceMinutes: body.GraceMinutes,
		BreakMinutes: body.BreakMinutes,
		LocationID:   body.LocationID,
		Active:       body.Active,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, map[string]any{"shift": toShiftResponse(sh)})
}

func (h *ShiftHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Code         *string `json:"code"`
		Name         *string `json:"name"`
		StartTime    *string `json:"start_time"`
		EndTime      *string `json:"end_time"`
		GraceMinutes *int    `json:"grace_minutes"`
		BreakMinutes *int    `json:"break_minutes"`
		LocationID   *string `json:"location_id"`
		Active       *bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}
	sh, err := h.svc.UpdateShift(r.Context(), id, service.ShiftPatch{
		Code:         body.Code,
		Name:         body.Name,
		StartTime:    body.StartTime,
		EndTime:      body.EndTime,
		GraceMinutes: body.GraceMinutes,
		BreakMinutes: body.BreakMinutes,
		LocationID:   body.LocationID,
		Active:       body.Active,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"shift": toShiftResponse(sh)})
}
