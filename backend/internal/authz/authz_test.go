package authz

import (
	"context"
	"testing"
)

// ---- fake store ----------------------------------------------------------

type fakeStore struct {
	rolePerms map[string][]string
	allow     []string
	deny      []string
	err       error
}

func (f fakeStore) RolePermissions(_ context.Context, role string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rolePerms[role], nil
}

func (f fakeStore) UserOverrides(_ context.Context, _ string) ([]string, []string, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.allow, f.deny, nil
}

func TestRegistryUniqueMenusAndKeys(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range Menus() {
		if m.Key == "" || m.Label == "" {
			t.Fatalf("menu without key/label: %+v", m)
		}
		if seen[m.Key] {
			t.Fatalf("duplicate menu key %q", m.Key)
		}
		seen[m.Key] = true
		if len(m.Actions) == 0 {
			t.Fatalf("menu %q has no actions", m.Key)
		}
		for _, a := range m.Actions {
			if !actionKnown(a) {
				t.Fatalf("menu %q declares unknown action %q", m.Key, a)
			}
		}
		switch m.Scope {
		case ScopeAll, ScopeTeam, ScopeSelf:
		default:
			t.Fatalf("menu %q has invalid scope %q", m.Key, m.Scope)
		}
	}
}

func actionKnown(a Action) bool {
	for _, k := range KnownActions {
		if k == a {
			return true
		}
	}
	return false
}

func TestPermissionNamingAndKnown(t *testing.T) {
	p := Permission("pengguna", ActionUpdate)
	if p != "menu.pengguna.update" {
		t.Fatalf("unexpected permission key %q", p)
	}
	if !Known(p) {
		t.Fatalf("%q should be known", p)
	}
	for _, bad := range []string{"", "menu.", "menu.pengguna", "menu.pengguna.explode", "menu.ghost.view", "pengguna.view"} {
		if Known(bad) {
			t.Fatalf("%q must not be known", bad)
		}
	}
	if !Known(ViewPermission("laporan")) {
		t.Fatal("view permission of laporan must be known")
	}
}

func TestAllPermissionsAreKnownAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range AllPermissions() {
		if !Known(p) {
			t.Fatalf("registry produced unknown permission %q", p)
		}
		if seen[p] {
			t.Fatalf("duplicate permission %q", p)
		}
		seen[p] = true
	}
}

// ---- route table ---------------------------------------------------------

func TestRouteRulesAreConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Rules() {
		if r.Method == "" || r.Pattern == "" {
			t.Fatalf("incomplete rule %+v", r)
		}
		if !Known(r.Permission) {
			t.Fatalf("rule %s %s points to unknown permission %q", r.Method, r.Pattern, r.Permission)
		}
		if m := r.Pattern[0]; m != '/' {
			t.Fatalf("pattern must start with /: %q", r.Pattern)
		}
		key := r.Method + " " + r.Pattern
		if seen[key] {
			t.Fatalf("duplicate rule for %s", key)
		}
		seen[key] = true
	}
}

func TestPermissionForNormalisesPrefix(t *testing.T) {
	got, ok := PermissionFor("GET", "/api/v1/admin/users")
	if !ok || got != "menu.pengguna.view" {
		t.Fatalf("got %q %v, want menu.pengguna.view", got, ok)
	}
	if _, ok := PermissionFor("GET", "/api/v1/admin/unknown"); ok {
		t.Fatal("unknown admin route must not resolve")
	}
	if !IsAdminPattern("/api/v1/admin/users") {
		t.Fatal("admin pattern must be detected with prefix")
	}
	if IsAdminPattern("/api/v1/me") {
		t.Fatal("non admin pattern must not be detected")
	}
}

// ---- resolver ------------------------------------------------------------

func TestResolverRoleOnly(t *testing.T) {
	r := NewResolver(fakeStore{rolePerms: map[string][]string{
		"hr": {Permission("dashboard", ActionView), Permission("cuti", ActionView)},
	}})
	set, err := r.Effective(context.Background(), "u1", "hr")
	if err != nil {
		t.Fatal(err)
	}
	if !set.Has(Permission("cuti", ActionView)) || !set.Has(Permission("dashboard", ActionView)) {
		t.Fatal("role permissions must be granted")
	}
	if set.Has(Permission("pengaturan", ActionUpdate)) {
		t.Fatal("unexpected permission granted")
	}
}

func TestResolverAllowOverride(t *testing.T) {
	r := NewResolver(fakeStore{
		rolePerms: map[string][]string{"hr": {Permission("dashboard", ActionView)}},
		allow:     []string{Permission("laporan", ActionExport)},
	})
	set, err := r.Effective(context.Background(), "u1", "hr")
	if err != nil {
		t.Fatal(err)
	}
	if !set.Has(Permission("laporan", ActionExport)) {
		t.Fatal("allow override must be granted")
	}
}

func TestResolverDenyWins(t *testing.T) {
	r := NewResolver(fakeStore{
		rolePerms: map[string][]string{"hr": {Permission("pengguna", ActionView), Permission("laporan", ActionView)}},
		allow:     []string{Permission("pengguna", ActionUpdate)},
		deny:      []string{Permission("pengguna", ActionView)},
	})
	set, err := r.Effective(context.Background(), "u1", "hr")
	if err != nil {
		t.Fatal(err)
	}
	if set.Has(Permission("pengguna", ActionView)) {
		t.Fatal("deny must win over the role template")
	}
	if !set.Has(Permission("pengguna", ActionUpdate)) {
		t.Fatal("allow override that is not denied must stay granted")
	}
	if !set.Has(Permission("laporan", ActionView)) {
		t.Fatal("other role permissions must be untouched")
	}
}

func TestResolverIgnoresUnknownPermissions(t *testing.T) {
	r := NewResolver(fakeStore{
		rolePerms: map[string][]string{"hr": {"menu.ghost.view", "not-a-permission"}},
		allow:     []string{"menu.ghost.delete"},
	})
	set, err := r.Effective(context.Background(), "u1", "hr")
	if err != nil {
		t.Fatal(err)
	}
	if len(set) != 0 {
		t.Fatalf("unknown permissions must be dropped, got %v", set.Sorted())
	}
}

func TestSetMenusFollowsViewPermission(t *testing.T) {
	set := Set{
		Permission("dashboard", ActionView): true,
		Permission("cuti", ActionView):      true,
		Permission("cuti", ActionApprove):   true,
	}
	menus := set.Menus()
	if len(menus) != 2 {
		t.Fatalf("expected 2 visible menus, got %d", len(menus))
	}
	if menus[0].Key != "dashboard" || menus[1].Key != "cuti" {
		t.Fatalf("menus must follow registry order, got %v", []string{menus[0].Key, menus[1].Key})
	}
}
