-- 010_partner_project.sql — master data Mitra (kru lepas) + Proyek/Kapal + Penugasan.
-- Requirement proposal: "Master Data terpisah antara Karyawan Tetap/Kontrak Kantor
-- dengan Karyawan Partner / Kru Lepas Proyek ... riwayat penugasan per nomor
-- proyek/nama kapal".
--
-- Sengaja TIDAK mengubah tabel users: pemisahan internal vs partner dinyatakan
-- lewat assignments.partner_id (NULL = karyawan internal, terisi = kru mitra).
-- Dengan begitu modul absensi/payroll yang sudah jalan tidak perlu diubah.

CREATE TABLE IF NOT EXISTS partners (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    contact_name  TEXT NOT NULL DEFAULT '',
    contact_phone TEXT NOT NULL DEFAULT '',
    active        INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id           TEXT PRIMARY KEY,
    code         TEXT NOT NULL UNIQUE,          -- nomor proyek/SPK
    name         TEXT NOT NULL,
    vessel_name  TEXT NOT NULL DEFAULT '',      -- nama kapal (maritim)
    client_name  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'done', 'cancelled')),
    start_date   TEXT NOT NULL DEFAULT '',      -- YYYY-MM-DD, boleh kosong
    end_date     TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS assignments (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    project_id  TEXT NOT NULL REFERENCES projects(id),
    partner_id  TEXT REFERENCES partners(id),   -- NULL = karyawan internal
    role        TEXT NOT NULL DEFAULT '',       -- diver, NDT technician, rigger, ...
    start_date  TEXT NOT NULL,                  -- YYYY-MM-DD
    end_date    TEXT NOT NULL DEFAULT '',       -- kosong = masih berjalan
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_assignments_user    ON assignments (user_id);
CREATE INDEX IF NOT EXISTS idx_assignments_project ON assignments (project_id);
CREATE INDEX IF NOT EXISTS idx_assignments_partner ON assignments (partner_id);
