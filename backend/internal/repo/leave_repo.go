package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// leaveTypeDAO maps a leave_types row.
type leaveTypeDAO struct {
	ID               string
	Code             string
	Name             string
	DefaultDays      int
	MaxExtraDays     int
	RequiresAttachment int
	Active           int
	CreatedAt        string
}

// leaveDAO maps a leaves row.
type leaveDAO struct {
	ID              string
	UserID          string
	LeaveType       string
	StartDate       string
	EndDate         string
	TotalDays       int
	Reason          string
	Status          string
	ApprovedBy      sql.NullString
	ApprovedAt      sql.NullString
	RejectionReason string
	ExtraDays       int
	AttachmentPath  string
	CreatedAt       string
	UpdatedAt       string
}

// LeaveTypeRepository is the SQLite implementation.
type LeaveTypeRepository struct {
	db *sql.DB
}

func NewLeaveTypeRepository(db *sql.DB) *LeaveTypeRepository {
	return &LeaveTypeRepository{db: db}
}

func (r *LeaveTypeRepository) ListActive(ctx context.Context) ([]domain.LeaveType, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, code, name, default_days, max_extra_days, requires_attachment, active, created_at
		 FROM leave_types WHERE active = 1 ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("repo.LeaveTypeRepository.ListActive: %w", err)
	}
	defer rows.Close()
	out := []domain.LeaveType{}
	for rows.Next() {
		var d leaveTypeDAO
		if err := rows.Scan(&d.ID, &d.Code, &d.Name, &d.DefaultDays, &d.MaxExtraDays,
			&d.RequiresAttachment, &d.Active, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("repo.LeaveTypeRepository.ListActive scan: %w", err)
		}
		out = append(out, leaveTypeFromDAO(d))
	}
	return out, rows.Err()
}

func (r *LeaveTypeRepository) FindByCode(ctx context.Context, code string) (*domain.LeaveType, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, code, name, default_days, max_extra_days, requires_attachment, active, created_at
		 FROM leave_types WHERE code = ?`, code)
	var d leaveTypeDAO
	if err := row.Scan(&d.ID, &d.Code, &d.Name, &d.DefaultDays, &d.MaxExtraDays,
		&d.RequiresAttachment, &d.Active, &d.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.NotFound("leave_type", code)
		}
		return nil, fmt.Errorf("repo.LeaveTypeRepository.FindByCode: %w", err)
	}
	lt := leaveTypeFromDAO(d)
	return &lt, nil
}

func leaveTypeFromDAO(d leaveTypeDAO) domain.LeaveType {
	return domain.LeaveType{
		ID:               d.ID,
		Code:             d.Code,
		Name:             d.Name,
		DefaultDays:      d.DefaultDays,
		MaxExtraDays:     d.MaxExtraDays,
		RequiresAttachment: d.RequiresAttachment == 1,
		Active:           d.Active == 1,
		CreatedAt:        parseTime(d.CreatedAt),
	}
}

// LeaveRepository is the SQLite implementation.
type LeaveRepository struct {
	db *sql.DB
}

func NewLeaveRepository(db *sql.DB) *LeaveRepository {
	return &LeaveRepository{db: db}
}

const leaveCols = `id, user_id, leave_type, start_date, end_date, total_days,
	reason, status, approved_by, approved_at, rejection_reason, extra_days, attachment_path,
	created_at, updated_at`

func (r *LeaveRepository) Create(ctx context.Context, l *domain.Leave) error {
	if l.ID == "" {
		l.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	l.CreatedAt = now
	l.UpdatedAt = now
	if l.Status == "" {
		l.Status = domain.LeavePending
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO leaves (id, user_id, leave_type, start_date, end_date, total_days,
			reason, status, approved_by, approved_at, rejection_reason, extra_days, attachment_path,
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.UserID, l.LeaveType, l.StartDate, l.EndDate, l.TotalDays,
		l.Reason, string(l.Status), nullString(l.ApprovedBy), nullTimeT(l.ApprovedAt),
		l.RejectionReason, l.ExtraDays, l.AttachmentPath,
		formatTime(now), formatTime(now),
	)
	if err != nil {
		return fmt.Errorf("repo.LeaveRepository.Create: %w", err)
	}
	return nil
}

func (r *LeaveRepository) FindByID(ctx context.Context, id string) (*domain.Leave, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+leaveCols+` FROM leaves WHERE id = ?`, id)
	return scanLeave(row)
}

func (r *LeaveRepository) List(ctx context.Context, status, userID string) ([]domain.Leave, error) {
	query := `SELECT ` + leaveCols + ` FROM leaves`
	var conds []string
	var args []any
	if userID != "" {
		conds = append(conds, `user_id = ?`)
		args = append(args, userID)
	}
	if status != "" {
		conds = append(conds, `status = ?`)
		args = append(args, status)
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY created_at DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("repo.LeaveRepository.List: %w", err)
	}
	defer rows.Close()
	out := []domain.Leave{}
	for rows.Next() {
		var d leaveDAO
		if err := rows.Scan(&d.ID, &d.UserID, &d.LeaveType, &d.StartDate, &d.EndDate,
			&d.TotalDays, &d.Reason, &d.Status, &d.ApprovedBy, &d.ApprovedAt, &d.RejectionReason,
			&d.ExtraDays, &d.AttachmentPath, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("repo.LeaveRepository.List scan: %w", err)
		}
		out = append(out, leaveFromDAO(d))
	}
	return out, rows.Err()
}

func (r *LeaveRepository) ListOverlapping(ctx context.Context, userID, start, end string) ([]domain.Leave, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+leaveCols+` FROM leaves
		WHERE user_id = ? AND status IN ('pending', 'approved')
		  AND start_date <= ? AND end_date >= ?`,
		userID, end, start)
	if err != nil {
		return nil, fmt.Errorf("repo.LeaveRepository.ListOverlapping: %w", err)
	}
	defer rows.Close()
	out := []domain.Leave{}
	for rows.Next() {
		var d leaveDAO
		if err := rows.Scan(&d.ID, &d.UserID, &d.LeaveType, &d.StartDate, &d.EndDate,
			&d.TotalDays, &d.Reason, &d.Status, &d.ApprovedBy, &d.ApprovedAt, &d.RejectionReason,
			&d.ExtraDays, &d.AttachmentPath, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("repo.LeaveRepository.ListOverlapping scan: %w", err)
		}
		out = append(out, leaveFromDAO(d))
	}
	return out, rows.Err()
}

func (r *LeaveRepository) CountApprovedDays(ctx context.Context, userID, leaveType string) (int, error) {
	year := time.Now().UTC().Format("2006")
	var total sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(total_days), 0)
		FROM leaves
		WHERE user_id = ? AND leave_type = ? AND status = 'approved'
		  AND strftime('%Y', created_at) = ?`,
		userID, leaveType, year).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("repo.LeaveRepository.CountApprovedDays: %w", err)
	}
	return int(total.Int64), nil
}

