package domain

import (
	"context"
	"time"
)

// Partner is an external crew supplier (PT/vendor kru lepas). Internal employees
// have no partner row; the distinction lives in Assignment.PartnerID.
type Partner struct {
	ID           string
	Name         string
	ContactName  string
	ContactPhone string
	Active       bool
	CreatedAt    time.Time
}

// Project is one job/SPK, usually tied to a vessel. Attendance and payroll can
// later be grouped by project.
type Project struct {
	ID         string
	Code       string // nomor proyek / SPK
	Name       string
	VesselName string
	ClientName string
	Status     string // active | done | cancelled
	StartDate  string // YYYY-MM-DD, "" when unset
	EndDate    string
	CreatedAt  time.Time
}

// Assignment links a user to a project for a period. PartnerID nil/empty means
// the person is an internal employee; a value means partner (lepas) crew.
type Assignment struct {
	ID          string
	UserID      string
	UserName    string // filled by list queries
	UserEmail   string
	ProjectID   string
	ProjectCode string
	ProjectName string
	VesselName  string
	PartnerID   string // "" = internal
	PartnerName string
	Role        string
	StartDate   string
	EndDate     string // "" = still running
	CreatedAt   time.Time
}

// PartnerRepository persists external crew suppliers.
type PartnerRepository interface {
	Create(ctx context.Context, p *Partner) error
	FindByID(ctx context.Context, id string) (*Partner, error)
	List(ctx context.Context) ([]Partner, error)
}

// ProjectRepository persists projects/SPK.
type ProjectRepository interface {
	Create(ctx context.Context, p *Project) error
	FindByID(ctx context.Context, id string) (*Project, error)
	FindByCode(ctx context.Context, code string) (*Project, error)
	List(ctx context.Context, status string) ([]Project, error)
}

// AssignmentRepository persists crew assignments.
type AssignmentRepository interface {
	Create(ctx context.Context, a *Assignment) error
	FindByID(ctx context.Context, id string) (*Assignment, error)
	// List returns assignments filtered by project and/or user; both may be
	// empty. Newest start_date first.
	List(ctx context.Context, projectID, userID string) ([]Assignment, error)
}
