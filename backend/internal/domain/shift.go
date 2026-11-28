package domain

import (
	"context"
	"fmt"
	"time"
)

// Shift adalah satu definisi jam kerja (mendukung lintas tengah malam).
type Shift struct {
	ID              string
	Code            string
	Name            string
	StartTime       string // HH:MM
	EndTime         string // HH:MM
	CrossesMidnight bool
	GraceMinutes    int
	BreakMinutes    int
	LocationID      string
	Active          bool
	CreatedAt       time.Time
}

// ShiftAssignment menautkan shift ke pegawai pada rentang tanggal (inklusif).
type ShiftAssignment struct {
	ID        string
	ShiftID   string
	UserID    string
	StartDate string // YYYY-MM-DD
	EndDate   string // YYYY-MM-DD
	Source    string // manual | pattern | import
	CreatedBy string
	CreatedAt time.Time
}

// ShiftRepository menyimpan shift dan penugasannya.
type ShiftRepository interface {
	CreateShift(ctx context.Context, s *Shift) error
	ListShifts(ctx context.Context) ([]Shift, error)
	FindShiftByID(ctx context.Context, id string) (*Shift, error)
	UpdateShift(ctx context.Context, s *Shift) error
	// CreateAssignment menolak rentang yang bertumpuk untuk pegawai yang sama.
	CreateAssignment(ctx context.Context, a *ShiftAssignment) error
	AssignmentsForUser(ctx context.Context, userID, from, to string) ([]ShiftAssignment, error)
}

// ErrShiftOverlap dikembalikan bila penugasan bertumpuk untuk pegawai yang sama.
var ErrShiftOverlap = fmt.Errorf("penugasan shift bertumpuk untuk pegawai ini")

// ErrShiftNotFound dikembalikan bila shift tidak ada.
var ErrShiftNotFound = fmt.Errorf("shift tidak ditemukan")
