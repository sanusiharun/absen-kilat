-- 013_seed_akses_permissions.sql — change identity-hardening (R2), jalur upgrade
-- Instance yang sudah di-seed (access_seed) tidak menerima permission baru dari
-- registry. Menu "akses" baru ditambahkan di kode, jadi permission-nya harus
-- di-grant eksplisit ke super_admin saat upgrade. Idempoten.
INSERT INTO role_permissions (role, permission_key, created_at)
SELECT 'super_admin', p, datetime('now')
  FROM (SELECT 'menu.akses.view' AS p UNION ALL SELECT 'menu.akses.update') AS new_perms
 WHERE NOT EXISTS (
   SELECT 1 FROM role_permissions rp
    WHERE rp.role = 'super_admin' AND rp.permission_key = new_perms.p
 );
