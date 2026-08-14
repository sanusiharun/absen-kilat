// Package repo implements the repository interfaces from internal/domain
// on top of SQLite using a thin DAO layer.
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// userDAO maps a users table row to/from a domain.User.
type userDAO struct {
	ID                  string
	Email               string
	PasswordHash        string
	Name                string
	AvatarPath          string
	IsEmailVerified     int
	NotificationEnabled int
	CreatedAt           string
	UpdatedAt           string
}

// UserRepository is the SQLite implementation of domain.UserRepository.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a UserRepository backed by the given pool.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, name, avatar_path,
			is_email_verified, notification_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.PasswordHash, u.Name, u.AvatarPath,
		boolToInt(u.IsEmailVerified), boolToInt(u.NotificationEnabled),
		formatTime(u.CreatedAt), formatTime(u.UpdatedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return pkgerrors.Conflict("user", "email", u.Email)
		}
		return fmt.Errorf("repo.UserRepository.Create: %w", err)
	}
	return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, name, avatar_path,
		       is_email_verified, notification_enabled, created_at, updated_at
		FROM users WHERE email = ? COLLATE NOCASE`, email)
	return scanUser(row)
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, name, avatar_path,
		       is_email_verified, notification_enabled, created_at, updated_at
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	u.UpdatedAt = time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, name = ?, avatar_path = ?,
			is_email_verified = ?, notification_enabled = ?, updated_at = ?
		WHERE id = ?`,
		u.PasswordHash, u.Name, u.AvatarPath,
		boolToInt(u.IsEmailVerified), boolToInt(u.NotificationEnabled),
		formatTime(u.UpdatedAt), u.ID,
	)
	if err != nil {
		return fmt.Errorf("repo.UserRepository.Update: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repo.UserRepository.Update: rows: %w", err)
	}
	if n == 0 {
		return pkgerrors.NotFound("user", u.ID)
	}
	return nil
}

// scanUser scans a users row into a domain.User.
func scanUser(row *sql.Row) (*domain.User, error) {
	var d userDAO
	err := row.Scan(&d.ID, &d.Email, &d.PasswordHash, &d.Name, &d.AvatarPath,
		&d.IsEmailVerified, &d.NotificationEnabled, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.NotFound("user", "")
		}
		return nil, fmt.Errorf("repo.scanUser: %w", err)
	}
	return &domain.User{
		ID:                  d.ID,
		Email:               d.Email,
		PasswordHash:        d.PasswordHash,
		Name:                d.Name,
		AvatarPath:          d.AvatarPath,
		IsEmailVerified:     d.IsEmailVerified == 1,
		NotificationEnabled: d.NotificationEnabled == 1,
		CreatedAt:           parseTime(d.CreatedAt),
		UpdatedAt:           parseTime(d.UpdatedAt),
	}, nil
}
