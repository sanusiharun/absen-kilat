package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// AdminService provides the admin dashboard's data needs: account seeding,
// aggregate statistics, location management, system settings, and
// attendance overrides. It composes the concrete repositories (app-level
// concern) rather than going through the domain interfaces.
type AdminService struct {
	users       *repo.UserRepository
	attendances *repo.AttendanceRepository
	locations   *repo.LocationRepository
	settings    *SettingsService
	adminEmail  string
}

// NewAdminService creates an AdminService.
func NewAdminService(users *repo.UserRepository, attendances *repo.AttendanceRepository, locations *repo.LocationRepository, settings *SettingsService, adminEmail string) *AdminService {
	return &AdminService{users: users, attendances: attendances, locations: locations, settings: settings, adminEmail: adminEmail}
}

// SeedAdmin creates the initial admin account (role=admin) if one with the
// configured email does not already exist. A no-op when the account exists.
func (s *AdminService) SeedAdmin(ctx context.Context, password string) error {
	if s.adminEmail == "" {
		return nil
	}
	if _, err := s.users.FindByEmail(ctx, s.adminEmail); err == nil {
		return nil // already exists
	} else if !pkgerrors.IsNotFound(err) {
		return fmt.Errorf("service.AdminService.SeedAdmin: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("service.AdminService.SeedAdmin: hash: %w", err)
	}
	admin := &domain.User{
		Email:               s.adminEmail,
		PasswordHash:        string(hash),
		Name:                "Admin",
		Role:                domain.RoleSuperAdmin,
		IsActive:            true,
		IsEmailVerified:     true,
		NotificationEnabled: true,
	}
	if err := s.users.Create(ctx, admin); err != nil {
		return fmt.Errorf("service.AdminService.SeedAdmin: create: %w", err)
	}
	return nil
}

// TodayStats is the set of headline dashboard numbers.
type TodayStats struct {
	TotalUsers int
	Present    int
	Late       int
	Absent     int
}

// TodayStats computes today's attendance summary in the given location.
func (s *AdminService) TodayStats(ctx context.Context, loc *time.Location) (*TodayStats, error) {
	total, err := s.users.CountActive(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).UTC()
	end := start.Add(24 * time.Hour)

	present, late, err := s.attendances.CheckInStatusCountsOnDate(ctx, start, end)
	if err != nil {
		return nil, err
	}
	// Present = checked in on time; absent = active users who never checked in.
	absent := total - present
	if absent < 0 {
		absent = 0
	}
	return &TodayStats{TotalUsers: total, Present: present, Late: late, Absent: absent}, nil
}

// UserUpdate describes an optional administrative change to a user account.
// Nil fields are left unchanged.
type UserUpdate struct {
	Role         *domain.UserRole
	IsActive     *bool
	NIP          *string
	Phone        *string
	Department   *string
	Position     *string
	Address      *string
	JoinDate     *string
	ManagerID    *string
	EmployeeCode *string
}

// ListUsers returns users, optionally filtered by a case-insensitive search
// (name/email) and an is_active status.
func (s *AdminService) ListUsers(ctx context.Context, search, status string) ([]domain.User, error) {
	return s.users.ListFiltered(ctx, search, status)
}

// UpdateUser applies an administrative change to the target user. An admin
// cannot deactivate or demote their own account, which would lock them out
// of admin access.
func (s *AdminService) UpdateUser(ctx context.Context, targetID, actorID string, up UserUpdate) (*domain.User, error) {
	u, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if targetID == actorID {
		if up.IsActive != nil && !*up.IsActive {
			return nil, pkgerrors.Forbidden("you cannot deactivate your own account")
		}
		if up.Role != nil && isAdminRole(u.Role) && !isAdminRole(*up.Role) {
			return nil, pkgerrors.Forbidden("you cannot revoke your own admin role")
		}
	}
	if up.Role != nil {
		u.Role = *up.Role
	}
	if up.IsActive != nil {
		u.IsActive = *up.IsActive
	}
	applyProfileUpdate(u, up)
	if err := s.users.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// applyProfileUpdate copies the non-nil profile fields onto the user.
func applyProfileUpdate(u *domain.User, up UserUpdate) {
	if up.NIP != nil {
		u.NIP = *up.NIP
	}
	if up.Phone != nil {
		u.Phone = *up.Phone
	}
	if up.Department != nil {
		u.Department = *up.Department
	}
	if up.Position != nil {
		u.Position = *up.Position
	}
	if up.Address != nil {
		u.Address = *up.Address
	}
	if up.JoinDate != nil {
		u.JoinDate = *up.JoinDate
	}
	if up.ManagerID != nil {
		u.ManagerID = *up.ManagerID
	}
	if up.EmployeeCode != nil {
		u.EmployeeCode = *up.EmployeeCode
	}
}

// validateManagerID verifies that a non-empty manager_id references an
// existing user.
func (s *AdminService) validateManagerID(ctx context.Context, managerID string) error {
	if managerID == "" {
		return nil
	}
	if _, err := s.users.FindByID(ctx, managerID); err != nil {
		return pkgerrors.Validation("manager_id", "pengguna tidak ditemukan")
	}
	return nil
}

// DayTrend is one row of the attendance line chart: a calendar date with the
// counts of present (on-time), late, and absent active users.
type DayTrend struct {
	Date    string // "2006-01-02" in the given location
	Present int
	Late    int
	Absent  int
}

// DayReport is one row of the daily summary table: a calendar date with
// present, late, absent, and total active users.
type DayReport struct {
	Date    string
	Present int
	Late    int
	Absent  int
	Total   int
}

// Trends computes per-day attendance counts for the last days days (inclusive
// of today). Absent is derived as total active users minus on-time present.
func (s *AdminService) Trends(ctx context.Context, loc *time.Location, days int) ([]DayTrend, error) {
	reports, err := s.Reports(ctx, loc, days)
	if err != nil {
		return nil, err
	}
	out := make([]DayTrend, 0, len(reports))
	for _, r := range reports {
		out = append(out, DayTrend{Date: r.Date, Present: r.Present, Late: r.Late, Absent: r.Absent})
	}
	return out, nil
}

// Reports computes the daily summary for the last days days (inclusive of
// today). It snapshots the active-user count once, then derives present/late
// per day from check-in status counts.
func (s *AdminService) Reports(ctx context.Context, loc *time.Location, days int) ([]DayReport, error) {
	if days <= 0 {
		days = 1
	}
	total, err := s.users.CountActive(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(loc)
	out := make([]DayReport, 0, days)
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc).UTC()
		end := start.Add(24 * time.Hour)

		present, late, err := s.attendances.CheckInStatusCountsOnDate(ctx, start, end)
		if err != nil {
			return nil, err
		}
		absent := total - present
		if absent < 0 {
			absent = 0
		}
		out = append(out, DayReport{
			Date:    day.Format("2006-01-02"),
			Present: present,
			Late:    late,
			Absent:  absent,
			Total:   total,
		})
	}
	return out, nil
}

// UserReport is the per-user attendance summary for the dashboard's
// "Laporan per Pengguna" view.
type UserReport struct {
	UserID        string  `json:"user_id"`
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	Department    string  `json:"department"`
	PresentDays   int     `json:"present_days"`
	LateDays      int     `json:"late_days"`
	AbsentDays    int     `json:"absent_days"`
	CheckInCount  int     `json:"check_in_count"`
	CheckOutCount int     `json:"check_out_count"`
	TotalMinutes  int     `json:"total_minutes"`
	TotalHours    float64 `json:"total_hours"`
	LastCheckIn   *string `json:"last_check_in,omitempty"`
}

// UserReports aggregates attendance per user over the last `days` calendar
// days (default 14). Absent days = days in range minus present days.
func (s *AdminService) UserReports(ctx context.Context, loc *time.Location, days int) ([]UserReport, error) {
	if days <= 0 {
		days = 14
	}
	now := time.Now().In(loc)
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).Add(24 * time.Hour).UTC()
	start := end.Add(-time.Duration(days) * 24 * time.Hour)

	rows, err := s.attendances.UserReports(ctx, start, end, days)
	if err != nil {
		return nil, err
	}
	out := make([]UserReport, 0, len(rows))
	for _, r := range rows {
		ur := UserReport{
			UserID:        r.UserID,
			Name:          r.Name,
			Email:         r.Email,
			Department:    r.Department,
			PresentDays:   r.PresentDays,
			LateDays:      r.LateDays,
			AbsentDays:    r.AbsentDays,
			CheckInCount:  r.CheckInCount,
			CheckOutCount: r.CheckOutCount,
			TotalMinutes:  r.TotalMinutes,
			TotalHours:    math.Round(float64(r.TotalMinutes)/60*100) / 100,
		}
		if r.LastCheckIn != nil {
			t := r.LastCheckIn.In(loc).Format("2006-01-02 15:04")
			ur.LastCheckIn = &t
		}
		out = append(out, ur)
	}
	return out, nil
}

