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

// attendanceDAO maps an attendances table row to/from a domain.Attendance.
type attendanceDAO struct {
	ID         string
	UserID     string
	Type       string
	Status     string
	PhotoPath  string
	Latitude   sql.NullFloat64
	Longitude  sql.NullFloat64
	OccurredAt string
	CreatedAt  string
}

// AttendanceRepository is the SQLite implementation of domain.AttendanceRepository.
type AttendanceRepository struct {
	db *sql.DB
}

// NewAttendanceRepository creates an AttendanceRepository backed by the pool.
func NewAttendanceRepository(db *sql.DB) *AttendanceRepository {
	return &AttendanceRepository{db: db}
}

func (r *AttendanceRepository) Create(ctx context.Context, a *domain.Attendance) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO attendances (id, user_id, type, status, photo_path,
			latitude, longitude, occurred_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.UserID, string(a.Type), string(a.Status), a.PhotoPath,
		nullFloat(a.Latitude), nullFloat(a.Longitude),
		formatTime(a.OccurredAt), formatTime(a.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("repo.AttendanceRepository.Create: %w", err)
	}
	return nil
}

func (r *AttendanceRepository) ListByUserDate(ctx context.Context, userID string, date time.Time, loc *time.Location) ([]domain.Attendance, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc).UTC()
	end := start.Add(24 * time.Hour)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, type, status, photo_path, latitude, longitude,
		       occurred_at, created_at
		FROM attendances
		WHERE user_id = ? AND occurred_at >= ? AND occurred_at < ?
		ORDER BY occurred_at ASC`, userID, formatTime(start), formatTime(end))
	if err != nil {
		return nil, fmt.Errorf("repo.AttendanceRepository.ListByUserDate: %w", err)
	}
	defer rows.Close()

	var out []domain.Attendance
	for rows.Next() {
		a, err := scanAttendance(rows)
		if err != nil {
			return nil, fmt.Errorf("repo.AttendanceRepository.ListByUserDate: %w", err)
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo.AttendanceRepository.ListByUserDate: rows: %w", err)
	}
	return out, nil
}

func (r *AttendanceRepository) FindByID(ctx context.Context, id string) (*domain.Attendance, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, type, status, photo_path, latitude, longitude,
		       occurred_at, created_at
		FROM attendances WHERE id = ?`, id)
	return scanAttendance(row)
}

// scanAttendance scans a single row into a domain.Attendance.
func scanAttendance(row interface{ Scan(...any) error }) (*domain.Attendance, error) {
	var d attendanceDAO
	err := row.Scan(&d.ID, &d.UserID, &d.Type, &d.Status, &d.PhotoPath,
		&d.Latitude, &d.Longitude, &d.OccurredAt, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.NotFound("attendance", "")
		}
		return nil, fmt.Errorf("repo.scanAttendance: %w", err)
	}
	a := &domain.Attendance{
		ID:         d.ID,
		UserID:     d.UserID,
		Type:       domain.AttendanceType(d.Type),
		Status:     domain.AttendanceStatus(d.Status),
		PhotoPath:  d.PhotoPath,
		OccurredAt: parseTime(d.OccurredAt),
		CreatedAt:  parseTime(d.CreatedAt),
	}
	if d.Latitude.Valid {
		a.Latitude = &d.Latitude.Float64
	}
	if d.Longitude.Valid {
		a.Longitude = &d.Longitude.Float64
	}
	return a, nil
}

// nullFloat converts a *float64 to sql.NullFloat64.
func nullFloat(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}
