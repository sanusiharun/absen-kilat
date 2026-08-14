// Package service contains the application business logic, one type per
// bounded concern (auth, attendance, profile).
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
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
}

// NewAuthService creates an AuthService.
func NewAuthService(users domain.UserRepository, secret string) *AuthService {
	return &AuthService{
		users:      users,
		secret:     []byte(secret),
		accessTTL:  15 * time.Minute,
		refreshTTL: 30 * 24 * time.Hour,
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

	user := &domain.User{
		Email:               email,
		PasswordHash:        string(hash),
		Name:                name,
		NotificationEnabled: true,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err // already wrapped as conflict/internal by repo
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

	pair, err := s.issueTokens(user.ID)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

// Refresh validates a refresh token and issues a new access token.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
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
	return s.issueTokens(userID)
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
	user, err := s.users.FindByID(ctx, token)
	if err != nil {
		return pkgerrors.Validation("token", "invalid verification token")
	}
	user.IsEmailVerified = true
	return s.users.Update(ctx, user)
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
func (s *AuthService) issueTokens(userID string) (*TokenPair, error) {
	now := time.Now().UTC()
	access, err := s.signToken(tokenTypeAccess, userID, now, s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("service.AuthService.issueTokens: access: %w", err)
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
