package service

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// AdminService provides the admin dashboard's data needs: account seeding and
// aggregate statistics. It composes the concrete repositories (app-level
// concern) rather than going through the domain interfaces.
type AdminService struct {
	users       *repo.UserRepository
	attendances *repo.AttendanceRepository
	adminEmail  string
}

// NewAdminService creates an AdminService.
func NewAdminService(users *repo.UserRepository, attendances *repo.AttendanceRepository, adminEmail string) *AdminService {
	return &AdminService{users: users, attendances: attendances, adminEmail: adminEmail}
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
		Role:                domain.RoleAdmin,
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
	Role     *domain.UserRole
	IsActive *bool
}

// ListUsers returns users, optionally filtered by a case-insensitive search
// (name/email) and an is_active status.
func (s *AdminService) ListUsers(ctx context.Context, search, status string) ([]domain.User, error) {
	return s.users.ListFiltered(ctx, search, status)
}

// UpdateUser applies an administrative change (role and/or is_active) to the
// target user. An admin cannot deactivate or demote their own account, which
// would lock them out of admin access.
func (s *AdminService) UpdateUser(ctx context.Context, targetID, actorID string, up UserUpdate) (*domain.User, error) {
	u, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if targetID == actorID {
		if up.IsActive != nil && !*up.IsActive {
			return nil, pkgerrors.Forbidden("you cannot deactivate your own account")
		}
		if up.Role != nil && *up.Role != domain.RoleAdmin && u.Role == domain.RoleAdmin {
			return nil, pkgerrors.Forbidden("you cannot revoke your own admin role")
		}
	}
	if up.Role != nil {
		u.Role = *up.Role
	}
	if up.IsActive != nil {
		u.IsActive = *up.IsActive
	}
	if err := s.users.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
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
	ID         string
	UserID     string
	UserName   string
	UserEmail  string
	Type       domain.AttendanceType
	Status     domain.AttendanceStatus
	PhotoURL   string
	Address    string
	Latitude   *float64
	Longitude  *float64
	Device     string
	IPAddress  string
	DistanceM  *int
	OccurredAt time.Time
}

// AttendanceFilter describes the optional filters of the admin attendance
// listing. From/To are calendar dates (YYYY-MM-DD) in the server's local
// timezone, inclusive/exclusive respectively; empty means unbounded.
type AttendanceFilter struct {
	From   string
	To     string
	Status string
	Type   string
	Search string
	Limit  int
	Offset int
}

// ListAttendances returns attendance rows joined with user info, newest
// first, along with the total number of matching rows (before limit/offset).
func (s *AdminService) ListAttendances(ctx context.Context, f AttendanceFilter) ([]AdminAttendance, int, error) {
	rf := repo.AdminAttendanceFilter{
		Status: f.Status,
		Type:   f.Type,
		Search: f.Search,
		Limit:  f.Limit,
		Offset: f.Offset,
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
			ID:        r.ID,
			UserID:    r.UserID,
			UserName:  r.UserName,
			UserEmail: r.UserEmail,
			Type:      domain.AttendanceType(r.Type),
			Status:    domain.AttendanceStatus(r.Status),
			PhotoURL:  r.PhotoPath,
			Address:   r.Address,
			Device:    r.Device,
			IPAddress: r.IPAddress,
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
		out = append(out, a)
	}
	return out, total, nil
}

// CreateUser creates a new account on behalf of an admin. The account starts
// active and email-verified (the admin vouches for it).
func (s *AdminService) CreateUser(ctx context.Context, email, password, name string, role domain.UserRole) (*domain.User, error) {
	fields := map[string]string{}
	if email == "" || !emailRe.MatchString(email) {
		fields["email"] = "must be a valid email address"
	}
	if len(password) < 8 {
		fields["password"] = "must be at least 8 characters"
	}
	if name == "" {
		fields["name"] = "is required"
	}
	if role != "" && role != domain.RoleAdmin && role != domain.RoleEmployee {
		fields["role"] = "must be admin or employee"
	}
	if len(fields) > 0 {
		return nil, pkgerrors.ValidationMulti(fields)
	}
	if role == "" {
		role = domain.RoleEmployee
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("service.AdminService.CreateUser: hash: %w", err)
	}
	user := &domain.User{
		Email:               email,
		PasswordHash:        string(hash),
		Name:                name,
		Role:                role,
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
