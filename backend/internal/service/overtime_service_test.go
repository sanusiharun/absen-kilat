package service

import (
	"context"
	"testing"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// fakeOvertimeRepo is an in-memory OvertimeRepository for service tests.
type fakeOvertimeRepo struct {
	items []domain.Overtime
}

func (f *fakeOvertimeRepo) Create(ctx context.Context, o *domain.Overtime) error {
	if o.ID == "" {
		o.ID = "ot-" + time.Now().UTC().Format("150405.000000000")
	}
	if o.Status == "" {
		o.Status = domain.OvertimePending
	}
	f.items = append(f.items, *o)
	return nil
}

func (f *fakeOvertimeRepo) FindByID(ctx context.Context, id string) (*domain.Overtime, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			cp := f.items[i]
			return &cp, nil
		}
	}
	return nil, pkgerrors.NotFound("overtime", id)
}

func (f *fakeOvertimeRepo) List(ctx context.Context, status, userID string) ([]domain.Overtime, error) {
	out := make([]domain.Overtime, 0, len(f.items))
	for _, o := range f.items {
		if status != "" && string(o.Status) != status {
			continue
		}
		if userID != "" && o.UserID != userID {
			continue
		}
		out = append(out, o)
	}
	return out, nil
}

func (f *fakeOvertimeRepo) ListOverlapping(ctx context.Context, userID string, start, end time.Time) ([]domain.Overtime, error) {
	out := make([]domain.Overtime, 0)
	for _, o := range f.items {
		if o.UserID != userID {
			continue
		}
		if o.Status != domain.OvertimePending && o.Status != domain.OvertimeApproved {
			continue
		}
		if o.StartAt.Before(end) && o.EndAt.After(start) {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeOvertimeRepo) UpdateStatus(ctx context.Context, id, status, actorID, reason string, approvedAt *time.Time) error {
	for i := range f.items {
		if f.items[i].ID != id {
			continue
		}
		f.items[i].Status = domain.OvertimeStatus(status)
		f.items[i].ApprovedBy = actorID
		f.items[i].RejectionReason = reason
		f.items[i].ApprovedAt = approvedAt
		return nil
	}
	return pkgerrors.NotFound("overtime", id)
}

func (f *fakeOvertimeRepo) SumApprovedMinutes(ctx context.Context, userID, fromDate, toDate string) (int, error) {
	total := 0
	for _, o := range f.items {
		if o.UserID != userID || o.Status != domain.OvertimeApproved {
			continue
		}
		if o.WorkDate >= fromDate && o.WorkDate <= toDate {
			total += o.Minutes
		}
	}
	return total, nil
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad fixture time %q: %v", s, err)
	}
	return tt
}

func TestOvertimeSubmitValid(t *testing.T) {
	repo := &fakeOvertimeRepo{}
	svc := NewOvertimeService(repo)

	o, err := svc.Submit(context.Background(), SubmitInput{
		UserID:   "u1",
		WorkDate: "2026-11-20",
		StartAt:  "2026-11-20T14:00:00Z",
		EndAt:    "2026-11-20T16:30:00Z",
		Reason:   "  penyelaman malam  ",
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if o.Minutes != 150 {
		t.Fatalf("minutes = %d, want 150", o.Minutes)
	}
	if o.Status != domain.OvertimePending {
		t.Fatalf("status = %q, want pending", o.Status)
	}
	if o.Reason != "penyelaman malam" {
		t.Fatalf("reason not trimmed: %q", o.Reason)
	}
	if o.WorkDate != "2026-11-20" {
		t.Fatalf("work_date = %q", o.WorkDate)
	}
}

func TestOvertimeSubmitRejectsBadRanges(t *testing.T) {
	svc := NewOvertimeService(&fakeOvertimeRepo{})
	cases := []struct {
		name     string
		input    SubmitInput
		wantCode string
	}{
		{
			name:     "end before start",
			input:    SubmitInput{UserID: "u1", StartAt: "2026-11-20T16:00:00Z", EndAt: "2026-11-20T15:00:00Z"},
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:     "under 30 minutes",
			input:    SubmitInput{UserID: "u1", StartAt: "2026-11-20T16:00:00Z", EndAt: "2026-11-20T16:20:00Z"},
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:     "over 12 hours",
			input:    SubmitInput{UserID: "u1", StartAt: "2026-11-20T06:00:00Z", EndAt: "2026-11-20T20:00:00Z"},
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:     "bad time format",
			input:    SubmitInput{UserID: "u1", StartAt: "20-11-2026 16:00", EndAt: "2026-11-20T18:00:00Z"},
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:     "no user",
			input:    SubmitInput{StartAt: "2026-11-20T16:00:00Z", EndAt: "2026-11-20T18:00:00Z"},
			wantCode: "UNAUTHORIZED",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.Submit(context.Background(), c.input)
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := httpxCode(err); got != c.wantCode {
				t.Fatalf("error code = %q, want %q (err: %v)", got, c.wantCode, err)
			}
		})
	}
}

func TestOvertimeSubmitRejectsOverlap(t *testing.T) {
	repo := &fakeOvertimeRepo{}
	svc := NewOvertimeService(repo)

	if _, err := svc.Submit(context.Background(), SubmitInput{
		UserID: "u1", StartAt: "2026-11-20T14:00:00Z", EndAt: "2026-11-20T17:00:00Z",
	}); err != nil {
		t.Fatalf("first submit: %v", err)
	}

	_, err := svc.Submit(context.Background(), SubmitInput{
		UserID: "u1", StartAt: "2026-11-20T16:00:00Z", EndAt: "2026-11-20T18:00:00Z",
	})
	if err == nil {
		t.Fatal("overlapping claim must be rejected")
	}
	if got := httpxCode(err); got != "CONFLICT" {
		t.Fatalf("error code = %q, want CONFLICT", got)
	}

	// A different user may claim the same window.
	if _, err := svc.Submit(context.Background(), SubmitInput{
		UserID: "u2", StartAt: "2026-11-20T16:00:00Z", EndAt: "2026-11-20T18:00:00Z",
	}); err != nil {
		t.Fatalf("other user's claim must be allowed: %v", err)
	}
}

func TestOvertimeApproveRejectCancel(t *testing.T) {
	repo := &fakeOvertimeRepo{}
	svc := NewOvertimeService(repo)
	ctx := context.Background()

	o, err := svc.Submit(ctx, SubmitInput{UserID: "u1", StartAt: "2026-11-20T14:00:00Z", EndAt: "2026-11-20T16:00:00Z"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	approved, err := svc.Approve(ctx, o.ID, "sup1")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != domain.OvertimeApproved || approved.ApprovedBy != "sup1" {
		t.Fatalf("unexpected approved claim: %+v", approved)
	}

	// Deciding twice is a conflict, not a silent overwrite.
	if _, err := svc.Approve(ctx, o.ID, "sup1"); err == nil {
		t.Fatal("second decision must conflict")
	} else if got := httpxCode(err); got != "CONFLICT" {
		t.Fatalf("error code = %q, want CONFLICT", got)
	}

	// Reject requires a reason.
	o2, err := svc.Submit(ctx, SubmitInput{UserID: "u1", StartAt: "2026-11-21T14:00:00Z", EndAt: "2026-11-21T16:00:00Z"})
	if err != nil {
		t.Fatalf("submit 2: %v", err)
	}
	if _, err := svc.Reject(ctx, o2.ID, "sup1", "   "); err == nil {
		t.Fatal("reject without reason must fail")
	}
	rejected, err := svc.Reject(ctx, o2.ID, "sup1", "bukan lembur terjadwal")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != domain.OvertimeRejected || rejected.RejectionReason == "" {
		t.Fatalf("unexpected rejected claim: %+v", rejected)
	}

	// Only the owner can cancel, and only while pending.
	o3, err := svc.Submit(ctx, SubmitInput{UserID: "u1", StartAt: "2026-11-22T14:00:00Z", EndAt: "2026-11-22T16:00:00Z"})
	if err != nil {
		t.Fatalf("submit 3: %v", err)
	}
	if _, err := svc.Cancel(ctx, o3.ID, "someone-else"); err == nil {
		t.Fatal("cancel by another user must be forbidden")
	} else if got := httpxCode(err); got != "FORBIDDEN" && got != "UNAUTHORIZED" {
		t.Fatalf("error code = %q, want FORBIDDEN", got)
	}
	cancelled, err := svc.Cancel(ctx, o3.ID, "u1")
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.Status != domain.OvertimeCancelled {
		t.Fatalf("status = %q, want cancelled", cancelled.Status)
	}
}

func TestOvertimeListValidatesStatus(t *testing.T) {
	svc := NewOvertimeService(&fakeOvertimeRepo{})
	if _, err := svc.List(context.Background(), "bogus", ""); err == nil {
		t.Fatal("unknown status must be rejected")
	}
	list, err := svc.List(context.Background(), "pending", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("len = %d, want 0", len(list))
	}
}

func TestOvertimeMonthlyMinutesCountsApprovedOnly(t *testing.T) {
	repo := &fakeOvertimeRepo{}
	svc := NewOvertimeService(repo)
	ctx := context.Background()

	a, _ := svc.Submit(ctx, SubmitInput{UserID: "u1", StartAt: "2026-11-20T14:00:00Z", EndAt: "2026-11-20T16:00:00Z"})
	b, _ := svc.Submit(ctx, SubmitInput{UserID: "u1", StartAt: "2026-11-21T14:00:00Z", EndAt: "2026-11-21T15:00:00Z"})
	// pending one must not count
	if _, err := svc.Submit(ctx, SubmitInput{UserID: "u1", StartAt: "2026-11-22T14:00:00Z", EndAt: "2026-11-22T17:00:00Z"}); err != nil {
		t.Fatalf("submit pending: %v", err)
	}
	if _, err := svc.Approve(ctx, a.ID, "sup1"); err != nil {
		t.Fatalf("approve a: %v", err)
	}
	if _, err := svc.Approve(ctx, b.ID, "sup1"); err != nil {
		t.Fatalf("approve b: %v", err)
	}
	// outside the month
	c, _ := svc.Submit(ctx, SubmitInput{UserID: "u1", StartAt: "2026-12-01T14:00:00Z", EndAt: "2026-12-01T16:00:00Z"})
	if _, err := svc.Approve(ctx, c.ID, "sup1"); err != nil {
		t.Fatalf("approve c: %v", err)
	}

	got, err := svc.MonthlyMinutes(ctx, "u1", "2026-11")
	if err != nil {
		t.Fatalf("MonthlyMinutes: %v", err)
	}
	if got != 180 {
		t.Fatalf("minutes = %d, want 180 (120 approved + 60 approved; pending and December excluded)", got)
	}

	if _, err := svc.MonthlyMinutes(ctx, "u1", "11-2026"); err == nil {
		t.Fatal("bad month format must be rejected")
	}
}

// httpxCode digs the domain error code out of pkgerrors values without importing
// the transport package (service tests stay at the service layer).
func httpxCode(err error) string {
	switch e := err.(type) {
	case *pkgerrors.ErrValidation:
		return "VALIDATION_ERROR"
	case *pkgerrors.ErrConflict:
		return "CONFLICT"
	case *pkgerrors.ErrNotFound:
		return "NOT_FOUND"
	case *pkgerrors.ErrUnauthorized:
		return "UNAUTHORIZED"
	case *pkgerrors.ErrForbidden:
		return "FORBIDDEN"
	default:
		_ = e
		return "OTHER"
	}
}

var _ = mustTime
