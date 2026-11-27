package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
)

// RefreshTokenRepository is the SQLite implementation of the rotation store.
type RefreshTokenRepository struct {
	db *sql.DB
}

// NewRefreshTokenRepository creates a RefreshTokenRepository.
func NewRefreshTokenRepository(db *sql.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func fmtTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// Create stores a brand-new token (start of a family).
func (r *RefreshTokenRepository) Create(ctx context.Context, t *domain.RefreshToken) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, issued_at, expires_at, ip, user_agent)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.UserID, t.FamilyID, t.TokenHash, fmtTime(t.IssuedAt), fmtTime(t.ExpiresAt), t.IP, t.UserAgent)
	if err != nil {
		return fmt.Errorf("typeof refresh.Create: %w", err)
	}
	return nil
}

// Rotate consumes tokenHash and inserts next in the same family, all in one
// transaction. BEGIN IMMEDIATE ensures only one concurrent request can win.
func (r *RefreshTokenRepository) Rotate(ctx context.Context, tokenHash string, next *domain.RefreshToken) (domain.RotateOutcome, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.RotateNotFound, fmt.Errorf("refresh.Rotate: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		id, userID, familyID, expiresAt string
		consumedAt, revokedAt           sql.NullString
		revokedReason                   sql.NullString
	)
	err = tx.QueryRowContext(ctx,
		`SELECT id, user_id, family_id, expires_at, consumed_at, revoked_at, revoked_reason
		   FROM refresh_tokens WHERE token_hash = ?`, tokenHash).
		Scan(&id, &userID, &familyID, &expiresAt, &consumedAt, &revokedAt, &revokedReason)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RotateNotFound, nil
	}
	if err != nil {
		return domain.RotateNotFound, fmt.Errorf("refresh.Rotate: select: %w", err)
	}

	now := time.Now().UTC()

	// Replay: token sudah dipakai. Cabut seluruh keluarga lalu tolak.
	if consumedAt.Valid {
		if err := revokeFamilyTx(ctx, tx, familyID, "replay", now); err != nil {
			return domain.RotateNotFound, err
		}
		if err := tx.Commit(); err != nil {
			return domain.RotateNotFound, fmt.Errorf("refresh.Rotate: commit replay: %w", err)
		}
		return domain.RotateReplay, nil
	}
	if revokedAt.Valid {
		if err := tx.Commit(); err != nil {
			return domain.RotateNotFound, fmt.Errorf("refresh.Rotate: commit revoked: %w", err)
		}
		return domain.RotateRevoked, nil
	}
	exp, perr := time.Parse(time.RFC3339, expiresAt)
	if perr != nil {
		// Fail closed: expiry tidak terbaca berarti token tidak boleh dipakai.
		if err := tx.Commit(); err != nil {
			return domain.RotateNotFound, fmt.Errorf("refresh.Rotate: commit malformed: %w", err)
		}
		return domain.RotateExpired, nil
	}
	if now.After(exp) {
		if err := tx.Commit(); err != nil {
			return domain.RotateNotFound, fmt.Errorf("refresh.Rotate: commit expired: %w", err)
		}
		return domain.RotateExpired, nil
	}

	var active int
	if err := tx.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id = ?`, userID).Scan(&active); err != nil {
		return domain.RotateNotFound, fmt.Errorf("refresh.Rotate: user: %w", err)
	}
	if active == 0 {
		if err := revokeFamilyTx(ctx, tx, familyID, "account_inactive", now); err != nil {
			return domain.RotateNotFound, err
		}
		if err := tx.Commit(); err != nil {
			return domain.RotateNotFound, fmt.Errorf("refresh.Rotate: commit inactive: %w", err)
		}
		return domain.RotateInactive, nil
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE refresh_tokens SET consumed_at = ?, replaced_by = ? WHERE id = ?`,
		fmtTime(now), next.ID, id); err != nil {
		return domain.RotateNotFound, fmt.Errorf("refresh.Rotate: consume: %w", err)
	}

	next.FamilyID = familyID
	next.UserID = userID
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, issued_at, expires_at, ip, user_agent)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		next.ID, userID, familyID, next.TokenHash, fmtTime(next.IssuedAt), fmtTime(next.ExpiresAt),
		next.IP, next.UserAgent); err != nil {
		return domain.RotateNotFound, fmt.Errorf("refresh.Rotate: insert next: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.RotateNotFound, fmt.Errorf("refresh.Rotate: commit: %w", err)
	}
	return domain.RotateOK, nil
}

