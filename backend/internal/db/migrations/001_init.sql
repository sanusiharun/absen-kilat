-- 001_init.sql — skema awal Absen Kilat (users + attendances)
-- Waktu disimpan UTC (RFC3339) dalam TEXT; konversi ke zona lokal di aplikasi.

CREATE TABLE IF NOT EXISTS users (
    id              TEXT PRIMARY KEY,                 -- UUID v7
    email           TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash   TEXT NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    avatar_path     TEXT NOT NULL DEFAULT '',         -- path relatif di uploads/
    is_email_verified INTEGER NOT NULL DEFAULT 0,     -- 0/1
    notification_enabled INTEGER NOT NULL DEFAULT 1,  -- 0/1
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS attendances (
    id          TEXT PRIMARY KEY,                     -- UUID v7
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT NOT NULL CHECK (type IN ('check_in', 'check_out')),
    status      TEXT NOT NULL CHECK (status IN ('on_time', 'late')),
    photo_path  TEXT NOT NULL DEFAULT '',             -- path relatif di uploads/
    latitude    REAL,
    longitude   REAL,
    occurred_at TEXT NOT NULL,                        -- waktu absen (RFC3339 UTC)
    created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_attendances_user_date
    ON attendances (user_id, occurred_at);

CREATE INDEX IF NOT EXISTS idx_attendances_user_type_date
    ON attendances (user_id, type, occurred_at);
