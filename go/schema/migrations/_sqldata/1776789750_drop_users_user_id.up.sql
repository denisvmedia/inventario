-- Drop the legacy self-referencing users.user_id column (issue #1289 Gap B).
--
-- users.user_id was introduced by the original multi-tenant migration when
-- TenantAwareEntityID had a `user_id` field embedded on every model. For the
-- users table that field ended up pointing at the user's own id — a self-FK
-- that never carried any semantic value. Spec #1219 §1 requires
-- TenantAwareEntityID to be {id, uuid, tenant_id} only; the refactor in the
-- same PR as this migration moves user_id declarations out of the shared
-- base struct and into the concrete types that actually need them
-- (refresh_tokens, settings, operation_slots, thumbnail_generation_jobs,
-- user_concurrency_slots). Users no longer has a user_id column.

ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_entity_user;
ALTER TABLE users DROP COLUMN IF EXISTS user_id;
