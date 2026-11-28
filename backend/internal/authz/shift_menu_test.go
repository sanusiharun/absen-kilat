package authz

import "testing"

// Menu shift adalah satu-satunya sumber kebenaran untuk permission shift
// (EPIC #60 S2 / tiket #62): view, create, update — tanpa delete.
func TestRegistry_HasShiftMenu(t *testing.T) {
	m, ok := MenuByKey("shift")
	if !ok {
		t.Fatal("menu shift belum terdaftar di registry")
	}
	if m.Href != "/shift" {
		t.Fatalf("href = %q, want /shift", m.Href)
	}
	if m.Scope != ScopeAll {
		t.Fatalf("scope = %q, want all (shift = master data)", m.Scope)
	}
	want := map[Action]bool{ActionView: false, ActionCreate: false, ActionUpdate: false}
	for _, a := range m.Actions {
		if _, ok := want[a]; !ok {
			t.Fatalf("aksi tak terduga %q pada menu shift", a)
		}
		want[a] = true
	}
	for a, seen := range want {
		if !seen {
			t.Fatalf("aksi %q belum ada di menu shift", a)
		}
	}
	for _, p := range []string{"menu.shift.view", "menu.shift.create", "menu.shift.update"} {
		if !Known(p) {
			t.Fatalf("%q harus dikenali registry", p)
		}
	}
	if Known("menu.shift.delete") {
		t.Fatal("menu.shift.delete tidak boleh ada (soft delete lewat active=false)")
	}
}

// Setiap route admin shift harus dipetakan ke permission yang tepat, karena
// middleware menolak route admin yang tidak terpetakan.
func TestRoutes_ShiftEndpointsAreMapped(t *testing.T) {
	cases := []struct {
		method, pattern, want string
	}{
		{"GET", "/admin/shifts", "menu.shift.view"},
		{"GET", "/api/v1/admin/shifts", "menu.shift.view"},
		{"POST", "/admin/shifts", "menu.shift.create"},
		{"PATCH", "/admin/shifts/{id}", "menu.shift.update"},
	}
	for _, tc := range cases {
		got, ok := PermissionFor(tc.method, tc.pattern)
		if !ok {
			t.Fatalf("%s %s belum dipetakan", tc.method, tc.pattern)
		}
		if got != tc.want {
			t.Fatalf("%s %s = %q, want %q", tc.method, tc.pattern, got, tc.want)
		}
	}
	if _, ok := PermissionFor("DELETE", "/admin/shifts/{id}"); ok {
		t.Fatal("tidak boleh ada rule DELETE untuk shift")
	}
}
