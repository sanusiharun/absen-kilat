package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// Di non-production: token dikirim ke transport (mock), TIDAK dikembalikan
// lewat service, dan hanya bisa dipakai lewat jalur verifikasi.
func TestRegister_DeliversTokenViaSenderNotReturnValue(t *testing.T) {
	ctx := context.Background()
	pool, err := db.Open(ctx, filepath.Join(t.TempDir(), "mail.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	users := repo.NewUserRepository(pool)
	svc := NewAuthService(users, "secret")
	svc.SetEmailVerifications(repo.NewEmailVerificationRepository(pool))
	sender := NewLogEmailSender()
	svc.SetEmailSender(sender, false)

	user, err := svc.Register(ctx, "kirim@test.dev", "rahasia-panjang-123", "U")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if user.IsEmailVerified {
		t.Fatal("akun baru tidak boleh langsung terverifikasi")
	}
	token, ok := sender.TokenFor("kirim@test.dev")
	if !ok || token == "" {
		t.Fatal("token harus terkirim lewat transport email")
	}
	if token == user.ID {
		t.Fatal("token tidak boleh sama dengan user id")
	}
	if err := svc.VerifyEmail(ctx, token); err != nil {
		t.Fatalf("token dari transport harus bisa memverifikasi: %v", err)
	}
}

// Di production tanpa transport email: registrasi DITOLAK (fail closed) dan
// tidak ada akun yang tercipta tanpa kemungkinan verifikasi.
func TestRegister_ProductionWithoutTransportFailsClosed(t *testing.T) {
	ctx := context.Background()
	pool, err := db.Open(ctx, filepath.Join(t.TempDir(), "prod.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	users := repo.NewUserRepository(pool)
	svc := NewAuthService(users, "secret")
	svc.SetEmailVerifications(repo.NewEmailVerificationRepository(pool))
	svc.SetEmailSender(nil, true)

	_, rerr := svc.Register(ctx, "prod@test.dev", "rahasia-panjang-123", "U")
	if rerr == nil {
		t.Fatal("tanpa transport email di production, registrasi harus ditolak")
	}
	if !pkgerrors.IsInternal(rerr) {
		t.Fatalf("error harus internal (fail closed), dapat: %v", rerr)
	}
	if _, ferr := users.FindByEmail(ctx, "prod@test.dev"); ferr == nil {
		t.Fatal("akun tidak boleh tercipta saat registrasi ditolak")
	}
}
