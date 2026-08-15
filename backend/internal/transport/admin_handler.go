package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// adminService is the subset of service.AdminService used by handlers.
type adminService interface {
	TodayStats(ctx context.Context, loc *time.Location) (*service.TodayStats, error)
	ListUsers(ctx context.Context, search, status string) ([]domain.User, error)
	UpdateUser(ctx context.Context, targetID, actorID string, up service.UserUpdate) (*domain.User, error)
	Trends(ctx context.Context, loc *time.Location, days int) ([]service.DayTrend, error)
	Reports(ctx context.Context, loc *time.Location, days int) ([]service.DayReport, error)
	Locations(ctx context.Context, limit int) ([]domain.LocationStat, error)
	ListAttendances(ctx context.Context, f service.AttendanceFilter) ([]service.AdminAttendance, int, error)
	CreateUser(ctx context.Context, email, password, name string, role domain.UserRole) (*domain.User, error)
	Devices(ctx context.Context) ([]domain.DeviceStat, error)
}

// AdminHandler wires admin-only endpoints onto a router.
type AdminHandler struct {
	svc adminService
}

// NewAdminHandler creates an AdminHandler.
func NewAdminHandler(svc adminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// RegisterRoutes mounts admin routes. The caller is expected to guard them with
// RequireAuth + RequireAdmin.
func (h *AdminHandler) RegisterRoutes(r chi.Router) {
	r.Get("/admin/stats", h.stats)
	r.Get("/admin/users", h.listUsers)
	r.Post("/admin/users", h.createUser)
	r.Patch("/admin/users/{id}", h.updateUser)
	r.Get("/admin/trends", h.trends)
	r.Get("/admin/locations", h.locations)
	r.Get("/admin/reports", h.reports)
	r.Get("/admin/attendance", h.attendance)
	r.Get("/admin/devices", h.devices)
}

func (h *AdminHandler) stats(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.TodayStats(r.Context(), time.Local)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	total := st.TotalUsers
	httpx.WriteOK(w, map[string]any{
		"total_users": total,
		"present":     st.Present,
		"late":        st.Late,
		"absent":      st.Absent,
		"present_pct": pct(st.Present, total),
		"late_pct":    pct(st.Late, total),
		"absent_pct":  pct(st.Absent, total),
	})
}

// adminUserResponse is the admin-safe projection of a domain.User.
type adminUserResponse struct {
	ID              string `json:"id"`
	Email           string `json:"email"`
	Name            string `json:"name"`
	AvatarURL       string `json:"avatar_url"`
	Role            string `json:"role"`
	IsActive        bool   `json:"is_active"`
	IsEmailVerified bool   `json:"is_email_verified"`
	CreatedAt       string `json:"created_at"`
}

func toAdminUserResponse(u *domain.User) adminUserResponse {
	return adminUserResponse{
		ID:              u.ID,
		Email:           u.Email,
		Name:            u.Name,
		AvatarURL:       u.AvatarPath,
		Role:            string(u.Role),
		IsActive:        u.IsActive,
		IsEmailVerified: u.IsEmailVerified,
		CreatedAt:       u.CreatedAt.Format(time.RFC3339),
	}
}

func (h *AdminHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("search")
	status := q.Get("status")
	if status != "" && status != "active" && status != "inactive" {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "status must be active or inactive"))
		return
	}
	users, err := h.svc.ListUsers(r.Context(), search, status)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	resp := make([]adminUserResponse, 0, len(users))
	for i := range users {
		resp = append(resp, toAdminUserResponse(&users[i]))
	}
	httpx.WriteOK(w, map[string]any{"users": resp})
}

func (h *AdminHandler) updateUser(w http.ResponseWriter, r *http.Request) {
	actorID := UserIDFromContext(r.Context())
	if actorID == "" {
		httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")

	var body struct {
		Role     *string `json:"role"`
		IsActive *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}
	if body.Role == nil && body.IsActive == nil {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "provide role and/or is_active"))
		return
	}

	var up service.UserUpdate
	if body.Role != nil {
		switch domain.UserRole(*body.Role) {
		case domain.RoleAdmin, domain.RoleEmployee:
			role := domain.UserRole(*body.Role)
			up.Role = &role
		default:
			httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "role must be admin or employee"))
			return
		}
	}
	if body.IsActive != nil {
		up.IsActive = body.IsActive
	}

	u, err := h.svc.UpdateUser(r.Context(), targetID, actorID, up)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"user": toAdminUserResponse(u)})
}