func (r *LeaveRepository) UpdateStatus(ctx context.Context, id, status, actorID, rejectionReason string, approvedAt *time.Time) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE leaves SET status = ?, approved_by = ?, approved_at = ?,
			rejection_reason = ?, updated_at = ? WHERE id = ?`,
		status, nullString(actorID), nullTimeT(approvedAt), rejectionReason,
		formatTime(now), id)
	if err != nil {
		return fmt.Errorf("repo.LeaveRepository.UpdateStatus: %w", err)
	}
	return nil
}

func (r *LeaveRepository) SetAttachment(ctx context.Context, id, path string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE leaves SET attachment_path = ?, updated_at = ? WHERE id = ?`,
		path, formatTime(time.Now().UTC()), id)
	if err != nil {
		return fmt.Errorf("repo.LeaveRepository.SetAttachment: %w", err)
	}
	return nil
}

func scanLeave(row *sql.Row) (*domain.Leave, error) {
	var d leaveDAO
	if err := row.Scan(&d.ID, &d.UserID, &d.LeaveType, &d.StartDate, &d.EndDate,
		&d.TotalDays, &d.Reason, &d.Status, &d.ApprovedBy, &d.ApprovedAt, &d.RejectionReason,
		&d.ExtraDays, &d.AttachmentPath, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.NotFound("leave", "")
		}
		return nil, fmt.Errorf("repo.scanLeave: %w", err)
	}
	var l = leaveFromDAO(d)
	return &l, nil
}

func leaveFromDAO(d leaveDAO) domain.Leave {
	l := domain.Leave{
		ID:              d.ID,
		UserID:          d.UserID,
		LeaveType:       d.LeaveType,
		StartDate:       d.StartDate,
		EndDate:         d.EndDate,
		TotalDays:       d.TotalDays,
		Reason:          d.Reason,
		Status:          domain.LeaveStatus(d.Status),
		RejectionReason: d.RejectionReason,
		ExtraDays:       d.ExtraDays,
		AttachmentPath:  d.AttachmentPath,
		CreatedAt:       parseTime(d.CreatedAt),
		UpdatedAt:       parseTime(d.UpdatedAt),
	}
	if d.ApprovedBy.Valid {
		l.ApprovedBy = d.ApprovedBy.String
	}
	if d.ApprovedAt.Valid {
		t := parseTime(d.ApprovedAt.String)
		l.ApprovedAt = &t
	}
	return l
}

var _ domain.LeaveRepository = (*LeaveRepository)(nil)
var _ domain.LeaveTypeRepository = (*LeaveTypeRepository)(nil)
