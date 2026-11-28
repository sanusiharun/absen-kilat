package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTPEmailSender mengirim token verifikasi lewat SMTP standar (stdlib, tanpa
// dependency berbayar). Konfigurasi berasal dari environment yang dimuat systemd
// dari file di luar repo (mis. /etc/absen/email.env).
type SMTPEmailSender struct {
	Host string
	Port int
	User string
	Pass string
	From string
	// DialOpsional untuk tes: mengganti koneksi TCP.
	Dial func(addr string) (net.Conn, error)
}

// NewSMTPEmailSenderFromEnv membuat sender dari fungsi getenv. Mengembalikan
// (nil, false) bila konfigurasi belum lengkap — pemanggil wajib fail closed.
func NewSMTPEmailSenderFromEnv(getenv func(string) string) (*SMTPEmailSender, bool) {
	host := strings.TrimSpace(getenv("SMTP_HOST"))
	from := strings.TrimSpace(getenv("SMTP_FROM"))
	if host == "" || from == "" {
		return nil, false
	}
	port := 587
	if raw := strings.TrimSpace(getenv("SMTP_PORT")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			port = n
		}
	}
	return &SMTPEmailSender{
		Host: host,
		Port: port,
		User: strings.TrimSpace(getenv("SMTP_USER")),
		Pass: getenv("SMTP_PASS"),
		From: from,
	}, true
}

// buildVerificationMessage menyusun pesan MIME. Token HANYA ada di body yang
// dikirim ke alamat tujuan — tidak di subject, tidak dikembalikan lewat API,
// tidak pernah ditulis ke log.
func buildVerificationMessage(from, to, token string) []byte {
	const subject = "Verifikasi email akun AbsenKilat"
	body := "Halo,\n\n" +
		"Klik/kirim token berikut untuk memverifikasi email Anda:\n\n" +
		token + "\n\n" +
		"Token berlaku 24 jam dan hanya bisa dipakai sekali.\n" +
		"Abaikan email ini bila Anda tidak mendaftar.\n"
	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n" +
		"\r\n" + body
	return []byte(msg)
}

// SendVerification mengirim token ke pemilik alamat tujuan.
func (s *SMTPEmailSender) SendVerification(_ context.Context, to, token string) error {
	if s == nil || s.Host == "" || s.From == "" {
		return errNoEmailTransport
	}
	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	msg := buildVerificationMessage(s.From, to, token)

	var auth smtp.Auth
	if s.User != "" {
		auth = smtp.PlainAuth("", s.User, s.Pass, s.Host)
	}

	if s.Dial != nil {
		// Jalur tes: kirim lewat koneksi yang disuntikkan.
		conn, err := s.Dial(addr)
		if err != nil {
			slog.Error("email: koneksi SMTP gagal", "host", s.Host, "error", err)
			return fmt.Errorf("email: dial: %w", err)
		}
		c, err := smtp.NewClient(conn, s.Host)
		if err != nil {
			return fmt.Errorf("email: client: %w", err)
		}
		defer func() { _ = c.Close() }()
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("email: starttls: %w", err)
			}
		}
		if auth != nil {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("email: auth: %w", err)
			}
		}
		if err := c.Mail(s.From); err != nil {
			return fmt.Errorf("email: mail from: %w", err)
		}
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("email: rcpt: %w", err)
		}
		wc, err := c.Data()
		if err != nil {
			return fmt.Errorf("email: data: %w", err)
		}
		if _, err := wc.Write(msg); err != nil {
			return fmt.Errorf("email: write: %w", err)
		}
		if err := wc.Close(); err != nil {
			return fmt.Errorf("email: close: %w", err)
		}
		return c.Quit()
	}

	if err := smtp.SendMail(addr, auth, s.From, []string{to}, msg); err != nil {
		// Pesan error SMTP tidak memuat body, jadi token tidak bocor lewat log.
		slog.Error("email: kirim verifikasi gagal", "host", s.Host, "error", err)
		return fmt.Errorf("email: send: %w", err)
	}
	slog.Info("email: token verifikasi terkirim", "host", s.Host)
	return nil
}
