package transport

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// authService is the subset of service.AuthService used by the handlers.
type authService interface {
	Register(ctx context.Context, email, password, name string) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*domain.User, *service.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (*service.TokenPair, error)
	VerifyEmail(ctx context.Context, token string) error
	// IssueVerificationToken menerbitkan token verifikasi email sekali pakai.
	IssueVerificationToken(ctx context.Context, userID string) (string, error)
	ForgotPassword(ctx context.Context, email string) (string, error)
	ResetPassword(ctx context.Context, resetToken, newPassword string) error
}

// AuthHandler wires auth endpoints onto a router.
type AuthHandler struct {
	svc authService
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(svc authService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// RegisterRoutes mounts the auth routes under r.
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.register)
		r.Post("/login", h.login)
		r.Post("/refresh", h.refresh)
		r.Post("/forgot-password", h.forgotPassword)
		r.Post("/reset-password", h.resetPassword)
		r.Post("/verify-email", h.verifyEmail)
	})
}

// --- request/response types ---

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

type userResponse struct {
	ID                  string `json:"id"`
	Email               string `json:"email"`
	Name                string `json:"name"`
	AvatarURL           string `json:"avatar_url,omitempty"`
	IsEmailVerified     bool   `json:"is_email_verified"`
	NotificationEnabled bool   `json:"notification_enabled"`
}

// --- handlers ---

// registerResponse membawa token verifikasi sekali pakai (pilot tanpa email).
type registerResponse struct {
	userResponse
	VerificationToken string `json:"verification_token"`
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	user, err := h.svc.Register(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	// Pilot tanpa transport email: token verifikasi dikembalikan SEKALI di respons
	// pendaftaran supaya alur penerbitan -> verifikasi bisa diuji end-to-end.
	verificationToken, verr := h.svc.IssueVerificationToken(r.Context(), user.ID)
	if verr != nil {
		httpx.WriteError(w, verr)
		return
	}
	httpx.WriteCreated(w, registerResponse{
		userResponse: userResponse{
			ID:                  user.ID,
			Email:               user.Email,
			Name:                user.Name,
			IsEmailVerified:     user.IsEmailVerified,
			NotificationEnabled: user.NotificationEnabled,
		},
		VerificationToken: verificationToken,
	})
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	user, pair, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, tokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		TokenType:    "Bearer",
	})
	_ = user // user is re-fetched via /me by the client
}

func (h *AuthHandler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	pair, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, tokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		TokenType:    "Bearer",
	})
}

func (h *AuthHandler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var req verifyEmailRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	if err := h.svc.VerifyEmail(r.Context(), req.Token); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]bool{"verified": true})
}

func (h *AuthHandler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	// Always respond success (even when the email is unknown) to avoid
	// leaking which accounts exist.
	token, err := h.svc.ForgotPassword(r.Context(), req.Email)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	_ = token // in pilot: token would be emailed; log it server-side instead
	httpx.WriteOK(w, map[string]bool{"sent": true})
}

func (h *AuthHandler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	if err := h.svc.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]bool{"reset": true})
}

// decodeJSON decodes a JSON body into v, rejecting unknown trailing data.
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return err
	}
	// Reject trailing garbage after the JSON value.
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return errUnexpectedTrailingJSON
	}
	return nil
}

var errUnexpectedTrailingJSON = &trailingJSONError{}

type trailingJSONError struct{}

func (*trailingJSONError) Error() string { return "unexpected trailing JSON" }
