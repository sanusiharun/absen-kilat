// Package service contains the application business logic, one type per
// bounded concern (auth, attendance, profile).
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
	"log/slog"
	"strings"
)

// TokenPair carries the access and refresh tokens issued at login.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// AuthService implements authentication business logic: registration,
// password hashing, JWT issuing/validation, and account recovery flows.
type AuthService struct {
	users     domain.UserRepository
	secret    []byte
	accessTTL time.Duration
	// refreshTTL bounds how long a refresh token remains valid.
	refreshTTL time.Duration
	// legacyDeadline membatasi sampai kapan token JWT lama masih diterima
	// (release + 7 hari). Setelah itu token legacy ditolak.
	legacyDeadline time.Time
	// verifications adalah store token verifikasi email sekali pakai (R5).
	verifications EmailVerificationStore
	// mailer mengirim token verifikasi ke pemilik email. Nil = tidak tersedia.
	mailer EmailSender
	// production menandai lingkungan produksi: tanpa mailer, registrasi ditolak.
	production bool
	// tokens adalah store rotasi refresh token. Bila nil, service berjalan dalam
	// mode legacy (JWT stateless) — hanya dipakai tes lama; main selalu mengisi.
	tokens domain.RefreshTokenRepository
}

// LoadLegacyRefreshCutoff membaca LEGACY_REFRESH_ACCEPT_UNTIL (RFC3339). Bila tidak
// diset atau tidak valid, cutoff dibiarkan nol sehingga token legacy DITOLAK
// (fail closed) — bukan lagi "7 hari sejak proses start" (C2).
func (s *AuthService) LoadLegacyRefreshCutoff(env func(string) string) {
	raw := strings.TrimSpace(env("LEGACY_REFRESH_ACCEPT_UNTIL"))
	if raw == "" {
		slog.Warn("auth: LEGACY_REFRESH_ACCEPT_UNTIL tidak diset — token legacy akan ditolak")
		return
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		slog.Error("auth: LEGACY_REFRESH_ACCEPT_UNTIL tidak valid — token legacy ditolak", "value", raw)
		return
	}
	s.legacyDeadline = ts
}

// SetEmailSender memasang transport email. production=true berarti tanpa transport
// registrasi DITOLAK (fail closed) — tidak ada token yang dikembalikan lewat API.
func (s *AuthService) SetEmailSender(sender EmailSender, production bool) {
	s.mailer = sender
	s.production = production
}

// SetLegacyRefreshCutoff menetapkan batas waktu penerimaan token legacy.
func (s *AuthService) SetLegacyRefreshCutoff(t time.Time) { s.legacyDeadline = t }

// NewAuthServiceWithTokens membuat AuthService dengan store rotasi refresh token.
// Dipakai produksi: refresh token menjadi sekali pakai dan bisa dicabut.
func NewAuthServiceWithTokens(users domain.UserRepository, secret string, tokens domain.RefreshTokenRepository) *AuthService {
	svc := NewAuthService(users, secret)
	svc.tokens = tokens
	return svc
}

// NewAuthService creates an AuthService.
func NewAuthService(users domain.UserRepository, secret string) *AuthService {
	return &AuthService{
		users:      users,
		secret:     []byte(secret),
		accessTTL:  15 * time.Minute,
		refreshTTL: 30 * 24 * time.Hour,
		// legacyDeadline sengaja NOL sampai diset eksplisit: token legacy ditolak
		// bila cutoff tidak dikonfigurasi (fail closed, C2).
	}
}

