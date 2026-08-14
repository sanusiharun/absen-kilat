// Package db manages the SQLite database connection and migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens (and creates if needed) the SQLite database at path, then applies
// any pending migrations. The connection pool is configured for a single
// writer with WAL mode enabled.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("db.Open: mkdir: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	pool, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db.Open: sql.Open: %w", err)
	}
	pool.SetMaxOpenConns(1) // SQLite: single writer
	pool.SetMaxIdleConns(1)
	pool.SetConnMaxLifetime(0)

	if err := pool.PingContext(ctx); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("db.Open: ping: %w", err)
	}

	if err := migrate(ctx, pool); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("db.Open: migrate: %w", err)
	}

	return pool, nil
}

// migrate applies embedded migration files in lexicographic order, tracking
// applied versions in the schema_migrations table.
func migrate(ctx context.Context, pool *sql.DB) error {
	if _, err := pool.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("db.migrate: create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("db.migrate: readdir: %w", err)
	}

	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var exists int
		if err := pool.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, name,
		).Scan(&exists); err != nil {
			return fmt.Errorf("db.migrate: check %s: %w", name, err)
		}
		if exists > 0 {
			continue
		}

		body, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("db.migrate: read %s: %w", name, err)
		}

		tx, err := pool.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("db.migrate: begin %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("db.migrate: apply %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			name, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("db.migrate: record %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("db.migrate: commit %s: %w", name, err)
		}

		slog.Info("db: migration applied", "version", name)
	}
	return nil
}
