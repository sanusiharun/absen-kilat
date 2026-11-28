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
	ID              string
	UserID          string
	Type            string
	Status          string
	PhotoPath       string
	Latitude        sql.NullFloat64
	Longitude       sql.NullFloat64
	Address         string
	Notes           string
	Device          string
	IPAddress       string
	DistanceM       sql.NullInt64
	Source          string
	ApprovalStatus  string
	ApprovedBy      sql.NullString
	ApprovedAt      sql.NullString
	RejectionReason string
	LocationID      sql.NullString
	OccurredAt      string
	CreatedAt       string
}

const attendanceCols = `id, user_id, type, status, photo_path, latitude, longitude,
	address, notes, device, ip_address, distance_m, source, approval_status,
	approved_by, approved_at, rejection_reason, location_id, occurred_at, created_at`

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
	if a.Source == "" {
		a.Source = domain.SourceApp
	}
	if a.ApprovalStatus == "" {
		a.ApprovalStatus = domain.ApprovalPending
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO attendances (id, user_id, type, status, photo_path,
			latitude, longitude, address, notes, device, ip_address, distance_m,
			source, approval_status, approved_by, approved_at, rejection_reason, location_id,
			occurred_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.UserID, string(a.Type), string(a.Status), a.PhotoPath,
		nullFloat(a.Latitude), nullFloat(a.Longitude), a.Address, a.Notes,
		a.Device, a.IPAddress, nullInt(a.DistanceM),
		string(a.Source), string(a.ApprovalStatus),
		nullString(a.ApprovedBy), nullTime(a.ApprovedAt), a.RejectionReason,
		nullString(a.LocationID),
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

// UpdateAttendance persists the editable fields of an attendance record
// (type, status, occurred_at, approval state, location). Used by admin
// overrides; always write the whole record after mutating it.
func (r *AttendanceRepository) UpdateAttendance(ctx context.Context, a *domain.Attendance) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE attendances SET type = ?, status = ?, occurred_at = ?,
			approval_status = ?, approved_by = ?, approved_at = ?,
			rejection_reason = ?, location_id = ?
		WHERE id = ?`,
		string(a.Type), string(a.Status), formatTime(a.OccurredAt),
		string(a.ApprovalStatus), nullString(a.ApprovedBy), nullTime(a.ApprovedAt),
		a.RejectionReason, nullString(a.LocationID), a.ID)
	if err != nil {
		return fmt.Errorf("repo.AttendanceRepository.UpdateAttendance: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repo.AttendanceRepository.UpdateAttendance: rows: %w", err)
	}
	if n == 0 {
		return pkgerrors.NotFound("attendance", a.ID)
	}
	return nil
}

// InsertApprovalLog appends one entry to the approval audit trail.
func (r *AttendanceRepository) InsertApprovalLog(ctx context.Context, l *domain.ApprovalLog) error {
	if l.ID == "" {
		l.ID = uuid.NewString()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO approval_log (id, attendance_id, actor_id, action, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		l.ID, l.AttendanceID, l.ActorID, l.Action, l.Reason, formatTime(l.CreatedAt))
	if err != nil {
		return fmt.Errorf("repo.AttendanceRepository.InsertApprovalLog: %w", err)
	}
	return nil
}

// ListApprovalLogs returns the audit trail for an attendance, oldest first.
func (r *AttendanceRepository) ListApprovalLogs(ctx context.Context, attendanceID string) ([]domain.ApprovalLog, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, attendance_id, actor_id, action, reason, created_at
		FROM approval_log
		WHERE attendance_id = ?
		ORDER BY created_at ASC`, attendanceID)
	if err != nil {
		return nil, fmt.Errorf("repo.AttendanceRepository.ListApprovalLogs: %w", err)
	}
	defer rows.Close()

	var out []domain.ApprovalLog
	for rows.Next() {
		var l domain.ApprovalLog
		var createdAt string
		if err := rows.Scan(&l.ID, &l.AttendanceID, &l.ActorID, &l.Action,
			&l.Reason, &createdAt); err != nil {
			return nil, fmt.Errorf("repo.AttendanceRepository.ListApprovalLogs: scan: %w", err)
		}
		l.CreatedAt = parseTime(createdAt)
		out = append(out, l)
	}
	return out, rows.Err()
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
	ID              string
	UserID          string
	UserName        string
	UserEmail       string
	Type            string
	Status          string
	PhotoPath       string
	Address         string
	Latitude        sql.NullFloat64
	Longitude       sql.NullFloat64
	Device          string
	IPAddress       string
	DistanceM       sql.NullInt64
	Source          string
	ApprovalStatus  string
	ApprovedBy      sql.NullString
	ApprovedAt      sql.NullString
	RejectionReason string
	LocationID      sql.NullString
	OccurredAt      string
}