// Register creates a new user account after validating the input.
// The account starts unverified; verification happens via VerifyEmail.
func (s *AuthService) Register(ctx context.Context, email, password, name string) (*domain.User, error) {
	fields := map[string]string{}
	if email == "" || !emailRe.MatchString(email) {
		fields["email"] = "must be a valid email address"
	}
	if len(password) < 8 {
		fields["password"] = "must be at least 8 characters"
	}
	if name == "" {
		fields["name"] = "is required"
	}
	if len(fields) > 0 {
		return nil, pkgerrors.ValidationMulti(fields)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("service.AuthService.Register: hash: %w", err)
	}

	if s.production && s.mailer == nil {
		// Fail closed: tanpa transport email, akun tidak boleh dibuat karena
		// pemilik email tidak akan pernah bisa memverifikasi.
		return nil, pkgerrors.Internal(errNoEmailTransport)
	}
	user := &domain.User{
		Email:               email,
		PasswordHash:        string(hash),
		Name:                name,
		NotificationEnabled: true,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err // already wrapped as conflict/internal by repo
	}
	// Terbitkan token verifikasi dan kirim ke PEMILIK EMAIL. Token tidak pernah
	// dikembalikan ke klien: kepemilikan email harus dibuktikan lewat transport.
	if s.verifications != nil && s.mailer != nil {
		token, terr := s.IssueVerificationToken(ctx, user.ID)
		if terr != nil {
			return nil, terr
		}
		if serr := s.mailer.SendVerification(ctx, user.Email, token); serr != nil {
			slog.Error("auth: gagal mengirim token verifikasi", "user_id", user.ID, "error", serr)
			return nil, pkgerrors.Internal(serr)
		}
	}
	return user, nil
}

// Login verifies credentials and returns a token pair.
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, *TokenPair, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if pkgerrors.IsNotFound(err) {
			return nil, nil, pkgerrors.Unauthorized("invalid email or password")
		}
		return nil, nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, pkgerrors.Unauthorized("invalid email or password")
	}

	pair, err := s.issueTokens(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

// Refresh menukar refresh token dengan pasangan token baru.
//
// Token opaque (terbitan baru) WAJIB sekali pakai: satu transaksi menandai token
// lama terpakai dan menerbitkan tepat satu pengganti. Pemakaian ulang dianggap
// replay dan mencabut seluruh keluarga token (identity-hardening R3).
//
// Token JWT lama tetap diterima sampai kedaluwarsa (R8) lalu "di-upgrade" menjadi
// token opaque, tanpa memaksa logout massal.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	if s.tokens == nil {
		return s.refreshLegacyStateless(ctx, refreshToken)
	}
	if strings.Count(refreshToken, ".") == 2 {
		return s.refreshLegacyUpgrade(ctx, refreshToken)
	}

	next, rawNext, err := s.newRefreshToken("", "", "")
	if err != nil {
		return nil, err
	}
	outcome, err := s.tokens.Rotate(ctx, hashToken(refreshToken), next)
	if err != nil {
		return nil, err
	}
	if outcome != domain.RotateOK {
		return nil, pkgerrors.Unauthorized("invalid or expired refresh token")
	}
	access, err := s.signToken(tokenTypeAccess, next.UserID, time.Now().UTC(), s.accessTTL)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: rawNext}, nil
}

// refreshLegacyStateless preserves the pre-rotation behaviour for services
// constructed without a token store (tests and compatibility only).
func (s *AuthService) refreshLegacyStateless(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := s.parseToken(refreshToken, tokenTypeRefresh)
	if err != nil {
		return nil, pkgerrors.Unauthorized("invalid or expired refresh token")
	}
	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return nil, pkgerrors.Unauthorized("invalid refresh token")
	}
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, userID)
}

