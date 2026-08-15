// Package domain defines the core models and repository interfaces for
// Absen Kilat. Layering rule: transport → service → repo → DAO, so
// repository interfaces live here while implementations live in repo/.
package domain

import (
	"context"
	"time"
)

// UserRole distinguishes admin from regular employee accounts.
type UserRole string

const (
	RoleEmployee UserRole = "employee"
	RoleAdmin    UserRole = "admin"
)

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
	NIP          string // nomor induk pegawai
	Phone        string
	Department   string
	Position     string
	Address      string
	JoinDate     string // YYYY-MM-DD
	ManagerID    string // supervisor (users.id); "" when none
	EmployeeCode string // unique identifier (QR/manual attendance)
	CreatedAt    time.Time
	UpdatedAt    time.Time
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

// SettingsRepository persists key/value system settings.
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
