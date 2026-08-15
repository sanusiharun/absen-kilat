package service

import (
	"context"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// ErrCheckOutBeforeCheckIn is returned when a user tries to check out
// without having checked in first.
var ErrCheckOutBeforeCheckIn = pkgerrors.Validation("type", "check out requires a check in first")

// ErrAlreadyCheckedIn is returned when a check-in already exists today.
var ErrAlreadyCheckedIn = pkgerrors.Validation("type", "already checked in today")

// ErrAlreadyCheckedOut is returned when a check-out already exists today.
var ErrAlreadyCheckedOut = pkgerrors.Validation("type", "already checked out today")

// AttendanceInput carries the fields needed to record an attendance.
type AttendanceInput struct {
	UserID    string
	Type      domain.AttendanceType
	PhotoPath string
	Latitude  *float64
	Longitude *float64
	Address   string
	Notes     string
	Device    string
	IPAddress string
	Now       time.Time // wall-clock time of the check (testable)
	Location  *time.Location
}

// AttendanceService implements attendance business logic.
type AttendanceService struct {
	repo       domain.AttendanceRepository
	cutoffHour int         // local hour after which check-ins are "late"
	office     OfficeConfig // geofence: office point + allowed radius
}

// NewAttendanceService creates an AttendanceService.
func NewAttendanceService(repo domain.AttendanceRepository, cutoffHour int, office OfficeConfig) *AttendanceService {
	return &AttendanceService{repo: repo, cutoffHour: cutoffHour, office: office}
}

// CheckIn records a check-in for the user. The second check-in of the same
// day is rejected.
func (s *AttendanceService) CheckIn(ctx context.Context, in AttendanceInput) (*domain.Attendance, error) {
	now, loc := resolveNow(in)
	date := now.In(loc)
	list, err := s.repo.ListByUserDate(ctx, in.UserID, date, loc)
	if err != nil {
		return nil, err
	}
	for _, a := range list {
		if a.Type == domain.AttendanceCheckIn {
			return nil, ErrAlreadyCheckedIn
		}
	}

	// Geofence: reject check-ins reported outside the office radius.
	distM, err := s.office.checkGeofence(in.Latitude, in.Longitude)
	if err != nil {
		return nil, err
	}

	a := &domain.Attendance{
		UserID:     in.UserID,
		Type:       domain.AttendanceCheckIn,
		Status:     s.statusFor(now, loc),
		PhotoPath:  in.PhotoPath,
		Latitude:   in.Latitude,
		Longitude:  in.Longitude,
		Address:    in.Address,
		Notes:      in.Notes,
		Device:     in.Device,
		IPAddress:  in.IPAddress,
		OccurredAt: now.UTC(),
		CreatedAt:  time.Now().UTC(),
	}
	if in.Latitude != nil && in.Longitude != nil {
		a.DistanceM = &distM
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// CheckOut records a check-out, requiring a prior check-in on the same day.
func (s *AttendanceService) CheckOut(ctx context.Context, in AttendanceInput) (*domain.Attendance, error) {
	now, loc := resolveNow(in)
	date := now.In(loc)
	list, err := s.repo.ListByUserDate(ctx, in.UserID, date, loc)
	if err != nil {
		return nil, err
	}

	hasCheckIn := false
	for _, a := range list {
		if a.Type == domain.AttendanceCheckIn {
			hasCheckIn = true
		}
		if a.Type == domain.AttendanceCheckOut {
			return nil, ErrAlreadyCheckedOut
		}
	}
	if !hasCheckIn {
		return nil, ErrCheckOutBeforeCheckIn
	}

	// Geofence applies to check-outs too.
	distM, err := s.office.checkGeofence(in.Latitude, in.Longitude)
	if err != nil {
		return nil, err
	}

	a := &domain.Attendance{
		UserID:     in.UserID,
		Type:       domain.AttendanceCheckOut,
		Status:     domain.StatusOnTime, // status only meaningful for check-ins
		PhotoPath:  in.PhotoPath,
		Latitude:   in.Latitude,
		Longitude:  in.Longitude,
		Address:    in.Address,
		Notes:      in.Notes,
		Device:     in.Device,
		IPAddress:  in.IPAddress,
		OccurredAt: now.UTC(),
		CreatedAt:  time.Now().UTC(),
	}
	if in.Latitude != nil && in.Longitude != nil {
		a.DistanceM = &distM
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// TodayState summarises the user's attendance state for the current day,
// which drives the Home screen button ("Absen Masuk" / "Absen Keluar").
type TodayState struct {
	CheckedIn  bool
	CheckedOut bool
	Records    []domain.Attendance
}

// Today returns the day state and records for the user.
func (s *AttendanceService) Today(ctx context.Context, userID string, now time.Time, loc *time.Location) (*TodayState, error) {
	date := now.In(loc)
	list, err := s.repo.ListByUserDate(ctx, userID, date, loc)
	if err != nil {
		return nil, err
	}
	st := &TodayState{Records: list}
	for _, a := range list {
		switch a.Type {
		case domain.AttendanceCheckIn:
			st.CheckedIn = true
		case domain.AttendanceCheckOut:
			st.CheckedOut = true
		}
	}
	return st, nil
}

// ListByDate returns records for a specific date.
func (s *AttendanceService) ListByDate(ctx context.Context, userID string, date time.Time, loc *time.Location) ([]domain.Attendance, error) {
	return s.repo.ListByUserDate(ctx, userID, date, loc)
}

// Get returns a single attendance record by id.
func (s *AttendanceService) Get(ctx context.Context, id string) (*domain.Attendance, error) {
	return s.repo.FindByID(ctx, id)
}

// statusFor derives on_time/late for a check-in. A check-in at or before
// cutoffHour:00 (local) is on time; anything after is late.
func (s *AttendanceService) statusFor(now time.Time, loc *time.Location) domain.AttendanceStatus {
	local := now.In(loc)
	cutoff := time.Date(local.Year(), local.Month(), local.Day(), s.cutoffHour, 0, 0, 0, loc)
	if local.After(cutoff) {
		return domain.StatusLate
	}
	return domain.StatusOnTime
}

// resolveNow returns the provided time (or time.Now) and location.
func resolveNow(in AttendanceInput) (time.Time, *time.Location) {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	loc := in.Location
	if loc == nil {
		loc = time.Local
	}
	return now, loc
}