// Locations returns the top attendance addresses by record count.
func (s *AdminService) Locations(ctx context.Context, limit int) ([]domain.LocationStat, error) {
	if limit <= 0 {
		limit = 5
	}
	return s.attendances.TopLocations(ctx, limit)
}

// AdminAttendance is one row of the admin attendance listing: an attendance
// record joined with the user who recorded it.
type AdminAttendance struct {
	ID              string
	UserID          string
	UserName        string
	UserEmail       string
	Type            domain.AttendanceType
	Status          domain.AttendanceStatus
	PhotoURL        string
	Address         string
	Latitude        *float64
	Longitude       *float64
	Device          string
	IPAddress       string
	DistanceM       *int
	Source          domain.AttendanceSource
	ApprovalStatus  domain.ApprovalStatus
	ApprovedBy      string
	ApprovedAt      *time.Time
	RejectionReason string
	LocationID      string
	OccurredAt      time.Time
}

// AttendanceFilter describes the optional filters of the admin attendance
// listing. From/To are calendar dates (YYYY-MM-DD) in the server's local
// timezone, inclusive/exclusive respectively; empty means unbounded.
type AttendanceFilter struct {
	From           string
	To             string
	Status         string
	Type           string
	ApprovalStatus string
	Search         string
	Limit          int
	Offset         int
}

