package domain

import (
	"context"
	"time"
)

// OvertimeStatus is the lifecycle of one lembur (overtime) claim.
type OvertimeStatus string

const (
	OvertimePending   OvertimeStatus = "pending"
	OvertimeApproved  OvertimeStatus = "approved"
	OvertimeRejected  OvertimeStatus = "rejected"
	OvertimeCancelled OvertimeStatus = "cancelled"
)

// Overtime is one overtime claim: an explicit time range on a work date. The
// duration in minutes is derived by the service so the rounding rule lives in
// exactly one place. Money is deliberately absent: this module records approved
// time, payroll converts it.
type Overtime struct {
	ID              string
	UserID          string
	UserName        string // filled by list queries (join to users)
	UserEmail       string
	WorkDate        string // YYYY-MM-DD
	StartAt         time.Time
	EndAt           time.Time
	Minutes         int
	Reason          string
	Status          OvertimeStatus
	ApprovedBy      string
	ApprovedAt      *time.Time
	RejectionReason string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// OvertimeRepository persists overtime claims.
type OvertimeRepository interface {
	Create(ctx context.Context, o *Overtime) error
	FindByID(ctx context.Context, id string) (*Overtime, error)
	// List returns claims filtered by status and/or user; empty filters mean
	// "everything". Newest work_date first.
	List(ctx context.Context, status, userID string) ([]Overtime, error)
	// ListOverlapping returns pending or approved claims that intersect
	// [start, end) for the same user, used to reject double claims.
	ListOverlapping(ctx context.Context, userID string, start, end time.Time) ([]Overtime, error)
	UpdateStatus(ctx context.Context, id, status, actorID, reason string, approvedAt *time.Time) error
	// SumApprovedMinutes totals approved minutes for a user between two
	// work_date values (inclusive, YYYY-MM-DD) — the payroll hand-off.
	SumApprovedMinutes(ctx context.Context, userID, fromDate, toDate string) (int, error)
}
