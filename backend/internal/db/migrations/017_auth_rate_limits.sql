-- 017_auth_rate_limits.sql — change identity-hardening R4 (slice 2b-2)
-- Penghitung percobaan autentikasi bersama di SQLite supaya batas berlaku
-- agregat lintas proses/instance (I22). Hanya hash sha256 yang disimpan;
-- IP dan email mentah tidak pernah ditulis ke tabel ini.
CREATE TABLE IF NOT EXISTS auth_rate_limits (
    scope        TEXT    NOT NULL,
    key_hash     TEXT    NOT NULL,
    window_start INTEGER NOT NULL,
    hits         INTEGER NOT NULL,
    PRIMARY KEY (scope, key_hash, window_start)
);
CREATE INDEX IF NOT EXISTS idx_auth_rate_limits_window ON auth_rate_limits (window_start);
