package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
)

// SettingsRepository is the SQLite implementation of domain.SettingsRepository.
type SettingsRepository struct {
	db *sql.DB
}

// NewSettingsRepository creates a SettingsRepository backed by the pool.
func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// All returns every settings key/value pair. Missing keys are simply absent.
func (r *SettingsRepository) All(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("repo.SettingsRepository.All: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("repo.SettingsRepository.All: scan: %w", err)
		}
		out[k] = v
	}
	return out, rows.Err()
}

// Set upserts a single settings key.
func (r *SettingsRepository) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value)
	if err != nil {
		return fmt.Errorf("repo.SettingsRepository.Set(%s): %w", key, err)
	}
	return nil
}

var _ domain.SettingsRepository = (*SettingsRepository)(nil)