// RevokeFamily revokes one login family.
func (r *RefreshTokenRepository) RevokeFamily(ctx context.Context, familyID, reason string) error {
	return revokeFamilyTx(ctx, r.db, familyID, reason, time.Now().UTC())
}

// RevokeAllForUser revokes every family owned by the user.
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID, reason string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = ?, revoked_reason = ?
		  WHERE user_id = ? AND revoked_at IS NULL`,
		fmtTime(time.Now().UTC()), reason, userID)
	if err != nil {
		return fmt.Errorf("refresh.RevokeAllForUser: %w", err)
	}
	return nil
}

// execer is satisfied by both *sql.DB and *sql.Tx.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func revokeFamilyTx(ctx context.Context, e execer, familyID, reason string, now time.Time) error {
	_, err := e.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = ?, revoked_reason = ?
		  WHERE family_id = ? AND revoked_at IS NULL`,
		fmtTime(now), reason, familyID)
	if err != nil {
		return fmt.Errorf("refresh.revokeFamily: %w", err)
	}
	return nil
}

// ConsumeLegacy menandai token legacy (JWT stateless) sebagai terpakai secara
// atomik dan menerbitkan pengganti pada keluarga baru. Pemakaian ulang token
// legacy yang sama terdeteksi sebagai replay dan mencabut keluarganya.
func (r *RefreshTokenRepository) ConsumeLegacy(ctx context.Context, tokenHash string, next *domain.RefreshToken) (domain.RotateOutcome, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.RotateNotFound, fmt.Errorf("refresh.ConsumeLegacy: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existingID, familyID string
	err = tx.QueryRowContext(ctx, `SELECT id, family_id FROM refresh_tokens WHERE token_hash = ?`, tokenHash).
		Scan(&existingID, &familyID)
	switch {
	case err == nil:
		// Sudah pernah di-upgrade: replay.
		if err := revokeFamilyTx(ctx, tx, familyID, "replay", time.Now().UTC()); err != nil {
			return domain.RotateNotFound, err
		}
		if err := tx.Commit(); err != nil {
			return domain.RotateNotFound, fmt.Errorf("refresh.ConsumeLegacy: commit replay: %w", err)
		}
		return domain.RotateReplay, nil
	case !errors.Is(err, sql.ErrNoRows):
		return domain.RotateNotFound, fmt.Errorf("refresh.ConsumeLegacy: select: %w", err)
	}

	now := time.Now().UTC()
	// Baris penanda untuk token legacy: langsung consumed, menunjuk penggantinya.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, issued_at, expires_at, consumed_at, replaced_by, ip, user_agent)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', 'legacy')`,
		"legacy-"+next.ID, next.UserID, next.FamilyID, tokenHash, fmtTime(now), fmtTime(next.ExpiresAt), fmtTime(now), next.ID); err != nil {
		return domain.RotateNotFound, fmt.Errorf("refresh.ConsumeLegacy: insert marker: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, issued_at, expires_at, ip, user_agent)
		 VALUES (?, ?, ?, ?, ?, ?, '', 'legacy_upgrade')`,
		next.ID, next.UserID, next.FamilyID, next.TokenHash, fmtTime(next.IssuedAt), fmtTime(next.ExpiresAt)); err != nil {
		return domain.RotateNotFound, fmt.Errorf("refresh.ConsumeLegacy: insert next: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.RotateNotFound, fmt.Errorf("refresh.ConsumeLegacy: commit: %w", err)
	}
	return domain.RotateOK, nil
}
