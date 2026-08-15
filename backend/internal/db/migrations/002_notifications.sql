-- 002_notifications.sql — dukungan desain mockup: alamat & catatan absen,
-- tabel notifikasi, dan tabel sesi (untuk pencabutan token).

-- Alamat hasil reverse-geocoding & catatan untuk record absen.
ALTER TABLE attendances ADD COLUMN address TEXT NOT NULL DEFAULT '';
ALTER TABLE attendances ADD COLUMN notes   TEXT NOT NULL DEFAULT '';

-- Notifikasi per-user (tab Notifikasi di app).
CREATE TABLE IF NOT EXISTS notifications (
    id          TEXT PRIMARY KEY,                       -- UUID v7
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL DEFAULT '',
    is_read     INTEGER NOT NULL DEFAULT 0,             -- 0/1
    created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_created
    ON notifications (user_id, created_at);

-- Sesi refresh-token (memungkinkan logout semua perangkat).
CREATE TABLE IF NOT EXISTS sessions (
    id            TEXT PRIMARY KEY,                     -- UUID v7
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    expires_at    TEXT NOT NULL,
    revoked_at    TEXT                                -- NULL jika masih aktif
);

CREATE INDEX IF NOT EXISTS idx_sessions_user
    ON sessions (user_id);
