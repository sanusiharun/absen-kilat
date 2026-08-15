package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// qrService is the subset of service.QrService used by the handlers.
type qrService interface {
	Generate(ctx context.Context, locationID, createdBy string) (*domain.QrToken, string, error)
	ListTokens(ctx context.Context) ([]domain.QrTokenView, error)
	Revoke(ctx context.Context, id string) error
	RateLimited(ip string) bool
	RecordQRAttendance(ctx context.Context, in service.QrAttendanceInput) (*domain.Attendance, error)
}

// QrHandler wires the admin QR management endpoints and the public QR
// attendance endpoint onto routers.
type QrHandler struct {
	svc   qrService
	store photoStorer
}

// NewQrHandler creates a QrHandler.
func NewQrHandler(svc qrService, store photoStorer) *QrHandler {
	return &QrHandler{svc: svc, store: store}
}

// RegisterRoutes mounts the admin QR routes. The caller is expected to guard
// them with RequireAuth + RequireRole(super_admin, hr).
func (h *QrHandler) RegisterRoutes(r chi.Router) {
	r.Post("/admin/qr", h.generate)
	r.Get("/admin/qr", h.list)
	r.Delete("/admin/qr/{id}", h.revoke)
}

// RegisterPublicRoutes mounts the unauthenticated QR attendance endpoint.
// It must be registered OUTSIDE the RequireAuth group.
func (h *QrHandler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/public/attendance/qr", h.publicAttendance)
}

func (h *QrHandler) generate(w http.ResponseWriter, r *http.Request) {
	actorID := UserIDFromContext(r.Context())
	if actorID == "" {
		httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
		return
	}
	var body struct {
		LocationID string `json:"location_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}
	t, url, err := h.svc.Generate(r.Context(), body.LocationID, actorID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, map[string]any{
		"token": t.Token,
		"url":   url,
	})
}

func (h *QrHandler) list(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.svc.ListTokens(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	resp := make([]map[string]any, 0, len(tokens))
	for _, t := range tokens {
		row := map[string]any{
			"id":            t.ID,
			"location_id":   t.LocationID,
			"location_name": t.LocationName,
			"token":         t.Token,
			"active":        t.Active,
			"created_by":    t.CreatedBy,
			"created_at":    t.CreatedAt.Format(time.RFC3339),
		}
		if t.ExpiresAt != nil {
			row["expires_at"] = t.ExpiresAt.Format(time.RFC3339)
		}
		resp = append(resp, row)
	}
	httpx.WriteOK(w, map[string]any{"tokens": resp})
}

func (h *QrHandler) revoke(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Revoke(r.Context(), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]any{"ok": true})
}

// publicAttendance records a check-in from the public QR web form. No
// authentication: the request is throttled per IP, the token is validated,
// and the identity is resolved by email or employee code.
func (h *QrHandler) publicAttendance(w http.ResponseWriter, r *http.Request) {
	if h.svc.RateLimited(clientIP(r.RemoteAddr)) {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusTooManyRequests, "RATE_LIMITED", "terlalu banyak percobaan, coba lagi sebentar lagi"))
		return
	}

	if err := r.ParseMultipartForm(16 << 20); err != nil { // 16 MB
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	photoPath := ""
	if file, header, err := r.FormFile("photo"); err == nil {
		defer file.Close()
		p, serr := h.store.Store(file, header)
		if serr != nil {
			httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", serr.Error()))
			return
		}
		photoPath = p
	}

	att, err := h.svc.RecordQRAttendance(r.Context(), service.QrAttendanceInput{
		Token:        r.FormValue("token"),
		Email:        r.FormValue("email"),
		EmployeeCode: r.FormValue("employee_code"),
		Name:         r.FormValue("name"),
		PhotoPath:    photoPath,
		IPAddress:    clientIP(r.RemoteAddr),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, map[string]any{
		"attendance": map[string]any{
			"id":               att.ID,
			"approval_status":  string(att.ApprovalStatus),
			"source":           string(att.Source),
		},
	})
}
