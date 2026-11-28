// Package domain defines the core models and repository interfaces for
// Absen Kilat. Layering rule: transport → service → repo → DAO, so
// repository interfaces live here while implementations live in repo/.
package domain

import (
	"context"
	"time"
)

// UserRole distinguishes privileged (super_admin/hr/supervisor) from
// regular employee accounts.
type UserRole string

const (
	RoleSuperAdmin UserRole = "super_admin"
	RoleHR         UserRole = "hr"
	RoleSupervisor UserRole = "supervisor"
	RoleEmployee   UserRole = "employee"
)

// LeaveType is a master leave category (annual/sick/emergency/maternity/paternity).
type LeaveType struct {
	ID                 string
	Code               string // annual | sick | emergency | maternity | paternity
	Name               string
	DefaultDays        int
	MaxExtraDays       int // paternity 3/5/7 option
	RequiresAttachment bool
	Active             bool
	CreatedAt          time.Time
}

// LeaveStatus is the lifecycle of a leave request.
type LeaveStatus string

const (
	LeavePending   LeaveStatus = "pending"
	LeaveApproved  LeaveStatus = "approved"
	LeaveRejected  LeaveStatus = "rejected"
	LeaveCancelled LeaveStatus = "cancelled"
)

// Leave is an employee leave request.
type Leave struct {
	ID              string
	UserID          string
	LeaveType       string // leave_types.code
	StartDate       string // YYYY-MM-DD
	EndDate         string // YYYY-MM-DD
	TotalDays       int    // working days
	Reason          string
	Status          LeaveStatus
	ApprovedBy      string
	ApprovedAt      *time.Time
	RejectionReason string
	ExtraDays       int // paternity 3/5/7
	AttachmentPath  string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// User is an application user who records attendances.
type User struct {
	ID                  string
	Email               string
	PasswordHash        string
	Name                string
	AvatarPath          string
	IsEmailVerified     bool
	NotificationEnabled bool
	Role                UserRole
	IsActive            bool
	// Profile fields (migration 006): employee identity & hierarchy.
	NIP          string
	Phone        string
	Department   string
	Position     string
	Address      string
	JoinDate     string // YYYY-MM-DD
	ManagerID    string // supervisor (users.id); "" when none
	EmployeeCode string // unique identifier (QR/manual attendance)
	// Leave quota (migration 008): English column names.
	LeaveQuota int // annual allocation
	LeaveUsed  int // consumed working days
	CreatedAt  time.Time
	UpdatedAt  time.Time
	// CredentialVersion dinaikkan setiap password berubah; token dengan versi lama ditolak.
	CredentialVersion int
}

// AttendanceType distinguishes check-in from check-out records.
type AttendanceType string

const (
	AttendanceCheckIn  AttendanceType = "check_in"
	AttendanceCheckOut AttendanceType = "check_out"
)

// AttendanceStatus is derived at creation time.
type AttendanceStatus string

const (
	StatusOnTime AttendanceStatus = "on_time"
	StatusLate   AttendanceStatus = "late"
)

// AttendanceSource records how an attendance was captured.
type AttendanceSource string

const (
	SourceApp    AttendanceSource = "app"    // from the mobile app
	SourceQR     AttendanceSource = "qr"     // QR scan (public web form)
	SourceManual AttendanceSource = "manual" // admin-created/override
)

// ApprovalStatus is the lifecycle state of an attendance record.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
)

// Attendance is a single check-in or check-out record.
type Attendance struct {
	ID         string
	UserID     string
	Type       AttendanceType
	Status     AttendanceStatus
	PhotoPath  string
	Latitude   *float64
	Longitude  *float64
	Address    string
	Notes      string
	Device     string
	IPAddress  string
	DistanceM  *int // meters from the nearest/selected location (geofencing)
	OccurredAt time.Time
	CreatedAt  time.Time
	// Approval & source (migration 006).
	Source          AttendanceSource
	ApprovalStatus  ApprovalStatus
	ApprovedBy      string
	ApprovedAt      *time.Time
	RejectionReason string
	LocationID      string // locations.id the attendance is tied to ("" when none)
}

// Location is a geofence point (office/client/field) used for attendance.
type Location struct {
	ID        string
	Name      string
	Type      string // office | client | field
	Lat       float64
	Lng       float64
	RadiusM   int // allowed radius in meters
	Active    bool
	CreatedAt time.Time
}

// QrToken is a QR attendance token bound to a location. Tokens are
// permanent per location until revoked (active=0) or expired.
type QrToken struct {
	ID         string
	LocationID string
	Token      string
	Active     bool
	ExpiresAt  *time.Time // nil = never expires
	CreatedBy  string
	CreatedAt  time.Time
}

// QrTokenView is a token joined with its location name (admin listing).
type QrTokenView struct {
	ID           string
	LocationID   string
	LocationName string
	Token        string
	Active       bool
	ExpiresAt    *time.Time
	CreatedBy    string
	CreatedAt    time.Time
}

