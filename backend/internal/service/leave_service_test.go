package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
)

// fakeLeaveRepo is an in-memory LeaveRepository for service tests.
type fakeLeaveRepo struct {
	leaves []domain.Leave
}

func (f *fakeLeaveRepo) Create(ctx context.Context, l *domain.Leave) error {
	f.leaves = append(f.leaves, *l)
	return nil
}
func (f *fakeLeaveRepo) FindByID(ctx context.Context, id string) (*domain.Leave, error) {
	for i := range f.leaves {
		if f.leaves[i].ID == id {
			return &f.leaves[i], nil
		}
	}
	return nil, errNotFound
}
func (f *fakeLeaveRepo) List(ctx context.Context, status, userID string) ([]domain.Leave, error) {
	out := make([]domain.Leave, 0)
	for _, l := range f.leaves {
		if status != "" && string(l.Status) != status {
			continue
		}
		if userID != "" && l.UserID != userID {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}
func (f *fakeLeaveRepo) ListOverlapping(ctx context.Context, userID, start, end string) ([]domain.Leave, error) {
	return nil, nil
}
func (f *fakeLeaveRepo) CountApprovedDays(ctx context.Context, userID, leaveType string) (int, error) {
	return 0, nil
}
func (f *fakeLeaveRepo) UpdateStatus(ctx context.Context, id, status, actorID, rejectionReason string, approvedAt *time.Time) error {
	for i := range f.leaves {
		if f.leaves[i].ID == id {
			f.leaves[i].Status = domain.LeaveStatus(status)
			f.leaves[i].ApprovedBy = actorID
			f.leaves[i].RejectionReason = rejectionReason
		}
	}
	return nil
}
func (f *fakeLeaveRepo) SetAttachment(ctx context.Context, id, path string) error { return nil }

var errNotFound = errors.New("not found")

// ---- date fixtures -------------------------------------------------------
// Fixtures must be relative to today: this file used to hard-code August 2026
// and every date-dependent test started failing once the calendar passed them.
func dstr(t time.Time) string { return t.Format("2006-01-02") }

// nextWeekday returns the first date strictly after today that falls on wd,
// so "future" assertions stay valid no matter when the suite runs.
func nextWeekday(wd time.Weekday) time.Time {
	today := time.Now()
	for i := 1; i <= 7; i++ {
		if d := today.AddDate(0, 0, i); d.Weekday() == wd {
			return d
		}
	}
	return today.AddDate(0, 0, 7)
}

// nextWeekdayAfter returns the first date more than minDays after from that
// falls on wd (used to build Thu..Mon style spans).
func nextWeekdayAfter(from time.Time, minDays int, wd time.Weekday) time.Time {
	for i := minDays; i <= minDays+7; i++ {
		if d := from.AddDate(0, 0, i); d.Weekday() == wd {
			return d
		}
	}
	return from.AddDate(0, 0, minDays+7)
}

// fakeLeaveTypeRepo returns fixed leave types.
type fakeLeaveTypeRepo struct{}

func (f *fakeLeaveTypeRepo) FindByCode(ctx context.Context, code string) (*domain.LeaveType, error) {
	switch code {
	case "annual":
		return &domain.LeaveType{ID: "t1", Code: "annual", Name: "Cuti Tahunan", DefaultDays: 12}, nil
	case "sick":
		return &domain.LeaveType{ID: "t2", Code: "sick", Name: "Cuti Sakit", DefaultDays: 14}, nil
	case "emergency":
		return &domain.LeaveType{ID: "t3", Code: "emergency", Name: "Cuti Darurat", DefaultDays: 5}, nil
	case "maternity":
		return &domain.LeaveType{ID: "t4", Code: "maternity", Name: "Cuti Melahirkan", DefaultDays: 90}, nil
	case "paternity":
		return &domain.LeaveType{ID: "t5", Code: "paternity", Name: "Cuti Ayah", DefaultDays: 0}, nil
	}
	return nil, nil
}
func (f *fakeLeaveTypeRepo) ListActive(ctx context.Context) ([]domain.LeaveType, error) {
	return []domain.LeaveType{
		{ID: "t1", Code: "annual", Name: "Cuti Tahunan", DefaultDays: 12},
		{ID: "t2", Code: "sick", Name: "Cuti Sakit", DefaultDays: 14},
		{ID: "t3", Code: "emergency", Name: "Cuti Darurat", DefaultDays: 5},
		{ID: "t4", Code: "maternity", Name: "Cuti Melahirkan", DefaultDays: 90},
		{ID: "t5", Code: "paternity", Name: "Cuti Ayah", DefaultDays: 0},
	}, nil
}

// fakeUserRepo returns a user with quota.
type fakeUserRepoLeave struct {
	q int
}

func (f *fakeUserRepoLeave) Create(ctx context.Context, u *domain.User) error { return nil }
func (f *fakeUserRepoLeave) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return &domain.User{ID: "u1", Email: email, Name: "Budi", LeaveQuota: f.q, LeaveUsed: 0}, nil
}
func (f *fakeUserRepoLeave) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return &domain.User{ID: id, Name: "Budi", LeaveQuota: f.q, LeaveUsed: 0}, nil
}
func (f *fakeUserRepoLeave) Update(ctx context.Context, u *domain.User) error { return nil }
func (f *fakeUserRepoLeave) Get(ctx context.Context, id string) (*domain.User, error) {
	return &domain.User{ID: id, Name: "Budi", LeaveQuota: f.q, LeaveUsed: 0}, nil
}

