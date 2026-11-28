package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// EmailSender mengirim token verifikasi email ke pemilik alamat. Implementasi
// produksi harus transport sungguhan; tanpa transport, verifikasi email TIDAK
// boleh jalan diam-diam (lihat AuthService.Register).
type EmailSender interface {
	SendVerification(ctx context.Context, to, token string) error
}

// LogEmailSender adalah mock KHUSUS non-production dan testing: token hanya
// dicatat di log server dan disimpan di memori untuk diperiksa tes. Token tidak
// pernah dikembalikan lewat API, sehingga kepemilikan email tetap harus dibuktikan
// lewat jalur verifikasi.
type LogEmailSender struct {
	mu   sync.Mutex
	sent map[string]string
}

// NewLogEmailSender membuat mock sender non-production.
func NewLogEmailSender() *LogEmailSender {
	return &LogEmailSender{sent: map[string]string{}}
}

// SendVerification mencatat token ke log server (bukan ke klien).
func (s *LogEmailSender) SendVerification(_ context.Context, to, token string) error {
	s.mu.Lock()
	s.sent[to] = token
	s.mu.Unlock()
	slog.Info("auth: verification token issued (mock non-production)", "email_domain_only", true)
	return nil
}

// TokenFor mengembalikan token terakhir untuk alamat tersebut. Hanya dipakai tes.
func (s *LogEmailSender) TokenFor(to string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tok, ok := s.sent[to]
	return tok, ok
}

// errNoEmailTransport dipakai saat verifikasi email diminta tanpa transport di
// lingkungan produksi: fail closed, bukan mengembalikan token ke klien.
var errNoEmailTransport = fmt.Errorf("email transport belum dikonfigurasi")