// ListAttendances returns attendance rows joined with user info, newest
// first, along with the total number of matching rows (before limit/offset).
func (s *AdminService) ListAttendances(ctx context.Context, f AttendanceFilter) ([]AdminAttendance, int, error) {
	rf := repo.AdminAttendanceFilter{
		Status:         f.Status,
		Type:           f.Type,
		ApprovalStatus: f.ApprovalStatus,
		Search:         f.Search,
		Limit:          f.Limit,
		Offset:         f.Offset,
	}
	if f.From != "" {
		start, err := time.ParseInLocation("2006-01-02", f.From, time.Local)
		if err != nil {
			return nil, 0, pkgerrors.Validation("from", "must be YYYY-MM-DD")
		}
		rf.From = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.Local).UTC()
	}
	if f.To != "" {
		end, err := time.ParseInLocation("2006-01-02", f.To, time.Local)
		if err != nil {
			return nil, 0, pkgerrors.Validation("to", "must be YYYY-MM-DD")
		}
		rf.To = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.Local).Add(24 * time.Hour).UTC()
	}

	rows, total, err := s.attendances.ListAdmin(ctx, rf)
	if err != nil {
		return nil, 0, err
	}
	out := make([]AdminAttendance, 0, len(rows))
	for _, r := range rows {
		a := AdminAttendance{
			ID:              r.ID,
			UserID:          r.UserID,
			UserName:        r.UserName,
			UserEmail:       r.UserEmail,
			Type:            domain.AttendanceType(r.Type),
			Status:          domain.AttendanceStatus(r.Status),
			PhotoURL:        r.PhotoPath,
			Address:         r.Address,
			Device:          r.Device,
			IPAddress:       r.IPAddress,
			Source:          domain.AttendanceSource(r.Source),
			ApprovalStatus:  domain.ApprovalStatus(r.ApprovalStatus),
			RejectionReason: r.RejectionReason,
			LocationID:      r.LocationID.String,
			// Stored as UTC; present in the server's local zone so admin
			// UI timestamps read naturally.
			OccurredAt: parseRFC3339Nano(r.OccurredAt).In(time.Local),
		}
		if r.Latitude.Valid {
			a.Latitude = &r.Latitude.Float64
		}
		if r.Longitude.Valid {
			a.Longitude = &r.Longitude.Float64
		}
		if r.DistanceM.Valid {
			dm := int(r.DistanceM.Int64)
			a.DistanceM = &dm
		}
		if r.ApprovedBy.Valid {
			a.ApprovedBy = r.ApprovedBy.String
		}
		if r.ApprovedAt.Valid {
			t := parseRFC3339Nano(r.ApprovedAt.String).In(time.Local)
			a.ApprovedAt = &t
		}
		out = append(out, a)
	}
	return out, total, nil
}