// ApprovalLog is one entry of the approval audit trail.
type ApprovalLog struct {
	ID           string
	AttendanceID string
	ActorID      string
	Action       string // approve | reject | override
	Reason       string
	CreatedAt    time.Time
}

// Settings is the system configuration (work hours, mode, photo policy).
type Settings struct {
	WorkStart   string   // HH:MM
	WorkEnd     string   // HH:MM
	AbsenMode   string   // both | checkin_only
	Workdays    []string // ["mon".."sun"]
	PhotoPolicy string   // radius | always | off
}

// Notification is a user-facing message shown in the Notifications tab.
type Notification struct {
	ID        string
	UserID    string
	Title     string
	Body      string
	IsRead    bool
	CreatedAt time.Time
}

// LocationStat is an aggregate of attendance records grouped by address.
type LocationStat struct {
	Location string
	Count    int
}

// DeviceStat is an aggregate of attendance records grouped by device.
type DeviceStat struct {
	Device string
	Count  int
}

// UserRepository persists users.
type UserRepository interface {
	// Create inserts a new user. Returns ErrConflict on duplicate email
	// or duplicate employee_code.
	Create(ctx context.Context, u *User) error
	// FindByEmail returns the user with the given email, or ErrNotFound.
	FindByEmail(ctx context.Context, email string) (*User, error)
	// FindByID returns the user with the given id, or ErrNotFound.
	FindByID(ctx context.Context, id string) (*User, error)
	// Update persists changes to an existing user.
	Update(ctx context.Context, u *User) error
}

// AttendanceRepository persists attendance records.
type AttendanceRepository interface {
	// Create inserts a new attendance record.
	Create(ctx context.Context, a *Attendance) error
	// ListByUserDate returns records for a user on a calendar date
	// (in the given location), ordered by occurred_at ascending.
	ListByUserDate(ctx context.Context, userID string, date time.Time, loc *time.Location) ([]Attendance, error)
	// FindByID returns the attendance with the given id, or ErrNotFound.
	FindByID(ctx context.Context, id string) (*Attendance, error)
}

// LocationRepository persists geofence locations.
type LocationRepository interface {
	// ListActive returns all active locations (for geofence checks).
	ListActive(ctx context.Context) ([]Location, error)
}

// QrTokenRepository persists QR attendance tokens.
type QrTokenRepository interface {
	// Create inserts a new QR token.
	Create(ctx context.Context, t *QrToken) error
	// FindByToken returns the token with the given value, or ErrNotFound.
	FindByToken(ctx context.Context, token string) (*QrToken, error)
	// ListAll returns every token joined with its location name,
	// newest first.
	ListAll(ctx context.Context) ([]QrTokenView, error)
	// SetActive revokes (active=false) or re-activates a token.
	// Returns ErrNotFound when the id is unknown.
	SetActive(ctx context.Context, id string, active bool) error
}

// LeaveRepository persists leave requests.
type LeaveRepository interface {
	// Create inserts a new leave request (status defaults to pending).
	Create(ctx context.Context, l *Leave) error
	// FindByID returns the leave with the given id, or ErrNotFound.
	FindByID(ctx context.Context, id string) (*Leave, error)
	// List returns leave requests filtered by status and/or userID.
	// Empty status/userID means "all". Ordered by created_at DESC.
	List(ctx context.Context, status, userID string) ([]Leave, error)
	// ListOverlapping returns pending/approved leaves for a user whose date
	// range intersects [start, end] (for overlap validation).
	ListOverlapping(ctx context.Context, userID, start, end string) ([]Leave, error)
	// CountApprovedDays sums total_days of approved leaves for a user within the
	// current year (used for annual quota checks). Empty leaveType = all types.
	CountApprovedDays(ctx context.Context, userID, leaveType string) (int, error)
	// UpdateStatus approves/rejects/cancels a leave.
	UpdateStatus(ctx context.Context, id, status, actorID, rejectionReason string, approvedAt *time.Time) error
	// SetAttachment stores the attachment path (used during create).
	SetAttachment(ctx context.Context, id, path string) error
}

// LeaveTypeRepository persists leave type master data.
type LeaveTypeRepository interface {
	ListActive(ctx context.Context) ([]LeaveType, error)
	FindByCode(ctx context.Context, code string) (*LeaveType, error)
}
type SettingsRepository interface {
	// All returns every settings key/value pair.
	All(ctx context.Context) (map[string]string, error)
	// Set upserts a single settings key.
	Set(ctx context.Context, key, value string) error
}

// NotificationRepository persists user notifications.
type NotificationRepository interface {
	// Create inserts a new notification.
	Create(ctx context.Context, n *Notification) error
	// ListByUser returns a user's notifications newest first.
	ListByUser(ctx context.Context, userID string) ([]Notification, error)
	// MarkRead marks a user's notification (or all if id == "") as read.
	// Returns ErrNotFound when id is given but does not belong to the user.
	MarkRead(ctx context.Context, userID, id string) error
	// UnreadCount returns how many notifications a user has not read.
	UnreadCount(ctx context.Context, userID string) (int, error)
}
