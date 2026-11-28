package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
)

type fakeUserRepo struct {
	byEmail map[string]*domain.User
	byID    map[string]*domain.User
	seq     int
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byEmail: map[string]*domain.User{}, byID: map[string]*domain.User{}}
}

func (f *fakeUserRepo) Create(_ context.Context, u *domain.User) error {
	if _, dup := f.byEmail[strings.ToLower(u.Email)]; dup {
		return &domainConflict{}
	}
	f.seq++
	u.ID = "user-" + string(rune('a'+f.seq-1))
	f.byEmail[strings.ToLower(u.Email)] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	u, ok := f.byEmail[strings.ToLower(email)]
	if !ok {
		return nil, &domainNotFound{}
	}
	return u, nil
}

func (f *fakeUserRepo) FindByID(_ context.Context, id string) (*domain.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, &domainNotFound{}
	}
	return u, nil
}

func (f *fakeUserRepo) Update(_ context.Context, u *domain.User) error {
	if _, ok := f.byID[u.ID]; !ok {
		return &domainNotFound{}
	}
	f.byID[u.ID] = u
	f.byEmail[strings.ToLower(u.Email)] = u
	return nil
}

// domainConflict/domainNotFound let tests assert via the repo's own error types
// without importing pkg/errors in the fake.
type domainConflict struct{}

func (*domainConflict) Error() string { return "conflict" }

type domainNotFound struct{}

func (*domainNotFound) Error() string { return "not found" }

func TestAuthRegisterAndLogin(t *testing.T) {
	ctx := context.Background()
	svc := NewAuthService(newFakeUserRepo(), "test-secret")

	user, err := svc.Register(ctx, "user@example.com", "password123", "User One")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if user.Email != "user@example.com" {
		t.Fatalf("email = %q", user.Email)
	}
	if user.IsEmailVerified {
		t.Fatal("new user should be unverified")
	}

	// Duplicate email must fail.
	if _, err := svc.Register(ctx, "USER@example.com", "password123", "Dup"); err == nil {
		t.Fatal("duplicate register should fail")
	}

	// Login with correct password.
	_, pair, err := svc.Login(ctx, "user@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("tokens must be non-empty")
	}

	// Login with wrong password.
	if _, _, err := svc.Login(ctx, "user@example.com", "wrong"); err == nil {
		t.Fatal("wrong password should fail")
	}
}

func TestAuthRegisterValidation(t *testing.T) {
	svc := NewAuthService(newFakeUserRepo(), "secret")

	_, err := svc.Register(context.Background(), "not-an-email", "short", "")
	if err == nil {
		t.Fatal("invalid input should fail")
	}
}

func TestAuthTokenRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc := NewAuthService(newFakeUserRepo(), "test-secret")

	user, err := svc.Register(ctx, "tok@example.com", "password123", "Tok")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, pair, err := svc.Login(ctx, "tok@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Access token parses to the right user.
	got, err := svc.ParseAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if got != user.ID {
		t.Fatalf("subject = %q, want %q", got, user.ID)
	}

	// Refresh token issues a new pair.
	newPair, err := svc.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if newPair.AccessToken == "" {
		t.Fatal("refresh must issue a new access token")
	}

	// A refresh token must not be accepted as an access token.
	if _, err := svc.ParseAccessToken(pair.RefreshToken); err == nil {
		t.Fatal("refresh token should not parse as access token")
	}
}

func TestAuthResetPassword(t *testing.T) {
	ctx := context.Background()
	svc := NewAuthService(newFakeUserRepo(), "secret")

	user, err := svc.Register(ctx, "reset@example.com", "password123", "Reset")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	token, err := svc.ForgotPassword(ctx, "reset@example.com")
	if err != nil {
		t.Fatalf("ForgotPassword: %v", err)
	}
	if !strings.Contains(token, user.ID) {
		t.Fatalf("reset token should embed user id, got %q", token)
	}

	if err := svc.ResetPassword(ctx, token, "newpassword1"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	if _, _, err := svc.Login(ctx, "reset@example.com", "newpassword1"); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	if _, _, err := svc.Login(ctx, "reset@example.com", "password123"); err == nil {
		t.Fatal("old password should no longer work")
	}
}

func TestVerifyEmail(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	pool, err := db.Open(ctx, filepath.Join(dir, "verify.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	users := repo.NewUserRepository(pool)
	svc := NewAuthService(users, "secret")
	svc.SetEmailVerifications(repo.NewEmailVerificationRepository(pool))

	user, err := svc.Register(ctx, "verify@example.com", "password123", "Ver")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Token berbasis user id (perilaku lama) HARUS ditolak (R5).
	if err := svc.VerifyEmail(ctx, user.ID); err == nil {
		t.Fatal("token berbasis user id harus ditolak")
	}

	token, err := svc.IssueVerificationToken(ctx, user.ID)
	if err != nil {
		t.Fatalf("IssueVerificationToken: %v", err)
	}
	if token == user.ID {
		t.Fatal("token verifikasi tidak boleh sama dengan user id")
	}
	if err := svc.VerifyEmail(ctx, token); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	got, err := users.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !got.IsEmailVerified {
		t.Fatal("email should be verified")
	}
	// Sekali pakai: pemakaian kedua harus ditolak.
	if err := svc.VerifyEmail(ctx, token); err == nil {
		t.Fatal("token verifikasi hanya boleh dipakai sekali")
	}
}
