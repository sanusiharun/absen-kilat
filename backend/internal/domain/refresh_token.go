package domain

import (
	"context"
	"time"
)

// RotateOutcome is the result of attempting to consume a refresh token. It is
// deliberately explicit so callers cannot treat "replay" or "revoked" as success
// (change identity-hardening R3).
type RotateOutcome string

const (
	RotateOK       RotateOutcome = "ok"
	RotateNotFound RotateOutcome = "not_found"
	RotateExpired  RotateOutcome = "expired"
	RotateRevoked  RotateOutcome = "revoked"
	RotateReplay   RotateOutcome = "replay"
	RotateInactive RotateOutcome = "inactive"
)

// RefreshToken is one row of the rotation store. The raw token is never stored;
// only its SHA-256 hash is.
type RefreshToken struct {
	ID            string
	UserID        string
	FamilyID      string
	TokenHash     string
	IssuedAt      time.Time
	ExpiresAt     time.Time
	ConsumedAt    *time.Time
	ReplacedBy    string
	RevokedAt     *time.Time
	RevokedReason string
	IP            string
	UserAgent     string
}

// RefreshTokenRepository persists rotation state.
type RefreshTokenRepository interface {
	// Create stores a brand-new token (new family) — used at login.
	Create(ctx context.Context, t *RefreshToken) error
	// Rotate atomically consumes tokenHash and inserts next into the same family.
	// It reports replay/revocation/expiry explicitly and never succeeds silently.
	Rotate(ctx context.Context, tokenHash string, next *RefreshToken) (RotateOutcome, error)
	// ConsumeLegacy marks a stateless legacy (JWT) refresh token as consumed and
	// issues exactly one replacement, atomically. Reuse is reported as replay.
	ConsumeLegacy(ctx context.Context, tokenHash string, next *RefreshToken) (RotateOutcome, error)
	// RevokeFamily revokes every token of one login family.
	RevokeFamily(ctx context.Context, familyID, reason string) error
	// RevokeAllForUser revokes every family of a user (password/role change).
	RevokeAllForUser(ctx context.Context, userID, reason string) error
}
