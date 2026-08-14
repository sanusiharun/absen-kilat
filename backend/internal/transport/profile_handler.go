package transport

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// profileService is the subset of service.ProfileService used by handlers.
type profileService interface {
	Get(ctx context.Context, userID string) (*domain.User, error)
	Update(ctx context.Context, userID string, upd service.ProfileUpdate) (*domain.User, error)
	SetAvatar(ctx context.Context, userID, avatarPath string) (*domain.User, error)
}

// passwordChanger is implemented by service.AuthService.
type passwordChanger interface {
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
}

// ProfileHandler wires profile endpoints onto a router.
type ProfileHandler struct {
	svc   profileService
	store photoStorer
	pw    passwordChanger
}

// NewProfileHandler creates a ProfileHandler.
func NewProfileHandler(svc profileService, store photoStorer, pw passwordChanger) *ProfileHandler {
	return &ProfileHandler{svc: svc, store: store, pw: pw}
}

// RegisterRoutes mounts profile routes under an authenticated router.
func (h *ProfileHandler) RegisterRoutes(r chi.Router) {
	r.Get("/me", h.get)
	r.Patch("/me", h.patch)
	r.Post("/me/avatar", h.avatar)
	r.Post("/me/password", h.password)
}

type updateProfileRequest struct {
	Name                *string `json:"name"`
	NotificationEnabled *bool   `json:"notification_enabled"`
}

func (h *ProfileHandler) get(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	user, err := h.svc.Get(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, profileResponse(user))
}

func (h *ProfileHandler) patch(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	var req updateProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	user, err := h.svc.Update(r.Context(), userID, service.ProfileUpdate{
		Name:                req.Name,
		NotificationEnabled: req.NotificationEnabled,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, profileResponse(user))
}

func (h *ProfileHandler) avatar(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	if err := r.ParseMultipartForm(16 << 20); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}
	file, header, err := r.FormFile("avatar")
	if err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}
	defer file.Close()

	relPath, serr := h.store.Store(file, header)
	if serr != nil {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", serr.Error()))
		return
	}

	user, err := h.svc.SetAvatar(r.Context(), userID, relPath)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, profileResponse(user))
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *ProfileHandler) password(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	if err := h.pw.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]bool{"changed": true})
}

// profileResponse maps a domain.User to the API response shape.
func profileResponse(u *domain.User) map[string]any {
	return map[string]any{
		"id":                   u.ID,
		"email":                u.Email,
		"name":                 u.Name,
		"avatar_url":           u.AvatarPath,
		"is_email_verified":    u.IsEmailVerified,
		"notification_enabled": u.NotificationEnabled,
	}
}
