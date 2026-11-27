package authz

import "context"

// Store is the persistence needed to resolve a user's effective permissions.
type Store interface {
	// RolePermissions returns the permission keys granted to a role template.
	RolePermissions(ctx context.Context, role string) ([]string, error)
	// UserOverrides returns explicit per-user grants and denials.
	UserOverrides(ctx context.Context, userID string) (allow, deny []string, err error)
}

// Set is a resolved permission set.
type Set map[string]bool

// Has reports whether the permission is granted. Unknown permissions are simply
// absent, which keeps the resolver deny-by-default.
func (s Set) Has(permission string) bool { return s[permission] }

// Sorted returns the granted permissions in registry order (stable output for
// API responses and tests).
func (s Set) Sorted() []string {
	out := make([]string, 0, len(s))
	for _, p := range AllPermissions() {
		if s[p] {
			out = append(out, p)
		}
	}
	return out
}

// Menus returns the menus the set can see, in navigation order.
func (s Set) Menus() []Menu {
	out := make([]Menu, 0, len(s))
	for _, m := range Menus() {
		if s[Permission(m.Key, ActionView)] {
			out = append(out, m)
		}
	}
	return out
}

// Resolver combines the role template with per-user overrides.
//
//	effective = (role ∪ override-allow) − override-deny
//
// A denial always wins, which is what makes it safe for an admin to hand out a
// broad role and take single menus back.
type Resolver struct {
	store Store
}

// NewResolver wires a resolver to its store.
func NewResolver(store Store) *Resolver { return &Resolver{store: store} }

// Effective resolves the permission set for one user/role pair.
func (r *Resolver) Effective(ctx context.Context, userID, role string) (Set, error) {
	set := Set{}

	rolePerms, err := r.store.RolePermissions(ctx, role)
	if err != nil {
		return nil, err
	}
	for _, p := range rolePerms {
		if Known(p) {
			set[p] = true
		}
	}

	if userID == "" {
		return set, nil
	}
	allow, deny, err := r.store.UserOverrides(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, p := range allow {
		if Known(p) {
			set[p] = true
		}
	}
	for _, p := range deny {
		if Known(p) {
			delete(set, p)
		}
	}
	return set, nil
}
