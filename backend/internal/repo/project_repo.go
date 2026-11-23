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

// ---------------------------------------------------------------- partners ---

type partnerDAO struct {
	ID           string
	Name         string
	ContactName  string
	ContactPhone string
	Active       int
	CreatedAt    string
}

// PartnerRepository is the SQLite implementation of domain.PartnerRepository.
type PartnerRepository struct{ db *sql.DB }

// NewPartnerRepository builds the repository.
func NewPartnerRepository(db *sql.DB) *PartnerRepository { return &PartnerRepository{db: db} }

// Create inserts a partner.
func (r *PartnerRepository) Create(ctx context.Context, p *domain.Partner) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO partners (id, name, contact_name, contact_phone, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.ContactName, p.ContactPhone, boolToInt(p.Active), formatTime(p.CreatedAt), formatTime(now))
	if err != nil {
		return fmt.Errorf("repo.PartnerRepository.Create: %w", err)
	}
	return nil
}

// FindByID returns one partner.
func (r *PartnerRepository) FindByID(ctx context.Context, id string) (*domain.Partner, error) {
	var d partnerDAO
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, contact_name, contact_phone, active, created_at FROM partners WHERE id = ?`, id).
		Scan(&d.ID, &d.Name, &d.ContactName, &d.ContactPhone, &d.Active, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.NotFound("partner", id)
		}
		return nil, fmt.Errorf("repo.PartnerRepository.FindByID: %w", err)
	}
	p := partnerFromDAO(d)
	return &p, nil
}

// List returns every partner, newest first.
func (r *PartnerRepository) List(ctx context.Context) ([]domain.Partner, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, contact_name, contact_phone, active, created_at
		FROM partners ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("repo.PartnerRepository.List: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]domain.Partner, 0)
	for rows.Next() {
		var d partnerDAO
		if err := rows.Scan(&d.ID, &d.Name, &d.ContactName, &d.ContactPhone, &d.Active, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("repo.PartnerRepository.List: %w", err)
		}
		out = append(out, partnerFromDAO(d))
	}
	return out, rows.Err()
}

func partnerFromDAO(d partnerDAO) domain.Partner {
	return domain.Partner{
		ID:           d.ID,
		Name:         d.Name,
		ContactName:  d.ContactName,
		ContactPhone: d.ContactPhone,
		Active:       d.Active == 1,
		CreatedAt:    parseTime(d.CreatedAt),
	}
}

// ---------------------------------------------------------------- projects ---

type projectDAO struct {
	ID         string
	Code       string
	Name       string
	VesselName string
	ClientName string
	Status     string
	StartDate  string
	EndDate    string
	CreatedAt  string
}

const projectColumns = `id, code, name, vessel_name, client_name, status, start_date, end_date, created_at`

// ProjectRepository is the SQLite implementation of domain.ProjectRepository.
type ProjectRepository struct{ db *sql.DB }

// NewProjectRepository builds the repository.
func NewProjectRepository(db *sql.DB) *ProjectRepository { return &ProjectRepository{db: db} }

// Create inserts a project (SPK).
func (r *ProjectRepository) Create(ctx context.Context, p *domain.Project) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.Status == "" {
		p.Status = "active"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO projects (id, code, name, vessel_name, client_name, status, start_date, end_date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Code, p.Name, p.VesselName, p.ClientName, p.Status, p.StartDate, p.EndDate,
		formatTime(p.CreatedAt), formatTime(now))
	if err != nil {
		if isUniqueViolation(err) {
			return pkgerrors.Conflict("project", "code", p.Code)
		}
		return fmt.Errorf("repo.ProjectRepository.Create: %w", err)
	}
	return nil
}

// FindByID returns one project.
func (r *ProjectRepository) FindByID(ctx context.Context, id string) (*domain.Project, error) {
	return r.find(ctx, `SELECT `+projectColumns+` FROM projects WHERE id = ?`, id)
}

// FindByCode returns one project by its SPK code.
func (r *ProjectRepository) FindByCode(ctx context.Context, code string) (*domain.Project, error) {
	return r.find(ctx, `SELECT `+projectColumns+` FROM projects WHERE code = ?`, code)
}

func (r *ProjectRepository) find(ctx context.Context, query, arg string) (*domain.Project, error) {
	var d projectDAO
	err := r.db.QueryRowContext(ctx, query, arg).Scan(&d.ID, &d.Code, &d.Name, &d.VesselName,
		&d.ClientName, &d.Status, &d.StartDate, &d.EndDate, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.NotFound("project", arg)
		}
		return nil, fmt.Errorf("repo.ProjectRepository.find: %w", err)
	}
	p := projectFromDAO(d)
	return &p, nil
}

// List returns projects filtered by status ("" = all).
func (r *ProjectRepository) List(ctx context.Context, status string) ([]domain.Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+projectColumns+` FROM projects
		WHERE (? = '' OR status = ?)
		ORDER BY (status = 'active') DESC, start_date DESC, created_at DESC`, status, status)
	if err != nil {
		return nil, fmt.Errorf("repo.ProjectRepository.List: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]domain.Project, 0)
	for rows.Next() {
		var d projectDAO
		if err := rows.Scan(&d.ID, &d.Code, &d.Name, &d.VesselName, &d.ClientName,
			&d.Status, &d.StartDate, &d.EndDate, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("repo.ProjectRepository.List: %w", err)
		}
		out = append(out, projectFromDAO(d))
	}
	return out, rows.Err()
}

func projectFromDAO(d projectDAO) domain.Project {
	return domain.Project{
		ID:         d.ID,
		Code:       d.Code,
		Name:       d.Name,
		VesselName: d.VesselName,
		ClientName: d.ClientName,
		Status:     d.Status,
		StartDate:  d.StartDate,
		EndDate:    d.EndDate,
		CreatedAt:  parseTime(d.CreatedAt),
	}
}

// ------------------------------------------------------------- assignments ---

type assignmentDAO struct {
	ID          string
	UserID      string
	UserName    string
	UserEmail   string
	ProjectID   string
	ProjectCode string
	ProjectName string
	VesselName  string
	PartnerID   sql.NullString
	PartnerName sql.NullString
	Role        string
	StartDate   string
	EndDate     string
	CreatedAt   string
}

const assignmentColumns = `a.id, a.user_id, COALESCE(u.name, ''), COALESCE(u.email, ''),
	a.project_id, COALESCE(p.code, ''), COALESCE(p.name, ''), COALESCE(p.vessel_name, ''),
	a.partner_id, pt.name, a.role, a.start_date, a.end_date, a.created_at`

const assignmentFrom = `FROM assignments a
	LEFT JOIN users u ON u.id = a.user_id
	LEFT JOIN projects p ON p.id = a.project_id
	LEFT JOIN partners pt ON pt.id = a.partner_id`

// AssignmentRepository is the SQLite implementation of domain.AssignmentRepository.
type AssignmentRepository struct{ db *sql.DB }

// NewAssignmentRepository builds the repository.
func NewAssignmentRepository(db *sql.DB) *AssignmentRepository {
	return &AssignmentRepository{db: db}
}

// Create inserts an assignment.
func (r *AssignmentRepository) Create(ctx context.Context, a *domain.Assignment) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO assignments (id, user_id, project_id, partner_id, role, start_date, end_date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.UserID, a.ProjectID, nullString(a.PartnerID), a.Role, a.StartDate, a.EndDate,
		formatTime(a.CreatedAt), formatTime(now))
	if err != nil {
		return fmt.Errorf("repo.AssignmentRepository.Create: %w", err)
	}
	return nil
}

// FindByID returns one assignment with joined names.
func (r *AssignmentRepository) FindByID(ctx context.Context, id string) (*domain.Assignment, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+assignmentColumns+` `+assignmentFrom+` WHERE a.id = ?`, id)
	var d assignmentDAO
	if err := row.Scan(&d.ID, &d.UserID, &d.UserName, &d.UserEmail, &d.ProjectID, &d.ProjectCode,
		&d.ProjectName, &d.VesselName, &d.PartnerID, &d.PartnerName, &d.Role, &d.StartDate,
		&d.EndDate, &d.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.NotFound("assignment", id)
		}
		return nil, fmt.Errorf("repo.AssignmentRepository.FindByID: %w", err)
	}
	a := assignmentFromDAO(d)
	return &a, nil
}

// List returns assignments filtered by project and/or user.
func (r *AssignmentRepository) List(ctx context.Context, projectID, userID string) ([]domain.Assignment, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+assignmentColumns+` `+assignmentFrom+`
		WHERE (? = '' OR a.project_id = ?) AND (? = '' OR a.user_id = ?)
		ORDER BY a.start_date DESC, a.created_at DESC`,
		projectID, projectID, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("repo.AssignmentRepository.List: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]domain.Assignment, 0)
	for rows.Next() {
		var d assignmentDAO
		if err := rows.Scan(&d.ID, &d.UserID, &d.UserName, &d.UserEmail, &d.ProjectID, &d.ProjectCode,
			&d.ProjectName, &d.VesselName, &d.PartnerID, &d.PartnerName, &d.Role, &d.StartDate,
			&d.EndDate, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("repo.AssignmentRepository.List: %w", err)
		}
		out = append(out, assignmentFromDAO(d))
	}
	return out, rows.Err()
}

func assignmentFromDAO(d assignmentDAO) domain.Assignment {
	a := domain.Assignment{
		ID:          d.ID,
		UserID:      d.UserID,
		UserName:    d.UserName,
		UserEmail:   d.UserEmail,
		ProjectID:   d.ProjectID,
		ProjectCode: d.ProjectCode,
		ProjectName: d.ProjectName,
		VesselName:  d.VesselName,
		Role:        d.Role,
		StartDate:   d.StartDate,
		EndDate:     d.EndDate,
		CreatedAt:   parseTime(d.CreatedAt),
	}
	if d.PartnerID.Valid {
		a.PartnerID = d.PartnerID.String
	}
	if d.PartnerName.Valid {
		a.PartnerName = d.PartnerName.String
	}
	return a
}
