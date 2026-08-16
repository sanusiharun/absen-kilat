-- 008_leave.sql — modul Cuti (Leave Management).
-- English column names per project convention.
--
-- 1) leave_types: master tipe cuti (durasi dinamis diatur HR).
--    default_days: kuota default (0 = tidak terbatas/dinamis).
--    max_extra_days: untuk paternity (3/5/7 dinamis), 0 = tidak ada opsi ekstra.
-- 2) leaves: pengajuan cuti per user.
-- 3) users: leave_quota (alokasi) + leave_used (terpakai).

-- ===== leave_types =====
CREATE TABLE IF NOT EXISTS leave_types (
    id           TEXT PRIMARY KEY,
    code         TEXT NOT NULL UNIQUE,            -- annual | sick | emergency | maternity | paternity
    name         TEXT NOT NULL,                   -- "Cuti Tahunan"
    default_days INTEGER NOT NULL DEFAULT 0,      -- 0 = dinamis / tidak terbatas
    max_extra_days INTEGER NOT NULL DEFAULT 0,    -- paternity: opsi 3/5/7
    requires_attachment INTEGER NOT NULL DEFAULT 0, -- 0/1
    active       INTEGER NOT NULL DEFAULT 1,
    created_at   TEXT NOT NULL
);

INSERT INTO leave_types (id, code, name, default_days, max_extra_days, requires_attachment, active, created_at)
VALUES
    ('lt_annual',   'annual',   'Cuti Tahunan',  12, 0, 0, 1, datetime('now')),
    ('lt_sick',     'sick',     'Cuti Sakit',     0, 0, 1, 1, datetime('now')),
    ('lt_emergency','emergency', 'Cuti Darurat',  0, 0, 0, 1, datetime('now')),
    ('lt_maternity','maternity', 'Cuti Melahirkan', 90, 0, 1, 1, datetime('now')),  -- 3 bulan
    ('lt_paternity','paternity', 'Cuti Ayah',      0, 7, 1, 1, datetime('now'));     -- dinamis 3/5/7

-- ===== leaves =====
CREATE TABLE IF NOT EXISTS leaves (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id),
    leave_type      TEXT NOT NULL REFERENCES leave_types(code),
    start_date      TEXT NOT NULL,                -- YYYY-MM-DD
    end_date        TEXT NOT NULL,                -- YYYY-MM-DD
    total_days      INTEGER NOT NULL,             -- hari kerja (Sen-Jum)
    reason          TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
    approved_by     TEXT,                          -- users.id
    approved_at     TEXT,                          -- RFC3339 UTC
    rejection_reason TEXT NOT NULL DEFAULT '',
    extra_days      INTEGER NOT NULL DEFAULT 0,    -- paternity 3/5/7
    attachment_path TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_leaves_user     ON leaves (user_id);
CREATE INDEX IF NOT EXISTS idx_leaves_status    ON leaves (status);
CREATE INDEX IF NOT EXISTS idx_leaves_dates     ON leaves (start_date, end_date);

-- ===== users: kuota & pemakaian cuti =====
ALTER TABLE users ADD COLUMN leave_quota INTEGER NOT NULL DEFAULT 12;  -- alokasi tahunan
ALTER TABLE users ADD COLUMN leave_used  INTEGER NOT NULL DEFAULT 0;   -- terpakai (hari kerja)
