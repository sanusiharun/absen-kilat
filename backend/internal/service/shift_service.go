package service

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// shiftStore is the subset of the shift repository used by ShiftService.
// Keeping it narrow makes the validation rules testable without a database.
type shiftStore interface {
	ListShifts(ctx context.Context) ([]domain.Shift, error)
	FindShiftByID(ctx context.Context, id string) (*domain.Shift, error)
	CreateShift(ctx context.Context, s *domain.Shift) error
	UpdateShift(ctx context.Context, s *domain.Shift) error
}

// shiftLocationFinder looks up the location a shift is attached to.
// repo.LocationRepository satisfies it.
type shiftLocationFinder interface {
	FindByID(ctx context.Context, id string) (*domain.Location, error)
}

// ShiftService applies the shift business rules and persists shifts.
// It deliberately does not touch attendance logic: deriving lateness from a
// shift is a separate change (S3 of EPIC #60).
type ShiftService struct {
	shifts    shiftStore
	locations shiftLocationFinder
}

// NewShiftService creates a ShiftService.
func NewShiftService(shifts shiftStore, locations shiftLocationFinder) *ShiftService {
	return &ShiftService{shifts: shifts, locations: locations}
}

// ShiftInput carries the fields accepted when creating a shift. Optional
// fields are pointers so "absent" can be distinguished from "zero".
type ShiftInput struct {
	Code         string
	Name         string
	StartTime    string
	EndTime      string
	GraceMinutes *int
	BreakMinutes *int
	LocationID   *string
	Active       *bool
}

// ShiftPatch carries optional changes; nil fields are left unchanged.
type ShiftPatch struct {
	Code         *string
	Name         *string
	StartTime    *string
	EndTime      *string
	GraceMinutes *int
	BreakMinutes *int
	LocationID   *string
	Active       *bool
}

// DefaultGraceMinutes is the owner-approved default grace per shift.
const DefaultGraceMinutes = 15

var (
	shiftCodeRE = regexp.MustCompile(`^[A-Z0-9-]{2,20}$`)
	shiftTimeRE = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
)

// ListShifts returns shifts ordered by code. all=false filters to active ones.
func (s *ShiftService) ListShifts(ctx context.Context, all bool) ([]domain.Shift, error) {
	shifts, err := s.shifts.ListShifts(ctx)
	if err != nil {
		return nil, err
	}
	if all {
		return shifts, nil
	}
	out := make([]domain.Shift, 0, len(shifts))
	for _, sh := range shifts {
		if sh.Active {
			out = append(out, sh)
		}
	}
	return out, nil
}

// CreateShift validates and stores a new shift.
func (s *ShiftService) CreateShift(ctx context.Context, in ShiftInput) (*domain.Shift, error) {
	sh := &domain.Shift{
		Code:         in.Code,
		Name:         in.Name,
		StartTime:    in.StartTime,
		EndTime:      in.EndTime,
		GraceMinutes: DefaultGraceMinutes,
		Active:       true,
	}
	if in.GraceMinutes != nil {
		sh.GraceMinutes = *in.GraceMinutes
	}
	if in.BreakMinutes != nil {
		sh.BreakMinutes = *in.BreakMinutes
	}
	if in.LocationID != nil {
		sh.LocationID = *in.LocationID
	}
	if in.Active != nil {
		sh.Active = *in.Active
	}

	if err := s.validate(ctx, sh); err != nil {
		return nil, err
	}
	sh.CrossesMidnight = crossesMidnight(sh.StartTime, sh.EndTime)

	if err := s.shifts.CreateShift(ctx, sh); err != nil {
		if errors.Is(err, domain.ErrShiftCodeExists) {
			return nil, pkgerrors.Validation("code", "shift code already exists")
		}
		return nil, err
	}
	return sh, nil
}

