package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
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
				return
			}
			if !pkgerrors.IsUnauthorized(err) {
				t.Errorf("yang kalah harus 401 unauthorized, dapat: %v", err)
			}
			failed++
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

// Fake store untuk menyuntikkan kegagalan pencabutan (fail-closed password).
type failingRevokeStore struct{ domain.RefreshTokenRepository }

func (f *failingRevokeStore) Create(context.Context, *domain.RefreshToken) error { return nil }

func (f *failingRevokeStore) Rotate(context.Context, string, *domain.RefreshToken) (domain.RotateOutcome, error) {
	return domain.RotateNotFound, nil
}

func (f *failingRevokeStore) ConsumeLegacy(context.Context, string, *domain.RefreshToken) (domain.RotateOutcome, error) {
	return domain.RotateNotFound, nil
}

func (f *failingRevokeStore) RevokeFamily(context.Context, string, string) error { return nil }

func (f *failingRevokeStore) RevokeAllForUser(context.Context, string, string) error {
	return errors.New("storage unavailable")
}

// Password TIDAK boleh berubah bila pencabutan sesi gagal (fail closed).
func TestRotation_PasswordChangeFailsClosedWhenRevokeFails(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "f.css.db")
	pool, err := db.Open(ctx, path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	users := repo.NewUserRepository(pool)
	svc := NewAuthServiceWithTokens(users, "secret", &failingRevokeStore{})

	if _, err := svc.Register(ctx, "failclosed@test.dev", "rahasia-panjang-123", "U"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	u, err := users.FindByEmail(ctx, "failclosed@test.dev")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	before := u.PasswordHash

	if err := svc.ChangePassword(ctx, u.ID, "rahasia-panjang-123", "rahasia-baru-456"); err == nil {
		t.Fatal("ganti password harus gagal bila pencabutan sesi gagal")
	}
	after, err := users.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if after.PasswordHash != before {
		t.Fatal("password tidak boleh berubah saat pencabutan sesi gagal")
	}
}

// Penguncian transaksi harus terbukti lintas koneksi: dua pool terpisah pada file
// DB yang sama, satu token, dua request paralel -> tepat satu menang.
func TestRotation_LockingAcrossConnections(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "two.db")

	poolA, err := db.Open(ctx, path)
	if err != nil {
		t.Fatalf("db.Open A: %v", err)
	}
	t.Cleanup(func() { poolA.Close() })
	poolB, err := db.Open(ctx, path)
	if err != nil {
		t.Fatalf("db.Open B: %v", err)
	}
	t.Cleanup(func() { poolB.Close() })

	svcA := NewAuthServiceWithTokens(repo.NewUserRepository(poolA), "secret", repo.NewRefreshTokenRepository(poolA))
	svcB := NewAuthServiceWithTokens(repo.NewUserRepository(poolB), "secret", repo.NewRefreshTokenRepository(poolB))

	if _, err := svcA.Register(ctx, "cross@test.dev", "rahasia-panjang-123", "U"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, pair, err := svcA.Login(ctx, "cross@test.dev", "rahasia-panjang-123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		success int
		unauth  int
	)
	wg.Add(2)
	for _, svc := range []*AuthService{svcA, svcB} {
		go func(s *AuthService) {
			defer wg.Done()
			_, rerr := s.Refresh(ctx, pair.RefreshToken)
			mu.Lock()
			defer mu.Unlock()
			if rerr == nil {
				success++
			} else if pkgerrors.IsUnauthorized(rerr) {
				unauth++
			}
		}(svc)
	}
	wg.Wait()

	if success != 1 || unauth != 1 {
		t.Fatalf("lintas koneksi: harus tepat 1 sukses dan 1 unauthorized, dapat sukses=%d unauth=%d", success, unauth)
	}
}

// Pemakaian ulang token legacy harus terdeteksi replay dan mencabut keluarga.
func TestRotation_LegacyReplayRevokesFamily(t *testing.T) {
	svc, users := newRotationEnv(t)
	ctx := context.Background()
	registerUser(t, svc, "legacyreplay@test.dev", "rahasia-panjang-123")
	u, err := users.FindByEmail(ctx, "legacyreplay@test.dev")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	legacy, err := svc.signToken(tokenTypeRefresh, u.ID, time.Now().UTC(), svc.refreshTTL)
	if err != nil {
		t.Fatalf("signToken: %v", err)
	}

	first, err := svc.Refresh(ctx, legacy)
	if err != nil {
		t.Fatalf("pemakaian pertama token legacy harus berhasil: %v", err)
	}
	if _, err := svc.Refresh(ctx, legacy); !pkgerrors.IsUnauthorized(err) {
		t.Fatalf("pemakaian ulang token legacy harus 401, dapat: %v", err)
	}
	if _, err := svc.Refresh(ctx, first.RefreshToken); !pkgerrors.IsUnauthorized(err) {
		t.Fatalf("token pengganti harus ikut tercabut, dapat: %v", err)
	}
}

// Setelah cutoff release+7 hari, token legacy ditolak.
func TestRotation_LegacyCutoffRejects(t *testing.T) {
	svc, users := newRotationEnv(t)
	ctx := context.Background()
	registerUser(t, svc, "cutoff@test.dev", "rahasia-panjang-123")
	u, err := users.FindByEmail(ctx, "cutoff@test.dev")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	legacy, err := svc.signToken(tokenTypeRefresh, u.ID, time.Now().UTC(), svc.refreshTTL)
	if err != nil {
		t.Fatalf("signToken: %v", err)
	}
	svc.SetLegacyRefreshCutoff(time.Now().Add(-time.Hour))
	if _, err := svc.Refresh(ctx, legacy); !pkgerrors.IsUnauthorized(err) {
		t.Fatalf("token legacy setelah cutoff harus 401, dapat: %v", err)
	}
}

// Expiry yang tidak terbaca harus fail closed (tidak boleh lolos jadi sukses).
func TestRotation_MalformedExpiryFailsClosed(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "mal.db")
	pool, err := db.Open(ctx, path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	users := repo.NewUserRepository(pool)
	tokens := repo.NewRefreshTokenRepository(pool)
	svc := NewAuthServiceWithTokens(users, "secret", tokens)

	if _, err := svc.Register(ctx, "malformed@test.dev", "rahasia-panjang-123", "U"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, pair, err := svc.Login(ctx, "malformed@test.dev", "rahasia-panjang-123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if _, err := pool.ExecContext(ctx,
		`UPDATE refresh_tokens SET expires_at = 'bukan-tanggal' WHERE token_hash = ?`,
		hashToken(pair.RefreshToken)); err != nil {
		t.Fatalf("update expiry: %v", err)
	}
	if _, err := svc.Refresh(ctx, pair.RefreshToken); !pkgerrors.IsUnauthorized(err) {
		t.Fatalf("expiry rusak harus ditolak 401, dapat: %v", err)
	}
}
