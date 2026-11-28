package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
)

// Token hanya boleh muncul di body pesan ke alamat tujuan.
func TestSMTPMessage_TokenOnlyInBody(t *testing.T) {
	const token = "TOKEN-RAHASIA-123"
	msg := string(buildVerificationMessage("noreply@absenkilat.dev", "user@tujuan.dev", token))

	if !strings.Contains(msg, "To: user@tujuan.dev") {
		t.Fatal("pesan harus dialamatkan ke pemilik email")
	}
	headerEnd := strings.Index(msg, "\r\n\r\n")
	if headerEnd < 0 {
		t.Fatal("pesan bukan MIME valid")
	}
	header, body := msg[:headerEnd], msg[headerEnd:]
	if strings.Contains(header, token) {
		t.Fatal("token tidak boleh ada di header (termasuk Subject)")
	}
	if !strings.Contains(body, token) {
		t.Fatal("token harus ada di body")
	}
}

// Kegagalan kirim tidak boleh membocorkan token ke log.
func TestSMTP_LogsDoNotContainToken(t *testing.T) {
	const token = "TOKEN-LOG-BOCOR"
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	sender := &SMTPEmailSender{
		Host: "smtp.invalid", Port: 587, From: "noreply@absenkilat.dev",
		Dial: func(string) (net.Conn, error) { return nil, errors.New("connection refused") },
	}
	if err := sender.SendVerification(context.Background(), "user@tujuan.dev", token); err == nil {
		t.Fatal("kirim ke host tidak valid harus gagal")
	}
	if strings.Contains(buf.String(), token) {
		t.Fatalf("token bocor ke log: %s", buf.String())
	}
}

// Konfigurasi SMTP tidak lengkap -> dianggap tidak tersedia (fail closed).
func TestSMTP_FromEnvRequiresHostAndFrom(t *testing.T) {
	if _, ok := NewSMTPEmailSenderFromEnv(func(string) string { return "" }); ok {
		t.Fatal("tanpa SMTP_HOST/SMTP_FROM harus dianggap tidak tersedia")
	}
	sender, ok := NewSMTPEmailSenderFromEnv(func(k string) string {
		switch k {
		case "SMTP_HOST":
			return "smtp-relay.example.com"
		case "SMTP_FROM":
			return "noreply@absenkilat.dev"
		case "SMTP_PORT":
			return "587"
		}
		return ""
	})
	if !ok || sender == nil || sender.Port != 587 {
		t.Fatal("konfigurasi lengkap harus menghasilkan sender dengan port 587")
	}
}

// Registrasi tetap berjalan di production BILA transport tersedia, dan token
// hanya dikirim ke tujuan (direkam fake sender), bukan dikembalikan.
func TestRegister_ProductionWithTransportWorks(t *testing.T) {
	ctx := context.Background()
	pool, err := db.Open(ctx, filepath.Join(t.TempDir(), "smtpprod.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	users := repo.NewUserRepository(pool)
	svc := NewAuthService(users, "secret")
	svc.SetEmailVerifications(repo.NewEmailVerificationRepository(pool))
	sender := NewLogEmailSender() // pengganti transport pada tes
	svc.SetEmailSender(sender, true)

	user, rerr := svc.Register(ctx, "prodok@test.dev", "rahasia-panjang-123", "U")
	if rerr != nil {
		t.Fatalf("registrasi dengan transport harus berhasil: %v", rerr)
	}
	if user.IsEmailVerified {
		t.Fatal("akun baru belum boleh terverifikasi")
	}
	token, ok := sender.TokenFor("prodok@test.dev")
	if !ok || token == "" {
		t.Fatal("token harus terkirim ke alamat tujuan")
	}
	if err := svc.VerifyEmail(ctx, token); err != nil {
		t.Fatalf("token dari transport harus bisa memverifikasi: %v", err)
	}
}
