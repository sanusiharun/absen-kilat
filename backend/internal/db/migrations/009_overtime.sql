-- 009_overtime.sql — modul Lembur (Overtime).
-- English column names per project convention.
--
-- 1) overtimes: satu baris = satu klaim lembur per user, dengan rentang waktu
--    eksplisit (start_at/end_at UTC) sehingga durasi bisa diaudit, bukan hanya
--    disimpan dalam menit.
-- 2) Menit dihitung di service (bukan generated column) supaya aturan pembulatan
--    tetap di satu tempat.
--
-- Catatan: nilai lembur (uang) TIDAK disimpan di sini. Modul ini hanya mencatat
-- waktu yang disetujui; konversi ke rupiah ada di modul payroll (belum ada).

CREATE TABLE IF NOT EXISTS overtimes (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL REFERENCES users(id),
    work_date        TEXT NOT NULL,                       -- YYYY-MM-DD (tanggal kerja, lokal)
    start_at         TEXT NOT NULL,                       -- RFC3339 UTC
    end_at           TEXT NOT NULL,                       -- RFC3339 UTC
    minutes          INTEGER NOT NULL,                    -- durasi, dihitung service
    reason           TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
    approved_by      TEXT,                                -- users.id
    approved_at      TEXT,                                -- RFC3339 UTC
    rejection_reason TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_overtimes_user   ON overtimes (user_id);
CREATE INDEX IF NOT EXISTS idx_overtimes_status ON overtimes (status);
CREATE INDEX IF NOT EXISTS idx_overtimes_date   ON overtimes (work_date);
