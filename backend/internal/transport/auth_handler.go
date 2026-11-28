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
	ForgotPassword(ctx context.Context, email string) (string, error)
	ResetPassword(ctx context.Context, resetToken, newPassword string) error
}

// AuthHandler wires auth endpoints onto a router.
type AuthHandler struct {
	svc authService
	// limiter dan clientIP opsional; bila nil, pembatasan tidak aktif (tes lama).
	limiter  *service.AuthRateLimiter
	clientIP *ClientIPResolver
}

// NewAuthHandler creates an AuthHandler without rate limiting. Dipakai tes yang
// tidak menguji pembatasan; produksi memakai NewAuthHandlerWithRateLimits.
func NewAuthHandler(svc authService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// NewAuthHandlerWithRateLimits creates an AuthHandler with auth rate limiting
// (identity-hardening R4) and trusted-proxy aware client IP resolution.
func NewAuthHandlerWithRateLimits(svc authService, limiter *service.AuthRateLimiter, resolver *ClientIPResolver) *AuthHandler {
	return &AuthHandler{svc: svc, limiter: limiter, clientIP: resolver}
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
	httpx.WriteCreated(w, userResponse{
		ID:                  user.ID,
		Email:               user.Email,
		Name:                user.Name,
		IsEmailVerified:     user.IsEmailVerified,
		NotificationEnabled: user.NotificationEnabled,
	})
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	ip := h.clientIPOf(r)
	if h.rateBlocked(r.Context(), ip, req.Email, service.ScopeLoginIP, service.ScopeLoginAccount) {
		httpx.WriteHTTPError(w, httpx.ErrTooManyRequests)
		return
	}

	user, pair, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.recordAttempt(r.Context(), ip, req.Email, service.ScopeLoginIP, service.ScopeLoginAccount)
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

	ip := h.clientIPOf(r)
	if h.rateBlocked(r.Context(), ip, "", service.ScopeVerifyEmailIP, "") {
		httpx.WriteHTTPError(w, httpx.ErrTooManyRequests)
		return
	}
	h.recordAttempt(r.Context(), ip, "", service.ScopeVerifyEmailIP, "")

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
	ip := h.clientIPOf(r)
	if h.rateBlocked(r.Context(), ip, req.Email, service.ScopeForgotIP, service.ScopeForgotAccount) {
		httpx.WriteHTTPError(w, httpx.ErrTooManyRequests)
		return
	}
	h.recordAttempt(r.Context(), ip, req.Email, service.ScopeForgotIP, service.ScopeForgotAccount)
	// Setiap permintaan dihitung, termasuk email tak dikenal, supaya respons
	// tetap identik dan penghitung tidak bisa dihindari dengan email acak.
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

	ip := h.clientIPOf(r)
	if h.rateBlocked(r.Context(), ip, "", service.ScopeResetIP, "") {
		httpx.WriteHTTPError(w, httpx.ErrTooManyRequests)
		return
	}
	h.recordAttempt(r.Context(), ip, "", service.ScopeResetIP, "")

	if err := h.svc.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]bool{"reset": true})
}

// clientIPOf resolves the client IP, honouring trusted proxies when configured.
func (h *AuthHandler) clientIPOf(r *http.Request) string {
	if h.clientIP != nil {
		return h.clientIP.Resolve(r)
	}
	return clientIP(r.RemoteAddr)
}

// rateBlocked reports whether the request must be refused with the generic 429.
// Store errors fail closed: a limiter that cannot read state refuses the request.
func (h *AuthHandler) rateBlocked(ctx context.Context, ip, account, ipScope, accountScope string) bool {
	if h.limiter == nil {
		return false
	}
	if ok, err := h.limiter.Allowed(ctx, ipScope, ip); err != nil || !ok {
		return true
	}
	if accountScope != "" {
		if ok, err := h.limiter.Allowed(ctx, accountScope, account); err != nil || !ok {
			return true
		}
	}
	return false
}

// recordAttempt menambah penghitung untuk scope IP (dan akun bila ada).
func (h *AuthHandler) recordAttempt(ctx context.Context, ip, account, ipScope, accountScope string) {
	if h.limiter == nil {
		return
	}
	_ = h.limiter.Record(ctx, ipScope, ip)
	if accountScope != "" {
		_ = h.limiter.Record(ctx, accountScope, account)
	}
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