// UpdateShift applies the provided changes to an existing shift.
func (s *ShiftService) UpdateShift(ctx context.Context, id string, p ShiftPatch) (*domain.Shift, error) {
	sh, err := s.shifts.FindShiftByID(ctx, id)
	if err != nil {
		return nil, mapShiftNotFound(err, id)
	}
	if p.Code != nil {
		sh.Code = *p.Code
	}
	if p.Name != nil {
		sh.Name = *p.Name
	}
	if p.StartTime != nil {
		sh.StartTime = *p.StartTime
	}
	if p.EndTime != nil {
		sh.EndTime = *p.EndTime
	}
	if p.GraceMinutes != nil {
		sh.GraceMinutes = *p.GraceMinutes
	}
	if p.BreakMinutes != nil {
		sh.BreakMinutes = *p.BreakMinutes
	}
	if p.LocationID != nil {
		sh.LocationID = *p.LocationID
	}
	if p.Active != nil {
		sh.Active = *p.Active
	}

	if err := s.validate(ctx, sh); err != nil {
		return nil, err
	}
	sh.CrossesMidnight = crossesMidnight(sh.StartTime, sh.EndTime)

	if err := s.shifts.UpdateShift(ctx, sh); err != nil {
		if errors.Is(err, domain.ErrShiftCodeExists) {
			return nil, pkgerrors.Validation("code", "shift code already exists")
		}
		return nil, mapShiftNotFound(err, id)
	}
	return sh, nil
}

// validate normalises mutable fields and enforces the documented rules.
// crosses_midnight is derived by the server, never taken from the client.
func (s *ShiftService) validate(ctx context.Context, sh *domain.Shift) error {
	sh.Code = strings.ToUpper(strings.TrimSpace(sh.Code))
	sh.Name = strings.TrimSpace(sh.Name)
	sh.StartTime = strings.TrimSpace(sh.StartTime)
	sh.EndTime = strings.TrimSpace(sh.EndTime)
	sh.LocationID = strings.TrimSpace(sh.LocationID)

	fields := map[string]string{}
	if !shiftCodeRE.MatchString(sh.Code) {
		fields["code"] = "must be 2-20 characters of A-Z, 0-9, or -"
	}
	if sh.Name == "" {
		fields["name"] = "is required"
	} else if len(sh.Name) > 80 {
		fields["name"] = "must be at most 80 characters"
	}
	if !shiftTimeRE.MatchString(sh.StartTime) {
		fields["start_time"] = "must be HH:MM (00:00-23:59)"
	}
	if !shiftTimeRE.MatchString(sh.EndTime) {
		fields["end_time"] = "must be HH:MM (00:00-23:59)"
	}
	if shiftTimeRE.MatchString(sh.StartTime) && sh.StartTime == sh.EndTime {
		fields["end_time"] = "must differ from start_time (24-hour shifts are not supported)"
	}
	if sh.GraceMinutes < 0 || sh.GraceMinutes > 120 {
		fields["grace_minutes"] = "must be between 0 and 120"
	}
	if sh.BreakMinutes < 0 || sh.BreakMinutes > 480 {
		fields["break_minutes"] = "must be between 0 and 480"
	}
	if len(fields) > 0 {
		return pkgerrors.ValidationMulti(fields)
	}

	if sh.LocationID == "" {
		return nil
	}
	loc, err := s.locations.FindByID(ctx, sh.LocationID)
	if err != nil {
		if pkgerrors.IsNotFound(err) {
			return pkgerrors.NotFound("location", sh.LocationID)
		}
		return err
	}
	if !loc.Active {
		return pkgerrors.Validation("location_id", "location is inactive")
	}
	return nil
}

// crossesMidnight reports whether the shift ends at or before it starts, i.e.
// it runs past midnight. Both inputs are validated HH:MM strings.
func crossesMidnight(start, end string) bool { return end <= start }

// mapShiftNotFound turns the repository's not-found error into an application
// error carrying the requested id.
func mapShiftNotFound(err error, id string) error {
	if errors.Is(err, domain.ErrShiftNotFound) {
		return pkgerrors.NotFound("shift", id)
	}
	return err
}
