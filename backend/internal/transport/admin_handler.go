package transport

import (
	"context"
	"encoding/json"
	"log/slog"
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
	UserReports(ctx context.Context, loc *time.Location, days int) ([]service.UserReport, error)
	ExportAttendance(ctx context.Context, loc *time.Location, from, to string) ([]service.ExportRow, error)
	SummaryReports(ctx context.Context, loc *time.Location, from, to string) (*service.ReportSummary, error)
	Locations(ctx context.Context, limit int) ([]domain.LocationStat, error)
	ListAttendances(ctx context.Context, f service.AttendanceFilter) ([]service.AdminAttendance, int, error)
	CreateUser(ctx context.Context, in service.CreateUserInput) (*domain.User, error)
	Devices(ctx context.Context) ([]domain.DeviceStat, error)
	GetSettings(ctx context.Context) (*domain.Settings, error)
	UpdateSettings(ctx context.Context, up service.SettingsUpdate) (*domain.Settings, error)
	ListLocations(ctx context.Context, all bool) ([]domain.Location, error)
	CreateLocation(ctx context.Context, in service.LocationInput) (*domain.Location, error)
	UpdateLocation(ctx context.Context, id string, up service.LocationUpdate) (*domain.Location, error)
	DeleteLocation(ctx context.Context, id string) error
	OverrideAttendance(ctx context.Context, id, actorID string, ov service.OverrideInput) (*domain.Attendance, error)
	Team(ctx context.Context, managerID string) ([]domain.User, error)
	Approvals(ctx context.Context, actorRole domain.UserRole, actorID, status string, limit, offset int) ([]service.ApprovalItem, int, error)
	DecideApproval(ctx context.Context, actorRole domain.UserRole, actorID, attendanceID, action, reason string) (*domain.Attendance, error)
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
	r.Get("/admin/locations", h.listLocations)
	r.Post("/admin/locations", h.createLocation)
	r.Patch("/admin/locations/{id}", h.updateLocation)
	r.Delete("/admin/locations/{id}", h.deleteLocation)
	// Breakdown of attendance addresses (dashboard chart) — kept separate
	// from the locations CRUD list.
	r.Get("/admin/locations/breakdown", h.locationsBreakdown)
	r.Get("/admin/reports", h.reports)
	r.Get("/admin/reports/users", h.userReports)
	r.Get("/admin/reports/export", h.reportsExport)
	r.Get("/admin/reports/summary", h.reportsSummary)
	r.Get("/admin/attendance", h.attendance)
	r.Patch("/admin/attendance/{id}", h.overrideAttendance)
	r.Get("/admin/devices", h.devices)
	r.Get("/admin/settings", h.getSettings)
	r.Put("/admin/settings", h.putSettings)
}

// RegisterApprovalRoutes mounts the approval endpoints (team + approval
// queue). The caller is expected to guard them with RequireAuth +
// RequireRole(supervisor, super_admin, hr).
func (h *AdminHandler) RegisterApprovalRoutes(r chi.Router) {
	r.Get("/admin/team", h.team)
	r.Get("/admin/approvals", h.approvals)
	r.Post("/admin/approvals/{id}", h.decideApproval)
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
	NIP             string `json:"nip"`
	Phone           string `json:"phone"`
	Department      string `json:"department"`
	Position        string `json:"position"`
	Address         string `json:"address"`
	JoinDate        string `json:"join_date"`
	ManagerID       string `json:"manager_id"`
	EmployeeCode    string `json:"employee_code"`
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
		NIP:             u.NIP,
		Phone:           u.Phone,
		Department:      u.Department,
		Position:        u.Position,
		Address:         u.Address,
		JoinDate:        u.JoinDate,
		ManagerID:       u.ManagerID,
		EmployeeCode:    u.EmployeeCode,
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
		Role         *string `json:"role"`
		IsActive     *bool   `json:"is_active"`
		NIP          *string `json:"nip"`
		Phone        *string `json:"phone"`
		Department   *string `json:"department"`
		Position     *string `json:"position"`
		Address      *string `json:"address"`
		JoinDate     *string `json:"join_date"`
		ManagerID    *string `json:"manager_id"`
		EmployeeCode *string `json:"employee_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	var up service.UserUpdate
	if body.Role != nil {
		role := domain.UserRole(*body.Role)
		if !validUserRole(role) {
			httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "role must be super_admin, hr, supervisor, or employee"))
			return
		}
		up.Role = &role
	}
	up.IsActive = body.IsActive
	up.NIP = body.NIP
	up.Phone = body.Phone
	up.Department = body.Department
	up.Position = body.Position
	up.Address = body.Address
	up.JoinDate = body.JoinDate
	up.ManagerID = body.ManagerID
	up.EmployeeCode = body.EmployeeCode

	u, err := h.svc.UpdateUser(r.Context(), targetID, actorID, up)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"user": toAdminUserResponse(u)})
}

func (h *AdminHandler) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string `json:"name"`
		Email        string `json:"email"`
		Password     string `json:"password"`
		Role         string `json:"role"`
		EmployeeCode string `json:"employee_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	role := domain.UserRole(body.Role)
	if body.Role != "" && !validUserRole(role) {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "role must be super_admin, hr, supervisor, or employee"))
		return
	}

	u, err := h.svc.CreateUser(r.Context(), service.CreateUserInput{
		ActorID:      UserIDFromContext(r.Context()),
		Email:        body.Email,
		Password:     body.Password,
		Name:         body.Name,
		Role:         role,
		EmployeeCode: body.EmployeeCode,
	})
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteCreated(w, map[string]any{"user": toAdminUserResponse(u)})
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

