package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// overtimeDAO is the flat row shape of overtimes (+ joined user columns).
type overtimeDAO struct {
	ID              string
	UserID          string
	UserName        string
	UserEmail       string
	WorkDate        string
	StartAt         string
	EndAt           string
	Minutes         int
	Reason          string
	Status          string
	ApprovedBy      sql.NullString
	ApprovedAt      sql.NullString
	RejectionReason string
	CreatedAt       string
	UpdatedAt       string
}

const overtimeColumns = `o.id, o.user_id, COALESCE(u.name, ''), COALESCE(u.email, ''),
	o.work_date, o.start_at, o.end_at, o.minutes, o.reason, o.status,
	o.approved_by, o.approved_at, o.rejection_reason, o.created_at, o.updated_at`

// OvertimeRepository is the SQLite implementation of domain.OvertimeRepository.
type OvertimeRepository struct {
	db *sql.DB
}

// NewOvertimeRepository builds the repository.
func NewOvertimeRepository(db *sql.DB) *OvertimeRepository {
	return &OvertimeRepository{db: db}
}

// Create inserts a claim, generating the id and timestamps when empty.
func (r *OvertimeRepository) Create(ctx context.Context, o *domain.Overtime) error {
	if o.ID == "" {
		o.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if o.CreatedAt.IsZero() {
		o.CreatedAt = now
	}
	o.UpdatedAt = now
	if o.Status == "" {
		o.Status = domain.OvertimePending
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO overtimes (id, user_id, work_date, start_at, end_at, minutes, reason,
		                       status, approved_by, approved_at, rejection_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.ID, o.UserID, o.WorkDate, formatTime(o.StartAt), formatTime(o.EndAt), o.Minutes,
		o.Reason, string(o.Status), nullString(o.ApprovedBy), nullTime(o.ApprovedAt),
		o.RejectionReason, formatTime(o.CreatedAt), formatTime(o.UpdatedAt))
	if err != nil {
		return fmt.Errorf("repo.OvertimeRepository.Create: %w", err)
	}
	return nil
}

// FindByID returns one claim or a not-found error.
func (r *OvertimeRepository) FindByID(ctx context.Context, id string) (*domain.Overtime, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+overtimeColumns+`
		FROM overtimes o LEFT JOIN users u ON u.id = o.user_id
		WHERE o.id = ?`, id)
	return scanOvertime(row)
}

// List returns claims filtered by status and/or user.
func (r *OvertimeRepository) List(ctx context.Context, status, userID string) ([]domain.Overtime, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+overtimeColumns+`
		FROM overtimes o LEFT JOIN users u ON u.id = o.user_id
		WHERE (? = '' OR o.status = ?) AND (? = '' OR o.user_id = ?)
		ORDER BY o.work_date DESC, o.start_at DESC`,
		status, status, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("repo.OvertimeRepository.List: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]domain.Overtime, 0)
	for rows.Next() {
		d, err := scanOvertimeRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, overtimeFromDAO(d))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo.OvertimeRepository.List: %w", err)
	}
	return out, nil
}

// ListOverlapping returns pending/approved claims intersecting [start, end).
func (r *OvertimeRepository) ListOverlapping(ctx context.Context, userID string, start, end time.Time) ([]domain.Overtime, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+overtimeColumns+`
		FROM overtimes o LEFT JOIN users u ON u.id = o.user_id
		WHERE o.user_id = ? AND o.status IN ('pending', 'approved')
		  AND o.start_at < ? AND o.end_at > ?
		ORDER BY o.start_at`,
		userID, formatTime(end), formatTime(start))
	if err != nil {
		return nil, fmt.Errorf("repo.OvertimeRepository.ListOverlapping: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]domain.Overtime, 0)
	for rows.Next() {
		d, err := scanOvertimeRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, overtimeFromDAO(d))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo.OvertimeRepository.ListOverlapping: %w", err)
	}
	return out, nil
}

// UpdateStatus moves a claim to a new status (approve/reject/cancel).
func (r *OvertimeRepository) UpdateStatus(ctx context.Context, id, status, actorID, reason string, approvedAt *time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE overtimes
		SET status = ?, approved_by = ?, approved_at = ?, rejection_reason = ?, updated_at = ?
		WHERE id = ?`,
		status, nullString(actorID), nullTime(approvedAt), reason, formatTime(time.Now().UTC()), id)
	if err != nil {
		return fmt.Errorf("repo.OvertimeRepository.UpdateStatus: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repo.OvertimeRepository.UpdateStatus: %w", err)
	}
	if n == 0 {
		return pkgerrors.NotFound("overtime", id)
	}
	return nil
}

// SumApprovedMinutes totals approved minutes inside a work_date range.
func (r *OvertimeRepository) SumApprovedMinutes(ctx context.Context, userID, fromDate, toDate string) (int, error) {
	var total sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT SUM(minutes) FROM overtimes
		WHERE user_id = ? AND status = 'approved' AND work_date BETWEEN ? AND ?`,
		userID, fromDate, toDate).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("repo.OvertimeRepository.SumApprovedMinutes: %w", err)
	}
	return int(total.Int64), nil
}

func scanOvertime(row *sql.Row) (*domain.Overtime, error) {
	var d overtimeDAO
	if err := row.Scan(&d.ID, &d.UserID, &d.UserName, &d.UserEmail, &d.WorkDate,
		&d.StartAt, &d.EndAt, &d.Minutes, &d.Reason, &d.Status, &d.ApprovedBy,
		&d.ApprovedAt, &d.RejectionReason, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.NotFound("overtime", "")
		}
		return nil, fmt.Errorf("repo.scanOvertime: %w", err)
	}
	o := overtimeFromDAO(d)
	return &o, nil
}

func scanOvertimeRow(rows *sql.Rows) (overtimeDAO, error) {
	var d overtimeDAO
	if err := rows.Scan(&d.ID, &d.UserID, &d.UserName, &d.UserEmail, &d.WorkDate,
		&d.StartAt, &d.EndAt, &d.Minutes, &d.Reason, &d.Status, &d.ApprovedBy,
		&d.ApprovedAt, &d.RejectionReason, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return d, fmt.Errorf("repo.scanOvertimeRow: %w", err)
	}
	return d, nil
}

func overtimeFromDAO(d overtimeDAO) domain.Overtime {
	o := domain.Overtime{
		ID:              d.ID,
		UserID:          d.UserID,
		UserName:        d.UserName,
		UserEmail:       d.UserEmail,
		WorkDate:        d.WorkDate,
		StartAt:         parseTime(d.StartAt),
		EndAt:           parseTime(d.EndAt),
		Minutes:         d.Minutes,
		Reason:          d.Reason,
		Status:          domain.OvertimeStatus(d.Status),
		RejectionReason: d.RejectionReason,
		CreatedAt:       parseTime(d.CreatedAt),
		UpdatedAt:       parseTime(d.UpdatedAt),
	}
	if d.ApprovedBy.Valid {
		o.ApprovedBy = d.ApprovedBy.String
	}
	if d.ApprovedAt.Valid {
		t := parseTime(d.ApprovedAt.String)
		o.ApprovedAt = &t
	}
	return o
}
