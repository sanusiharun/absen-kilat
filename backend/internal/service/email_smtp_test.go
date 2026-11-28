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
	"time"

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

// TLS wajib: server tanpa STARTTLS harus ditolak, token tidak dikirim.
func TestSMTP_RequiresStartTLS(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, aerr := ln.Accept()
		if aerr != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("220 fake ESMTP\r\n"))
		buf := make([]byte, 512)
		_, _ = conn.Read(buf) // EHLO
		// Sengaja TANPA baris STARTTLS.
		_, _ = conn.Write([]byte("250-fake\r\n250 OK\r\n"))
		time.Sleep(300 * time.Millisecond)
	}()

	sender := &SMTPEmailSender{
		Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port,
		From: "noreply@absenkilat.dev", Timeout: 3 * time.Second, RequireTLS: true,
	}
	err = sender.SendVerification(context.Background(), "user@tujuan.dev", "TOKEN-TLS")
	if err == nil {
		t.Fatal("server tanpa STARTTLS harus ditolak")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "starttls") {
		t.Fatalf("error harus menyebut STARTTLS, dapat: %v", err)
	}
	if strings.Contains(err.Error(), "TOKEN-TLS") {
		t.Fatal("token tidak boleh muncul di pesan error")
	}
}

// Timeout: server yang tidak pernah menyapa tidak boleh membuat pengiriman menggantung.
func TestSMTP_TimeoutEnforced(t *testing.T) {
	client, server := net.Pipe() // server tidak pernah menulis
	defer func() { _ = server.Close() }()

	sender := &SMTPEmailSender{
		Host: "pipe.local", Port: 587, From: "noreply@absenkilat.dev",
		Timeout: 200 * time.Millisecond, RequireTLS: true,
		Dial: func(string) (net.Conn, error) { return client, nil },
	}
	start := time.Now()
	err := sender.SendVerification(context.Background(), "user@tujuan.dev", "TOKEN-TIMEOUT")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("pengiriman ke server diam harus gagal")
	}
	if elapsed > 3*time.Second {
		t.Fatalf("timeout tidak ditegakkan, butuh %v", elapsed)
	}
	if strings.Contains(err.Error(), "TOKEN-TIMEOUT") {
		t.Fatal("token tidak boleh muncul di pesan error")
	}
}
