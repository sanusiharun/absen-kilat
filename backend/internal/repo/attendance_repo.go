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

// attendanceDAO maps an attendances table row to/from a domain.Attendance.
type attendanceDAO struct {
	ID         string
	UserID     string
	Type       string
	Status     string
	PhotoPath  string
	Latitude   sql.NullFloat64
	Longitude  sql.NullFloat64
	Address    string
	Notes      string
	Device     string
	IPAddress  string
	DistanceM  sql.NullInt64
	OccurredAt string
	CreatedAt  string
}

const attendanceCols = `id, user_id, type, status, photo_path, latitude, longitude,
	address, notes, device, ip_address, distance_m, occurred_at, created_at`

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
			latitude, longitude, address, notes, device, ip_address, distance_m,
			occurred_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.UserID, string(a.Type), string(a.Status), a.PhotoPath,
		nullFloat(a.Latitude), nullFloat(a.Longitude), a.Address, a.Notes,
		a.Device, a.IPAddress, nullInt(a.DistanceM),
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
		SELECT `+attendanceCols+`
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
	row := r.db.QueryRowContext(ctx,
		`SELECT `+attendanceCols+` FROM attendances WHERE id = ?`, id)
	return scanAttendance(row)
}

// CheckInStatusCountsOnDate returns the number of distinct users who
// checked in within [start, end), split by status (on_time / late).
func (r *AttendanceRepository) CheckInStatusCountsOnDate(ctx context.Context, start, end time.Time) (present, late int, err error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT status, COUNT(DISTINCT user_id)
		FROM attendances
		WHERE type = 'check_in' AND occurred_at >= ? AND occurred_at < ?
		GROUP BY status`, formatTime(start), formatTime(end))
	if err != nil {
		return 0, 0, fmt.Errorf("repo.AttendanceRepository.CheckInStatusCountsOnDate: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return 0, 0, fmt.Errorf("repo.AttendanceRepository.CheckInStatusCountsOnDate: scan: %w", err)
		}
		if status == string(domain.StatusOnTime) {
			present = n
		} else if status == string(domain.StatusLate) {
			late = n
		}
	}
	return present, late, rows.Err()
}

// TopLocations returns the most frequent attendance addresses, ordered by
// count descending, limited to limit rows. Empty addresses are excluded.
func (r *AttendanceRepository) TopLocations(ctx context.Context, limit int) ([]domain.LocationStat, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT address, COUNT(*)
		FROM attendances
		WHERE address != ''
		GROUP BY address
		ORDER BY COUNT(*) DESC, address ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("repo.AttendanceRepository.TopLocations: %w", err)
	}
	defer rows.Close()

	var out []domain.LocationStat
	for rows.Next() {
		var l domain.LocationStat
		if err := rows.Scan(&l.Location, &l.Count); err != nil {
			return nil, fmt.Errorf("repo.AttendanceRepository.TopLocations: scan: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// AdminAttendanceRow is one attendances row joined with its user (name and
// email), used by the admin attendance listing.
type AdminAttendanceRow struct {
	ID         string
	UserID     string
	UserName   string
	UserEmail  string
	Type       string
	Status     string
	PhotoPath  string
	Address    string
	Latitude   sql.NullFloat64
	Longitude  sql.NullFloat64
	Device     string
	IPAddress  string
	DistanceM  sql.NullInt64
	OccurredAt string
}

// AdminAttendanceFilter bounds the admin attendance listing. From is an
// inclusive start instant and To an exclusive end instant (UTC); zero times
// mean unbounded. Status/Type/Search are optional equality/substring
// filters. Limit is clamped to [1, 500] and Offset to >= 0 by the query.
type AdminAttendanceFilter struct {
	From   time.Time
	To     time.Time
	Status string
	Type   string
	Search string
	Limit  int
	Offset int
}

// ListAdmin returns attendance rows joined with user name/email, newest
// first, plus the total number of matching rows (before limit/offset).
func (r *AttendanceRepository) ListAdmin(ctx context.Context, f AdminAttendanceFilter) ([]AdminAttendanceRow, int, error) {
	conds := []string{"1=1"}
	var args []any
	if !f.From.IsZero() {
		conds = append(conds, "a.occurred_at >= ?")
		args = append(args, formatTime(f.From))
	}
	if !f.To.IsZero() {
		conds = append(conds, "a.occurred_at < ?")
		args = append(args, formatTime(f.To))
	}
	if f.Status != "" {
		conds = append(conds, "a.status = ?")
		args = append(args, f.Status)
	}
	if f.Type != "" {
		conds = append(conds, "a.type = ?")
		args = append(args, f.Type)
	}
	if f.Search != "" {
		conds = append(conds, "(u.name LIKE ? COLLATE NOCASE OR u.email LIKE ? COLLATE NOCASE)")
		like := "%" + f.Search + "%"
		args = append(args, like, like)
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM attendances a JOIN users u ON u.id = a.user_id`+where,
		args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repo.AttendanceRepository.ListAdmin: count: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id, a.user_id, u.name, u.email, a.type, a.status, a.photo_path,
			a.address, a.latitude, a.longitude, a.device, a.ip_address, a.distance_m,
			a.occurred_at
		FROM attendances a
		JOIN users u ON u.id = a.user_id`+where+`
		ORDER BY a.occurred_at DESC
		LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("repo.AttendanceRepository.ListAdmin: %w", err)
	}
	defer rows.Close()

	var out []AdminAttendanceRow
	for rows.Next() {
		var d AdminAttendanceRow
		if err := rows.Scan(&d.ID, &d.UserID, &d.UserName, &d.UserEmail, &d.Type, &d.Status,
			&d.PhotoPath, &d.Address, &d.Latitude, &d.Longitude, &d.Device, &d.IPAddress,
			&d.DistanceM, &d.OccurredAt); err != nil {
			return nil, 0, fmt.Errorf("repo.AttendanceRepository.ListAdmin: scan: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repo.AttendanceRepository.ListAdmin: rows: %w", err)
	}
	return out, total, nil
}

