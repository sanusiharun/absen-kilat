package service

import (
	"context"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// ---- fakes -----------------------------------------------------------------

type fakePartnerRepo struct{ items []domain.Partner }

func (f *fakePartnerRepo) Create(ctx context.Context, p *domain.Partner) error {
	p.ID = "pt-" + p.Name
	f.items = append(f.items, *p)
	return nil
}
func (f *fakePartnerRepo) FindByID(ctx context.Context, id string) (*domain.Partner, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			cp := f.items[i]
			return &cp, nil
		}
	}
	return nil, pkgerrors.NotFound("partner", id)
}
func (f *fakePartnerRepo) List(ctx context.Context) ([]domain.Partner, error) { return f.items, nil }

type fakeProjectRepo struct{ items []domain.Project }

func (f *fakeProjectRepo) Create(ctx context.Context, p *domain.Project) error {
	p.ID = "prj-" + p.Code
	f.items = append(f.items, *p)
	return nil
}
func (f *fakeProjectRepo) FindByID(ctx context.Context, id string) (*domain.Project, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			cp := f.items[i]
			return &cp, nil
		}
	}
	return nil, pkgerrors.NotFound("project", id)
}
func (f *fakeProjectRepo) FindByCode(ctx context.Context, code string) (*domain.Project, error) {
	for i := range f.items {
		if f.items[i].Code == code {
			cp := f.items[i]
			return &cp, nil
		}
	}
	return nil, pkgerrors.NotFound("project", code)
}
func (f *fakeProjectRepo) List(ctx context.Context, status string) ([]domain.Project, error) {
	out := make([]domain.Project, 0, len(f.items))
	for _, p := range f.items {
		if status != "" && p.Status != status {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

type fakeAssignmentRepo struct{ items []domain.Assignment }

func (f *fakeAssignmentRepo) Create(ctx context.Context, a *domain.Assignment) error {
	a.ID = "as-" + a.UserID + "-" + a.ProjectID
	f.items = append(f.items, *a)
	return nil
}
func (f *fakeAssignmentRepo) FindByID(ctx context.Context, id string) (*domain.Assignment, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			cp := f.items[i]
			return &cp, nil
		}
	}
	return nil, pkgerrors.NotFound("assignment", id)
}
func (f *fakeAssignmentRepo) List(ctx context.Context, projectID, userID string) ([]domain.Assignment, error) {
	out := make([]domain.Assignment, 0, len(f.items))
	for _, a := range f.items {
		if projectID != "" && a.ProjectID != projectID {
			continue
		}
		if userID != "" && a.UserID != userID {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

type fakeUserLookup struct{ users map[string]domain.User }

func (f *fakeUserLookup) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if u, ok := f.users[id]; ok {
		return &u, nil
	}
	return nil, pkgerrors.NotFound("user", id)
}

func newProjectSvc() (*ProjectService, *fakePartnerRepo, *fakeProjectRepo, *fakeAssignmentRepo) {
	partners := &fakePartnerRepo{}
	projects := &fakeProjectRepo{}
	assignments := &fakeAssignmentRepo{}
	users := &fakeUserLookup{users: map[string]domain.User{
		"u1": {ID: "u1", Email: "internal@x.test", Name: "Internal"},
		"u2": {ID: "u2", Email: "diver@x.test", Name: "Diver Mitra"},
	}}
	return NewProjectService(partners, projects, assignments, users), partners, projects, assignments
}

// ---- partner ---------------------------------------------------------------

func TestCreatePartnerValidation(t *testing.T) {
	svc, _, _, _ := newProjectSvc()

	if _, err := svc.CreatePartner(context.Background(), PartnerInput{Name: "   "}); err == nil {
		t.Fatal("blank partner name must be rejected")
	} else if httpxCode(err) != "VALIDATION_ERROR" {
		t.Fatalf("code = %q", httpxCode(err))
	}

	p, err := svc.CreatePartner(context.Background(), PartnerInput{Name: " PT Kru Selam ", ContactPhone: "0812"})
	if err != nil {
		t.Fatalf("CreatePartner: %v", err)
	}
	if p.Name != "PT Kru Selam" || !p.Active {
		t.Fatalf("unexpected partner: %+v", p)
	}
}

// ---- project ---------------------------------------------------------------

func TestCreateProjectNormalisesAndValidates(t *testing.T) {
	svc, _, _, _ := newProjectSvc()
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, ProjectInput{
		Code: " spk-2026-001 ", Name: "Inspeksi Lambung MV Andalas",
		VesselName: "MV Andalas", ClientName: "PT SCM", StartDate: "2026-11-01",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.Code != "SPK-2026-001" {
		t.Fatalf("code not normalised: %q", p.Code)
	}
	if p.Status != "active" {
		t.Fatalf("default status = %q, want active", p.Status)
	}

	// duplicate code
	if _, err := svc.CreateProject(ctx, ProjectInput{Code: "SPK-2026-001", Name: "dup"}); err == nil {
		t.Fatal("duplicate code must conflict")
	} else if httpxCode(err) != "CONFLICT" {
		t.Fatalf("code = %q, want CONFLICT", httpxCode(err))
	}

	// bad status / bad dates / missing fields
	cases := []ProjectInput{
		{Code: "X1", Name: "n", Status: "selesai"},
		{Code: "X2", Name: "n", StartDate: "2026-12-01", EndDate: "2026-11-01"},
		{Code: "X3", Name: "n", StartDate: "01-12-2026"},
		{Code: "", Name: "n"},
		{Code: "X4", Name: "  "},
	}
	for i, c := range cases {
		if _, err := svc.CreateProject(ctx, c); err == nil {
			t.Fatalf("case %d must be rejected: %+v", i, c)
		} else if httpxCode(err) != "VALIDATION_ERROR" {
			t.Fatalf("case %d code = %q, want VALIDATION_ERROR", i, httpxCode(err))
		}
	}
}

func TestListProjectsValidatesStatus(t *testing.T) {
	svc, _, _, _ := newProjectSvc()
	if _, err := svc.ListProjects(context.Background(), "ngawur"); err == nil {
		t.Fatal("unknown status must be rejected")
	}
}

// ---- assignment ------------------------------------------------------------

func TestCreateAssignmentHappyPathInternalAndPartner(t *testing.T) {
	svc, partners, projects, _ := newProjectSvc()
	ctx := context.Background()

	partner, err := svc.CreatePartner(ctx, PartnerInput{Name: "PT Kru Selam"})
	if err != nil {
		t.Fatalf("CreatePartner: %v", err)
	}
	proj, err := svc.CreateProject(ctx, ProjectInput{Code: "SPK-1", Name: "Docking"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	_ = partners
	_ = projects

	internal, err := svc.CreateAssignment(ctx, AssignmentInput{
		UserID: "u1", ProjectID: proj.ID, Role: "supervisor", StartDate: "2026-11-01",
	})
	if err != nil {
		t.Fatalf("internal assignment: %v", err)
	}
	if internal.PartnerID != "" {
		t.Fatalf("internal assignment must have no partner, got %q", internal.PartnerID)
	}

	crew, err := svc.CreateAssignment(ctx, AssignmentInput{
		UserID: "u2", ProjectID: proj.ID, PartnerID: partner.ID, Role: "commercial diver",
		StartDate: "2026-11-01", EndDate: "2026-11-10",
	})
	if err != nil {
		t.Fatalf("partner assignment: %v", err)
	}
	if crew.PartnerID != partner.ID {
		t.Fatalf("partner id lost: %+v", crew)
	}
}

func TestCreateAssignmentRejectsBadInput(t *testing.T) {
	svc, _, _, _ := newProjectSvc()
	ctx := context.Background()

	proj, err := svc.CreateProject(ctx, ProjectInput{Code: "SPK-2", Name: "Inspeksi"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	cancelled, err := svc.CreateProject(ctx, ProjectInput{Code: "SPK-3", Name: "Batal", Status: "cancelled"})
	if err != nil {
		t.Fatalf("CreateProject cancelled: %v", err)
	}

	cases := []struct {
		name string
		in   AssignmentInput
		code string
	}{
		{"no user", AssignmentInput{ProjectID: proj.ID, StartDate: "2026-11-01"}, "VALIDATION_ERROR"},
		{"no project", AssignmentInput{UserID: "u1", StartDate: "2026-11-01"}, "VALIDATION_ERROR"},
		{"unknown user", AssignmentInput{UserID: "ghost", ProjectID: proj.ID, StartDate: "2026-11-01"}, "VALIDATION_ERROR"},
		{"unknown project", AssignmentInput{UserID: "u1", ProjectID: "ghost", StartDate: "2026-11-01"}, "VALIDATION_ERROR"},
		{"unknown partner", AssignmentInput{UserID: "u1", ProjectID: proj.ID, PartnerID: "ghost", StartDate: "2026-11-01"}, "VALIDATION_ERROR"},
		{"no start date", AssignmentInput{UserID: "u1", ProjectID: proj.ID}, "VALIDATION_ERROR"},
		{"bad date", AssignmentInput{UserID: "u1", ProjectID: proj.ID, StartDate: "1 Nov 2026"}, "VALIDATION_ERROR"},
		{"end before start", AssignmentInput{UserID: "u1", ProjectID: proj.ID, StartDate: "2026-11-10", EndDate: "2026-11-01"}, "VALIDATION_ERROR"},
		{"cancelled project", AssignmentInput{UserID: "u1", ProjectID: cancelled.ID, StartDate: "2026-11-01"}, "VALIDATION_ERROR"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.CreateAssignment(ctx, c.in)
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := httpxCode(err); got != c.code {
				t.Fatalf("code = %q, want %q (err: %v)", got, c.code, err)
			}
		})
	}
}

func TestCreateAssignmentRejectsOverlap(t *testing.T) {
	svc, _, _, _ := newProjectSvc()
	ctx := context.Background()

	proj, err := svc.CreateProject(ctx, ProjectInput{Code: "SPK-4", Name: "Underwater survey"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := svc.CreateAssignment(ctx, AssignmentInput{
		UserID: "u1", ProjectID: proj.ID, StartDate: "2026-11-01", EndDate: "2026-11-10",
	}); err != nil {
		t.Fatalf("first assignment: %v", err)
	}

	// overlap -> conflict
	if _, err := svc.CreateAssignment(ctx, AssignmentInput{
		UserID: "u1", ProjectID: proj.ID, StartDate: "2026-11-05", EndDate: "2026-11-15",
	}); err == nil {
		t.Fatal("overlapping assignment must conflict")
	} else if httpxCode(err) != "CONFLICT" {
		t.Fatalf("code = %q, want CONFLICT", httpxCode(err))
	}

	// back-to-back (starts the day after the previous ends) is allowed
	if _, err := svc.CreateAssignment(ctx, AssignmentInput{
		UserID: "u1", ProjectID: proj.ID, StartDate: "2026-11-11", EndDate: "2026-11-20",
	}); err != nil {
		t.Fatalf("non-overlapping follow-up must be allowed: %v", err)
	}
}

func TestMyAssignmentsRequiresUser(t *testing.T) {
	svc, _, _, _ := newProjectSvc()
	if _, err := svc.MyAssignments(context.Background(), ""); err == nil {
		t.Fatal("empty user must be rejected")
	}
}

func TestRangesOverlap(t *testing.T) {
	cases := []struct {
		name                   string
		aStart, aEnd           string
		bStart, bEnd           string
		want                   bool
	}{
		{"identical", "2026-11-01", "2026-11-10", "2026-11-01", "2026-11-10", true},
		{"inside", "2026-11-03", "2026-11-05", "2026-11-01", "2026-11-10", true},
		{"before", "2026-10-01", "2026-10-05", "2026-11-01", "2026-11-10", false},
		{"after", "2026-12-01", "2026-12-05", "2026-11-01", "2026-11-10", false},
		{"touching edges", "2026-11-10", "2026-11-15", "2026-11-01", "2026-11-10", true},
		{"open ended overlaps", "2026-11-05", "", "2026-11-01", "2026-11-10", true},
		{"open ended starts later", "2026-11-11", "", "2026-11-01", "2026-11-10", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rangesOverlap(c.aStart, c.aEnd, c.bStart, c.bEnd); got != c.want {
				t.Fatalf("rangesOverlap(%q,%q,%q,%q) = %v, want %v",
					c.aStart, c.aEnd, c.bStart, c.bEnd, got, c.want)
			}
		})
	}
}