// userReports returns the per-user attendance summary ("Laporan per
// Pengguna") for the dashboard. Accepts ?days= (default 14). Each row carries
// present/late/absent days plus total_minutes/total_hours worked in the
// window, all bounded to the last ?days= calendar days.
func (h *AdminHandler) userReports(w http.ResponseWriter, r *http.Request) {
	days := parseDays(r, 14)
	reports, err := h.svc.UserReports(r.Context(), time.Local, days)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"reports": reports, "days": days})
}

// locationsBreakdown returns the top attendance addresses (dashboard chart).
func (h *AdminHandler) locationsBreakdown(w http.ResponseWriter, r *http.Request) {
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
	approval := q.Get("approval")
	if approval != "" && approval != string(domain.ApprovalPending) &&
		approval != string(domain.ApprovalApproved) && approval != string(domain.ApprovalRejected) {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "approval must be pending, approved, or rejected"))
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
		From:           q.Get("from"),
		To:             q.Get("to"),
		Status:         status,
		Type:           atype,
		ApprovalStatus: approval,
		Search:         q.Get("search"),
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		h.ttpxErr(w, err)
		return
	}

	resp := make([]map[string]any, 0, len(list))
	for _, a := range list {
		row := map[string]any{
			"id":               a.ID,
			"user_id":          a.UserID,
			"user_name":        a.UserName,
			"user_email":       a.UserEmail,
			"type":             string(a.Type),
			"status":           string(a.Status),
			"photo_url":        a.PhotoURL,
			"address":          a.Address,
			"latitude":         a.Latitude,
			"longitude":        a.Longitude,
			"device":           a.Device,
			"ip_address":       a.IPAddress,
			"distance_m":       a.DistanceM,
			"source":           string(a.Source),
			"approval_status":  string(a.ApprovalStatus),
			"approved_by":      a.ApprovedBy,
			"rejection_reason": a.RejectionReason,
			"location_id":      a.LocationID,
			"occurred_at":      a.OccurredAt.Format(time.RFC3339),
		}
		if a.ApprovedAt != nil {
			row["approved_at"] = a.ApprovedAt.Format(time.RFC3339)
		}
		resp = append(resp, row)
	}
	httpx.WriteOK(w, map[string]any{"attendances": resp, "total": total})
}

