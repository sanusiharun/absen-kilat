package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
)

// hashToken menghasilkan SHA-256 hex dari token mentah. Token mentah tidak pernah
// disimpan di database maupun log.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// randomToken membuat token acak 32 byte (base64url tanpa padding).
func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth.randomToken: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// newRefreshToken membuat baris token baru beserta nilai mentahnya. Keluarga token
// (family) ditentukan pemanggil; kosong berarti keluarga baru (login).
func (s *AuthService) newRefreshToken(userID, familyID, ip string) (*domain.RefreshToken, string, error) {
	raw, err := randomToken()
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC()
	if familyID == "" {
		familyID = uuid.NewString()
	}
	return &domain.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		FamilyID:  familyID,
		TokenHash: hashToken(raw),
		IssuedAt:  now,
		ExpiresAt: now.Add(s.refreshTTL),
		IP:        ip,
	}, raw, nil
}
