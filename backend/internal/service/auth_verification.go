package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// EmailVerificationStore menyimpan token verifikasi email sekali pakai.
type EmailVerificationStore interface {
	Issue(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) error
	Consume(ctx context.Context, tokenHash string) (string, bool, error)
}

// verificationTTL membatasi umur token verifikasi email.
const verificationTTL = 24 * time.Hour

// SetEmailVerifications memasang store token verifikasi email. Tanpa store,
// verifikasi email DITOLAK (fail closed) — bukan memakai user id seperti versi lama.
func (s *AuthService) SetEmailVerifications(store EmailVerificationStore) {
	s.verifications = store
}

// IssueVerificationToken membuat token acak sekali pakai untuk user tersebut.
func (s *AuthService) IssueVerificationToken(ctx context.Context, userID string) (string, error) {
	if s.verifications == nil {
		return "", pkgerrors.Internal(errors.New("verifikasi email belum dikonfigurasi"))
	}
	raw, err := randomToken()
	if err != nil {
		return "", err
	}
	if err := s.verifications.Issue(ctx, uuid.NewString(), userID, hashToken(raw), time.Now().UTC().Add(verificationTTL)); err != nil {
		return "", fmt.Errorf("service.AuthService.IssueVerificationToken: %w", err)
	}
	return raw, nil
}