func (h *AdminHandler) overrideAttendance(w http.ResponseWriter, r *http.Request) {
	actorID := UserIDFromContext(r.Context())
	if actorID == "" {
		httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")

	var body struct {
		Type           *string `json:"type"`
		Status         *string `json:"status"` // on_time|late OR approval shorthand pending|approved|rejected
		ApprovalStatus *string `json:"approval_status"`
		OccurredAt     *string `json:"occurred_at"`
		Reason         string  `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	ov := service.OverrideInput{Reason: body.Reason}

	// `status` doubles as attendance status (on_time|late) or, for
	// compatibility with the issue spec, an approval status shorthand.
	if body.Status != nil {
		switch *body.Status {
		case string(domain.StatusOnTime), string(domain.StatusLate):
			st := domain.AttendanceStatus(*body.Status)
			ov.Status = &st
		case string(domain.ApprovalPending), string(domain.ApprovalApproved), string(domain.ApprovalRejected):
			as := domain.ApprovalStatus(*body.Status)
			ov.ApprovalStatus = &as
		default:
			httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "status must be on_time, late, pending, approved, or rejected"))
			return
		}
	}
	if body.ApprovalStatus != nil {
		as := domain.ApprovalStatus(*body.ApprovalStatus)
		if as != domain.ApprovalPending && as != domain.ApprovalApproved && as != domain.ApprovalRejected {
			httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "approval_status must be pending, approved, or rejected"))
			return
		}
		ov.ApprovalStatus = &as
	}
	if body.Type != nil {
		t := domain.AttendanceType(*body.Type)
		if t != domain.AttendanceCheckIn && t != domain.AttendanceCheckOut {
			httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "type must be check_in or check_out"))
			return
		}
		ov.Type = &t
	}
	if body.OccurredAt != nil && *body.OccurredAt != "" {
		t, err := time.Parse(time.RFC3339, *body.OccurredAt)
		if err != nil {
			httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "occurred_at must be RFC3339"))
			return
		}
		ov.OccurredAt = &t
	}

	a, err := h.svc.OverrideAttendance(r.Context(), id, actorID, ov)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"attendance": toAdminAttendanceRow(a)})
}

// toAdminAttendanceRow renders a single attendance for admin responses.
func toAdminAttendanceRow(a *domain.Attendance) map[string]any {
	row := map[string]any{
		"id":               a.ID,
		"user_id":          a.UserID,
		"type":             string(a.Type),
		"status":           string(a.Status),
		"photo_url":        a.PhotoPath,
		"source":           string(a.Source),
		"approval_status":  string(a.ApprovalStatus),
		"approved_by":      a.ApprovedBy,
		"rejection_reason": a.RejectionReason,
		"location_id":      a.LocationID,
		"occurred_at":      a.OccurredAt.In(time.Local).Format(time.RFC3339),
	}
	if a.ApprovedAt != nil {
		row["approved_at"] = a.ApprovedAt.In(time.Local).Format(time.RFC3339)
	}
	return row
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

// ===== Settings =====

func (h *AdminHandler) getSettings(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.GetSettings(r.Context())
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"settings": settingsResponse(st)})
}

func (h *AdminHandler) putSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WorkStart   *string   `json:"work_start"`
		WorkEnd     *string   `json:"work_end"`
		AbsenMode   *string   `json:"absen_mode"`
		Workdays    *[]string `json:"workdays"`
		PhotoPolicy *string   `json:"photo_policy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}
	st, err := h.svc.UpdateSettings(r.Context(), service.SettingsUpdate{
		WorkStart:   body.WorkStart,
		WorkEnd:     body.WorkEnd,
		AbsenMode:   body.AbsenMode,
		Workdays:    body.Workdays,
		PhotoPolicy: body.PhotoPolicy,
	})
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"settings": settingsResponse(st)})
}

func settingsResponse(st *domain.Settings) map[string]any {
	return map[string]any{
		"work_start":   st.WorkStart,
		"work_end":     st.WorkEnd,
		"absen_mode":   st.AbsenMode,
		"workdays":     st.Workdays,
		"photo_policy": st.PhotoPolicy,
	}
}

// ===== Locations CRUD =====

func (h *AdminHandler) listLocations(w http.ResponseWriter, r *http.Request) {
	all := r.URL.Query().Get("all") == "1"
	locs, err := h.svc.ListLocations(r.Context(), all)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"locations": locationResponses(locs)})
}

func (h *AdminHandler) createLocation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string  `json:"name"`
		Type    string  `json:"type"`
		Lat     float64 `json:"lat"`
		Lng     float64 `json:"lng"`
		RadiusM int     `json:"radius_m"`
		Active  *bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}
	active := true
	if body.Active != nil {
		active = *body.Active
	}
	l, err := h.svc.CreateLocation(r.Context(), service.LocationInput{
		Name:    body.Name,
		Type:    body.Type,
		Lat:     body.Lat,
		Lng:     body.Lng,
		RadiusM: body.RadiusM,
		Active:  active,
	})
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteCreated(w, map[string]any{"location": locationResponse(l)})
}

