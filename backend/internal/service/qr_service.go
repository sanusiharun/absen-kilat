package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// qrPageBaseURL is the public web form the QR code points to; the token is
// appended to it (e.g. "http://43.157.243.110:3000/qr?t=<token>").
const qrPageBaseURL = "http://43.157.243.110:3000/qr?t="

// QrRateLimitMax is the maximum number of public QR submissions per IP per
// minute (sliding window).
const QrRateLimitMax = 10

// QrService manages QR attendance tokens and the unauthenticated QR
// attendance flow. It composes the concrete repositories (app-level
// concern), mirroring AdminService.
type QrService struct {
	tokens     *repo.QrTokenRepository
	locations  *repo.LocationRepository
	users      *repo.UserRepository
	attendances *repo.AttendanceRepository
	limiter    *ipRateLimiter
}

// NewQrService creates a QrService with an in-memory per-IP rate limiter
// (max QrRateLimitMax requests per minute).
func NewQrService(tokens *repo.QrTokenRepository, locations *repo.LocationRepository, users *repo.UserRepository, attendances *repo.AttendanceRepository) *QrService {
	return &QrService{
		tokens:      tokens,
		locations:   locations,
		users:       users,
		attendances: attendances,
		limiter:     newIPRateLimiter(QrRateLimitMax, time.Minute),
	}
}

// RateLimited reports whether the IP has exceeded the per-minute limit. It
// records the hit as a side effect, so it must be called once per request.
func (s *QrService) RateLimited(ip string) bool {
	return !s.limiter.Allow(ip)
}

// Generate creates a new permanent (revocable) QR token for a location and
// returns the token plus the public page URL.
func (s *QrService) Generate(ctx context.Context, locationID, createdBy string) (*domain.QrToken, string, error) {
	if strings.TrimSpace(locationID) == "" {
		return nil, "", pkgerrors.Validation("location_id", "wajib diisi")
	}
	loc, err := s.locations.FindByID(ctx, locationID)
	if err != nil {
		return nil, "", err
	}
	if !loc.Active {
		return nil, "", pkgerrors.Validation("location_id", "lokasi tidak aktif")
	}

	token, err := randomHex(32)
	if err != nil {
		return nil, "", fmt.Errorf("service.QrService.Generate: token: %w", err)
	}
	t := &domain.QrToken{
		LocationID: locationID,
		Token:      token,
		Active:     true,
		CreatedBy:  createdBy,
	}
	if err := s.tokens.Create(ctx, t); err != nil {
		return nil, "", err
	}
	return t, qrPageBaseURL + token, nil
}

// ListTokens returns every token joined with its location name, newest first.
func (s *QrService) ListTokens(ctx context.Context) ([]domain.QrTokenView, error) {
	return s.tokens.ListAll(ctx)
}

// Revoke disables a token (active=0); scanning it afterwards is rejected.
func (s *QrService) Revoke(ctx context.Context, id string) error {
	return s.tokens.SetActive(ctx, id, false)
}

// ValidForAttendance resolves an active, unexpired token to its location.
// Revoked, expired, or unknown tokens yield ErrNotFound.
func (s *QrService) ValidForAttendance(ctx context.Context, token string) (*domain.Location, error) {
	t, err := s.tokens.FindByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if !t.Active {
		return nil, pkgerrors.NotFound("qr_token", "")
	}
	if t.ExpiresAt != nil && time.Now().UTC().After(*t.ExpiresAt) {
		return nil, pkgerrors.NotFound("qr_token", "")
	}
	loc, err := s.locations.FindByID(ctx, t.LocationID)
	if err != nil {
		return nil, err
	}
	if !loc.Active {
		return nil, pkgerrors.NotFound("location", "")
	}
	return loc, nil
}

// QrAttendanceInput carries the public QR attendance form fields.
type QrAttendanceInput struct {
	Token        string
	Email        string // either Email or EmployeeCode must be set
	EmployeeCode string
	Name         string // optional, informational only
	PhotoPath    string // optional selfie
	IPAddress    string
	Now          time.Time
}

