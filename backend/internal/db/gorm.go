// Package db manages the SQLite database connections.
package db

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenGORM opens a GORM handle on the same SQLite database file used by the
// database/sql repos. It opens a dedicated pool (glebarez v1.11.0 exposes only
// Open(dsn)); both pools use WAL + busy_timeout so SQLite's single-writer rule
// is respected under contention.
//
// It does NOT run migrations; call repo.AutoMigrate so schema changes are
// explicit and reversible.
func OpenGORM(path string) (*gorm.DB, error) {
	g, err := gorm.Open(sqlite.Open(dsn(path)), &gorm.Config{
		// Quiet: the app uses its own slog request logging.
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("db.OpenGORM: gorm.Open: %w", err)
	}
	// Single writer, same as the raw pool.
	sqlDB, err := g.DB()
	if err != nil {
		return nil, fmt.Errorf("db.OpenGORM: sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
	return g, nil
}
