-- 016_email_verifications.sql — change identity-hardening R5 (slice 2b)
-- Token verifikasi email: acak, disimpan sebagai hash, sekali pakai, kedaluwarsa.
-- Additif: hanya tabel baru.
CREATE TABLE IF NOT EXISTS email_verifications (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    token_hash  TEXT NOT NULL UNIQUE,
    created_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL,
    consumed_at TEXT NULL
);
CREATE INDEX IF NOT EXISTS idx_email_verifications_user ON email_verifications (user_id);
