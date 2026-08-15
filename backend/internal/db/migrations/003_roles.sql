-- 003_roles.sql — role & status akun untuk Admin Dashboard.
-- Role: 'admin' atau 'employee' (default). Is_active utk enable/disable akun.

ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'employee'
    CHECK (role IN ('admin', 'employee'));

ALTER TABLE users ADD COLUMN is_active INTEGER NOT NULL DEFAULT 1; -- 0/1

CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);