// ListDevices returns the attendance device identifiers ordered by record
// count descending. Empty device values are excluded.
func (r *AttendanceRepository) ListDevices(ctx context.Context) ([]domain.DeviceStat, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT device, COUNT(*)
		FROM attendances
		WHERE device != ''
		GROUP BY device
		ORDER BY COUNT(*) DESC, device ASC`)
	if err != nil {
		return nil, fmt.Errorf("repo.AttendanceRepository.ListDevices: %w", err)
	}
	defer rows.Close()

	var out []domain.DeviceStat
	for rows.Next() {
		var d domain.DeviceStat
		if err := rows.Scan(&d.Device, &d.Count); err != nil {
			return nil, fmt.Errorf("repo.AttendanceRepository.ListDevices: scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// scanAttendance scans a single row into a domain.Attendance.
func scanAttendance(row interface{ Scan(...any) error }) (*domain.Attendance, error) {
	var d attendanceDAO
	err := row.Scan(&d.ID, &d.UserID, &d.Type, &d.Status, &d.PhotoPath,
		&d.Latitude, &d.Longitude, &d.Address, &d.Notes, &d.Device, &d.IPAddress,
		&d.DistanceM, &d.OccurredAt, &d.CreatedAt)
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
		Address:    d.Address,
		Notes:      d.Notes,
		Device:     d.Device,
		IPAddress:  d.IPAddress,
		OccurredAt: parseTime(d.OccurredAt),
		CreatedAt:  parseTime(d.CreatedAt),
	}
	if d.Latitude.Valid {
		a.Latitude = &d.Latitude.Float64
	}
	if d.Longitude.Valid {
		a.Longitude = &d.Longitude.Float64
	}
	if d.DistanceM.Valid {
		dm := int(d.DistanceM.Int64)
		a.DistanceM = &dm
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

// nullInt converts a *int to sql.NullInt64.
func nullInt(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}
