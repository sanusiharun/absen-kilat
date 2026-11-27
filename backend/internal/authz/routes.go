package authz

// RouteRule maps one HTTP route to the permission required to call it.
//
// Patterns are written WITHOUT the "/api/v1" prefix because that is how the
// middleware normalises what chi reports (see NormalizePattern). Keeping the
// table here (not scattered in handlers) is what makes the
// "every admin route is guarded" test possible.
type RouteRule struct {
	Method     string
	Pattern    string
	Permission string
}

// APIPrefix is stripped from chi route patterns before lookup.
const APIPrefix = "/api/v1"

var routes = []RouteRule{
	// Dashboard
	{"GET", "/admin/stats", Permission("dashboard", ActionView)},
	{"GET", "/admin/trends", Permission("dashboard", ActionView)},
	{"GET", "/admin/locations/breakdown", Permission("dashboard", ActionView)},

	// Pengguna
	{"GET", "/admin/users", Permission("pengguna", ActionView)},
	{"POST", "/admin/users", Permission("pengguna", ActionCreate)},
	{"PATCH", "/admin/users/{id}", Permission("pengguna", ActionUpdate)},
	{"GET", "/admin/users/{id}/permissions", Permission("akses", ActionView)},
	{"PATCH", "/admin/users/{id}/permissions", Permission("akses", ActionUpdate)},

	// Lokasi (+ QR tokens are managed from the Lokasi page)
	{"GET", "/admin/locations", Permission("lokasi", ActionView)},
	{"POST", "/admin/locations", Permission("lokasi", ActionCreate)},
	{"PATCH", "/admin/locations/{id}", Permission("lokasi", ActionUpdate)},
	{"DELETE", "/admin/locations/{id}", Permission("lokasi", ActionDelete)},
	{"GET", "/admin/qr", Permission("lokasi", ActionView)},
	{"POST", "/admin/qr", Permission("lokasi", ActionCreate)},
	{"DELETE", "/admin/qr/{id}", Permission("lokasi", ActionDelete)},

	// Absensi
	{"GET", "/admin/attendance", Permission("absensi", ActionView)},
	{"PATCH", "/admin/attendance/{id}", Permission("absensi", ActionOverride)},

	// Laporan
	{"GET", "/admin/reports", Permission("laporan", ActionView)},
	{"GET", "/admin/reports/users", Permission("laporan", ActionView)},

	// Device
	{"GET", "/admin/devices", Permission("device", ActionView)},

	// Pengaturan
	{"GET", "/admin/settings", Permission("pengaturan", ActionView)},
	{"PUT", "/admin/settings", Permission("pengaturan", ActionUpdate)},

	// Approval (team queue + decisions)
	{"GET", "/admin/team", Permission("approval", ActionView)},
	{"GET", "/admin/approvals", Permission("approval", ActionView)},
	{"POST", "/admin/approvals/{id}", Permission("approval", ActionApprove)},

	// Cuti (admin)
	{"GET", "/admin/leave", Permission("cuti", ActionView)},
	{"GET", "/admin/leave/pending", Permission("cuti", ActionView)},
	{"POST", "/admin/leave/{id}/approve", Permission("cuti", ActionApprove)},
	{"POST", "/admin/leave/{id}/reject", Permission("cuti", ActionApprove)},

	// Lembur (admin)
	{"GET", "/admin/overtime", Permission("lembur", ActionView)},
	{"GET", "/admin/overtime/pending", Permission("lembur", ActionView)},
	{"POST", "/admin/overtime/{id}/approve", Permission("lembur", ActionApprove)},
	{"POST", "/admin/overtime/{id}/reject", Permission("lembur", ActionApprove)},

	// Proyek & Mitra
	{"GET", "/admin/partners", Permission("proyek", ActionView)},
	{"POST", "/admin/partners", Permission("proyek", ActionCreate)},
	{"GET", "/admin/projects", Permission("proyek", ActionView)},
	{"POST", "/admin/projects", Permission("proyek", ActionCreate)},
	{"GET", "/admin/projects/{id}", Permission("proyek", ActionView)},
	{"GET", "/admin/assignments", Permission("proyek", ActionView)},
	{"POST", "/admin/assignments", Permission("proyek", ActionCreate)},
}

// Rules returns a copy of the route table.
func Rules() []RouteRule {
	out := make([]RouteRule, len(routes))
	copy(out, routes)
	return out
}

// NormalizePattern strips the API prefix so lookups work regardless of where the
// route was mounted.
func NormalizePattern(pattern string) string {
	if len(pattern) > len(APIPrefix) && pattern[:len(APIPrefix)] == APIPrefix {
		return pattern[len(APIPrefix):]
	}
	return pattern
}

// PermissionFor resolves the permission required for a method+pattern pair.
func PermissionFor(method, pattern string) (string, bool) {
	p := NormalizePattern(pattern)
	for _, r := range routes {
		if r.Method == method && r.Pattern == p {
			return r.Permission, true
		}
	}
	return "", false
}

// IsAdminPattern reports whether a path belongs to the admin area, which is the
// area where an unmapped route must be denied by default.
func IsAdminPattern(pattern string) bool {
	p := NormalizePattern(pattern)
	return len(p) >= 7 && p[:7] == "/admin/"
}
