-- 015_credential_version.sql — change identity-refresh-corrective (C4)
-- Versi kredensial per user: dinaikkan setiap password berubah, sehingga token
-- yang diterbitkan sebelum perubahan tidak bisa dipakai walau sudah tersimpan.
ALTER TABLE users ADD COLUMN credential_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE refresh_tokens ADD COLUMN credential_version INTEGER NOT NULL DEFAULT 0;
