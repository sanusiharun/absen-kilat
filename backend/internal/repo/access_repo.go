package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sanusi/absen-kilat/backend/internal/authz"
)

// AccessRepository persists role templates, per-user overrides and the audit of
// permission changes. It satisfies authz.Store.
type AccessRepository struct {
	db *sql.DB
}

// NewAccessRepository builds the repository.
func NewAccessRepository(db *sql.DB) *AccessRepository { return &AccessRepository{db: db} }

// RolePermissions returns the permission keys granted to a role template.
func (r *AccessRepository) RolePermissions(ctx context.Context, role string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT permission_key FROM role_permissions WHERE role = ? ORDER BY permission_key`, role)
	if err != nil {
		return nil, fmt.Errorf("access.RolePermissions: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0, 16)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("access.RolePermissions: scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UserOverrides returns explicit grants and denials for one user.
func (r *AccessRepository) UserOverrides(ctx context.Context, userID string) (allow, deny []string, err error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT permission_key, effect FROM user_permission_overrides WHERE user_id = ? ORDER BY permission_key`,
		userID)
	if err != nil {
		return nil, nil, fmt.Errorf("access.UserOverrides: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key, effect string
		if err := rows.Scan(&key, &effect); err != nil {
			return nil, nil, fmt.Errorf("access.UserOverrides: scan: %w", err)
		}
		if effect == "deny" {
			deny = append(deny, key)
		} else {
			allow = append(allow, key)
		}
	}
	return allow, deny, rows.Err()
}

// ReplaceRolePermissions sets the permission template of one role.
func (r *AccessRepository) ReplaceRolePermissions(ctx context.Context, role string, perms []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("access.ReplaceRolePermissions: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role = ?`, role); err != nil {
		return fmt.Errorf("access.ReplaceRolePermissions: delete: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, p := range perms {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO role_permissions (role, permission_key, created_at) VALUES (?, ?, ?)`,
			role, p, now); err != nil {
			return fmt.Errorf("access.ReplaceRolePermissions: insert: %w", err)
		}
	}
	return tx.Commit()
}

// SetUserOverride stores an allow/deny override and records the change.
func (r *AccessRepository) SetUserOverride(ctx context.Context, userID, permission, effect, actorID string) error {
	if effect != "allow" && effect != "deny" {
		return fmt.Errorf("access.SetUserOverride: invalid effect %q", effect)
	}
	now := time.Now().UTC().Format(time.RFC3339)

	var before string
	_ = r.db.QueryRowContext(ctx,
		`SELECT effect FROM user_permission_overrides WHERE user_id = ? AND permission_key = ?`,
		userID, permission).Scan(&before)

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO user_permission_overrides (user_id, permission_key, effect, created_by, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (user_id, permission_key)
		DO UPDATE SET effect = excluded.effect, created_by = excluded.created_by, created_at = excluded.created_at`,
		userID, permission, effect, actorID, now); err != nil {
		return fmt.Errorf("access.SetUserOverride: upsert: %w", err)
	}
	return r.recordAudit(ctx, actorID, userID, permission, before, effect)
}

// ClearUserOverride removes an override so the user falls back to the role template.
func (r *AccessRepository) ClearUserOverride(ctx context.Context, userID, permission, actorID string) error {
	var before string
	_ = r.db.QueryRowContext(ctx,
		`SELECT effect FROM user_permission_overrides WHERE user_id = ? AND permission_key = ?`,
		userID, permission).Scan(&before)
	if before == "" {
		return nil
	}
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM user_permission_overrides WHERE user_id = ? AND permission_key = ?`,
		userID, permission); err != nil {
		return fmt.Errorf("access.ClearUserOverride: %w", err)
	}
	return r.recordAudit(ctx, actorID, userID, permission, before, "")
}

func (r *AccessRepository) recordAudit(ctx context.Context, actorID, targetUserID, permission, before, after string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO permission_audit (id, actor_id, target_user_id, permission_key, before, after, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), actorID, targetUserID, permission, before, after,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("access.recordAudit: %w", err)
	}
	return nil
}

// DefaultRoleTemplates returns the seeded permission set per role. The sets are
// written to reproduce today's access: super_admin everything, hr everything
// except Pengaturan, supervisor only its own team's approval surfaces, employee
// nothing in the admin web.
func DefaultRoleTemplates() map[string][]string {
	all := authz.AllPermissions()

	hr := make([]string, 0, len(all))
	for _, p := range all {
		if len(p) >= len("menu.pengaturan.") && p[:len("menu.pengaturan.")] == "menu.pengaturan." {
			continue
		}
		hr = append(hr, p)
	}

	supervisor := []string{
		authz.Permission("dashboard", authz.ActionView),
		authz.Permission("approval", authz.ActionView),
		authz.Permission("approval", authz.ActionApprove),
		authz.Permission("cuti", authz.ActionView),
		authz.Permission("cuti", authz.ActionApprove),
		authz.Permission("lembur", authz.ActionView),
		authz.Permission("lembur", authz.ActionApprove),
		authz.Permission("akun-admin", authz.ActionView),
		authz.Permission("akun-admin", authz.ActionUpdate),
	}

	employee := []string{
		authz.Permission("akun-admin", authz.ActionView),
		authz.Permission("akun-admin", authz.ActionUpdate),
	}

	return map[string][]string{
		"super_admin": all,
		"hr":          hr,
		"supervisor":  supervisor,
		"employee":    employee,
	}
}

// SeedDefaults writes the default role templates exactly once.
func (r *AccessRepository) SeedDefaults(ctx context.Context) (bool, error) {
	var exists int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM access_seed`).Scan(&exists); err != nil {
		return false, fmt.Errorf("access.SeedDefaults: check: %w", err)
	}
	if exists > 0 {
		return false, nil
	}

	for role, perms := range DefaultRoleTemplates() {
		if err := r.ReplaceRolePermissions(ctx, role, perms); err != nil {
			return false, err
		}
	}
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO access_seed (id, seeded_at) VALUES (1, ?)`,
		time.Now().UTC().Format(time.RFC3339)); err != nil {
		return false, fmt.Errorf("access.SeedDefaults: mark: %w", err)
	}
	return true, nil
}
