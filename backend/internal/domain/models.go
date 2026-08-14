// Package domain defines the core models and repository interfaces for
// Absen Kilat. Layering rule: transport → service → repo → DAO, so
// repository interfaces live here while implementations live in repo/.
package domain

import (
	"context"
	"time"
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
	CreatedAt           time.Time
	UpdatedAt           time.Time
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

// Attendance is a single check-in or check-out record.
type Attendance struct {
	ID         string
	UserID     string
	Type       AttendanceType
	Status     AttendanceStatus
	PhotoPath  string
	Latitude   *float64
	Longitude  *float64
	OccurredAt time.Time
	CreatedAt  time.Time
}

// UserRepository persists users.
type UserRepository interface {
	// Create inserts a new user. Returns ErrConflict on duplicate email.
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
