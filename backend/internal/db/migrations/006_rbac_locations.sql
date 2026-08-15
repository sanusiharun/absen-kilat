-- 006_rbac_locations.sql — RBAC awal & manajemen lokasi (issues #6,#7,#8,#9,#10,#14,#15)
-- 1) Profil pegawai & relasi manajer, 2) tabel locations (geofence multi-lokasi),
-- 3) tabel settings (konfigurasi jam kerja & kebijakan foto),
-- 4) attendances: source, approval_status + jejak approval.

-- ===== users: field profil & hierarki =====
ALTER TABLE users ADD COLUMN nip          TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN phone        TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN department   TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN position     TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN address      TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN join_date    TEXT NOT NULL DEFAULT '';  -- YYYY-MM-DD
ALTER TABLE users ADD COLUMN manager_id   TEXT REFERENCES users(id);
ALTER TABLE users ADD COLUMN employee_code TEXT NOT NULL DEFAULT ''; -- identitas utk QR/manual

-- UNIQUE tidak bisa lewat ADD COLUMN; pakai partial unique index
-- (baris dgn employee_code kosong tidak saling bentrok).
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_employee_code
    ON users (employee_code) WHERE employee_code != '';

-- ===== locations: titik geofence (office/client/field) =====
CREATE TABLE IF NOT EXISTS locations (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    type       TEXT NOT NULL CHECK (type IN ('office', 'client', 'field')),
    lat        REAL NOT NULL,
    lng        REAL NOT NULL,
    radius_m   INTEGER NOT NULL DEFAULT 500,
    active     INTEGER NOT NULL DEFAULT 1,   -- 0/1 (soft delete)
    created_at TEXT NOT NULL
);

-- ===== settings: konfigurasi sistem (key-value) =====
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO settings (key, value) VALUES
    ('work_start',   '09:00'),
    ('work_end',     '17:00'),
    ('absen_mode',   'both'),                -- both | checkin_only
    ('workdays',     '["mon","tue","wed","thu","fri"]'),
    ('photo_policy', 'radius');              -- radius: foto wajib di luar radius & utk type field

-- ===== attendances: source & alur approval =====
ALTER TABLE attendances ADD COLUMN source            TEXT NOT NULL DEFAULT 'app'
    CHECK (source IN ('app', 'qr', 'manual'));
ALTER TABLE attendances ADD COLUMN approval_status   TEXT NOT NULL DEFAULT 'pending'
    CHECK (approval_status IN ('pending', 'approved', 'rejected'));
ALTER TABLE attendances ADD COLUMN approved_by       TEXT;
ALTER TABLE attendances ADD COLUMN approved_at       TEXT;   -- RFC3339 UTC
ALTER TABLE attendances ADD COLUMN rejection_reason  TEXT NOT NULL DEFAULT '';
ALTER TABLE attendances ADD COLUMN location_id       TEXT REFERENCES locations(id);

-- ===== approval_log: jejak audit approve/reject/override =====
CREATE TABLE IF NOT EXISTS approval_log (
    id            TEXT PRIMARY KEY,
    attendance_id TEXT NOT NULL REFERENCES attendances(id),
    actor_id      TEXT NOT NULL REFERENCES users(id),
    action        TEXT NOT NULL,             -- approve | reject | override
    reason        TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_approval_log_attendance ON approval_log (attendance_id);
