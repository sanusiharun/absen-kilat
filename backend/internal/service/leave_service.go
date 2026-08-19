package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// LeaveService implements leave (cuti) business logic.
type LeaveService struct {
	repo       domain.LeaveRepository
	leaveTypes domain.LeaveTypeRepository
	users      domain.UserRepository
	workdays   []time.Weekday // from settings
}

// NewLeaveService constructs a LeaveService. workdays is the list of working
// weekdays used to count leave durations (e.g. Mon-Fri).
func NewLeaveService(
	repo domain.LeaveRepository,
	leaveTypes domain.LeaveTypeRepository,
	users domain.UserRepository,
	workdays []time.Weekday,
) *LeaveService {
	return &LeaveService{repo: repo, leaveTypes: leaveTypes, users: users, workdays: workdays}
}

// LeaveInput is the payload to create a leave request.
type LeaveInput struct {
	UserID    string
	TypeCode  string // annual | sick | emergency | maternity | paternity
	StartDate string // YYYY-MM-DD
	EndDate   string // YYYY-MM-DD
	Reason    string
	ExtraDays int // paternity dynamic: 3/5/7
}

// Apply creates a new pending leave request with validation.
func (s *LeaveService) Apply(ctx context.Context, in LeaveInput) (*domain.Leave, error) {
	lt, err := s.leaveTypes.FindByCode(ctx, in.TypeCode)
	if err != nil {
		return nil, pkgerrors.Validation("leave", "tipe cuti tidak valid")
	}

	// Validate date format + ordering.
	start, err := time.Parse("2006-01-02", in.StartDate)
	if err != nil {
		return nil, pkgerrors.Validation("leave", "format tanggal mulai salah (YYYY-MM-DD)")
	}
	// Start date must not be in the past.
	today := time.Now().Format("2006-01-02")
	if start.Before(time.Now()) && start.Format("2006-01-02") != today {
		return nil, pkgerrors.Validation("leave", "tanggal mulai tidak boleh di masa lalu")
	}
	end, err := time.Parse("2006-01-02", in.EndDate)
	if err != nil {
		return nil, pkgerrors.Validation("leave", "format tanggal selesai salah (YYYY-MM-DD)")
	}
	if end.Before(start) {
		return nil, pkgerrors.Validation("leave", "tanggal selesai harus >= tanggal mulai")
	}

	// Count working days between start and end inclusive.
	days := countWorkdays(start, end, s.workdays)
	if days <= 0 {
		return nil, pkgerrors.Validation("leave", "rentang tanggal tidak mencakup hari kerja")
	}

	// For paternity, extra_days dynamically extends the duration (3/5/7).
	totalDays := days
	if in.TypeCode == "paternity" && in.ExtraDays > 0 {
		if lt.MaxExtraDays > 0 && in.ExtraDays > lt.MaxExtraDays {
			return nil, pkgerrors.Validation("leave", fmt.Sprintf("maksimal extra hari paternity adalah %d", lt.MaxExtraDays))
		}
		totalDays = in.ExtraDays
	}
	if lt.DefaultDays > 0 && totalDays > lt.DefaultDays && in.TypeCode != "paternity" {
		return nil, pkgerrors.Validation("leave", fmt.Sprintf("jumlah hari melebihi kuota tipe cuti ini (%d)", lt.DefaultDays))
	}

	// Overlap check against existing pending/approved leaves.
	overlapping, err := s.repo.ListOverlapping(ctx, in.UserID, in.StartDate, in.EndDate)
	if err != nil {
		return nil, err
	}
	if len(overlapping) > 0 {
		return nil, pkgerrors.Conflict("leave","date_range","overlap")
	}

	// Validate remaining quota (annual leave only uses leave_quota).
	if in.TypeCode == "annual" {
		u, err := s.users.FindByID(ctx, in.UserID)
		if err != nil {
			return nil, err
		}
		used, err := s.repo.CountApprovedDays(ctx, in.UserID, "annual")
		if err != nil {
			return nil, err
		}
		remaining := u.LeaveQuota - used
		if remaining < totalDays {
			return nil, pkgerrors.Validation("leave", fmt.Sprintf("sisa kuota cuti tahunan tidak cukup (tersisa %d hari)", remaining))
		}
	}

	l := &domain.Leave{
		UserID:     in.UserID,
		LeaveType:  lt.Code,
		StartDate:  in.StartDate,
		EndDate:    in.EndDate,
		TotalDays:  totalDays,
		Reason:     strings.TrimSpace(in.Reason),
		Status:     domain.LeavePending,
		ExtraDays:  in.ExtraDays,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

// Get returns a single leave by id (used by the owner detail endpoint).
func (s *LeaveService) Get(ctx context.Context, id string) (*domain.Leave, error) {
	return s.repo.FindByID(ctx, id)
}

// List returns leaves optionally filtered by status and/or owner userID.
func (s *LeaveService) List(ctx context.Context, status, userID string) ([]domain.Leave, error) {
	return s.repo.List(ctx, status, userID)
}

// Approve approves a pending leave request.
func (s *LeaveService) Approve(ctx context.Context, id, actorID string) (*domain.Leave, error) {
	l, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if l.Status != domain.LeavePending {
		return nil, pkgerrors.Conflict("leave","status","already_processed")
	}
	now := time.Now().UTC()
	if err := s.repo.UpdateStatus(ctx, id, string(domain.LeaveApproved), actorID, "", &now); err != nil {
		return nil, err
	}
	l.Status = domain.LeaveApproved
	l.ApprovedBy = actorID
	l.ApprovedAt = &now
	return l, nil
}

// Reject rejects a pending leave request with a reason.
func (s *LeaveService) Reject(ctx context.Context, id, actorID, reason string) (*domain.Leave, error) {
	l, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if l.Status != domain.LeavePending {
		return nil, pkgerrors.Conflict("leave","status","already_processed")
	}
	now := time.Now().UTC()
	if err := s.repo.UpdateStatus(ctx, id, string(domain.LeaveRejected), actorID, reason, &now); err != nil {
		return nil, err
	}
	l.Status = domain.LeaveRejected
	l.ApprovedBy = actorID
	l.ApprovedAt = &now
	l.RejectionReason = reason
	return l, nil
}

// Cancel lets the owner withdraw a leave while it is still pending.
func (s *LeaveService) Cancel(ctx context.Context, id, ownerID string) (*domain.Leave, error) {
	l, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if l.UserID != ownerID {
		return nil, pkgerrors.Forbidden("hanya pemohon yang dapat membatalkan")
	}
	if l.Status != domain.LeavePending {
		return nil, pkgerrors.Validation("leave", "hanya pengajuan dengan status menunggu yang dapat dibatalkan")
	}
	now := time.Now().UTC()
	if err := s.repo.UpdateStatus(ctx, id, string(domain.LeaveCancelled), ownerID, "", &now); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// LeaveTypes returns active leave types for the apply form.
func (s *LeaveService) LeaveTypes(ctx context.Context) ([]domain.LeaveType, error) {
	return s.leaveTypes.ListActive(ctx)
}

// countWorkdays counts weekdays in [start, end] inclusive that are in workdays.
func countWorkdays(start, end time.Time, workdays []time.Weekday) int {
	if len(workdays) == 0 {
		workdays = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	}
	set := map[time.Weekday]bool{}
	for _, w := range workdays {
		set[w] = true
	}
	count := 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if set[d.Weekday()] {
			count++
		}
	}
	return count
}