// RecordQRAttendance records a check-in from the public QR form: validates
// the token, resolves the user by email or employee_code, rejects duplicates
// (one check-in per user per day), and stores the record as
// source='qr', approval_status='pending', pinned to the token's location.
func (s *QrService) RecordQRAttendance(ctx context.Context, in QrAttendanceInput) (*domain.Attendance, error) {
	if strings.TrimSpace(in.Token) == "" {
		return nil, pkgerrors.Validation("token", "wajib diisi")
	}
	loc, err := s.ValidForAttendance(ctx, strings.TrimSpace(in.Token))
	if err != nil {
		return nil, err
	}

	u, err := s.findUser(ctx, in)
	if err != nil {
		return nil, err
	}

	// Dedup: one check-in per user per local day (same rule as the app).
	now := resolveQrNow(in)
	list, err := s.attendances.ListByUserDate(ctx, u.ID, now, time.Local)
	if err != nil {
		return nil, err
	}
	for _, a := range list {
		if a.Type == domain.AttendanceCheckIn {
			return nil, pkgerrors.ConflictMessage("attendance", "type", "check_in", "Anda sudah absen hari ini")
		}
	}

	att := &domain.Attendance{
		UserID:         u.ID,
		Type:           domain.AttendanceCheckIn,
		Status:         domain.StatusOnTime,
		PhotoPath:      in.PhotoPath,
		Latitude:       &loc.Lat,
		Longitude:      &loc.Lng,
		Address:        "",
		Device:         "qr",
		IPAddress:      in.IPAddress,
		Source:         domain.SourceQR,
		ApprovalStatus: domain.ApprovalPending,
		LocationID:     loc.ID,
		OccurredAt:     now.UTC(),
		CreatedAt:      now.UTC(),
	}
	if err := s.attendances.Create(ctx, att); err != nil {
		return nil, err
	}
	return att, nil
}

// findUser resolves the identity by email (case-insensitive) or employee
// code. Unknown or inactive identities yield a single generic NotFound so
// the endpoint never reveals which identifier was wrong.
func (s *QrService) findUser(ctx context.Context, in QrAttendanceInput) (*domain.User, error) {
	var (
		u   *domain.User
		err error
	)
	switch {
	case strings.TrimSpace(in.Email) != "":
		u, err = s.users.FindByEmail(ctx, strings.TrimSpace(in.Email))
	case strings.TrimSpace(in.EmployeeCode) != "":
		u, err = s.users.FindByEmployeeCode(ctx, strings.TrimSpace(in.EmployeeCode))
	default:
		return nil, pkgerrors.Validation("email", "email atau kode karyawan wajib diisi")
	}
	if err != nil {
		if pkgerrors.IsNotFound(err) {
			return nil, pkgerrors.NotFoundMessage("user", "", "kode/email tidak dikenal, hubungi HR")
		}
		return nil, err
	}
	if !u.IsActive {
		return nil, pkgerrors.NotFoundMessage("user", "", "kode/email tidak dikenal, hubungi HR")
	}
	return u, nil
}

// resolveQrNow returns the provided time or time.Now.
func resolveQrNow(in QrAttendanceInput) time.Time {
	if in.Now.IsZero() {
		return time.Now()
	}
	return in.Now
}

// randomHex returns n cryptographically random bytes hex-encoded.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ipRateLimiter is an in-memory sliding-window rate limiter keyed by IP.
// Old hits (older than window) are pruned on each call, so the map never
// grows unboundedly for a given key and idle keys are dropped.
type ipRateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	window  time.Duration
	maxHits int
}

func newIPRateLimiter(maxHits int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		hits:    make(map[string][]time.Time),
		window:  window,
		maxHits: maxHits,
	}
}

// Allow records a hit for ip and reports whether it stays under the limit.
func (l *ipRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)
	hits := l.hits[ip]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.maxHits {
		if len(kept) == 0 {
			delete(l.hits, ip)
		} else {
			l.hits[ip] = kept
		}
		return false
	}
	l.hits[ip] = append(kept, now)
	return true
}
