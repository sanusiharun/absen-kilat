package service

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
)

// Tes invarian rotasi refresh token (identity-hardening R3/R8, I8/I9/I13/I14/I19).
// Semua memakai database sungguhan (migrasi diterapkan) — tidak ada mock yang bisa
// menyembunyikan kesalahan transaksi.

func newRotationEnv(t *testing.T) (*AuthService, *repo.UserRepository) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rot.db")
	pool, err := db.Open(ctx, path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	users := repo.NewUserRepository(pool)
	tokens := repo.NewRefreshTokenRepository(pool)
	svc := NewAuthServiceWithTokens(users, "test-secret", tokens)
	return svc, users
}

func registerUser(t *testing.T, svc *AuthService, email, password string) *TokenPair {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.Register(ctx, email, password, "U"); err != nil {
		t.Fatalf("Register(%s): %v", email, err)
	}
	_, pair, err := svc.Login(ctx, email, password)
	if err != nil {
		t.Fatalf("Login(%s): %v", email, err)
	}
	if pair == nil || pair.RefreshToken == "" {
		t.Fatalf("Login(%s) tidak mengembalikan refresh token", email)
	}
	return pair
}

// I13: dua refresh paralel dengan token yang sama — tepat satu berhasil.
func TestRotation_ConcurrentRefreshSingleWinner(t *testing.T) {
	svc, _ := newRotationEnv(t)
	pair := registerUser(t, svc, "concurrent@test.dev", "rahasia-panjang-123")

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		success int
		failed  int
	)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, err := svc.Refresh(context.Background(), pair.RefreshToken)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success++
			} else {
				failed++
			}
		}()
	}
	wg.Wait()

	if success != 1 || failed != 1 {
		t.Fatalf("harus tepat satu berhasil dan satu gagal, dapat sukses=%d gagal=%d", success, failed)
	}
}

// I14: replay token yang sudah dipakai mencabut seluruh keluarga token.
func TestRotation_ReplayRevokesFamily(t *testing.T) {
	svc, _ := newRotationEnv(t)
	ctx := context.Background()
	pair := registerUser(t, svc, "replay@test.dev", "rahasia-panjang-123")

	second, err := svc.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh pertama harus berhasil: %v", err)
	}

	// Pakai ulang token lama: harus 401 (replay) dan mencabut keluarga.
	if _, err := svc.Refresh(ctx, pair.RefreshToken); err == nil {
		t.Fatal("replay token lama harus ditolak")
	}
	// Anak token yang tadinya sah ikut tercabut.
	if _, err := svc.Refresh(ctx, second.RefreshToken); err == nil {
		t.Fatal("token keluarga yang sudah dicabut harus ditolak")
	}
}

// I8: akun dinonaktifkan tidak bisa memperpanjang sesi.
func TestRotation_InactiveAccountCannotRefresh(t *testing.T) {
	svc, users := newRotationEnv(t)
	ctx := context.Background()
	pair := registerUser(t, svc, "inactive@test.dev", "rahasia-panjang-123")

	u, err := users.FindByEmail(ctx, "inactive@test.dev")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	u.IsActive = false
	if err := users.Update(ctx, u); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if _, err := svc.Refresh(ctx, pair.RefreshToken); err == nil {
		t.Fatal("akun nonaktif tidak boleh bisa refresh")
	}
}

// I9: password berubah menggugurkan semua sesi.
func TestRotation_PasswordChangeRevokesAllSessions(t *testing.T) {
	svc, users := newRotationEnv(t)
	ctx := context.Background()
	pair := registerUser(t, svc, "pwd@test.dev", "rahasia-panjang-123")

	u, err := users.FindByEmail(ctx, "pwd@test.dev")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if err := svc.ChangePassword(ctx, u.ID, "rahasia-panjang-123", "rahasia-baru-456"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, err := svc.Refresh(ctx, pair.RefreshToken); err == nil {
		t.Fatal("refresh token lama harus gugur setelah password berubah")
	}
}

// I19: refresh token JWT legacy masih diterima sampai kedaluwarsa, lalu di-upgrade
// menjadi token opaque (tanpa paksa logout massal).
func TestRotation_LegacyRefreshUpgraded(t *testing.T) {
	svc, users := newRotationEnv(t)
	ctx := context.Background()
	pair := registerUser(t, svc, "legacy@test.dev", "rahasia-panjang-123")

	u, err := users.FindByEmail(ctx, "legacy@test.dev")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	// Token JWT gaya lama (stateless) dibuat manual seperti versi sebelumnya.
	legacy, err := svc.signToken(tokenTypeRefresh, u.ID, time.Now().UTC(), svc.refreshTTL)
	if err != nil {
		t.Fatalf("signToken: %v", err)
	}
	if strings.Count(legacy, ".") != 2 {
		t.Fatalf("token legacy harus berbentuk JWT, dapat %q", legacy)
	}

	upgraded, err := svc.Refresh(ctx, legacy)
	if err != nil {
		t.Fatalf("token legacy harus masih diterima: %v", err)
	}
	if upgraded == nil || upgraded.RefreshToken == "" {
		t.Fatal("hasil upgrade harus berisi refresh token baru")
	}
	if strings.Count(upgraded.RefreshToken, ".") != 0 {
		t.Fatalf("token hasil upgrade harus opaque (bukan JWT), dapat %q", upgraded.RefreshToken)
	}
	// Dan token opaque itu benar-benar bisa dipakai (sekali).
	if _, err := svc.Refresh(ctx, upgraded.RefreshToken); err != nil {
		t.Fatalf("token opaque hasil upgrade harus bisa dipakai: %v", err)
	}
	_ = pair
}

// Token baru tidak boleh sama dengan token lama (rotasi benar-benar berganti).
func TestRotation_TokenChangesOnRefresh(t *testing.T) {
	svc, _ := newRotationEnv(t)
	ctx := context.Background()
	pair := registerUser(t, svc, "rotate@test.dev", "rahasia-panjang-123")

	next, err := svc.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if next.RefreshToken == pair.RefreshToken {
		t.Fatal("refresh token harus berganti setiap rotasi")
	}
	if next.AccessToken == "" {
		t.Fatal("access token baru harus ada")
	}
}
