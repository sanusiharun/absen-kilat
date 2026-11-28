package transport

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// memoryLimitStore adalah penghitung in-memory untuk menguji handler.
type memoryLimitStore struct {
	mu     sync.Mutex
	counts map[string]int
}

func newMemoryLimitStore() *memoryLimitStore {
	return &memoryLimitStore{counts: map[string]int{}}
}

func (s *memoryLimitStore) key(scope, keyHash string) string { return scope + "|" + keyHash }

func (s *memoryLimitStore) Count(_ context.Context, scope, keyHash string, _ int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counts[s.key(scope, keyHash)], nil
}

func (s *memoryLimitStore) Increment(_ context.Context, scope, keyHash string, _, _ int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[s.key(scope, keyHash)]++
	return s.counts[s.key(scope, keyHash)], nil
}

// stubAuthService: login selalu gagal dengan error generik yang sama, seperti
// akun tidak ada maupun password salah.
type stubAuthService struct{}

func (stubAuthService) Register(context.Context, string, string, string) (*domain.User, error) {
	return nil, errors.New("not used")
}

func (stubAuthService) Login(context.Context, string, string) (*domain.User, *service.TokenPair, error) {
	return nil, nil, pkgerrors.Unauthorized("invalid email or password")
}

func (stubAuthService) Refresh(context.Context, string) (*service.TokenPair, error) {
	return nil, errors.New("not used")
}

func (stubAuthService) VerifyEmail(context.Context, string) error { return errors.New("not used") }

func (stubAuthService) ForgotPassword(context.Context, string) (string, error) { return "", nil }

func (stubAuthService) ResetPassword(context.Context, string, string) error {
	return errors.New("not used")
}

func postLogin(t *testing.T, h *AuthHandler, ip, forwardedFor, email string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"email":"` + email + `","password":"salah-sekali"}`
	r := httptest.NewRequest("POST", "/auth/login", strings.NewReader(body))
	r.RemoteAddr = ip + ":4321"
	if forwardedFor != "" {
		r.Header.Set("X-Forwarded-For", forwardedFor)
	}
	w := httptest.NewRecorder()
	h.login(w, r)
	return w
}

// I10/I17 pada level handler: 10 kegagalan login → 401, percobaan ke-11 → 429
// generik; memalsukan XFF dari peer tak-terpercaya tidak mereset penghitung.
func TestAuthHandler_LoginRateLimited(t *testing.T) {
	resolver, err := NewClientIPResolver("")
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	h := NewAuthHandlerWithRateLimits(stubAuthService{}, service.NewAuthRateLimiter(newMemoryLimitStore()), resolver)

	for i := 1; i <= 10; i++ {
		w := postLogin(t, h, "198.51.100.20", "", "user@example.com")
		if w.Code != 401 {
			t.Fatalf("attempt %d: got %d, want 401", i, w.Code)
		}
	}

	w := postLogin(t, h, "198.51.100.20", "203.0.113.250", "user@example.com")
	if w.Code != 429 {
		t.Fatalf("attempt 11: got %d, want 429", w.Code)
	}
	if got := w.Body.String(); !strings.Contains(got, "TOO_MANY_REQUESTS") {
		t.Fatalf("unexpected body: %s", got)
	}
}

// I18: respons untuk email terdaftar vs tidak terdaftar identik.
func TestAuthHandler_LoginResponsesIdenticalForUnknownAccount(t *testing.T) {
	resolver, _ := NewClientIPResolver("")
	h := NewAuthHandlerWithRateLimits(stubAuthService{}, service.NewAuthRateLimiter(newMemoryLimitStore()), resolver)

	known := postLogin(t, h, "198.51.100.30", "", "ada@example.com")
	unknown := postLogin(t, h, "198.51.100.31", "", "tidak-ada@example.com")

	if known.Code != unknown.Code {
		t.Fatalf("status differs: %d vs %d", known.Code, unknown.Code)
	}
	if known.Body.String() != unknown.Body.String() {
		t.Fatalf("body differs: %q vs %q", known.Body.String(), unknown.Body.String())
	}
	if known.Body.Len() != unknown.Body.Len() {
		t.Fatalf("body length differs: %d vs %d", known.Body.Len(), unknown.Body.Len())
	}
}
