package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// UserLookup is the slice of the user repository this module needs.
type UserLookup interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
}

// ProjectService owns the master data that separates internal employees from
// partner (lepas) crew: partners, projects/SPK, and the assignments that join
// them to users. It records no money and no attendance — it is the reference
// data the rest of the maritime workflow hangs on.
type ProjectService struct {
	partners    domain.PartnerRepository
	projects    domain.ProjectRepository
	assignments domain.AssignmentRepository
	users       UserLookup
}

// NewProjectService builds the service.
func NewProjectService(partners domain.PartnerRepository, projects domain.ProjectRepository,
	assignments domain.AssignmentRepository, users UserLookup) *ProjectService {
	return &ProjectService{partners: partners, projects: projects, assignments: assignments, users: users}
}

// PartnerInput is one crew supplier as sent by the admin app.
type PartnerInput struct {
	Name         string
	ContactName  string
	ContactPhone string
}

// CreatePartner validates and stores a partner.
func (s *ProjectService) CreatePartner(ctx context.Context, in PartnerInput) (*domain.Partner, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, pkgerrors.Validation("partner", "nama mitra wajib diisi")
	}
	p := &domain.Partner{
		Name:         name,
		ContactName:  strings.TrimSpace(in.ContactName),
		ContactPhone: strings.TrimSpace(in.ContactPhone),
		Active:       true,
	}
	if err := s.partners.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ListPartners returns every partner.
func (s *ProjectService) ListPartners(ctx context.Context) ([]domain.Partner, error) {
	return s.partners.List(ctx)
}

// ProjectInput is one project/SPK as sent by the admin app.
type ProjectInput struct {
	Code       string
	Name       string
	VesselName string
	ClientName string
	Status     string
	StartDate  string
	EndDate    string
}

// CreateProject validates and stores a project.
func (s *ProjectService) CreateProject(ctx context.Context, in ProjectInput) (*domain.Project, error) {
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	name := strings.TrimSpace(in.Name)
	if code == "" {
		return nil, pkgerrors.Validation("project", "kode/nomor proyek wajib diisi")
	}
	if name == "" {
		return nil, pkgerrors.Validation("project", "nama proyek wajib diisi")
	}

	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "active"
	}
	switch status {
	case "active", "done", "cancelled":
	default:
		return nil, pkgerrors.Validation("project", "status harus active, done, atau cancelled")
	}

	start, end, err := validateDateRange(in.StartDate, in.EndDate)
	if err != nil {
		return nil, err
	}

	if existing, err := s.projects.FindByCode(ctx, code); err == nil && existing != nil {
		return nil, pkgerrors.ConflictMessage("project", "code", code,
			"nomor proyek sudah dipakai")
	}

	p := &domain.Project{
		Code:       code,
		Name:       name,
		VesselName: strings.TrimSpace(in.VesselName),
		ClientName: strings.TrimSpace(in.ClientName),
		Status:     status,
		StartDate:  start,
		EndDate:    end,
	}
	if err := s.projects.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ListProjects returns projects filtered by status.
func (s *ProjectService) ListProjects(ctx context.Context, status string) ([]domain.Project, error) {
	if status != "" {
		switch status {
		case "active", "done", "cancelled":
		default:
			return nil, pkgerrors.Validation("project", "status tidak dikenal")
		}
	}
	return s.projects.List(ctx, status)
}

// GetProject returns one project.
func (s *ProjectService) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	return s.projects.FindByID(ctx, id)
}

// AssignmentInput is one crew assignment as sent by the admin app.
type AssignmentInput struct {
	UserID    string
	ProjectID string
	PartnerID string // "" = internal employee
	Role      string
	StartDate string
	EndDate   string // "" = still running
}

// CreateAssignment validates and stores an assignment.
func (s *ProjectService) CreateAssignment(ctx context.Context, in AssignmentInput) (*domain.Assignment, error) {
	if strings.TrimSpace(in.UserID) == "" {
		return nil, pkgerrors.Validation("assignment", "user wajib dipilih")
	}
	if strings.TrimSpace(in.ProjectID) == "" {
		return nil, pkgerrors.Validation("assignment", "proyek wajib dipilih")
	}

	if _, err := s.users.FindByID(ctx, in.UserID); err != nil {
		return nil, pkgerrors.Validation("assignment", "user tidak ditemukan")
	}
	project, err := s.projects.FindByID(ctx, in.ProjectID)
	if err != nil {
		return nil, pkgerrors.Validation("assignment", "proyek tidak ditemukan")
	}
	if project.Status == "cancelled" {
		return nil, pkgerrors.Validation("assignment", "proyek sudah dibatalkan")
	}

	partnerID := strings.TrimSpace(in.PartnerID)
	if partnerID != "" {
		if _, err := s.partners.FindByID(ctx, partnerID); err != nil {
			return nil, pkgerrors.Validation("assignment", "mitra tidak ditemukan")
		}
	}

	start, end, err := validateDateRange(in.StartDate, in.EndDate)
	if err != nil {
		return nil, err
	}
	if start == "" {
		return nil, pkgerrors.Validation("assignment", "tanggal mulai wajib diisi")
	}

	existing, err := s.assignments.List(ctx, in.ProjectID, in.UserID)
	if err != nil {
		return nil, err
	}
	for _, a := range existing {
		if rangesOverlap(start, end, a.StartDate, a.EndDate) {
			return nil, pkgerrors.ConflictMessage("assignment", "start_date", start,
				fmt.Sprintf("user sudah punya penugasan pada rentang ini di proyek %s", project.Code))
		}
	}

	a := &domain.Assignment{
		UserID:    in.UserID,
		ProjectID: in.ProjectID,
		PartnerID: partnerID,
		Role:      strings.TrimSpace(in.Role),
		StartDate: start,
		EndDate:   end,
	}
	if err := s.assignments.Create(ctx, a); err != nil {
		return nil, err
	}
	return s.assignments.FindByID(ctx, a.ID)
}

// ListAssignments returns assignments filtered by project and/or user.
func (s *ProjectService) ListAssignments(ctx context.Context, projectID, userID string) ([]domain.Assignment, error) {
	return s.assignments.List(ctx, projectID, userID)
}

// MyAssignments is the employee-facing history ("riwayat penugasan per proyek").
func (s *ProjectService) MyAssignments(ctx context.Context, userID string) ([]domain.Assignment, error) {
	if userID == "" {
		return nil, pkgerrors.Unauthorized("user tidak dikenal")
	}
	return s.assignments.List(ctx, "", userID)
}

// validateDateRange normalises two optional YYYY-MM-DD dates and checks order.
func validateDateRange(startIn, endIn string) (string, string, error) {
	start := strings.TrimSpace(startIn)
	end := strings.TrimSpace(endIn)
	if start != "" {
		if _, err := time.Parse("2006-01-02", start); err != nil {
			return "", "", pkgerrors.Validation("date", "format tanggal mulai salah (YYYY-MM-DD)")
		}
	}
	if end != "" {
		if _, err := time.Parse("2006-01-02", end); err != nil {
			return "", "", pkgerrors.Validation("date", "format tanggal selesai salah (YYYY-MM-DD)")
		}
	}
	if start != "" && end != "" && end < start {
		return "", "", pkgerrors.Validation("date", "tanggal selesai harus >= tanggal mulai")
	}
	return start, end, nil
}

// rangesOverlap compares two [start, end] ranges where an empty end means "open".
func rangesOverlap(aStart, aEnd, bStart, bEnd string) bool {
	if aEnd != "" && aEnd < bStart {
		return false
	}
	if bEnd != "" && bEnd < aStart {
		return false
	}
	return true
}
