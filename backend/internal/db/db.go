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

	// glebarez/go-sqlite is a pure-Go fork of modernc. Using it for BOTH the
	// raw database/sql pool and the GORM driver avoids a "driver sqlite
	// registered twice" panic (modernc + glebarez would both claim "sqlite").
	_ "github.com/glebarez/go-sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// dsn builds the SQLite DSN with pragmas applied at open time.
func dsn(path string) string {
	return fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
}

// Open opens (and creates if needed) the SQLite database at path, then applies
// any pending migrations. The connection pool is configured for a single
// writer with WAL mode enabled.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("db.Open: mkdir: %w", err)
	}

	pool, err := sql.Open("sqlite", dsn(path))
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

		// Run the migration on a dedicated connection so the
		// connection-scoped PRAGMA foreign_keys can be toggled around the
		// transaction. Schema-rebuild migrations (e.g. 007, which recreates
		// the users table) require foreign key enforcement OFF during the
		// rebuild — PRAGMA foreign_keys is a no-op inside a transaction, so
		// it must be set before BEGIN. Enforcement is re-enabled and the
		// schema verified (PRAGMA foreign_key_check) right after commit.
		conn, err := pool.Conn(ctx)
		if err != nil {
			return fmt.Errorf("db.migrate: conn %s: %w", name, err)
		}
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
			_ = conn.Close()
			return fmt.Errorf("db.migrate: foreign_keys off %s: %w", name, err)
		}

		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("db.migrate: begin %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			_ = conn.Close()
			return fmt.Errorf("db.migrate: apply %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			name, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			_ = tx.Rollback()
			_ = conn.Close()
			return fmt.Errorf("db.migrate: record %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			_ = conn.Close()
			return fmt.Errorf("db.migrate: commit %s: %w", name, err)
		}

		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=ON`); err != nil {
			_ = conn.Close()
			return fmt.Errorf("db.migrate: foreign_keys on %s: %w", name, err)
		}
		if err := checkForeignKeys(ctx, conn); err != nil {
			_ = conn.Close()
			return fmt.Errorf("db.migrate: foreign_key_check %s: %w", name, err)
		}
		if err := conn.Close(); err != nil {
			return fmt.Errorf("db.migrate: close %s: %w", name, err)
		}

		slog.Info("db: migration applied", "version", name)
	}
	return nil
}

// checkForeignKeys runs PRAGMA foreign_key_check and reports any violated
// foreign key constraints. It returns nil when the schema is consistent.
func checkForeignKeys(ctx context.Context, conn *sql.Conn) error {
	rows, err := conn.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var violations []string
	for rows.Next() {
		var table string
		var rowid int64
		var parent string
		var fkid int
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return err
		}
		violations = append(violations, fmt.Sprintf("%s(rowid=%d)", table, rowid))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf("foreign key violations: %s", strings.Join(violations, ", "))
	}
	return nil
}
