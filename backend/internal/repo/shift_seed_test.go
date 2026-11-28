package repo_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/db"
)

// Migrasi 019 harus memberi super_admin grant menu.shift.* pada instance yang
// sudah ter-seed (jalur upgrade), dan tetap idempoten bila dijalankan ulang.
func TestMigration019_SeedsShiftPermissionsForSuperAdmin(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "seed.db")

	pool, err := db.Open(ctx, path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}

	check := func(p *sql.DB, stage string) {
		t.Helper()
		for _, perm := range []string{"menu.shift.view", "menu.shift.create", "menu.shift.update"} {
			var n int
			if err := p.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM role_permissions WHERE role = 'super_admin' AND permission_key = ?`,
				perm).Scan(&n); err != nil {
				t.Fatalf("%s: query %s: %v", stage, perm, err)
			}
			if n != 1 {
				t.Fatalf("%s: grant %s = %d baris, want 1", stage, perm, n)
			}
		}
	}
	check(pool, "setelah Open")

	// Menjalankan ulang pernyataan migrasi tidak boleh menambah baris duplikat.
	if err := reseedShiftPermissions(ctx, pool); err != nil {
		t.Fatalf("reseed: %v", err)
	}
	check(pool, "setelah reseed (idempoten)")

	var total int
	if err := pool.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM role_permissions WHERE permission_key = 'menu.shift.delete'`).Scan(&total); err != nil {
		t.Fatalf("query delete grant: %v", err)
	}
	if total != 0 {
		t.Fatalf("menu.shift.delete tidak boleh di-grant, dapat %d baris", total)
	}

	if err := pool.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Buka ulang: migration runner harus melewati versi yang sudah tercatat.
	again, err := db.Open(ctx, path)
	if err != nil {
		t.Fatalf("db.Open kedua: %v", err)
	}
	defer again.Close()
	check(again, "setelah buka ulang")
}

// reseedShiftPermissions menjalankan isi 019 lagi untuk membuktikan idempotensi.
func reseedShiftPermissions(ctx context.Context, pool *sql.DB) error {
	_, err := pool.ExecContext(ctx, `
INSERT INTO role_permissions (role, permission_key, created_at)
SELECT 'super_admin', p, datetime('now')
  FROM (SELECT 'menu.shift.view' AS p
        UNION ALL SELECT 'menu.shift.create'
        UNION ALL SELECT 'menu.shift.update') AS new_perms
 WHERE NOT EXISTS (
   SELECT 1 FROM role_permissions rp
    WHERE rp.role = 'super_admin' AND rp.permission_key = new_perms.p
 );`)
	return err
}
