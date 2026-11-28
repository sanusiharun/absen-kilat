package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// AuthRateLimitRepository menyimpan penghitung percobaan autentikasi di SQLite
// sehingga batas berlaku agregat lintas proses (identity-hardening R4, I22).
// Kunci berupa hash sha256; tidak ada IP/email mentah di tabel.
type AuthRateLimitRepository struct {
	db *sql.DB
}

// NewAuthRateLimitRepository creates an AuthRateLimitRepository.
func NewAuthRateLimitRepository(db *sql.DB) *AuthRateLimitRepository {
	return &AuthRateLimitRepository{db: db}
}

// Count mengembalikan jumlah percobaan tercatat pada jendela tersebut.
// Baris tidak ada berarti 0 (bukan error).
func (r *AuthRateLimitRepository) Count(ctx context.Context, scope, keyHash string, windowStart int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT hits FROM auth_rate_limits WHERE scope = ? AND key_hash = ? AND window_start = ?`,
		scope, keyHash, windowStart).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("authratelimit.Count: %w", err)
	}
	return n, nil
}

// Increment menambah penghitung satu langkah secara atomik dan membuang
// jendela yang sudah lewat (pruneBefore). Mengembalikan nilai setelah tambah.
func (r *AuthRateLimitRepository) Increment(ctx context.Context, scope, keyHash string, windowStart, pruneBefore int64) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("authratelimit.Increment: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO auth_rate_limits (scope, key_hash, window_start, hits)
		 VALUES (?, ?, ?, 1)
		 ON CONFLICT(scope, key_hash, window_start) DO UPDATE SET hits = hits + 1`,
		scope, keyHash, windowStart); err != nil {
		return 0, fmt.Errorf("authratelimit.Increment: upsert: %w", err)
	}

	var n int
	if err := tx.QueryRowContext(ctx,
		`SELECT hits FROM auth_rate_limits WHERE scope = ? AND key_hash = ? AND window_start = ?`,
		scope, keyHash, windowStart).Scan(&n); err != nil {
		return 0, fmt.Errorf("authratelimit.Increment: select: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM auth_rate_limits WHERE window_start < ?`, pruneBefore); err != nil {
		return 0, fmt.Errorf("authratelimit.Increment: prune: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("authratelimit.Increment: commit: %w", err)
	}
	return n, nil
}
