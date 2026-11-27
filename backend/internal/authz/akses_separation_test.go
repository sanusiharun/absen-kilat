package authz_test

import (
	"strings"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/authz"
)

// identity-hardening R2: endpoint yang mengubah menu/permission per user wajib
// dijaga permission akses.*, bukan pengguna.*.
func TestRoutes_PermissionEndpointsUseAkses(t *testing.T) {
	for _, rt := range authz.Rules() {
		if rt.Pattern != "/admin/users/{id}/permissions" {
			continue
		}
		if !strings.HasPrefix(rt.Permission, "menu.akses.") {
			t.Fatalf("%s %s dijaga %q, harus menu.akses.*", rt.Method, rt.Pattern, rt.Permission)
		}
	}
	// PermissionFor adalah jalur yang dipakai middleware; pastikan hasilnya sama.
	perm, ok := authz.PermissionFor("PATCH", "/admin/users/{id}/permissions")
	if !ok {
		t.Fatal("PATCH permissions harus punya pemetaan permission")
	}
	if perm != authz.Permission("akses", authz.ActionUpdate) {
		t.Fatalf("PATCH permissions harus %q, dapat %q", authz.Permission("akses", authz.ActionUpdate), perm)
	}
}

// Menu akses harus ada di registry dengan action view dan update.
func TestRegistry_HasAksesMenu(t *testing.T) {
	var found bool
	for _, m := range authz.Menus() {
		if m.Key != "akses" {
			continue
		}
		found = true
		if len(m.Actions) != 2 {
			t.Fatalf("menu akses harus punya 2 action, dapat %d", len(m.Actions))
		}
	}
	if !found {
		t.Fatal("menu akses tidak ada di registry")
	}
}
