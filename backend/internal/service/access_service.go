package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/sanusi/absen-kilat/backend/internal/authz"
	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// accessStore is the persistence needed to edit menu access.
type accessStore interface {
	RolePermissions(ctx context.Context, role string) ([]string, error)
	UserOverrides(ctx context.Context, userID string) (allow, deny []string, err error)
	SetUserOverride(ctx context.Context, userID, permission, effect, actorID string) error
	ClearUserOverride(ctx context.Context, userID, permission, actorID string) error
}

// userLister is the subset of the user repository used here (ListAll powers the
// "don't lock out the last admin" guard).
type userLister interface {
	ListAll(ctx context.Context) ([]domain.User, error)
	FindByID(ctx context.Context, id string) (*domain.User, error)
}

// PermissionItem explains one permission for one user: whether the role grants
// it, whether an override changes it, and the resulting effect.
type PermissionItem struct {
	Permission  string `json:"permission"`
	MenuKey     string `json:"menu_key"`
	MenuLabel   string `json:"menu_label"`
	Action      string `json:"action"`
	RoleGranted bool   `json:"role_granted"`
	Override    string `json:"override"`  // "", "allow", "deny"
	Effective   bool   `json:"effective"` // what the middleware will do
}

// UserAccess is the editable view of one user's menu access.
type UserAccess struct {
	UserID      string           `json:"user_id"`
	Name        string           `json:"name"`
	Email       string           `json:"email"`
	Role        string           `json:"role"`
	Items       []PermissionItem `json:"items"`
	Permissions []string         `json:"permissions"`
}

// PermissionChange is one requested override. Effect "" clears the override so
// the user inherits the role template again.
type PermissionChange struct {
	Permission string `json:"permission"`
	Effect     string `json:"effect"` // "", "allow", "deny"
}

// accessManagementPermissions are the permissions the lock-out guard protects.
var accessManagementPermissions = []string{
	authz.Permission("pengguna", authz.ActionView),
	authz.Permission("pengguna", authz.ActionUpdate),
}

// AccessService edits per-user menu access on top of the role template.
type AccessService struct {
	store    accessStore
	users    userLister
	resolver *authz.Resolver
}

// NewAccessService wires the service.
func NewAccessService(store accessStore, users userLister, resolver *authz.Resolver) *AccessService {
	return &AccessService{store: store, users: users, resolver: resolver}
}

// ForUser returns every registry permission with its source for this user.
func (s *AccessService) ForUser(ctx context.Context, userID string) (*UserAccess, error) {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil || u == nil {
		return nil, pkgerrors.NotFound("user", userID)
	}

	rolePerms, err := s.store.RolePermissions(ctx, string(u.Role))
	if err != nil {
		return nil, err
	}
	roleSet := map[string]bool{}
	for _, p := range rolePerms {
		roleSet[p] = true
	}
	allow, deny, err := s.store.UserOverrides(ctx, userID)
	if err != nil {
		return nil, err
	}
	overrides := map[string]string{}
	for _, p := range allow {
		overrides[p] = "allow"
	}
	for _, p := range deny {
		overrides[p] = "deny"
	}

	effective, err := s.resolver.Effective(ctx, userID, string(u.Role))
	if err != nil {
		return nil, err
	}

	items := make([]PermissionItem, 0, 32)
	for _, m := range authz.Menus() {
		for _, a := range m.Actions {
			p := authz.Permission(m.Key, a)
			items = append(items, PermissionItem{
				Permission:  p,
				MenuKey:     m.Key,
				MenuLabel:   m.Label,
				Action:      string(a),
				RoleGranted: roleSet[p],
				Override:    overrides[p],
				Effective:   effective.Has(p),
			})
		}
	}

	return &UserAccess{
		UserID:      u.ID,
		Name:        u.Name,
		Email:       u.Email,
		Role:        string(u.Role),
		Items:       items,
		Permissions: effective.Sorted(),
	}, nil
}

