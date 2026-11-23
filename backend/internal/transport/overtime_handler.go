package transport

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// OvertimeHandler exposes lembur endpoints for employees and approvers.
type OvertimeHandler struct {
	svc *service.OvertimeService
}

// NewOvertimeHandler constructs the handler.
func NewOvertimeHandler(svc *service.OvertimeService) *OvertimeHandler {
	return &OvertimeHandler{svc: svc}
}

// RegisterRoutes mounts employee-facing endpoints (inside the authed group).
func (h *OvertimeHandler) RegisterRoutes(r chi.Router) {
	r.Route("/overtime", func(r chi.Router) {
		r.Post("/", h.submit)
		r.Get("/", h.myOvertime)
		r.Get("/summary", h.summary)
		r.Delete("/{id}", h.cancel)
	})
}

// RegisterAdminRoutes mounts approver-facing endpoints. The router must already
// enforce RequireRole for supervisors/hr/super_admin.
func (h *OvertimeHandler) RegisterAdminRoutes(r chi.Router) {
	r.Get("/admin/overtime", h.listAll)
	r.Get("/admin/overtime/pending", h.listPending)
	r.Post("/admin/overtime/{id}/approve", h.approve)
	r.Post("/admin/overtime/{id}/reject", h.reject)
}

type submitOvertimeRequest struct {
	WorkDate string `json:"work_date"`
	StartAt  string `json:"start_at"`
	EndAt    string `json:"end_at"`
	Reason   string `json:"reason"`
}

func (h *OvertimeHandler) submit(w http.ResponseWriter, r *http.Request) {
	var req submitOvertimeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	o, err := h.svc.Submit(r.Context(), service.SubmitInput{
		UserID:   UserIDFromContext(r.Context()),
		WorkDate: req.WorkDate,
		StartAt:  req.StartAt,
		EndAt:    req.EndAt,
		Reason:   req.Reason,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, map[string]any{"overtime": overtimeToResponse(o)})
}

func (h *OvertimeHandler) myOvertime(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), r.URL.Query().Get("status"), UserIDFromContext(r.Context()))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"overtimes": overtimeToResponseList(list)})
}

func (h *OvertimeHandler) summary(w http.ResponseWriter, r *http.Request) {
	month := strings.TrimSpace(r.URL.Query().Get("month"))
	minutes, err := h.svc.MonthlyMinutes(r.Context(), UserIDFromContext(r.Context()), month)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"month": month, "approved_minutes": minutes})
}

func (h *OvertimeHandler) cancel(w http.ResponseWriter, r *http.Request) {
	o, err := h.svc.Cancel(r.Context(), chi.URLParam(r, "id"), UserIDFromContext(r.Context()))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"overtime": overtimeToResponse(o)})
}

func (h *OvertimeHandler) listAll(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), r.URL.Query().Get("status"), r.URL.Query().Get("user_id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"overtimes": overtimeToResponseList(list)})
}

func (h *OvertimeHandler) listPending(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), "pending", "")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"overtimes": overtimeToResponseList(list)})
}

func (h *OvertimeHandler) approve(w http.ResponseWriter, r *http.Request) {
	o, err := h.svc.Approve(r.Context(), chi.URLParam(r, "id"), UserIDFromContext(r.Context()))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"overtime": overtimeToResponse(o)})
}

type rejectOvertimeRequest struct {
	Reason string `json:"reason"`
}

func (h *OvertimeHandler) reject(w http.ResponseWriter, r *http.Request) {
	var req rejectOvertimeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	o, err := h.svc.Reject(r.Context(), chi.URLParam(r, "id"), UserIDFromContext(r.Context()), req.Reason)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"overtime": overtimeToResponse(o)})
}

func overtimeToResponseList(list []domain.Overtime) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for i := range list {
		out = append(out, overtimeToResponse(&list[i]))
	}
	return out
}

func overtimeToResponse(o *domain.Overtime) map[string]any {
	resp := map[string]any{
		"id":         o.ID,
		"user_id":    o.UserID,
		"work_date":  o.WorkDate,
		"start_at":   o.StartAt,
		"end_at":     o.EndAt,
		"minutes":    o.Minutes,
		"reason":     o.Reason,
		"status":     string(o.Status),
		"created_at": o.CreatedAt,
	}
	if o.UserName != "" {
		resp["user_name"] = o.UserName
	}
	if o.UserEmail != "" {
		resp["user_email"] = o.UserEmail
	}
	if o.ApprovedBy != "" {
		resp["approved_by"] = o.ApprovedBy
	}
	if o.ApprovedAt != nil {
		resp["approved_at"] = *o.ApprovedAt
	}
	if o.RejectionReason != "" {
		resp["rejection_reason"] = o.RejectionReason
	}
	return resp
}
