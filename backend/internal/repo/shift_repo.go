package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
)

// ShiftRepository adalah implementasi SQLite untuk shift dan penugasannya.
type ShiftRepository struct {
	db *sql.DB
}

// NewShiftRepository creates a ShiftRepository.
func NewShiftRepository(db *sql.DB) *ShiftRepository { return &ShiftRepository{db: db} }

// constShiftCodeIndex adalah nama index unik lower(code) pada tabel shifts.
const constShiftCodeIndex = "idx_shifts_code"

// wrapShiftErr memetakan pelanggaran unique index kode ke error domain agar
// handler dapat menjawab 400 VALIDATION tanpa mencocokkan string SQLite sendiri.
func wrapShiftErr(op string, err error) error {
	if err != nil && strings.Contains(err.Error(), constShiftCodeIndex) {
		return fmt.Errorf("shift.%s: %w", op, domain.ErrShiftCodeExists)
	}
	return fmt.Errorf("shift.%s: %w", op, err)
}

// CreateShift menyimpan definisi shift baru.
func (r *ShiftRepository) CreateShift(ctx context.Context, s *domain.Shift) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO shifts (id, code, name, start_time, end_time, crosses_midnight,
		                     grace_minutes, break_minutes, location_id, active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Code, s.Name, s.StartTime, s.EndTime, boolToInt(s.CrossesMidnight),
		s.GraceMinutes, s.BreakMinutes, nullable(s.LocationID), boolToInt(s.Active), fmtTime(s.CreatedAt))
	if err != nil {
		return wrapShiftErr("CreateShift", err)
	}
	return nil
}

// ListShifts mengembalikan semua shift, urut kode.
func (r *ShiftRepository) ListShifts(ctx context.Context) ([]domain.Shift, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, code, name, start_time, end_time, crosses_midnight, grace_minutes,
		        break_minutes, COALESCE(location_id,''), active, created_at
		   FROM shifts ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("shift.ListShifts: %w", err)
	}
	defer rows.Close()

	var out []domain.Shift
	for rows.Next() {
		var s domain.Shift
		var crosses, active int
		var createdAt string
		if err := rows.Scan(&s.ID, &s.Code, &s.Name, &s.StartTime, &s.EndTime, &crosses,
			&s.GraceMinutes, &s.BreakMinutes, &s.LocationID, &active, &createdAt); err != nil {
			return nil, fmt.Errorf("shift.ListShifts: scan: %w", err)
		}
		s.CrossesMidnight = crosses == 1
		s.Active = active == 1
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, s)
	}
	return out, rows.Err()
}

// FindShiftByID mengembalikan satu shift atau domain.ErrShiftNotFound.
func (r *ShiftRepository) FindShiftByID(ctx context.Context, id string) (*domain.Shift, error) {
	var s domain.Shift
	var crosses, active int
	var createdAt string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, code, name, start_time, end_time, crosses_midnight, grace_minutes,
		        break_minutes, COALESCE(location_id,''), active, created_at
		   FROM shifts WHERE id = ?`, id).
		Scan(&s.ID, &s.Code, &s.Name, &s.StartTime, &s.EndTime, &crosses,
			&s.GraceMinutes, &s.BreakMinutes, &s.LocationID, &active, &createdAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrShiftNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("shift.FindShiftByID: %w", err)
	}
	s.CrossesMidnight = crosses == 1
	s.Active = active == 1
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &s, nil
}

// UpdateShift menyimpan perubahan definisi shift.
func (r *ShiftRepository) UpdateShift(ctx context.Context, s *domain.Shift) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE shifts SET code=?, name=?, start_time=?, end_time=?, crosses_midnight=?,
		        grace_minutes=?, break_minutes=?, location_id=?, active=? WHERE id=?`,
		s.Code, s.Name, s.StartTime, s.EndTime, boolToInt(s.CrossesMidnight),
		s.GraceMinutes, s.BreakMinutes, nullable(s.LocationID), boolToInt(s.Active), s.ID)
	if err != nil {
		return wrapShiftErr("UpdateShift", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("shift.UpdateShift: rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrShiftNotFound
	}
	return nil
}

// CreateAssignment menyimpan penugasan apa adanya. Aturan bentrok sengaja tidak
// diterapkan di sini (lihat komentar interface di domain.ShiftRepository).
func (r *ShiftRepository) CreateAssignment(ctx context.Context, a *domain.ShiftAssignment) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.Source == "" {
		a.Source = "manual"
	}

	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO shift_assignments (id, shift_id, user_id, start_date, end_date, source, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.ShiftID, nullable(a.UserID), a.StartDate, a.EndDate, a.Source, a.CreatedBy, fmtTime(a.CreatedAt)); err != nil {
		return fmt.Errorf("shift.CreateAssignment: %w", err)
	}
	return nil
}

// AssignmentsForUser mengembalikan penugasan yang beririsan dengan rentang.
func (r *ShiftRepository) AssignmentsForUser(ctx context.Context, userID, from, to string) ([]domain.ShiftAssignment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, shift_id, COALESCE(user_id,''), start_date, end_date, source, created_by, created_at
		   FROM shift_assignments
		  WHERE user_id = ? AND start_date <= ? AND end_date >= ?
		  ORDER BY start_date`, userID, to, from)
	if err != nil {
		return nil, fmt.Errorf("shift.AssignmentsForUser: %w", err)
	}
	defer rows.Close()

	var out []domain.ShiftAssignment
	for rows.Next() {
		var a domain.ShiftAssignment
		var createdAt string
		if err := rows.Scan(&a.ID, &a.ShiftID, &a.UserID, &a.StartDate, &a.EndDate, &a.Source, &a.CreatedBy, &createdAt); err != nil {
			return nil, fmt.Errorf("shift.AssignmentsForUser: scan: %w", err)
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, a)
	}
	return out, rows.Err()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
