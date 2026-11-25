-- 011_access_control.sql — menu-level permissions.
--
-- Menu & permission keys live in code (internal/authz); these tables only store
-- assignments: role templates and per-user overrides.

CREATE TABLE IF NOT EXISTS role_permissions (
    role           TEXT NOT NULL,
    permission_key TEXT NOT NULL,
    created_at     TEXT NOT NULL,
    PRIMARY KEY (role, permission_key)
);

CREATE TABLE IF NOT EXISTS user_permission_overrides (
    user_id        TEXT NOT NULL,
    permission_key TEXT NOT NULL,
    effect         TEXT NOT NULL CHECK (effect IN ('allow', 'deny')),
    created_by     TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL,
    PRIMARY KEY (user_id, permission_key),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_overrides_user ON user_permission_overrides (user_id);

-- Audit of permission changes: who changed what, from which value to which.
CREATE TABLE IF NOT EXISTS permission_audit (
    id             TEXT PRIMARY KEY,
    actor_id       TEXT NOT NULL DEFAULT '',
    target_user_id TEXT NOT NULL DEFAULT '',
    permission_key TEXT NOT NULL,
    before         TEXT NOT NULL DEFAULT '',
    after          TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_permission_audit_target ON permission_audit (target_user_id);
CREATE INDEX IF NOT EXISTS idx_permission_audit_created ON permission_audit (created_at);

-- Marks that the default role templates were seeded once. Without this marker a
-- restart would re-add permissions an admin had deliberately removed.
CREATE TABLE IF NOT EXISTS access_seed (
    id        INTEGER PRIMARY KEY CHECK (id = 1),
    seeded_at TEXT NOT NULL
);
