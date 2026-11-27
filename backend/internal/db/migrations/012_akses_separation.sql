-- 012_akses_separation.sql — change identity-hardening (R2)
-- Izin mengelola menu/permission per user (menu.akses.*) dipisahkan dari izin
-- mengubah data pengguna. Instance yang sudah pernah di-seed tidak akan
-- di-seed ulang (access_seed), jadi cabut grant lama dari template HR di sini.
-- Idempoten: DELETE tanpa hasil tidak berefek.
DELETE FROM role_permissions
 WHERE role = 'hr'
   AND permission_key LIKE 'menu.akses.%';

-- super_admin tetap memegang semua permission (tidak ada perubahan).
