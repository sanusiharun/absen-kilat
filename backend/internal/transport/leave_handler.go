package transport

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// LeaveHandler exposes leave (cuti) endpoints for employees and approvers.
type LeaveHandler struct {
	leaveSvc   *service.LeaveService
	profileSvc *service.ProfileService
	users      domain.UserRepository
	settings   domain.SettingsRepository
}

// NewLeaveHandler constructs a LeaveHandler. profileSvc is used for the
// RequireRole middleware (it exposes Get(ctx, id)).
func NewLeaveHandler(leaveSvc *service.LeaveService, profileSvc *service.ProfileService, users domain.UserRepository, settings domain.SettingsRepository) *LeaveHandler {
	return &LeaveHandler{leaveSvc: leaveSvc, profileSvc: profileSvc, users: users, settings: settings}
}

// RegisterRoutes mounts employee-facing leave endpoints. The router r must
// already enforce RequireAuth (it is mounted inside the authed group).
func (h *LeaveHandler) RegisterRoutes(r chi.Router) {
	r.Get("/leave/types", h.listTypes)
	r.Route("/leave", func(r chi.Router) {
		r.Post("/", h.apply)
		r.Get("/", h.myLeaves)
		r.Get("/{id}", h.getLeave)
		r.Delete("/{id}", h.cancel)
	})
}

// RegisterAdminRoutes mounts approver-facing leave endpoints. The router r must
// already enforce RequireRole for supervisors/hr/super_admin.
func (h *LeaveHandler) RegisterAdminRoutes(r chi.Router) {
	r.Get("/admin/leave", h.listAll)
	r.Get("/admin/leave/pending", h.listPending)
	r.Post("/admin/leave/{id}/approve", h.approve)
	r.Post("/admin/leave/{id}/reject", h.reject)
}

type applyLeaveRequest struct {
	LeaveType  string `json:"leave_type"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	Reason     string `json:"reason"`
	ExtraDays  int    `json:"extra_days"`
	Attachment string `json:"attachment"`
}

func (h *LeaveHandler) apply(w http.ResponseWriter, r *http.Request) {
	uid := UserIDFromContext(r.Context())
	var body applyLeaveRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, pkgerrors.Validation("body", "format JSON tidak valid"))
		return
	}
	body.LeaveType = strings.TrimSpace(body.LeaveType)
	if body.LeaveType == "" {
		httpx.WriteError(w, pkgerrors.Validation("leave_type", "tipe cuti wajib diisi"))
		return
	}
	l, err := h.leaveSvc.Apply(r.Context(), service.LeaveInput{
		UserID:    uid,
		TypeCode:  body.LeaveType,
		StartDate: body.StartDate,
		EndDate:   body.EndDate,
		Reason:    body.Reason,
		ExtraDays: body.ExtraDays,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, leaveToResponse(l))
}

func (h *LeaveHandler) myLeaves(w http.ResponseWriter, r *http.Request) {
	uid := UserIDFromContext(r.Context())
	list, err := h.leaveSvc.List(r.Context(), "", uid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"leaves": leavesToResponse(list)})
}

func (h *LeaveHandler) listTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.leaveSvc.LeaveTypes(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"leave_types": leaveTypesToResponse(types)})
}

func (h *LeaveHandler) getLeave(w http.ResponseWriter, r *http.Request) {
	uid := UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	l, err := h.leaveSvc.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if l.UserID != uid {
		httpx.WriteError(w, pkgerrors.Forbidden("akses ditolak"))
		return
	}
	httpx.WriteOK(w, map[string]any{"leave": leaveToResponse(l)})
}

func (h *LeaveHandler) cancel(w http.ResponseWriter, r *http.Request) {
	uid := UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	l, err := h.leaveSvc.Cancel(r.Context(), id, uid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"leave": leaveToResponse(l)})
}

func (h *LeaveHandler) listAll(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	list, err := h.leaveSvc.List(r.Context(), status, "")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"leaves": leavesToResponse(list)})
}

func (h *LeaveHandler) listPending(w http.ResponseWriter, r *http.Request) {
	list, err := h.leaveSvc.List(r.Context(), string(domain.LeavePending), "")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"leaves": leavesToResponse(list)})
}

func (h *LeaveHandler) approve(w http.ResponseWriter, r *http.Request) {
	actorID := UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	l, err := h.leaveSvc.Approve(r.Context(), id, actorID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"leave": leaveToResponse(l)})
}

func (h *LeaveHandler) reject(w http.ResponseWriter, r *http.Request) {
	actorID := UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if strings.TrimSpace(body.Reason) == "" {
		httpx.WriteError(w, pkgerrors.Validation("reason", "alasan penolakan wajib diisi"))
		return
	}
	l, err := h.leaveSvc.Reject(r.Context(), id, actorID, body.Reason)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"leave": leaveToResponse(l)})
}

func leaveToResponse(l *domain.Leave) map[string]any {
	resp := map[string]any{
		"id":          l.ID,
		"user_id":     l.UserID,
		"leave_type":  l.LeaveType,
		"start_date":  l.StartDate,
		"end_date":    l.EndDate,
		"total_days":  l.TotalDays,
		"reason":      l.Reason,
		"status":      string(l.Status),
		"created_at":  l.CreatedAt,
	}
	if l.ApprovedBy != "" {
		resp["approved_by"] = l.ApprovedBy
	}
	if l.ApprovedAt != nil {
		resp["approved_at"] = *l.ApprovedAt
	}
	if l.RejectionReason != "" {
		resp["rejection_reason"] = l.RejectionReason
	}
	if l.AttachmentPath != "" {
		resp["attachment_url"] = "/uploads/" + l.AttachmentPath
	}
	return resp
}

func leavesToResponse(list []domain.Leave) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for i := range list {
		out = append(out, leaveToResponse(&list[i]))
	}
	return out
}

func leaveTypesToResponse(list []domain.LeaveType) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, t := range list {
		out = append(out, map[string]any{
			"code":          t.Code,
			"name":          t.Name,
			"default_days":  t.DefaultDays,
			"max_extra_days": t.MaxExtraDays,
		})
	}
	return out
}
