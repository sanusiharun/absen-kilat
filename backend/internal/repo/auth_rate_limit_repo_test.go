package repo_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
	"github.com/sanusi/absen-kilat/backend/internal/service"
)

// I22: penghitung dipakai bersama lintas proses (dua pool terpisah ke file yang
// sama). 5 percobaan lewat pool A + 5 lewat pool B = 10 tercatat, percobaan ke-11
// ditolak secara agregat.
func TestAuthRateLimitsSharedAcrossPools(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rate-limit.db")

	poolA, err := db.Open(ctx, path)
	if err != nil {
		t.Fatalf("open pool A: %v", err)
	}
	defer poolA.Close()
	poolB, err := db.Open(ctx, path)
	if err != nil {
		t.Fatalf("open pool B: %v", err)
	}
	defer poolB.Close()

	limiterA := service.NewAuthRateLimiter(repo.NewAuthRateLimitRepository(poolA))
	limiterB := service.NewAuthRateLimiter(repo.NewAuthRateLimitRepository(poolB))
	const ip = "203.0.113.77"

	for i := 0; i < 5; i++ {
		if err := limiterA.Record(ctx, service.ScopeLoginIP, ip); err != nil {
			t.Fatalf("record A%d: %v", i+1, err)
		}
	}
	for i := 0; i < 5; i++ {
		if err := limiterB.Record(ctx, service.ScopeLoginIP, ip); err != nil {
			t.Fatalf("record B%d: %v", i+1, err)
		}
	}

	ok, err := limiterB.Allowed(ctx, service.ScopeLoginIP, ip)
	if err != nil {
		t.Fatalf("allowed: %v", err)
	}
	if ok {
		t.Fatal("attempt 11 must be rejected across pools (I22)")
	}

	// IP lain tidak terpengaruh.
	if ok, err := limiterA.Allowed(ctx, service.ScopeLoginIP, "198.51.100.4"); err != nil || !ok {
		t.Fatalf("other IP must stay allowed, ok=%v err=%v", ok, err)
	}
}

// Tabel hanya menyimpan hash: nilai mentah IP tidak boleh muncul.
func TestAuthRateLimitRepository_StoresHashOnly(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rate-limit.db")
	pool, err := db.Open(ctx, path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()

	r := repo.NewAuthRateLimitRepository(pool)
	limiter := service.NewAuthRateLimiter(r)
	const ip = "203.0.113.88"
	if err := limiter.Record(ctx, service.ScopeLoginIP, ip); err != nil {
		t.Fatalf("record: %v", err)
	}

	var raw int
	if err := pool.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM auth_rate_limits WHERE key_hash = ?`, ip).Scan(&raw); err != nil {
		t.Fatalf("query: %v", err)
	}
	if raw != 0 {
		t.Fatal("raw IP stored in auth_rate_limits")
	}
}
