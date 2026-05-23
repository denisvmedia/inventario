-- Migration rollback: drop backoffice_users (#1785)
-- Direction: DOWN

DROP INDEX IF EXISTS idx_backoffice_users_active;
DROP INDEX IF EXISTS idx_backoffice_users_email;
DROP INDEX IF EXISTS idx_backoffice_users_uuid;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS backoffice_users CASCADE;