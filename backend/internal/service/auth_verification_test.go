package service

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// Tes regresi slice 2b-1 (verifikasi email): atomisitas, konsumsi paralel,
// dan integrasi penerbitan -> pengiriman -> verifikasi.

func newVerificationEnv(t *testing.T) (*AuthService, *repo.UserRepository) {
	t.Helper()
	ctx := context.Background()
	pool, err := db.Open(ctx, filepath.Join(t.TempDir(), "verif.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	users := repo.NewUserRepository(pool)
	svc := NewAuthService(users, "secret")
	svc.SetEmailVerifications(repo.NewEmailVerificationRepository(pool))
	return svc, users
}

// Atomisitas: setelah verifikasi berhasil, token terpakai DAN status email berubah.
// Tidak boleh ada keadaan token hangus tetapi email tetap belum terverifikasi.
func TestVerifyEmail_ConsumeAndStatusAtomic(t *testing.T) {
	ctx := context.Background()
	svc, users := newVerificationEnv(t)

	user, err := svc.Register(ctx, "atomic@test.dev", "rahasia-panjang-123", "U")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	token, err := svc.IssueVerificationToken(ctx, user.ID)
	if err != nil {
		t.Fatalf("IssueVerificationToken: %v", err)
	}
	if err := svc.VerifyEmail(ctx, token); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	got, err := users.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !got.IsEmailVerified {
		t.Fatal("status email harus sudah terverifikasi setelah token dikonsumsi")
	}
	// Token sudah terpakai: pemakaian kedua ditolak, status tetap terverifikasi.
	if err := svc.VerifyEmail(ctx, token); err == nil {
		t.Fatal("pemakaian kedua token harus ditolak")
	}
	again, err := users.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !again.IsEmailVerified {
		t.Fatal("status terverifikasi tidak boleh berubah setelah pemakaian kedua")
	}
}

// Konsumsi paralel dengan token sama: tepat satu yang berhasil.
func TestVerifyEmail_ConcurrentConsumeOnlyOnce(t *testing.T) {
	ctx := context.Background()
	svc, users := newVerificationEnv(t)

	user, err := svc.Register(ctx, "raceverify@test.dev", "rahasia-panjang-123", "U")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	token, err := svc.IssueVerificationToken(ctx, user.ID)
	if err != nil {
		t.Fatalf("IssueVerificationToken: %v", err)
	}

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
			err := svc.VerifyEmail(ctx, token)
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
	got, err := users.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !got.IsEmailVerified {
		t.Fatal("email harus terverifikasi setelah pemenang balapan selesai")
	}
}

// Integrasi penerbitan -> pengiriman -> verifikasi, termasuk terbit ulang:
// token baru berlaku, token lama langsung tidak berlaku.
func TestVerifyEmail_IssueDeliveryReissueFlow(t *testing.T) {
	ctx := context.Background()
	svc, users := newVerificationEnv(t)

	user, err := svc.Register(ctx, "flow@test.dev", "rahasia-panjang-123", "U")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Penerbitan pertama (disimulasikan sebagai payload respons register).
	first, err := svc.IssueVerificationToken(ctx, user.ID)
	if err != nil {
		t.Fatalf("Issue #1: %v", err)
	}
	if first == user.ID {
		t.Fatal("token tidak boleh sama dengan user id")
	}

	// Terbit ulang: token lama harus mati (tidak menumpuk token aktif).
	second, err := svc.IssueVerificationToken(ctx, user.ID)
	if err != nil {
		t.Fatalf("Issue #2: %v", err)
	}
	if second == first {
		t.Fatal("token hasil terbit ulang harus berbeda")
	}
	if err := svc.VerifyEmail(ctx, first); err == nil {
		t.Fatal("token lama harus tidak berlaku setelah terbit ulang")
	}

	// Token terbaru berhasil memverifikasi.
	if err := svc.VerifyEmail(ctx, second); err != nil {
		t.Fatalf("token terbaru harus berhasil: %v", err)
	}
	got, err := users.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !got.IsEmailVerified {
		t.Fatal("email harus terverifikasi")
	}
}

// Tanpa store verifikasi, verifikasi email DITOLAK (fail closed), bukan memakai
// token berbasis user id seperti perilaku lama.
func TestVerifyEmail_FailClosedWithoutStore(t *testing.T) {
	ctx := context.Background()
	svc, users := newVerificationEnv(t)
	user, err := svc.Register(ctx, "nostore@test.dev", "rahasia-panjang-123", "U")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	bare := NewAuthService(users, "secret") // sengaja tanpa store verifikasi
	verr := bare.VerifyEmail(ctx, user.ID)
	if verr == nil {
		t.Fatal("tanpa store, verifikasi harus gagal (fail closed)")
	}
	if !pkgerrors.IsInternal(verr) && !pkgerrors.IsValidation(verr) {
		t.Fatalf("error harus internal/validation, dapat: %v", verr)
	}
	if _, err := bare.IssueVerificationToken(ctx, user.ID); err == nil {
		t.Fatal("tanpa store, penerbitan token harus gagal")
	}
}
