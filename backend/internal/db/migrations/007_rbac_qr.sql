-- 007_rbac_qr.sql — RBAC roles + QR attendance tokens (issues #17, #18)
--
-- users: SQLite cannot ALTER a CHECK constraint, so the table is rebuilt
-- with the new role set ('super_admin' | 'hr' | 'supervisor' | 'employee')
-- and legacy 'admin' rows are mapped to 'super_admin'. The rebuild must run
-- with foreign_keys=OFF (handled by the migration runner in internal/db),
-- otherwise DROP TABLE users fails because child tables reference it.
-- Column order matches 001_init.sql + 003_roles.sql + 006_rbac_locations.sql.

CREATE TABLE users_new (
    id                  TEXT PRIMARY KEY,
    email               TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash       TEXT NOT NULL,
    name                TEXT NOT NULL DEFAULT '',
    avatar_path         TEXT NOT NULL DEFAULT '',
    is_email_verified   INTEGER NOT NULL DEFAULT 0,
    notification_enabled INTEGER NOT NULL DEFAULT 1,
    role                TEXT NOT NULL DEFAULT 'employee'
        CHECK (role IN ('super_admin', 'hr', 'supervisor', 'employee')),
    is_active           INTEGER NOT NULL DEFAULT 1,
    nip                 TEXT NOT NULL DEFAULT '',
    phone               TEXT NOT NULL DEFAULT '',
    department          TEXT NOT NULL DEFAULT '',
    position            TEXT NOT NULL DEFAULT '',
    address             TEXT NOT NULL DEFAULT '',
    join_date           TEXT NOT NULL DEFAULT '',
    manager_id          TEXT REFERENCES users(id),
    employee_code       TEXT NOT NULL DEFAULT '',
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);

INSERT INTO users_new (id, email, password_hash, name, avatar_path,
    is_email_verified, notification_enabled, role, is_active,
    nip, phone, department, position, address, join_date, manager_id, employee_code,
    created_at, updated_at)
SELECT id, email, password_hash, name, avatar_path,
    is_email_verified, notification_enabled,
    CASE role WHEN 'admin' THEN 'super_admin' ELSE role END,
    is_active,
    nip, phone, department, position, address, join_date, manager_id, employee_code,
    created_at, updated_at
FROM users;

DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

-- Recreate indexes lost with the old table (the email UNIQUE column
-- constraint survives the rebuild as an autoindex; these are kept explicit
-- for clarity and for the partial unique employee_code index).
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_employee_code
    ON users (employee_code) WHERE employee_code != '';

-- ===== qr_tokens: QR attendance tokens bound to a location =====
CREATE TABLE IF NOT EXISTS qr_tokens (
    id          TEXT PRIMARY KEY,
    location_id TEXT NOT NULL REFERENCES locations(id),
    token       TEXT NOT NULL UNIQUE,
    active      INTEGER NOT NULL DEFAULT 1,   -- 0/1 (revoked)
    expires_at  TEXT,                          -- RFC3339 UTC; NULL = permanent
    created_by  TEXT NOT NULL REFERENCES users(id),
    created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_qr_tokens_token ON qr_tokens (token);