func (h *AdminHandler) trends(w http.ResponseWriter, r *http.Request) {
	days := parseDays(r, 7)
	trends, err := h.svc.Trends(r.Context(), time.Local, days)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	resp := make([]map[string]any, 0, len(trends))
	for _, t := range trends {
		resp = append(resp, map[string]any{
			"date":    t.Date,
			"present": t.Present,
			"late":    t.Late,
			"absent":  t.Absent,
		})
	}
	httpx.WriteOK(w, map[string]any{"trends": resp})
}

func (h *AdminHandler) reports(w http.ResponseWriter, r *http.Request) {
	days := parseDays(r, 14)
	reports, err := h.svc.Reports(r.Context(), time.Local, days)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	resp := make([]map[string]any, 0, len(reports))
	for _, rep := range reports {
		resp = append(resp, map[string]any{
			"date":    rep.Date,
			"present": rep.Present,
			"late":    rep.Late,
			"absent":  rep.Absent,
			"total":   rep.Total,
		})
	}
	httpx.WriteOK(w, map[string]any{"reports": resp})
}

func (h *AdminHandler) locations(w http.ResponseWriter, r *http.Request) {
	limit := 5
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	locs, err := h.svc.Locations(r.Context(), limit)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	resp := make([]map[string]any, 0, len(locs))
	for _, l := range locs {
		resp = append(resp, map[string]any{"location": l.Location, "count": l.Count})
	}
	httpx.WriteOK(w, map[string]any{"locations": resp})
}

func (h *AdminHandler) attendance(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	status := q.Get("status")
	if status != "" && status != string(domain.StatusOnTime) && status != string(domain.StatusLate) {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "status must be on_time or late"))
		return
	}
	atype := q.Get("type")
	if atype != "" && atype != string(domain.AttendanceCheckIn) && atype != string(domain.AttendanceCheckOut) {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "type must be check_in or check_out"))
		return
	}
	limit := 100
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}
	offset := 0
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			offset = n
		}
	}

	list, total, err := h.svc.ListAttendances(r.Context(), service.AttendanceFilter{
		From:   q.Get("from"),
		To:     q.Get("to"),
		Status: status,
		Type:   atype,
		Search: q.Get("search"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		h.ttpxErr(w, err)
		return
	}

	resp := make([]map[string]any, 0, len(list))
	for _, a := range list {
		resp = append(resp, map[string]any{
			"id":          a.ID,
			"user_id":     a.UserID,
			"user_name":   a.UserName,
			"user_email":  a.UserEmail,
			"type":        string(a.Type),
			"status":      string(a.Status),
			"photo_url":   a.PhotoURL,
			"address":     a.Address,
			"latitude":    a.Latitude,
			"longitude":   a.Longitude,
			"device":      a.Device,
			"ip_address":  a.IPAddress,
			"distance_m":  a.DistanceM,
			"occurred_at": a.OccurredAt.Format(time.RFC3339),
		})
	}
	httpx.WriteOK(w, map[string]any{"attendances": resp, "total": total})
}

func (h *AdminHandler) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	role := domain.UserRole(body.Role)
	if body.Role != "" && role != domain.RoleAdmin && role != domain.RoleEmployee {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "role must be admin or employee"))
		return
	}

	u, err := h.svc.CreateUser(r.Context(), body.Email, body.Password, body.Name, role)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteCreated(w, map[string]any{"user": toAdminUserResponse(u)})
}

func (h *AdminHandler) devices(w http.ResponseWriter, r *http.Request) {
	devs, err := h.svc.Devices(r.Context())
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	resp := make([]map[string]any, 0, len(devs))
	for _, d := range devs {
		resp = append(resp, map[string]any{"device": d.Device, "count": d.Count})
	}
	httpx.WriteOK(w, map[string]any{"devices": resp})
}

// parseDays reads the ?days= query parameter, clamped to a sane range, falling
// back to def when absent or invalid.
func parseDays(r *http.Request, def int) int {
	v := r.URL.Query().Get("days")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	if n > 365 {
		n = 365
	}
	return n
}

// ttpxErr routes an error to the shared error envelope writer.
func (h *AdminHandler) ttpxErr(w http.ResponseWriter, err error) {
	httpx.WriteError(w, err)
}

func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}
