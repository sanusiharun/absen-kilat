package service

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"sync"
	"testing"
)

// fakeLimitStore meniru penyimpanan penghitung dan merekam kunci yang ditulis.
type fakeLimitStore struct {
	mu     sync.Mutex
	counts map[string]int
	keys   []string
	err    error
}

func newFakeLimitStore() *fakeLimitStore {
	return &fakeLimitStore{counts: map[string]int{}}
}

func bucket(scope, keyHash string, windowStart int64) string {
	return scope + "|" + keyHash + "|" + strconv.FormatInt(windowStart, 10)
}

func (f *fakeLimitStore) Count(_ context.Context, scope, keyHash string, windowStart int64) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[bucket(scope, keyHash, windowStart)], nil
}

func (f *fakeLimitStore) Increment(_ context.Context, scope, keyHash string, windowStart, _ int64) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys = append(f.keys, keyHash)
	key := bucket(scope, keyHash, windowStart)
	f.counts[key]++
	return f.counts[key], nil
}

// I10: sepuluh kegagalan login per IP masih dicatat, percobaan ke-11 ditolak.
func TestAuthRateLimiter_BlocksEleventhLoginFailure(t *testing.T) {
	store := newFakeLimitStore()
	limiter := NewAuthRateLimiter(store)
	ctx := context.Background()
	const ip = "203.0.113.10"

	for i := 0; i < 10; i++ {
		ok, err := limiter.Allowed(ctx, ScopeLoginIP, ip)
		if err != nil || !ok {
			t.Fatalf("attempt %d should be allowed, ok=%v err=%v", i+1, ok, err)
		}
		if err := limiter.Record(ctx, ScopeLoginIP, ip); err != nil {
			t.Fatalf("record attempt %d: %v", i+1, err)
		}
	}

	ok, err := limiter.Allowed(ctx, ScopeLoginIP, ip)
	if err != nil {
		t.Fatalf("allowed: %v", err)
	}
	if ok {
		t.Fatal("attempt 11 must be blocked (I10)")
	}
}

// Kunci tersimpan harus hash, bukan IP/email mentah.
func TestAuthRateLimiter_StoresHashedKeysOnly(t *testing.T) {
	store := newFakeLimitStore()
	limiter := NewAuthRateLimiter(store)
	ctx := context.Background()
	const email = "Budi.Santoso@Example.com"

	if err := limiter.Record(ctx, ScopeLoginAccount, email); err != nil {
		t.Fatalf("record: %v", err)
	}
	if len(store.keys) != 1 {
		t.Fatalf("expected 1 stored key, got %d", len(store.keys))
	}
	stored := store.keys[0]
	if stored == email || stored == "budi.santoso@example.com" {
		t.Fatalf("raw email stored: %q", stored)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(stored) {
		t.Fatalf("expected sha256 hex key, got %q", stored)
	}
	if hashRateLimitKey(ScopeLoginAccount, email) != hashRateLimitKey(ScopeLoginAccount, "  budi.santoso@example.COM  ") {
		t.Fatal("email normalization must be case/space insensitive")
	}
}

func TestAuthRateLimiter_ScopesAreIndependent(t *testing.T) {
	store := newFakeLimitStore()
	limiter := NewAuthRateLimiter(store)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_ = limiter.Record(ctx, ScopeLoginAccount, "a@example.com")
	}
	if ok, _ := limiter.Allowed(ctx, ScopeLoginAccount, "a@example.com"); ok {
		t.Fatal("a@example.com must be blocked")
	}
	if ok, _ := limiter.Allowed(ctx, ScopeLoginAccount, "b@example.com"); !ok {
		t.Fatal("b@example.com must not be blocked by another account")
	}
	if ok, _ := limiter.Allowed(ctx, ScopeLoginIP, "203.0.113.10"); !ok {
		t.Fatal("IP scope must not be affected by account scope")
	}
}

// Fail closed: store error berarti permintaan ditolak, bukan diloloskan.
func TestAuthRateLimiter_FailsClosedOnStoreError(t *testing.T) {
	store := newFakeLimitStore()
	store.err = errors.New("db down")
	limiter := NewAuthRateLimiter(store)

	ok, err := limiter.Allowed(context.Background(), ScopeLoginIP, "203.0.113.10")
	if err == nil {
		t.Fatal("expected error")
	}
	if ok {
		t.Fatal("store error must fail closed")
	}
}

func TestAuthRateLimiter_NilLimiterAllows(t *testing.T) {
	var limiter *AuthRateLimiter
	if ok, err := limiter.Allowed(context.Background(), ScopeLoginIP, "1.2.3.4"); err != nil || !ok {
		t.Fatalf("nil limiter must allow, ok=%v err=%v", ok, err)
	}
	if err := limiter.Record(context.Background(), ScopeLoginIP, "1.2.3.4"); err != nil {
		t.Fatalf("nil limiter record: %v", err)
	}
}
