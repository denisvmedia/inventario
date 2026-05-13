-- Migration: per-group notification preferences (issue #1648 — blocks
-- #1537 item 2). Each row is a single user's opt-in/out for a single
-- notification category in a single group: (user_id, group_id,
-- category) is the primary key for upserts.
-- Direction: UP
--
-- Read semantics:
--   - A row here OVERRIDES the user-global pref from #1373 (the
--     `notifications.<category>` keys on the settings table).
--   - No row → fall through to the user-global pref → in-code default.
--
-- Categories are free-form TEXT and validated in Go (`notifications.Category`)
-- so the enum can evolve without a schema migration. v1 surfaces two
-- categories on the FE (`warranty_expiry`, `weekly_digest`); the table
-- happily stores the rest of the catalogue as it expands.
--
-- enabled is plain BOOLEAN — explicit-true means "deliver", explicit-false
-- means "suppress". The presence of the row itself is what flips the
-- per-group override on; absence = fall through to user-global.

CREATE TABLE group_notification_prefs (
    id TEXT PRIMARY KEY,
    uuid TEXT NOT NULL,
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    group_id TEXT NOT NULL REFERENCES location_groups(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category TEXT NOT NULL,
    enabled BOOLEAN NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_group_notification_prefs_uuid
    ON group_notification_prefs(uuid);

-- The (user, group, category) triple is the upsert key. Without this
-- index a second toggle of the same category would race the first row
-- and could leave duplicates behind.
CREATE UNIQUE INDEX idx_group_notification_prefs_unique
    ON group_notification_prefs(tenant_id, group_id, user_id, category);

-- Read paths: the GET endpoint reads (user, group) → all rows; the
-- warranty worker reads (user, group, category) directly through the
-- unique index above. This composite is the cheap "all rows for a
-- user inside one group" lookup.
CREATE INDEX idx_group_notification_prefs_user_group
    ON group_notification_prefs(user_id, group_id);

CREATE INDEX idx_group_notification_prefs_tenant_id
    ON group_notification_prefs(tenant_id);

-- RLS: same pattern as group_memberships. Tenant-only isolation at the
-- DB layer; the application layer further filters by user_id when the
-- caller is a regular user (the GET endpoint passes the auth'd user
-- through). Background workers (warranty reminder sweep) bypass the
-- policy via the inventario_background_worker role.
ALTER TABLE group_notification_prefs ENABLE ROW LEVEL SECURITY;

CREATE POLICY group_notification_prefs_tenant_isolation
    ON group_notification_prefs FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id()
        AND get_current_tenant_id() IS NOT NULL
        AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id()
        AND get_current_tenant_id() IS NOT NULL
        AND get_current_tenant_id() != '');

CREATE POLICY group_notification_prefs_background_worker_access
    ON group_notification_prefs FOR ALL TO inventario_background_worker
    USING (true) WITH CHECK (true);
