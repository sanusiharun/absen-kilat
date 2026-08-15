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
	NIP                 string
	Phone               string
	Department          string
	Position            string
	Address             string
	JoinDate            string
	ManagerID           sql.NullString
	EmployeeCode        string
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
	is_email_verified, notification_enabled, role, is_active,
	nip, phone, department, position, address, join_date, manager_id, employee_code,
	created_at, updated_at`

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
			is_email_verified, notification_enabled, role, is_active,
			nip, phone, department, position, address, join_date, manager_id, employee_code,
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.PasswordHash, u.Name, u.AvatarPath,
		boolToInt(u.IsEmailVerified), boolToInt(u.NotificationEnabled),
		string(u.Role), boolToInt(u.IsActive),
		u.NIP, u.Phone, u.Department, u.Position, u.Address, u.JoinDate,
		nullString(u.ManagerID), u.EmployeeCode,
		formatTime(u.CreatedAt), formatTime(u.UpdatedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return r.conflictFor(err, u)
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
			nip = ?, phone = ?, department = ?, position = ?, address = ?,
			join_date = ?, manager_id = ?, employee_code = ?,
			updated_at = ?
		WHERE id = ?`,
		u.PasswordHash, u.Name, u.AvatarPath,
		boolToInt(u.IsEmailVerified), boolToInt(u.NotificationEnabled),
		string(u.Role), boolToInt(u.IsActive),
		u.NIP, u.Phone, u.Department, u.Position, u.Address, u.JoinDate,
		nullString(u.ManagerID), u.EmployeeCode,
		formatTime(u.UpdatedAt), u.ID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return r.conflictFor(err, u)
		}
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

// conflictFor maps a unique-constraint violation to the right conflict error:
// an existing email wins over an existing employee_code.
func (r *UserRepository) conflictFor(err error, u *domain.User) error {
	if _, ferr := r.FindByEmail(context.Background(), u.Email); ferr == nil {
		return pkgerrors.Conflict("user", "email", u.Email)
	}
	if u.EmployeeCode != "" {
		return pkgerrors.Conflict("user", "employee_code", u.EmployeeCode)
	}
	return pkgerrors.Conflict("user", "email", u.Email)
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
		conds = append(conds, `(name LIKE ? COLLATE NOCASE OR email LIKE ? COLLATE NOCASE OR employee_code LIKE ? COLLATE NOCASE)`)
		like := "%" + search + "%"
		args = append(args, like, like, like)
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
			&d.NIP, &d.Phone, &d.Department, &d.Position, &d.Address, &d.JoinDate,
			&d.ManagerID, &d.EmployeeCode,
			&d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("repo.UserRepository.ListFiltered: scan: %w", err)
		}
		out = append(out, userFromDAO(d))
	}
	return out, rows.Err()
}

// scanUser scans a users row into a domain.User.
func scanUser(row *sql.Row) (*domain.User, error) {
	var d userDAO
	err := row.Scan(&d.ID, &d.Email, &d.PasswordHash, &d.Name, &d.AvatarPath,
		&d.IsEmailVerified, &d.NotificationEnabled, &d.Role, &d.IsActive,
		&d.NIP, &d.Phone, &d.Department, &d.Position, &d.Address, &d.JoinDate,
		&d.ManagerID, &d.EmployeeCode,
		&d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.NotFound("user", "")
		}
		return nil, fmt.Errorf("repo.scanUser: %w", err)
	}
	u := userFromDAO(d)
	return &u, nil
}

// userFromDAO maps a scanned DAO row to a domain.User.
func userFromDAO(d userDAO) domain.User {
	u := domain.User{
		ID:                  d.ID,
		Email:               d.Email,
		PasswordHash:        d.PasswordHash,
		Name:                d.Name,
		AvatarPath:          d.AvatarPath,
		IsEmailVerified:     d.IsEmailVerified == 1,
		NotificationEnabled: d.NotificationEnabled == 1,
		Role:                domain.UserRole(d.Role),
		IsActive:            d.IsActive == 1,
		NIP:                 d.NIP,
		Phone:               d.Phone,
		Department:          d.Department,
		Position:            d.Position,
		Address:             d.Address,
		JoinDate:            d.JoinDate,
		EmployeeCode:        d.EmployeeCode,
		CreatedAt:           parseTime(d.CreatedAt),
		UpdatedAt:           parseTime(d.UpdatedAt),
	}
	if d.ManagerID.Valid {
		u.ManagerID = d.ManagerID.String
	}
	return u
}
