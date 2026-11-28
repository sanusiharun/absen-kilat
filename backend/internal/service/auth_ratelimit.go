package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Scope rate limit autentikasi (identity-hardening R4). Dipisah per IP dan per
// akun supaya satu IP tidak bisa menebak banyak akun sekaligus, dan satu akun
// tidak bisa dibanjiri dari banyak IP.
const (
	ScopeLoginIP       = "auth.login.ip"
	ScopeLoginAccount  = "auth.login.account"
	ScopeForgotIP      = "auth.forgot.ip"
	ScopeForgotAccount = "auth.forgot.account"
	ScopeResetIP       = "auth.reset.ip"
	ScopeVerifyEmailIP = "auth.verify.ip"
)

// AuthRateWindow adalah lebar jendela tetap penghitung.
const AuthRateWindow = time.Minute

// Batas default per jendela. Angka login mengikuti I10 (percobaan ke-11 dalam
// satu menit ditolak). Nilai lain dipilih konservatif dan bisa ditinjau owner.
var authRateLimits = map[string]int{
	ScopeLoginIP:       10,
	ScopeLoginAccount:  10,
	ScopeForgotIP:      5,
	ScopeForgotAccount: 5,
	ScopeResetIP:       10,
	ScopeVerifyEmailIP: 10,
}

// AuthRateLimitStore adalah penyimpanan penghitung. Implementasi produksi
// memakai SQLite sehingga batas berlaku lintas proses (I22).
type AuthRateLimitStore interface {
	Count(ctx context.Context, scope, keyHash string, windowStart int64) (int, error)
	Increment(ctx context.Context, scope, keyHash string, windowStart, pruneBefore int64) (int, error)
}

// AuthRateLimiter membatasi percobaan autentikasi. Nil limiter berarti tidak
// ada pembatasan (hanya dipakai tes lama yang tidak menyentuh auth rate limit).
type AuthRateLimiter struct {
	store AuthRateLimitStore
	now   func() time.Time
}

// NewAuthRateLimiter creates an AuthRateLimiter backed by store.
func NewAuthRateLimiter(store AuthRateLimitStore) *AuthRateLimiter {
	return &AuthRateLimiter{store: store, now: func() time.Time { return time.Now().UTC() }}
}

// LimitFor mengembalikan batas untuk scope; 0 berarti scope tidak dibatasi.
func (l *AuthRateLimiter) LimitFor(scope string) int {
	if l == nil {
		return 0
	}
	return authRateLimits[scope]
}

// Allowed melaporkan apakah percobaan berikutnya masih di bawah batas.
// Kesalahan store bersifat fail closed: (false, err) — pemanggil menolak
// permintaan dengan respons generik, bukan meloloskan.
func (l *AuthRateLimiter) Allowed(ctx context.Context, scope, key string) (bool, error) {
	limit := l.LimitFor(scope)
	if limit == 0 || key == "" {
		return true, nil
	}
	n, err := l.store.Count(ctx, scope, hashRateLimitKey(scope, key), l.windowStart())
	if err != nil {
		return false, err
	}
	return n < limit, nil
}

// Record mencatat satu percobaan (kegagalan login, atau permintaan forgot/reset/
// verifikasi) pada jendela berjalan.
func (l *AuthRateLimiter) Record(ctx context.Context, scope, key string) error {
	if l.LimitFor(scope) == 0 || key == "" {
		return nil
	}
	start := l.windowStart()
	_, err := l.store.Increment(ctx, scope, hashRateLimitKey(scope, key), start, start-int64((2*AuthRateWindow)/time.Second))
	return err
}

func (l *AuthRateLimiter) windowStart() int64 {
	return l.now().Truncate(AuthRateWindow).Unix()
}

// hashRateLimitKey menghasilkan kunci tersimpan: sha256(scope + 0x00 + key).
// IP/email mentah tidak pernah masuk tabel maupun log.
func hashRateLimitKey(scope, key string) string {
	sum := sha256.Sum256([]byte(scope + "\x00" + strings.ToLower(strings.TrimSpace(key))))
	return hex.EncodeToString(sum[:])
}