// refreshLegacyUpgrade menerima JWT refresh lama lalu menerbitkan token opaque
// (upgrade bertahap, tanpa logout massal).
func (s *AuthService) refreshLegacyUpgrade(ctx context.Context, refreshToken string) (*TokenPair, error) {
	if s.legacyDeadline.IsZero() || time.Now().UTC().After(s.legacyDeadline) {
		// Tanpa cutoff eksplisit atau sudah lewat cutoff: token legacy ditolak.
		slog.Info("auth: legacy_refresh ditolak", "cutoff_set", !s.legacyDeadline.IsZero())
		return nil, pkgerrors.Unauthorized("invalid or expired refresh token")
	}
	claims, err := s.parseToken(refreshToken, tokenTypeRefresh)
	if err != nil {
		return nil, pkgerrors.Unauthorized("invalid or expired refresh token")
	}
	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return nil, pkgerrors.Unauthorized("invalid refresh token")
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, pkgerrors.Unauthorized("invalid refresh token")
	}

	// Upgrade atomik: token legacy ditandai terpakai dan penggantinya diterbitkan
	// dalam satu transaksi, sehingga pemakaian ulang terdeteksi sebagai replay.
	next, rawNext, nerr := s.newRefreshToken(userID, "", "")
	if nerr != nil {
		return nil, nerr
	}
	outcome, cerr := s.tokens.ConsumeLegacy(ctx, hashToken(refreshToken), next)
	if cerr != nil {
		return nil, cerr
	}
	if outcome != domain.RotateOK {
		return nil, pkgerrors.Unauthorized("invalid or expired refresh token")
	}
	access, aerr := s.signToken(tokenTypeAccess, userID, time.Now().UTC(), s.accessTTL)
	if aerr != nil {
		return nil, fmt.Errorf("service.AuthService.refreshLegacyUpgrade: access: %w", aerr)
	}
	slog.Info("auth: legacy_refresh upgraded to rotating token", "user_id", userID)
	return &TokenPair{AccessToken: access, RefreshToken: rawNext}, nil
}

// ParseAccessToken validates an access token and returns the subject user id.
func (s *AuthService) ParseAccessToken(tokenString string) (string, error) {
	claims, err := s.parseToken(tokenString, tokenTypeAccess)
	if err != nil {
		return "", err
	}
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", pkgerrors.Unauthorized("invalid access token")
	}
	return sub, nil
}

// VerifyEmail marks a user's email as verified given a verification token.
// The token is the user id itself for this pilot (no email transport);
// a production deployment would issue a random, expiring token.
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	if s.verifications == nil {
		return pkgerrors.Internal(errors.New("verifikasi email belum dikonfigurasi"))
	}
	// Konsumsi token dan penandaan email terverifikasi terjadi dalam satu
	// transaksi di repo: tidak ada jendela di mana token hangus tetapi status
	// email belum berubah, dan pemakaian paralel hanya bisa berhasil sekali.
	_, ok, err := s.verifications.ConsumeAndVerify(ctx, hashToken(token))
	if err != nil {
		return err
	}
	if !ok {
		// Token tidak dikenal, sudah dipakai, kedaluwarsa, atau kalah balapan.
		// Token lama berbasis user id juga jatuh ke sini (ditolak).
		// Spec R5/I11: replay token verifikasi dijawab 401, bukan 422.
		return pkgerrors.Unauthorized("invalid verification token")
	}
	return nil
}

// ForgotPassword generates a reset token for the given email. The token is
// logged (no email transport in this pilot) and returned so tests can use it.
func (s *AuthService) ForgotPassword(ctx context.Context, email string) (string, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		// Do not reveal whether the account exists.
		if pkgerrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return s.generateResetToken(user.ID)
}

// ResetPassword sets a new password for the user identified by resetToken.
func (s *AuthService) ResetPassword(ctx context.Context, resetToken, newPassword string) error {
	if len(newPassword) < 8 {
		return pkgerrors.Validation("password", "must be at least 8 characters")
	}
	userID, ok := s.parseResetToken(resetToken)
	if !ok {
		return pkgerrors.Validation("token", "invalid reset token")
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if pkgerrors.IsNotFound(err) {
			return pkgerrors.Validation("token", "invalid reset token")
		}
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("service.AuthService.ResetPassword: hash: %w", err)
	}
	// Atomik: hash + credential_version + pencabutan keluarga token dalam satu
	// transaksi (C4). Bila gagal, tidak ada perubahan sebagian.
	if s.tokens != nil {
		if rerr := s.tokens.UpdatePasswordAtomic(ctx, userID, string(hash), "password_change"); rerr != nil {
			return fmt.Errorf("service.AuthService.ResetPassword: update atomik: %w", rerr)
		}
		return nil
	}
	user.PasswordHash = string(hash)
	return s.users.Update(ctx, user)
}

