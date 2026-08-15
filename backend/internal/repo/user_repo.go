// Package repo implements the repository interfaces from internal/domain
// on top of SQLite using a thin DAO layer.
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
	Role                string
	IsActive            int
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

const userCols = `id, email, password_hash, name, avatar_path,
	is_email_verified, notification_enabled, role, is_active, created_at, updated_at`

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	if u.Role == "" {
		u.Role = domain.RoleEmployee
	}
	u.IsActive = true // new accounts start active; admin disables via Update

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, name, avatar_path,
			is_email_verified, notification_enabled, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.PasswordHash, u.Name, u.AvatarPath,
		boolToInt(u.IsEmailVerified), boolToInt(u.NotificationEnabled),
		string(u.Role), boolToInt(u.IsActive),
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
	row := r.db.QueryRowContext(ctx,
		`SELECT `+userCols+` FROM users WHERE email = ? COLLATE NOCASE`, email)
	return scanUser(row)
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+userCols+` FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	u.UpdatedAt = time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, name = ?, avatar_path = ?,
			is_email_verified = ?, notification_enabled = ?, role = ?, is_active = ?,
			updated_at = ?
		WHERE id = ?`,
		u.PasswordHash, u.Name, u.AvatarPath,
		boolToInt(u.IsEmailVerified), boolToInt(u.NotificationEnabled),
		string(u.Role), boolToInt(u.IsActive),
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

// CountActive returns the number of active (non-disabled) accounts.
func (r *UserRepository) CountActive(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM users WHERE is_active = 1`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("repo.UserRepository.CountActive: %w", err)
	}
	return n, nil
}

// ListAll returns every user, ordered by creation date ascending.
func (r *UserRepository) ListAll(ctx context.Context) ([]domain.User, error) {
	return r.ListFiltered(ctx, "", "")
}

// ListFiltered returns users optionally filtered by a case-insensitive search
// over name/email and an is_active status ("active" or "inactive"; empty for
// all), ordered by creation date ascending.
func (r *UserRepository) ListFiltered(ctx context.Context, search, status string) ([]domain.User, error) {
	query := `SELECT ` + userCols + ` FROM users`
	var conds []string
	var args []any
	if search != "" {
		conds = append(conds, `(name LIKE ? COLLATE NOCASE OR email LIKE ? COLLATE NOCASE)`)
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	switch status {
	case "active":
		conds = append(conds, `is_active = 1`)
	case "inactive":
		conds = append(conds, `is_active = 0`)
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY created_at ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("repo.UserRepository.ListFiltered: %w", err)
	}
	defer rows.Close()

	var out []domain.User
	for rows.Next() {
		var d userDAO
		if err := rows.Scan(&d.ID, &d.Email, &d.PasswordHash, &d.Name, &d.AvatarPath,
			&d.IsEmailVerified, &d.NotificationEnabled, &d.Role, &d.IsActive,
			&d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("repo.UserRepository.ListFiltered: scan: %w", err)
		}
		out = append(out, domain.User{
			ID:                  d.ID,
			Email:               d.Email,
			PasswordHash:        d.PasswordHash,
			Name:                d.Name,
			AvatarPath:          d.AvatarPath,
			IsEmailVerified:     d.IsEmailVerified == 1,
			NotificationEnabled: d.NotificationEnabled == 1,
			Role:                domain.UserRole(d.Role),
			IsActive:            d.IsActive == 1,
			CreatedAt:           parseTime(d.CreatedAt),
			UpdatedAt:           parseTime(d.UpdatedAt),
		})
	}
	return out, rows.Err()
}

// scanUser scans a users row into a domain.User.
func scanUser(row *sql.Row) (*domain.User, error) {
	var d userDAO
	err := row.Scan(&d.ID, &d.Email, &d.PasswordHash, &d.Name, &d.AvatarPath,
		&d.IsEmailVerified, &d.NotificationEnabled, &d.Role, &d.IsActive,
		&d.CreatedAt, &d.UpdatedAt)
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
		Role:                domain.UserRole(d.Role),
		IsActive:            d.IsActive == 1,
		CreatedAt:           parseTime(d.CreatedAt),
		UpdatedAt:           parseTime(d.UpdatedAt),
	}, nil
}