func (h *AdminHandler) updateLocation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Name    *string  `json:"name"`
		Type    *string  `json:"type"`
		Lat     *float64 `json:"lat"`
		Lng     *float64 `json:"lng"`
		RadiusM *int     `json:"radius_m"`
		Active  *bool    `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}
	l, err := h.svc.UpdateLocation(r.Context(), id, service.LocationUpdate{
		Name:    body.Name,
		Type:    body.Type,
		Lat:     body.Lat,
		Lng:     body.Lng,
		RadiusM: body.RadiusM,
		Active:  body.Active,
	})
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"location": locationResponse(l)})
}

func (h *AdminHandler) deleteLocation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteLocation(r.Context(), id); err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"deleted": true})
}

func locationResponses(locs []domain.Location) []map[string]any {
	out := make([]map[string]any, 0, len(locs))
	for i := range locs {
		out = append(out, locationResponse(&locs[i]))
	}
	return out
}

func locationResponse(l *domain.Location) map[string]any {
	return map[string]any{
		"id":         l.ID,
		"name":       l.Name,
		"type":       l.Type,
		"lat":        l.Lat,
		"lng":        l.Lng,
		"radius_m":   l.RadiusM,
		"active":     l.Active,
		"created_at": l.CreatedAt.Format(time.RFC3339),
	}
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

// validUserRole reports whether r is one of the four accepted roles.
func validUserRole(r domain.UserRole) bool {
	switch r {
	case domain.RoleSuperAdmin, domain.RoleHR, domain.RoleSupervisor, domain.RoleEmployee:
		return true
	}
	return false
}

// ===== Approval queue (issue #17) =====

func (h *AdminHandler) team(w http.ResponseWriter, r *http.Request) {
	actorID := UserIDFromContext(r.Context())
	if actorID == "" {
		httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
		return
	}
	users, err := h.svc.Team(r.Context(), actorID)
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

func (h *AdminHandler) approvals(w http.ResponseWriter, r *http.Request) {
	actorID := UserIDFromContext(r.Context())
	if actorID == "" {
		httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
		return
	}
	actorRole := domain.UserRole(UserRoleFromContext(r.Context()))

	q := r.URL.Query()
	status := q.Get("status")
	if status != "" && status != string(domain.ApprovalPending) &&
		status != string(domain.ApprovalApproved) && status != string(domain.ApprovalRejected) {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "status must be pending, approved, or rejected"))
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

	list, total, err := h.svc.Approvals(r.Context(), actorRole, actorID, status, limit, offset)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	resp := make([]map[string]any, 0, len(list))
	for _, a := range list {
		resp = append(resp, map[string]any{
			"id":              a.ID,
			"user_id":         a.UserID,
			"user_name":       a.UserName,
			"user_email":      a.UserEmail,
			"type":            string(a.Type),
			"status":          string(a.Status),
			"photo_url":       a.PhotoURL,
			"address":         a.Address,
			"source":          string(a.Source),
			"approval_status": string(a.ApprovalStatus),
			"location_id":     a.LocationID,
			"occurred_at":     a.OccurredAt.Format(time.RFC3339),
		})
	}
	httpx.WriteOK(w, map[string]any{"approvals": resp, "total": total})
}

func (h *AdminHandler) decideApproval(w http.ResponseWriter, r *http.Request) {
	actorID := UserIDFromContext(r.Context())
	if actorID == "" {
		httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
		return
	}
	actorRole := domain.UserRole(UserRoleFromContext(r.Context()))
	id := chi.URLParam(r, "id")

	var body struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	a, err := h.svc.DecideApproval(r.Context(), actorRole, actorID, id, body.Action, body.Reason)
	if err != nil {
		h.ttpxErr(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"attendance": toAdminAttendanceRow(a)})
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

// reportsExport menyajikan laporan kehadiran sebagai CSV berentang. Streaming
// dipakai supaya dataset besar tidak ditahan seluruhnya di memori.
func (h *AdminHandler) reportsExport(w http.ResponseWriter, r *http.Request) {
	if format := r.URL.Query().Get("format"); format != "" && format != "csv" {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	rows, err := h.svc.ExportAttendance(r.Context(), time.Local, from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	filename := "absen-" + from + "_" + to + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	if werr := service.WriteAttendanceCSV(w, rows); werr != nil {
		// Header sudah terkirim; kegagalan tulis hanya bisa dicatat.
		slog.Error("reports: gagal menulis CSV", "error", werr)
	}
}

// reportsSummary menyajikan ringkasan kehadiran per hari pada rentang tanggal.
func (h *AdminHandler) reportsSummary(w http.ResponseWriter, r *http.Request) {
	sum, err := h.svc.SummaryReports(r.Context(), time.Local,
		r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, sum)
}
