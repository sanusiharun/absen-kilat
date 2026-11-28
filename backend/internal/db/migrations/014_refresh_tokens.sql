-- 014_refresh_tokens.sql — change identity-hardening (R3)
-- Token refresh sekali pakai dengan rotasi atomik dan deteksi replay.
-- Additif: hanya tabel baru, tidak menyentuh users atau data lain.
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id             TEXT PRIMARY KEY,
    user_id        TEXT NOT NULL REFERENCES users(id),
    family_id      TEXT NOT NULL,
    token_hash     TEXT NOT NULL UNIQUE,
    issued_at      TEXT NOT NULL,
    expires_at     TEXT NOT NULL,
    consumed_at    TEXT NULL,
    replaced_by    TEXT NULL,
    revoked_at     TEXT NULL,
    revoked_reason TEXT NULL,
    ip             TEXT NOT NULL DEFAULT '',
    user_agent     TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_family ON refresh_tokens (user_id, family_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires ON refresh_tokens (expires_at);
