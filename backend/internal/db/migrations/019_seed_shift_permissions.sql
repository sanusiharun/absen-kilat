-- 019_seed_shift_permissions.sql — EPIC #60 (S2), jalur upgrade.
-- Instance yang sudah di-seed (access_seed) tidak menerima permission baru dari
-- registry kode. Menu "shift" baru ada di kode (internal/authz/registry.go), jadi
-- grant-nya harus ditulis eksplisit untuk super_admin. Idempoten (pola 013).
INSERT INTO role_permissions (role, permission_key, created_at)
SELECT 'super_admin', p, datetime('now')
  FROM (SELECT 'menu.shift.view' AS p
        UNION ALL SELECT 'menu.shift.create'
        UNION ALL SELECT 'menu.shift.update') AS new_perms
 WHERE NOT EXISTS (
   SELECT 1 FROM role_permissions rp
    WHERE rp.role = 'super_admin' AND rp.permission_key = new_perms.p
 );
