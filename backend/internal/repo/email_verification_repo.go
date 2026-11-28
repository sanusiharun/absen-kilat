package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// EmailVerificationRepository menyimpan token verifikasi email sekali pakai.
// Token mentah tidak pernah disimpan; hanya sha256-nya.
type EmailVerificationRepository struct {
	db *sql.DB
}

// NewEmailVerificationRepository creates an EmailVerificationRepository.
func NewEmailVerificationRepository(db *sql.DB) *EmailVerificationRepository {
	return &EmailVerificationRepository{db: db}
}

// Issue mengganti token lama user dengan token baru (sekali pakai).
func (r *EmailVerificationRepository) Issue(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("emailverif.Issue: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM email_verifications WHERE user_id = ? AND consumed_at IS NULL`, userID); err != nil {
		return fmt.Errorf("emailverif.Issue: delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO email_verifications (id, user_id, token_hash, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id, userID, tokenHash, fmtTime(time.Now().UTC()), fmtTime(expiresAt)); err != nil {
		return fmt.Errorf("emailverif.Issue: insert: %w", err)
	}
	return tx.Commit()
}

// ConsumeAndVerify menandai token terpakai DAN menandai email user terverifikasi
// dalam SATU transaksi, sehingga tidak mungkin token hangus tanpa status berubah
// (dan sebaliknya). ok=false berarti token tidak dikenal, sudah dipakai, atau
// kedaluwarsa. Pemanggilan paralel dengan token sama hanya bisa berhasil sekali.
func (r *EmailVerificationRepository) ConsumeAndVerify(ctx context.Context, tokenHash string) (string, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("emailverif.Consume: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		id, userID, expiresAt string
		consumedAt            sql.NullString
	)
	err = tx.QueryRowContext(ctx,
		`SELECT id, user_id, expires_at, consumed_at FROM email_verifications WHERE token_hash = ?`, tokenHash).
		Scan(&id, &userID, &expiresAt, &consumedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("emailverif.Consume: select: %w", err)
	}
	if consumedAt.Valid {
		return "", false, nil
	}
	exp, perr := time.Parse(time.RFC3339, expiresAt)
	if perr != nil || time.Now().UTC().After(exp) {
		return "", false, nil // fail closed
	}
	now := fmtTime(time.Now().UTC())
	// Guard konsumsi: hanya berhasil bila belum consumed. rowcount 0 berarti
	// request lain sudah lebih dulu memakai token ini (concurrent consume).
	res, err := tx.ExecContext(ctx,
		`UPDATE email_verifications SET consumed_at = ? WHERE id = ? AND consumed_at IS NULL`, now, id)
	if err != nil {
		return "", false, fmt.Errorf("emailverif.Consume: update: %w", err)
	}
	if n, rerr := res.RowsAffected(); rerr != nil || n != 1 {
		return "", false, nil
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET is_email_verified = 1 WHERE id = ?`, userID); err != nil {
		return "", false, fmt.Errorf("emailverif.Consume: verify user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("emailverif.Consume: commit: %w", err)
	}
	return userID, true, nil
}
