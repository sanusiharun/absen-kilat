package service

import (
	"context"
	"errors"
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
	// Mon 2026-08-17 .. Fri 2026-08-21 (5 working days).
	l, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "annual", StartDate: "2026-08-17", EndDate: "2026-08-21", Reason: "liburan"})
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
	// Fri 2026-08-21 .. Tue 2026-08-25 covers Sat/Sun -> 3 working days.
	l, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "annual", StartDate: "2026-08-21", EndDate: "2026-08-25", Reason: "x"})
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
	l, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "paternity", StartDate: "2026-08-17", EndDate: "2026-08-17", ExtraDays: 7})
	if err != nil {
		t.Fatalf("paternity apply failed: %v", err)
	}
	if l.TotalDays != 7 {
		t.Fatalf("expected 7 dynamic days, got %d", l.TotalDays)
	}
}

func TestApplyQuotaExceeded(t *testing.T) {
	ctx := context.Background()
	svc := newTestLeaveSvc(2) // quota 2 days
	_, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "annual", StartDate: "2026-08-17", EndDate: "2026-08-21", Reason: "x"})
	if err == nil {
		t.Fatal("expected quota error, got nil")
	}
}

func TestApplyEndBeforeStart(t *testing.T) {
	ctx := context.Background()
	svc := newTestLeaveSvc(12)
	_, err := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "annual", StartDate: "2026-08-21", EndDate: "2026-08-17", Reason: "x"})
	if err == nil {
		t.Fatal("expected end<start error")
	}
}

func TestApproveRejectFlow(t *testing.T) {
	ctx := context.Background()
	svc := newTestLeaveSvc(12)
	l, _ := svc.Apply(ctx, LeaveInput{UserID: "u1", TypeCode: "annual", StartDate: "2026-08-17", EndDate: "2026-08-19", Reason: "x"})
	appr, err := svc.Approve(ctx, l.ID, "admin1")
	if err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	if appr.Status != domain.LeaveApproved {
		t.Fatalf("expected approved, got %s", appr.Status)
	}
}