// AdminAttendanceFilter bounds the admin attendance listing. From is an
// inclusive start instant and To an exclusive end instant (UTC); zero times
// mean unbounded. Status/Type/Approval/Search are optional equality/substring
// filters. Limit is clamped to [1, 500] and Offset to >= 0 by the query.
type AdminAttendanceFilter struct {
	From           time.Time
	To             time.Time
	Status         string
	Type           string
	ApprovalStatus string
	Search         string
	Limit          int
	Offset         int
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
	if f.ApprovalStatus != "" {
		conds = append(conds, "a.approval_status = ?")
		args = append(args, f.ApprovalStatus)
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
			a.source, a.approval_status, a.approved_by, a.approved_at,
			a.rejection_reason, a.location_id, a.occurred_at
		FROM attendances a
		JOIN users u ON u.id = a.user_id`+where+`
		ORDER BY a.occurred_at DESC, a.id DESC
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
			&d.DistanceM, &d.Source, &d.ApprovalStatus, &d.ApprovedBy, &d.ApprovedAt,
			&d.RejectionReason, &d.LocationID, &d.OccurredAt); err != nil {
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
			return nil, fmt.Errorf("repo.AttendanceRepository.ListDevices: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// UserReportRow is a per-user attendance aggregation over a date window.
type UserReportRow struct {
	UserID        string
	Name          string
	Email         string
	Department    string
	PresentDays   int // distinct dates with a check_in (on_time or late)
	LateDays      int // distinct dates with a late check_in
	AbsentDays    int // working days in range minus PresentDays
	CheckInCount  int
	CheckOutCount int
	TotalMinutes  int // sum over days of (last check_out - first check_in)
	LastCheckIn   *time.Time
}

// UserReports aggregates attendance per user across [start, end). It joins the
// users table for display name/email/department and counts distinct
// check-in dates as "present days". Absent days = working days in range
// (calendar days, simplistic) minus present days; negative clamped to 0.
//
// Every attendance metric is bounded to [start, end) (check_in/check_out
// counts included), and total worked minutes are the per-day sum of
// (last check_out - first check_in); a day without a check_out adds zero.
func (r *AttendanceRepository) UserReports(ctx context.Context, start, end time.Time, workingDays int) ([]UserReportRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH daily AS (
			SELECT
				a.user_id AS user_id,
				DATE(a.occurred_at) AS day,
				COUNT(CASE WHEN a.type = 'check_in' THEN 1 END) AS ins,
				COUNT(CASE WHEN a.type = 'check_out' THEN 1 END) AS outs,
				MAX(CASE WHEN a.type = 'check_in' AND a.status = 'late' THEN 1 ELSE 0 END) AS late_ins,
				MIN(CASE WHEN a.type = 'check_in' THEN a.occurred_at END) AS first_in,
				MAX(CASE WHEN a.type = 'check_out' THEN a.occurred_at END) AS last_out
			FROM attendances a
			WHERE a.occurred_at >= ? AND a.occurred_at < ?
			GROUP BY a.user_id, DATE(a.occurred_at)
		)
		SELECT
			u.id, u.name, u.email, u.department,
			COALESCE(SUM(CASE WHEN d.ins > 0 THEN 1 ELSE 0 END), 0) AS present_days,
			COALESCE(SUM(d.late_ins), 0) AS late_days,
			COALESCE(SUM(d.ins), 0) AS checkin_count,
			COALESCE(SUM(d.outs), 0) AS checkout_count,
			COALESCE(CAST(ROUND(SUM(CASE
				WHEN d.first_in IS NOT NULL AND d.last_out IS NOT NULL AND d.last_out > d.first_in
				THEN (julianday(d.last_out) - julianday(d.first_in)) * 1440
				ELSE 0 END)) AS INTEGER), 0) AS total_minutes,
			MAX(CASE WHEN d.ins > 0 THEN d.first_in END) AS last_checkin
		FROM users u
		LEFT JOIN daily d ON d.user_id = u.id
		GROUP BY u.id, u.name, u.email, u.department
		ORDER BY u.name ASC`,
		formatTime(start), formatTime(end))
	if err != nil {
		return nil, fmt.Errorf("repo.AttendanceRepository.UserReports: query: %w", err)
	}
	defer rows.Close()

	out := make([]UserReportRow, 0)
	for rows.Next() {
		var row UserReportRow
		var lastCheckIn sql.NullString
		if err := rows.Scan(
			&row.UserID, &row.Name, &row.Email, &row.Department,
			&row.PresentDays, &row.LateDays, &row.CheckInCount, &row.CheckOutCount,
			&row.TotalMinutes, &lastCheckIn,
		); err != nil {
			return nil, fmt.Errorf("repo.AttendanceRepository.UserReports: scan: %w", err)
		}
		if lastCheckIn.Valid && lastCheckIn.String != "" {
			if t, perr := time.Parse(time.RFC3339Nano, lastCheckIn.String); perr == nil {
				row.LastCheckIn = &t
			}
		}
		if workingDays > 0 {
			absent := workingDays - row.PresentDays
			if absent < 0 {
				absent = 0
			}
			row.AbsentDays = absent
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// AttendanceApprovalFilter bounds the approval queue listing. Status is an
// optional equality filter on approval_status; ManagerID, when non-nil,
// scopes the listing to records owned by users whose manager_id matches
// (supervisor team scoping). Limit is clamped to [1, 500] and Offset to >= 0
// by the queries.
type AttendanceApprovalFilter struct {
	Status    string
	ManagerID *string
	Limit     int
	Offset    int
}

// ListForApproval returns attendance rows (joined with the owner's name and
// email) matching the filter, newest first.
func (r *AttendanceRepository) ListForApproval(ctx context.Context, f AttendanceApprovalFilter) ([]AdminAttendanceRow, error) {
	where, args := approvalWhere(f)
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
			a.source, a.approval_status, a.approved_by, a.approved_at,
			a.rejection_reason, a.location_id, a.occurred_at
		FROM attendances a
		JOIN users u ON u.id = a.user_id`+where+`
		ORDER BY a.occurred_at DESC, a.id DESC
		LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, fmt.Errorf("repo.AttendanceRepository.ListForApproval: %w", err)
	}
	defer rows.Close()

	var out []AdminAttendanceRow
	for rows.Next() {
		var d AdminAttendanceRow
		if err := rows.Scan(&d.ID, &d.UserID, &d.UserName, &d.UserEmail, &d.Type, &d.Status,
			&d.PhotoPath, &d.Address, &d.Latitude, &d.Longitude, &d.Device, &d.IPAddress,
			&d.DistanceM, &d.Source, &d.ApprovalStatus, &d.ApprovedBy, &d.ApprovedAt,
			&d.RejectionReason, &d.LocationID, &d.OccurredAt); err != nil {
			return nil, fmt.Errorf("repo.AttendanceRepository.ListForApproval: scan: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo.AttendanceRepository.ListForApproval: rows: %w", err)
	}
	return out, nil
}

// CountApproval returns the number of rows matching the filter (before
// limit/offset).
func (r *AttendanceRepository) CountApproval(ctx context.Context, f AttendanceApprovalFilter) (int, error) {
	where, args := approvalWhere(f)
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM attendances a JOIN users u ON u.id = a.user_id`+where,
		args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("repo.AttendanceRepository.CountApproval: %w", err)
	}
	return total, nil
}

// approvalWhere builds the WHERE clause and args for the approval listing.
func approvalWhere(f AttendanceApprovalFilter) (string, []any) {
	conds := []string{"1=1"}
	var args []any
	if f.Status != "" {
		conds = append(conds, "a.approval_status = ?")
		args = append(args, f.Status)
	}
	if f.ManagerID != nil && *f.ManagerID != "" {
		conds = append(conds, "u.manager_id = ?")
		args = append(args, *f.ManagerID)
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// UpdateApprovalStatus updates only the approval columns of an attendance
// record (approval_status, approved_by, approved_at, rejection_reason).
// The type/status/occurred_at columns are left untouched.
func (r *AttendanceRepository) UpdateApprovalStatus(ctx context.Context, id string, status domain.ApprovalStatus, approvedBy string, approvedAt time.Time, rejectionReason string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE attendances SET approval_status = ?, approved_by = ?,
			approved_at = ?, rejection_reason = ?
		WHERE id = ?`,
		string(status), nullString(approvedBy), formatTime(approvedAt),
		rejectionReason, id)
	if err != nil {
		return fmt.Errorf("repo.AttendanceRepository.UpdateApprovalStatus: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repo.AttendanceRepository.UpdateApprovalStatus: rows: %w", err)
	}
	if n == 0 {
		return pkgerrors.NotFound("attendance", id)
	}
	return nil
}

// scanAttendance scans a single row into a domain.Attendance.
func scanAttendance(row interface{ Scan(...any) error }) (*domain.Attendance, error) {
	var d attendanceDAO
	err := row.Scan(&d.ID, &d.UserID, &d.Type, &d.Status, &d.PhotoPath,
		&d.Latitude, &d.Longitude, &d.Address, &d.Notes, &d.Device, &d.IPAddress,
		&d.DistanceM, &d.Source, &d.ApprovalStatus, &d.ApprovedBy, &d.ApprovedAt,
		&d.RejectionReason, &d.LocationID, &d.OccurredAt, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.NotFound("attendance", "")
		}
		return nil, fmt.Errorf("repo.scanAttendance: %w", err)
	}
	a := &domain.Attendance{
		ID:              d.ID,
		UserID:          d.UserID,
		Type:            domain.AttendanceType(d.Type),
		Status:          domain.AttendanceStatus(d.Status),
		PhotoPath:       d.PhotoPath,
		Address:         d.Address,
		Notes:           d.Notes,
		Device:          d.Device,
		IPAddress:       d.IPAddress,
		Source:          domain.AttendanceSource(d.Source),
		ApprovalStatus:  domain.ApprovalStatus(d.ApprovalStatus),
		RejectionReason: d.RejectionReason,
		OccurredAt:      parseTime(d.OccurredAt),
		CreatedAt:       parseTime(d.CreatedAt),
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
	if d.ApprovedBy.Valid {
		a.ApprovedBy = d.ApprovedBy.String
	}
	if d.ApprovedAt.Valid {
		t := parseTime(d.ApprovedAt.String)
		a.ApprovedAt = &t
	}
	if d.LocationID.Valid {
		a.LocationID = d.LocationID.String
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