// Update applies overrides and enforces the lock-out guard.
func (s *AccessService) Update(ctx context.Context, actorID, userID string, changes []PermissionChange) error {
	target, err := s.users.FindByID(ctx, userID)
	if err != nil || target == nil {
		return pkgerrors.NotFound("user", userID)
	}

	// Validate everything before writing anything.
	for _, c := range changes {
		if !authz.Known(c.Permission) {
			return pkgerrors.Validation("permission", "permission tidak dikenal: "+c.Permission)
		}
		switch c.Effect {
		case "", "allow", "deny":
		default:
			return pkgerrors.Validation("effect", "effect harus allow, deny, atau kosong")
		}
	}

	// Simulate the resulting override state for the guard check.
	allow, deny, err := s.store.UserOverrides(ctx, userID)
	if err != nil {
		return err
	}
	pre := map[string]string{}
	for _, p := range allow {
		pre[p] = "allow"
	}
	for _, p := range deny {
		pre[p] = "deny"
	}
	sim := map[string]string{}
	for k, v := range pre {
		sim[k] = v
	}
	for _, c := range changes {
		if c.Effect == "" {
			delete(sim, c.Permission)
		} else {
			sim[c.Permission] = c.Effect
		}
	}

	rolePerms, err := s.store.RolePermissions(ctx, string(target.Role))
	if err != nil {
		return err
	}
	if err := s.guardAdminAccess(ctx, target, rolePerms, pre, sim); err != nil {
		return err
	}

	for _, c := range changes {
		if c.Effect == "" {
			if err := s.store.ClearUserOverride(ctx, userID, c.Permission, actorID); err != nil {
				return err
			}
			continue
		}
		if err := s.store.SetUserOverride(ctx, userID, c.Permission, c.Effect, actorID); err != nil {
			return err
		}
	}
	return nil
}

// guardAdminAccess refuses a change that would strip the last active
// super_admin of the ability to manage users and access. Without it an owner can
// lock the whole company out of its own admin panel.
//
// The guard only fires when the change is a REGRESSION for a super admin: editing
// an HR user (or adding permissions) is never blocked by it.
func (s *AccessService) guardAdminAccess(ctx context.Context, target *domain.User, rolePerms []string, pre, post map[string]string) error {
	if target.Role != domain.RoleSuperAdmin || !target.IsActive {
		return nil
	}

	canManage := func(overrides map[string]string) bool {
		roleSet := map[string]bool{}
		for _, p := range rolePerms {
			roleSet[p] = true
		}
		for _, required := range accessManagementPermissions {
			got := roleSet[required]
			if ov, ok := overrides[required]; ok {
				got = ov == "allow"
			}
			if !got {
				return false
			}
		}
		return true
	}

	if !canManage(pre) || canManage(post) {
		return nil // not a regression
	}

	users, err := s.users.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.Role != domain.RoleSuperAdmin || !u.IsActive || u.ID == target.ID {
			continue
		}
		perms, err := s.store.RolePermissions(ctx, string(u.Role))
		if err != nil {
			return err
		}
		allow, deny, err := s.store.UserOverrides(ctx, u.ID)
		if err != nil {
			return err
		}
		ov := map[string]string{}
		for _, p := range allow {
			ov[p] = "allow"
		}
		for _, p := range deny {
			ov[p] = "deny"
		}
		roleSet := map[string]bool{}
		for _, p := range perms {
			roleSet[p] = true
		}
		ok := true
		for _, required := range accessManagementPermissions {
			got := roleSet[required]
			if v, exists := ov[required]; exists {
				got = v == "allow"
			}
			if !got {
				ok = false
				break
			}
		}
		if ok {
			return nil
		}
	}

	return pkgerrors.Validation("permission",
		"perubahan ditolak: harus tersisa minimal satu super admin yang bisa mengelola pengguna & akses")
}

// SummariseOverride renders a short label for logs/messages.
func SummariseOverride(changes []PermissionChange) string {
	parts := make([]string, 0, len(changes))
	for _, c := range changes {
		e := c.Effect
		if e == "" {
			e = "inherit"
		}
		parts = append(parts, fmt.Sprintf("%s=%s", c.Permission, e))
	}
	return strings.Join(parts, ",")
}