func newTestLeaveSvc(quota int) *LeaveService {
	return NewLeaveService(
		&fakeLeaveRepo{},
		&fakeLeaveTypeRepo{},
		&fakeUserRepoLeave{q: quota},
		[]time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
	)
}

func TestApplyAnnualLeaveWorkdays(t *testing.T) {
	ctx := context.Background()
	svc := newTestLeaveSvc(12)
	// Monday .. Friday of next week (5 working days).
	monday := nextWeekday(time.Monday)
	l, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "annual", StartDate: dstr(monday), EndDate: dstr(monday.AddDate(0, 0, 4)), Reason: "liburan"})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if l.TotalDays != 5 {
		t.Fatalf("expected 5 working days, got %d", l.TotalDays)
	}
	if l.Status != domain.LeavePending {
		t.Fatalf("expected pending, got %s", l.Status)
	}
}

func TestApplyRejectsWeekendSpanCount(t *testing.T) {
	ctx := context.Background()
	svc := newTestLeaveSvc(12)
	// Friday .. Tuesday covers Sat/Sun -> 3 working days.
	friday := nextWeekday(time.Friday)
	l, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "annual", StartDate: dstr(friday), EndDate: dstr(friday.AddDate(0, 0, 4)), Reason: "x"})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if l.TotalDays != 3 {
		t.Fatalf("expected 3 working days (Fri,Mon,Tue), got %d", l.TotalDays)
	}
}

func TestApplyPaternityDynamic(t *testing.T) {
	ctx := context.Background()
	svc := newTestLeaveSvc(12)
	day := nextWeekday(time.Monday)
	l, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "paternity", StartDate: dstr(day), EndDate: dstr(day), ExtraDays: 7})
	if err != nil {
		t.Fatalf("paternity apply failed: %v", err)
	}
	if l.TotalDays != 7 {
		t.Fatalf("expected 7 dynamic days, got %d", l.TotalDays)
	}
}

func TestApplyStartInPastRejected(t *testing.T) {
	ctx := context.Background()
	svc := newTestLeaveSvc(12)
	// 2020-01-01 is firmly in the past.
	_, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "annual", StartDate: "2020-01-01", EndDate: "2020-01-05", Reason: "x"})
	if err == nil {
		t.Fatal("expected past-start error, got nil")
	}
}

func TestApplyQuotaExceeded(t *testing.T) {
	ctx := context.Background()
	svc := newTestLeaveSvc(2) // quota 2 days
	monday := nextWeekday(time.Monday)
	_, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "annual", StartDate: dstr(monday), EndDate: dstr(monday.AddDate(0, 0, 4)), Reason: "x"})
	if err == nil {
		t.Fatal("expected quota error, got nil")
	}
	// Must fail because of the quota, not because the fixture drifted into the past.
	if !strings.Contains(err.Error(), "kuota") {
		t.Fatalf("expected quota error, got %v", err)
	}
}

func TestApplyEndBeforeStart(t *testing.T) {
	ctx := context.Background()
	svc := newTestLeaveSvc(12)
	start := nextWeekday(time.Monday)
	_, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "annual", StartDate: dstr(start), EndDate: dstr(start.AddDate(0, 0, -4)), Reason: "x"})
	if err == nil {
		t.Fatal("expected end<start error")
	}
	if !strings.Contains(err.Error(), "selesai") {
		t.Fatalf("expected end-before-start error, got %v", err)
	}
}

func TestApproveRejectFlow(t *testing.T) {
	ctx := context.Background()
	svc := newTestLeaveSvc(12)
	monday := nextWeekday(time.Monday)
	l, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "annual", StartDate: dstr(monday), EndDate: dstr(monday.AddDate(0, 0, 2)), Reason: "x"})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	appr, err := svc.Approve(ctx, l.ID, "admin1")
	if err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	if appr.Status != domain.LeaveApproved {
		t.Fatalf("expected approved, got %s", appr.Status)
	}
}

// TestApplyUnknownLeaveTypeNoPanic guards the case where the type repository
// resolves nothing without returning an error: the service must reject it
// instead of dereferencing a nil *LeaveType.
func TestApplyUnknownLeaveTypeNoPanic(t *testing.T) {
	ctx := context.Background()
	svc := newTestLeaveSvc(12)
	day := nextWeekday(time.Monday)
	_, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "tidak-ada", StartDate: dstr(day), EndDate: dstr(day), Reason: "x"})
	if err == nil {
		t.Fatal("expected invalid-type error, got nil")
	}
}