// ChangePassword updates the password for an authenticated user after
// verifying their current password.
func (s *AuthService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return pkgerrors.Unauthorized("current password is incorrect")
	}
	if len(newPassword) < 8 {
		return pkgerrors.Validation("password", "must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("service.AuthService.ChangePassword: hash: %w", err)
	}
	if s.tokens != nil {
		if rerr := s.tokens.UpdatePasswordAtomic(ctx, userID, string(hash), "password_change"); rerr != nil {
			return fmt.Errorf("service.AuthService.ChangePassword: update atomik: %w", rerr)
		}
		return nil
	}
	user.PasswordHash = string(hash)
	return s.users.Update(ctx, user)
}

// --- internals ---

type tokenKind string

const (
	tokenTypeAccess  tokenKind = "access"
	tokenTypeRefresh tokenKind = "refresh"
)

// issueTokens mints a signed access + refresh token pair for the user.
func (s *AuthService) issueTokens(ctx context.Context, userID string) (*TokenPair, error) {
	now := time.Now().UTC()
	access, err := s.signToken(tokenTypeAccess, userID, now, s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("service.AuthService.issueTokens: access: %w", err)
	}
	if s.tokens != nil {
		user, uerr := s.users.FindByID(ctx, userID)
		if uerr != nil {
			return nil, uerr
		}
		row, raw, terr := s.newRefreshToken(userID, "", "")
		if terr == nil {
			row.CredentialVersion = user.CredentialVersion
		}
		if terr != nil {
			return nil, terr
		}
		if cerr := s.tokens.Create(ctx, row); cerr != nil {
			return nil, fmt.Errorf("service.AuthService.issueTokens: store refresh: %w", cerr)
		}
		access, aerr := s.signToken(tokenTypeAccess, userID, now, s.accessTTL)
		if aerr != nil {
			return nil, fmt.Errorf("service.AuthService.issueTokens: access: %w", aerr)
		}
		return &TokenPair{AccessToken: access, RefreshToken: raw}, nil
	}
	refresh, err := s.signToken(tokenTypeRefresh, userID, now, s.refreshTTL)
	if err != nil {
		return nil, fmt.Errorf("service.AuthService.issueTokens: refresh: %w", err)
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

// signToken creates a signed JWT with the given kind and expiry.
func (s *AuthService) signToken(kind tokenKind, userID string, now time.Time, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"typ": string(kind),
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

// parseToken validates signature/expiry and returns claims for the given kind.
func (s *AuthService) parseToken(tokenString string, kind tokenKind) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return nil, pkgerrors.Unauthorized("invalid or expired token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, pkgerrors.Unauthorized("invalid token claims")
	}
	if typ, _ := claims["typ"].(string); typ != string(kind) {
		return nil, pkgerrors.Unauthorized("invalid token type")
	}
	return claims, nil
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// generateResetToken creates an opaque reset token bound to the user id.
func (s *AuthService) generateResetToken(userID string) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("service.AuthService.generateResetToken: %w", err)
	}
	// token = <random 16 bytes hex>.<user id> — random part prevents guessing.
	return hex.EncodeToString(raw) + "." + userID, nil
}

// parseResetToken splits an opaque reset token into its user id component.
func (s *AuthService) parseResetToken(token string) (string, bool) {
	parts := splitN(token, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	if _, err := hex.DecodeString(parts[0]); err != nil {
		return "", false
	}
	return parts[1], true
}

func splitN(s, sep string, n int) []string {
	var out []string
	start := 0
	for i := 0; i < len(s) && len(out) < n-1; i++ {
		if s[i] == sep[0] && (len(sep) == 1 || s[i:i+len(sep)] == sep) {
			out = append(out, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	out = append(out, s[start:])
	return out
}
