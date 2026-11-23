package transport

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// ProjectHandler exposes the crew/project master data (mitra, proyek, penugasan).
type ProjectHandler struct {
	svc *service.ProjectService
}

// NewProjectHandler constructs the handler.
func NewProjectHandler(svc *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

// RegisterRoutes mounts employee-facing endpoints (own assignment history).
func (h *ProjectHandler) RegisterRoutes(r chi.Router) {
	r.Get("/my/assignments", h.myAssignments)
}

// RegisterAdminRoutes mounts master-data endpoints. The router must already
// enforce RequireRole for super_admin/hr.
func (h *ProjectHandler) RegisterAdminRoutes(r chi.Router) {
	r.Get("/admin/partners", h.listPartners)
	r.Post("/admin/partners", h.createPartner)
	r.Get("/admin/projects", h.listProjects)
	r.Post("/admin/projects", h.createProject)
	r.Get("/admin/projects/{id}", h.getProject)
	r.Get("/admin/assignments", h.listAssignments)
	r.Post("/admin/assignments", h.createAssignment)
}

type createPartnerRequest struct {
	Name         string `json:"name"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
}

func (h *ProjectHandler) createPartner(w http.ResponseWriter, r *http.Request) {
	var req createPartnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	p, err := h.svc.CreatePartner(r.Context(), service.PartnerInput{
		Name: req.Name, ContactName: req.ContactName, ContactPhone: req.ContactPhone,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, map[string]any{"partner": partnerToResponse(p)})
}

func (h *ProjectHandler) listPartners(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListPartners(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for i := range list {
		out = append(out, partnerToResponse(&list[i]))
	}
	httpx.WriteOK(w, map[string]any{"partners": out})
}

type createProjectRequest struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	VesselName string `json:"vessel_name"`
	ClientName string `json:"client_name"`
	Status     string `json:"status"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
}

func (h *ProjectHandler) createProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	p, err := h.svc.CreateProject(r.Context(), service.ProjectInput{
		Code: req.Code, Name: req.Name, VesselName: req.VesselName, ClientName: req.ClientName,
		Status: req.Status, StartDate: req.StartDate, EndDate: req.EndDate,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, map[string]any{"project": projectToResponse(p)})
}

func (h *ProjectHandler) listProjects(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListProjects(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for i := range list {
		out = append(out, projectToResponse(&list[i]))
	}
	httpx.WriteOK(w, map[string]any{"projects": out})
}

func (h *ProjectHandler) getProject(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetProject(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"project": projectToResponse(p)})
}

type createAssignmentRequest struct {
	UserID    string `json:"user_id"`
	ProjectID string `json:"project_id"`
	PartnerID string `json:"partner_id"`
	Role      string `json:"role"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

func (h *ProjectHandler) createAssignment(w http.ResponseWriter, r *http.Request) {
	var req createAssignmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	a, err := h.svc.CreateAssignment(r.Context(), service.AssignmentInput{
		UserID: req.UserID, ProjectID: req.ProjectID, PartnerID: req.PartnerID,
		Role: req.Role, StartDate: req.StartDate, EndDate: req.EndDate,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, map[string]any{"assignment": assignmentToResponse(a)})
}

func (h *ProjectHandler) listAssignments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := h.svc.ListAssignments(r.Context(), q.Get("project_id"), q.Get("user_id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"assignments": assignmentResponses(list)})
}

func (h *ProjectHandler) myAssignments(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.MyAssignments(r.Context(), UserIDFromContext(r.Context()))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"assignments": assignmentResponses(list)})
}

func assignmentResponses(list []domain.Assignment) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for i := range list {
		out = append(out, assignmentToResponse(&list[i]))
	}
	return out
}

func partnerToResponse(p *domain.Partner) map[string]any {
	return map[string]any{
		"id":            p.ID,
		"name":          p.Name,
		"contact_name":  p.ContactName,
		"contact_phone": p.ContactPhone,
		"active":        p.Active,
	}
}

func projectToResponse(p *domain.Project) map[string]any {
	return map[string]any{
		"id":          p.ID,
		"code":        p.Code,
		"name":        p.Name,
		"vessel_name": p.VesselName,
		"client_name": p.ClientName,
		"status":      p.Status,
		"start_date":  p.StartDate,
		"end_date":    p.EndDate,
	}
}

func assignmentToResponse(a *domain.Assignment) map[string]any {
	resp := map[string]any{
		"id":           a.ID,
		"user_id":      a.UserID,
		"user_name":    a.UserName,
		"user_email":   a.UserEmail,
		"project_id":   a.ProjectID,
		"project_code": a.ProjectCode,
		"project_name": a.ProjectName,
		"vessel_name":  a.VesselName,
		"role":         a.Role,
		"start_date":   a.StartDate,
		"end_date":     a.EndDate,
		"employment":   "internal",
	}
	if a.PartnerID != "" {
		resp["partner_id"] = a.PartnerID
		resp["partner_name"] = a.PartnerName
		resp["employment"] = "partner"
	}
	return resp
}
