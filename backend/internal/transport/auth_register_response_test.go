package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
)

type stubAuth struct{}

func (stubAuth) Register(context.Context, string, string, string) (*domain.User, error) {
	return &domain.User{ID: "u1", Email: "baru@test.dev", Name: "Baru", NotificationEnabled: true}, nil
}

func (stubAuth) Login(context.Context, string, string) (*domain.User, *service.TokenPair, error) {
	return nil, nil, nil
}

func (stubAuth) Refresh(context.Context, string) (*service.TokenPair, error) { return nil, nil }

func (stubAuth) VerifyEmail(context.Context, string) error { return nil }

func (stubAuth) ForgotPassword(context.Context, string) (string, error) { return "", nil }

func (stubAuth) ResetPassword(context.Context, string, string) error { return nil }

// Regresi keamanan: respons registrasi TIDAK boleh memuat token verifikasi.
// Mengembalikan token ke pemanggil API = melewati pembuktian kepemilikan email.
func TestRegisterResponse_NoVerificationTokenLeak(t *testing.T) {
	r := chi.NewRouter()
	NewAuthHandler(stubAuth{}).RegisterRoutes(r)

	body := strings.NewReader(`{"email":"baru@test.dev","password":"rahasia-panjang-123","name":"Baru"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status harus 201, dapat %d (body: %s)", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("body bukan JSON: %v", err)
	}
	for k := range payload {
		lower := strings.ToLower(k)
		if strings.Contains(lower, "verification") || strings.Contains(lower, "token") {
			t.Fatalf("respons registrasi tidak boleh memuat field %q (bypass verifikasi email)", k)
		}
	}
	// Raw body juga diperiksa: jangan sampai ada token terselip di dalam field lain.
	if strings.Contains(strings.ToLower(rec.Body.String()), "verification_token") {
		t.Fatal("body mengandung verification_token")
	}
}