// CreateUserInput carries the fields for admin-created accounts.
type CreateUserInput struct {
	Email        string
	Password     string
	Name         string
	Role         domain.UserRole
	EmployeeCode string
}

// CreateUser creates a new account on behalf of an admin. The account starts
// active and email-verified (the admin vouches for it).
func (s *AdminService) CreateUser(ctx context.Context, in CreateUserInput) (*domain.User, error) {
	fields := map[string]string{}
	if in.Email == "" || !emailRe.MatchString(in.Email) {
		fields["email"] = "must be a valid email address"
	}
	if len(in.Password) < 8 {
		fields["password"] = "must be at least 8 characters"
	}
	if in.Name == "" {
		fields["name"] = "is required"
	}
	if in.Role != "" && !validUserRole(in.Role) {
		fields["role"] = "must be super_admin, hr, supervisor, or employee"
	}
	if len(fields) > 0 {
		return nil, pkgerrors.ValidationMulti(fields)
	}
	if in.Role == "" {
		in.Role = domain.RoleEmployee
	}
	in.EmployeeCode = strings.TrimSpace(in.EmployeeCode)

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("service.AdminService.CreateUser: hash: %w", err)
	}
	user := &domain.User{
		Email:               in.Email,
		PasswordHash:        string(hash),
		Name:                in.Name,
		Role:                in.Role,
		EmployeeCode:        in.EmployeeCode,
		IsActive:            true,
		IsEmailVerified:     true,
		NotificationEnabled: true,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err // already wrapped as conflict/internal by repo
	}
	return user, nil
}

// Devices returns attendance devices by record count, most used first.
func (s *AdminService) Devices(ctx context.Context) ([]domain.DeviceStat, error) {
	return s.attendances.ListDevices(ctx)
}

