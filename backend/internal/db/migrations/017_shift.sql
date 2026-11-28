-- 017_shift.sql — EPIC #60 (S1): penjadwalan shift 24/7. Aditif.
CREATE TABLE IF NOT EXISTS shifts (
    id              TEXT PRIMARY KEY,
    code            TEXT NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    start_time      TEXT NOT NULL,              -- HH:MM
    end_time        TEXT NOT NULL,              -- HH:MM
    crosses_midnight INTEGER NOT NULL DEFAULT 0,
    grace_minutes   INTEGER NOT NULL DEFAULT 15,
    break_minutes   INTEGER NOT NULL DEFAULT 0,
    location_id     TEXT NULL REFERENCES locations(id),
    active          INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_shifts_code ON shifts (code);

CREATE TABLE IF NOT EXISTS shift_assignments (
    id         TEXT PRIMARY KEY,
    shift_id   TEXT NOT NULL REFERENCES shifts(id),
    user_id    TEXT NULL REFERENCES users(id),
    start_date TEXT NOT NULL,   -- YYYY-MM-DD, inklusif
    end_date   TEXT NOT NULL,
    source     TEXT NOT NULL DEFAULT 'manual',
    created_by TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_shift_assign_user ON shift_assignments (user_id, start_date);
CREATE INDEX IF NOT EXISTS idx_shift_assign_shift ON shift_assignments (shift_id);

ALTER TABLE attendances ADD COLUMN shift_id TEXT NULL REFERENCES shifts(id);
