package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

const (
	// minOvertimeMinutes filters out accidental taps: under half an hour of
	// lembur is not worth an approval round.
	minOvertimeMinutes = 30
	// maxOvertimeMinutes is a hard cap per claim (12 hours), a guard against a
	// mistyped end time that would otherwise inflate payroll.
	maxOvertimeMinutes = 12 * 60
)

// OvertimeService implements lembur business logic. It records approved time
// only; converting minutes to money belongs to the (not yet built) payroll
// module, which can read SumApprovedMinutes.
type OvertimeService struct {
	repo domain.OvertimeRepository
}

// NewOvertimeService builds the service.
func NewOvertimeService(repo domain.OvertimeRepository) *OvertimeService {
	return &OvertimeService{repo: repo}
}

// SubmitInput is one overtime claim as sent by the employee app.
type SubmitInput struct {
	UserID   string
	WorkDate string // YYYY-MM-DD
	StartAt  string // RFC3339
	EndAt    string // RFC3339
	Reason   string
}

// Submit validates and stores a claim.
func (s *OvertimeService) Submit(ctx context.Context, in SubmitInput) (*domain.Overtime, error) {
	if in.UserID == "" {
		return nil, pkgerrors.Unauthorized("user tidak dikenal")
	}

	start, err := time.Parse(time.RFC3339, in.StartAt)
	if err != nil {
		return nil, pkgerrors.Validation("overtime", "format start_at salah (RFC3339)")
	}
	end, err := time.Parse(time.RFC3339, in.EndAt)
	if err != nil {
		return nil, pkgerrors.Validation("overtime", "format end_at salah (RFC3339)")
	}
	if !end.After(start) {
		return nil, pkgerrors.Validation("overtime", "jam selesai harus setelah jam mulai")
	}

	minutes := int(end.Sub(start).Minutes())
	if minutes < minOvertimeMinutes {
		return nil, pkgerrors.Validation("overtime",
			fmt.Sprintf("lembur minimal %d menit", minOvertimeMinutes))
	}
	if minutes > maxOvertimeMinutes {
		return nil, pkgerrors.Validation("overtime",
			fmt.Sprintf("lembur maksimal %d menit per pengajuan", maxOvertimeMinutes))
	}

	workDate := in.WorkDate
	if workDate == "" {
		workDate = start.Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", workDate); err != nil {
		return nil, pkgerrors.Validation("overtime", "format work_date salah (YYYY-MM-DD)")
	}

	overlaps, err := s.repo.ListOverlapping(ctx, in.UserID, start, end)
	if err != nil {
		return nil, err
	}
	if len(overlaps) > 0 {
		return nil, pkgerrors.ConflictMessage("overtime", "start_at", in.StartAt,
			"rentang waktu bertabrakan dengan pengajuan lembur lain yang masih pending/disetujui")
	}

	o := &domain.Overtime{
		UserID:   in.UserID,
		WorkDate: workDate,
		StartAt:  start.UTC(),
		EndAt:    end.UTC(),
		Minutes:  minutes,
		Reason:   strings.TrimSpace(in.Reason),
		Status:   domain.OvertimePending,
	}
	if err := s.repo.Create(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

// List returns claims filtered by status and/or user.
func (s *OvertimeService) List(ctx context.Context, status, userID string) ([]domain.Overtime, error) {
	if status != "" {
		switch domain.OvertimeStatus(status) {
		case domain.OvertimePending, domain.OvertimeApproved, domain.OvertimeRejected, domain.OvertimeCancelled:
		default:
			return nil, pkgerrors.Validation("overtime", "status tidak dikenal")
		}
	}
	return s.repo.List(ctx, status, userID)
}

// Get returns one claim.
func (s *OvertimeService) Get(ctx context.Context, id string) (*domain.Overtime, error) {
	o, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Approve marks a pending claim as approved.
func (s *OvertimeService) Approve(ctx context.Context, id, actorID string) (*domain.Overtime, error) {
	return s.decide(ctx, id, actorID, domain.OvertimeApproved, "")
}

// Reject marks a pending claim as rejected; a reason is mandatory.
func (s *OvertimeService) Reject(ctx context.Context, id, actorID, reason string) (*domain.Overtime, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, pkgerrors.Validation("overtime", "alasan penolakan wajib diisi")
	}
	return s.decide(ctx, id, actorID, domain.OvertimeRejected, strings.TrimSpace(reason))
}

func (s *OvertimeService) decide(ctx context.Context, id, actorID string, status domain.OvertimeStatus, reason string) (*domain.Overtime, error) {
	o, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if o.Status != domain.OvertimePending {
		return nil, pkgerrors.ConflictMessage("overtime", "status", string(o.Status),
			"hanya pengajuan berstatus pending yang bisa diputuskan")
	}
	now := time.Now().UTC()
	if err := s.repo.UpdateStatus(ctx, id, string(status), actorID, reason, &now); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// Cancel lets the owner withdraw a claim that has not been decided yet.
func (s *OvertimeService) Cancel(ctx context.Context, id, ownerID string) (*domain.Overtime, error) {
	o, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if o.UserID != ownerID {
		return nil, pkgerrors.Forbidden("tidak boleh membatalkan pengajuan orang lain")
	}
	if o.Status != domain.OvertimePending {
		return nil, pkgerrors.ConflictMessage("overtime", "status", string(o.Status),
			"hanya pengajuan pending yang bisa dibatalkan")
	}
	if err := s.repo.UpdateStatus(ctx, id, string(domain.OvertimeCancelled), ownerID, "", nil); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// MonthlyMinutes totals approved minutes for one user in a "YYYY-MM" month —
// the number payroll will multiply by an hourly rate.
func (s *OvertimeService) MonthlyMinutes(ctx context.Context, userID, month string) (int, error) {
	start, err := time.Parse("2006-01", month)
	if err != nil {
		return 0, pkgerrors.Validation("overtime", "format bulan salah (YYYY-MM)")
	}
	end := start.AddDate(0, 1, -1)
	return s.repo.SumApprovedMinutes(ctx, userID, start.Format("2006-01-02"), end.Format("2006-01-02"))
}
