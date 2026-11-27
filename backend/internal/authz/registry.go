// Package authz is the single source of truth for admin menus and the
// permissions attached to them.
//
// Menus and their actions live in CODE, not in the database, so a menu cannot
// appear in the UI without a permission key and without something guarding its
// routes. The database only stores assignments (role templates + per-user
// overrides).
package authz

import "strings"

// Action is one operation available inside a menu.
type Action string

const (
	ActionView     Action = "view"
	ActionCreate   Action = "create"
	ActionUpdate   Action = "update"
	ActionDelete   Action = "delete"
	ActionExport   Action = "export"
	ActionApprove  Action = "approve"
	ActionOverride Action = "override"
)

// Scope describes how wide a menu's data can reach. Scope is derived from the
// role and is NOT assignable through overrides.
type Scope string

const (
	ScopeAll  Scope = "all"  // everything in the company
	ScopeTeam Scope = "team" // only the caller's team (supervisor)
	ScopeSelf Scope = "self" // only the caller's own records
)

// Menu is one entry of the admin navigation plus the actions it supports.
type Menu struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Href    string   `json:"href"`
	Actions []Action `json:"actions"`
	Scope   Scope    `json:"scope"`
}

// PermissionPrefix is the common prefix of every permission key.
const PermissionPrefix = "menu."

// menus is the canonical menu list. Order matters: it is the navigation order.
var menus = []Menu{
	{Key: "dashboard", Label: "Dashboard", Href: "/dashboard", Actions: []Action{ActionView}, Scope: ScopeAll},
	{Key: "pengguna", Label: "Pengguna", Href: "/pengguna", Actions: []Action{ActionView, ActionCreate, ActionUpdate}, Scope: ScopeAll},
	{Key: "akses", Label: "Akses Menu", Href: "/pengguna", Actions: []Action{ActionView, ActionUpdate}, Scope: ScopeAll},
	{Key: "absensi", Label: "Absensi", Href: "/absensi", Actions: []Action{ActionView, ActionOverride}, Scope: ScopeTeam},
	{Key: "laporan", Label: "Laporan", Href: "/laporan", Actions: []Action{ActionView, ActionExport}, Scope: ScopeTeam},
	{Key: "lokasi", Label: "Lokasi", Href: "/lokasi", Actions: []Action{ActionView, ActionCreate, ActionUpdate, ActionDelete}, Scope: ScopeAll},
	{Key: "device", Label: "Device", Href: "/device", Actions: []Action{ActionView}, Scope: ScopeAll},
	{Key: "cuti", Label: "Cuti", Href: "/cuti", Actions: []Action{ActionView, ActionApprove}, Scope: ScopeTeam},
	{Key: "lembur", Label: "Lembur", Href: "/lembur", Actions: []Action{ActionView, ActionApprove}, Scope: ScopeTeam},
	{Key: "proyek", Label: "Proyek & Mitra", Href: "/proyek", Actions: []Action{ActionView, ActionCreate, ActionUpdate}, Scope: ScopeAll},
	{Key: "approval", Label: "Approval", Href: "/approval", Actions: []Action{ActionView, ActionApprove}, Scope: ScopeTeam},
	{Key: "pengaturan", Label: "Pengaturan", Href: "/pengaturan", Actions: []Action{ActionView, ActionUpdate}, Scope: ScopeAll},
	{Key: "akun-admin", Label: "Profil", Href: "/akun-admin", Actions: []Action{ActionView, ActionUpdate}, Scope: ScopeSelf},
}

// KnownActions are the only actions a menu may declare.
var KnownActions = []Action{
	ActionView, ActionCreate, ActionUpdate, ActionDelete,
	ActionExport, ActionApprove, ActionOverride,
}

// Menus returns a copy of the canonical menu list.
func Menus() []Menu {
	out := make([]Menu, len(menus))
	copy(out, menus)
	return out
}

// MenuByKey looks a menu up by its key.
func MenuByKey(key string) (Menu, bool) {
	for _, m := range menus {
		if m.Key == key {
			return m, true
		}
	}
	return Menu{}, false
}

// Permission builds the permission key for a menu action: menu.<key>.<action>.
func Permission(menuKey string, a Action) string {
	return PermissionPrefix + menuKey + "." + string(a)
}

// ViewPermission is shorthand for Permission(menuKey, ActionView).
func ViewPermission(menuKey string) string { return Permission(menuKey, ActionView) }

// Known reports whether permission is declared in the registry. Anything not in
// the registry can never be granted.
func Known(permission string) bool {
	if !strings.HasPrefix(permission, PermissionPrefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(permission, PermissionPrefix), ".")
	if len(parts) != 2 {
		return false
	}
	m, ok := MenuByKey(parts[0])
	if !ok {
		return false
	}
	for _, a := range m.Actions {
		if string(a) == parts[1] {
			return true
		}
	}
	return false
}

// AllPermissions returns every permission declared in the registry, in a stable
// order (menu order, then action order).
func AllPermissions() []string {
	out := make([]string, 0, 32)
	for _, m := range menus {
		for _, a := range m.Actions {
			out = append(out, Permission(m.Key, a))
		}
	}
	return out
}

// MenuKeys returns the menu keys in navigation order.
func MenuKeys() []string {
	out := make([]string, 0, len(menus))
	for _, m := range menus {
		out = append(out, m.Key)
	}
	return out
}
