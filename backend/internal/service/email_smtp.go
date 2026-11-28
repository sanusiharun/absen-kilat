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
	// Timeout membatasi seluruh percakapan SMTP (dial, TLS, auth, kirim).
	Timeout time.Duration
	// RequireTLS menolak server yang tidak mendukung STARTTLS. Default true:
	// token verifikasi tidak boleh lewat koneksi tanpa TLS.
	RequireTLS bool
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
	timeout := 10 * time.Second
	if raw := strings.TrimSpace(getenv("SMTP_TIMEOUT_SECONDS")); raw != "" {
		if secs, err := strconv.Atoi(raw); err == nil && secs > 0 && secs <= 60 {
			timeout = time.Duration(secs) * time.Second
		}
	}
	return &SMTPEmailSender{
		Host:       host,
		Port:       port,
		User:       strings.TrimSpace(getenv("SMTP_USER")),
		Pass:       getenv("SMTP_PASS"),
		From:       from,
		Timeout:    timeout,
		RequireTLS: true,
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

// SendVerification mengirim token ke pemilik alamat tujuan lewat SMTP dengan
// TLS wajib dan batas waktu tegas (tidak boleh menggantung).
func (s *SMTPEmailSender) SendVerification(ctx context.Context, to, token string) error {
	if s == nil || s.Host == "" || s.From == "" {
		return errNoEmailTransport
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	dial := s.Dial
	if dial == nil {
		dialer := &net.Dialer{Timeout: timeout}
		dial = func(a string) (net.Conn, error) { return dialer.DialContext(ctx, "tcp", a) }
	}
	conn, err := dial(addr)
	if err != nil {
		slog.Error("email: koneksi SMTP gagal", "host", s.Host, "error", err)
		return fmt.Errorf("email: dial: %w", err)
	}
	// Deadline di level koneksi: mencegah hang pada TLS/auth/kirim.
	if derr := conn.SetDeadline(deadline); derr != nil {
		_ = conn.Close()
		return fmt.Errorf("email: set deadline: %w", derr)
	}

	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("email: client: %w", err)
	}
	defer func() { _ = c.Close() }()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if terr := c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); terr != nil {
			return fmt.Errorf("email: starttls: %w", terr)
		}
	} else if s.RequireTLS {
		// Token verifikasi tidak boleh terkirim tanpa TLS.
		return fmt.Errorf("email: server tidak mendukung STARTTLS, pengiriman dibatalkan")
	}

	if s.User != "" {
		if aerr := c.Auth(smtp.PlainAuth("", s.User, s.Pass, s.Host)); aerr != nil {
			return fmt.Errorf("email: auth: %w", aerr)
		}
	}
	if merr := c.Mail(s.From); merr != nil {
		return fmt.Errorf("email: mail from: %w", merr)
	}
	if rerr := c.Rcpt(to); rerr != nil {
		return fmt.Errorf("email: rcpt: %w", rerr)
	}
	wc, werr := c.Data()
	if werr != nil {
		return fmt.Errorf("email: data: %w", werr)
	}
	if _, xerr := wc.Write(buildVerificationMessage(s.From, to, token)); xerr != nil {
		return fmt.Errorf("email: write: %w", xerr)
	}
	if cerr := wc.Close(); cerr != nil {
		return fmt.Errorf("email: close: %w", cerr)
	}
	if qerr := c.Quit(); qerr != nil {
		return fmt.Errorf("email: quit: %w", qerr)
	}
	slog.Info("email: token verifikasi terkirim", "host", s.Host, "tls", true)
	return nil
}