// parseRFC3339Nano parses a stored time string, falling back to zero time.
func parseRFC3339Nano(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// isAdminRole reports whether the role has full admin-dashboard privileges
// (used to guard against an admin revoking their own access).
func isAdminRole(r domain.UserRole) bool {
	return r == domain.RoleSuperAdmin || r == domain.RoleHR
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

// Team returns the direct reports (users with manager_id == managerID),
// ordered by name.
func (s *AdminService) Team(ctx context.Context, managerID string) ([]domain.User, error) {
	return s.users.ListByManager(ctx, managerID)
}

// ApprovalItem is one row of the approval queue: an attendance record
// joined with the name/email of the user who recorded it.
type ApprovalItem struct {
	ID             string
	UserID         string
	UserName       string
	UserEmail      string
	Type           domain.AttendanceType
	Status         domain.AttendanceStatus
	PhotoURL       string
	Address        string
	Source         domain.AttendanceSource
	ApprovalStatus domain.ApprovalStatus
	LocationID     string
	OccurredAt     time.Time
}

// Approvals lists the approval queue. A supervisor only sees records of
// their own team (users with manager_id == actorID); super_admin/hr see
// everything. Status filters on approval_status ("" = all).
func (s *AdminService) Approvals(ctx context.Context, actorRole domain.UserRole, actorID, status string, limit, offset int) ([]ApprovalItem, int, error) {
	f := repo.AttendanceApprovalFilter{Status: status, Limit: limit, Offset: offset}
	if actorRole == domain.RoleSupervisor {
		f.ManagerID = &actorID
	}
	total, err := s.attendances.CountApproval(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.attendances.ListForApproval(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	out := make([]ApprovalItem, 0, len(rows))
	for _, r := range rows {
		item := ApprovalItem{
			ID:             r.ID,
			UserID:         r.UserID,
			UserName:       r.UserName,
			UserEmail:      r.UserEmail,
			Type:           domain.AttendanceType(r.Type),
			Status:         domain.AttendanceStatus(r.Status),
			PhotoURL:       r.PhotoPath,
			Address:        r.Address,
			Source:         domain.AttendanceSource(r.Source),
			ApprovalStatus: domain.ApprovalStatus(r.ApprovalStatus),
			LocationID:     r.LocationID.String,
			OccurredAt:     parseRFC3339Nano(r.OccurredAt).In(time.Local),
		}
		out = append(out, item)
	}
	return out, total, nil
}

// DecideApproval approves or rejects a pending attendance on behalf of the
// actor. A supervisor may only decide records of their own team; super_admin
// and hr may decide any record. The decision is stamped on the attendance
// and recorded in the approval audit trail.
func (s *AdminService) DecideApproval(ctx context.Context, actorRole domain.UserRole, actorID, attendanceID, action, reason string) (*domain.Attendance, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "approve" && action != "reject" {
		return nil, pkgerrors.Validation("action", "must be approve or reject")
	}
	if action == "reject" && strings.TrimSpace(reason) == "" {
		return nil, pkgerrors.Validation("reason", "alasan wajib diisi saat menolak")
	}

	a, err := s.attendances.FindByID(ctx, attendanceID)
	if err != nil {
		return nil, err
	}
	if a.ApprovalStatus != domain.ApprovalPending {
		return nil, pkgerrors.Conflict("attendance", "approval_status", string(a.ApprovalStatus))
	}

	// Supervisor scoping: the owner of the record must be in the actor's
	// team (manager_id == actorID).
	if actorRole == domain.RoleSupervisor {
		owner, err := s.users.FindByID(ctx, a.UserID)
		if err != nil {
			return nil, err
		}
		if owner.ManagerID != actorID {
			return nil, pkgerrors.Forbidden("attendance ini bukan milik tim anda")
		}
	}

	now := time.Now().UTC()
	if action == "approve" {
		a.ApprovalStatus = domain.ApprovalApproved
		a.RejectionReason = ""
	} else {
		a.ApprovalStatus = domain.ApprovalRejected
		a.RejectionReason = strings.TrimSpace(reason)
	}
	a.ApprovedBy = actorID
	a.ApprovedAt = &now

	if err := s.attendances.UpdateApprovalStatus(ctx, a.ID, a.ApprovalStatus, actorID, now, a.RejectionReason); err != nil {
		return nil, err
	}
	log := &domain.ApprovalLog{
		AttendanceID: a.ID,
		ActorID:      actorID,
		Action:       action,
		Reason:       strings.TrimSpace(reason),
	}
	if err := s.attendances.InsertApprovalLog(ctx, log); err != nil {
		return nil, err
	}
	return a, nil
}

// ===== Settings (admin) =====

// GetSettings returns the full system settings.
func (s *AdminService) GetSettings(ctx context.Context) (*domain.Settings, error) {
	return s.settings.Get(ctx)
}

// UpdateSettings validates and applies settings changes.
func (s *AdminService) UpdateSettings(ctx context.Context, up SettingsUpdate) (*domain.Settings, error) {
	return s.settings.Update(ctx, up)
}

// ===== Locations (admin CRUD) =====

// LocationInput carries the fields for creating a location.
type LocationInput struct {
	Name    string
	Type    string // office | client | field
	Lat     float64
	Lng     float64
	RadiusM int
	Active  bool
}

// LocationUpdate carries optional changes for a location; nil fields are
// left unchanged.
type LocationUpdate struct {
	Name    *string
	Type    *string
	Lat     *float64
	Lng     *float64
	RadiusM *int
	Active  *bool
}

// ListLocations returns locations; all=false filters to active only.
func (s *AdminService) ListLocations(ctx context.Context, all bool) ([]domain.Location, error) {
	if all {
		return s.locations.ListAll(ctx)
	}
	return s.locations.ListActive(ctx)
}

// CreateLocation validates and inserts a new location.
func (s *AdminService) CreateLocation(ctx context.Context, in LocationInput) (*domain.Location, error) {
	if err := validateLocationFields(in.Name, in.Type, in.Lat, in.Lng, in.RadiusM); err != nil {
		return nil, err
	}
	l := &domain.Location{
		Name:    in.Name,
		Type:    in.Type,
		Lat:     in.Lat,
		Lng:     in.Lng,
		RadiusM: in.RadiusM,
		Active:  in.Active,
	}
	if err := s.locations.Create(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

// UpdateLocation applies the provided changes to a location.
func (s *AdminService) UpdateLocation(ctx context.Context, id string, up LocationUpdate) (*domain.Location, error) {
	l, err := s.locations.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if up.Name != nil {
		l.Name = *up.Name
	}
	if up.Type != nil {
		l.Type = *up.Type
	}
	if up.Lat != nil {
		l.Lat = *up.Lat
	}
	if up.Lng != nil {
		l.Lng = *up.Lng
	}
	if up.RadiusM != nil {
		l.RadiusM = *up.RadiusM
	}
	if up.Active != nil {
		l.Active = *up.Active
	}
	if err := validateLocationFields(l.Name, l.Type, l.Lat, l.Lng, l.RadiusM); err != nil {
		return nil, err
	}
	if err := s.locations.Update(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

// DeleteLocation soft-deletes a location (active=0).
func (s *AdminService) DeleteLocation(ctx context.Context, id string) error {
	return s.locations.SetActive(ctx, id, false)
}

// validateLocationFields checks the location business rules.
func validateLocationFields(name, typ string, lat, lng float64, radiusM int) error {
	fields := map[string]string{}
	if name == "" {
		fields["name"] = "is required"
	}
	if typ != "office" && typ != "client" && typ != "field" {
		fields["type"] = "must be office, client, or field"
	}
	if lat < -90 || lat > 90 {
		fields["lat"] = "must be between -90 and 90"
	}
	if lng < -180 || lng > 180 {
		fields["lng"] = "must be between -180 and 180"
	}
	if radiusM < 0 || radiusM > 100000 {
		fields["radius_m"] = "must be between 0 and 100000"
	}
	if len(fields) > 0 {
		return pkgerrors.ValidationMulti(fields)
	}
	return nil
}

// ===== Attendance override (issue #10) =====

// OverrideInput carries the optional changes of an admin attendance
// override. Reason is always required and recorded in the approval log.
type OverrideInput struct {
	Type           *domain.AttendanceType
	Status         *domain.AttendanceStatus  // on_time | late
	ApprovalStatus *domain.ApprovalStatus    // pending | approved | rejected
	OccurredAt     *time.Time
	Reason         string
}

// OverrideAttendance edits an attendance record on behalf of an admin,
// writes an approval_log(action='override') entry, and reflects the new
// approval status (approved/rejected) on the record.
func (s *AdminService) OverrideAttendance(ctx context.Context, id, actorID string, ov OverrideInput) (*domain.Attendance, error) {
	if strings.TrimSpace(ov.Reason) == "" {
		return nil, pkgerrors.Validation("reason", "alasan wajib diisi")
	}
	a, err := s.attendances.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	changed := false
	if ov.Type != nil {
		if *ov.Type != domain.AttendanceCheckIn && *ov.Type != domain.AttendanceCheckOut {
			return nil, pkgerrors.Validation("type", "must be check_in or check_out")
		}
		a.Type = *ov.Type
		changed = true
	}
	if ov.Status != nil {
		if *ov.Status != domain.StatusOnTime && *ov.Status != domain.StatusLate {
			return nil, pkgerrors.Validation("status", "must be on_time or late")
		}
		a.Status = *ov.Status
		changed = true
	}
	if ov.OccurredAt != nil {
		a.OccurredAt = ov.OccurredAt.UTC()
		changed = true
	}
	if ov.ApprovalStatus != nil {
		if err := s.applyApprovalChange(a, actorID, *ov.ApprovalStatus, ov.Reason); err != nil {
			return nil, err
		}
		changed = true
	}
	if !changed {
		return nil, pkgerrors.Validation("body", "tidak ada perubahan yang diberikan")
	}

	if err := s.attendances.UpdateAttendance(ctx, a); err != nil {
		return nil, err
	}
	log := &domain.ApprovalLog{
		AttendanceID: id,
		ActorID:      actorID,
		Action:       "override",
		Reason:       ov.Reason,
	}
	if err := s.attendances.InsertApprovalLog(ctx, log); err != nil {
		return nil, err
	}
	return a, nil
}

// applyApprovalChange sets the approval state of a record (and stamps the
// actor/time when it is finalized).
func (s *AdminService) applyApprovalChange(a *domain.Attendance, actorID string, status domain.ApprovalStatus, reason string) error {
	switch status {
	case domain.ApprovalApproved:
		now := time.Now().UTC()
		a.ApprovalStatus = domain.ApprovalApproved
		a.ApprovedBy = actorID
		a.ApprovedAt = &now
		a.RejectionReason = ""
	case domain.ApprovalRejected:
		now := time.Now().UTC()
		a.ApprovalStatus = domain.ApprovalRejected
		a.ApprovedBy = actorID
		a.ApprovedAt = &now
		a.RejectionReason = reason
	case domain.ApprovalPending:
		a.ApprovalStatus = domain.ApprovalPending
		a.ApprovedBy = ""
		a.ApprovedAt = nil
		a.RejectionReason = ""
	default:
		return pkgerrors.Validation("status", "must be pending, approved, or rejected")
	}
	return nil
}
