-- Migration rollback
-- Generated on: 2026-03-02T16:30:39+01:00
-- Direction: DOWN

-- Add/modify columns for table: user_concurrency_slots --
-- ALTER statements: --
ALTER TABLE user_concurrency_slots ALTER COLUMN updated_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "user_concurrency_slots" WHERE "updated_at" IS NULL LIMIT 1) THEN
        UPDATE "user_concurrency_slots" SET "updated_at" = CURRENT_TIMESTAMP WHERE "updated_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE user_concurrency_slots ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE user_concurrency_slots ALTER COLUMN updated_at DROP DEFAULT;
-- Modify column user_concurrency_slots.updated_at: default_expr: CURRENT_TIMESTAMP ->  --
-- ALTER statements: --
ALTER TABLE user_concurrency_slots ALTER COLUMN created_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "user_concurrency_slots" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "user_concurrency_slots" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE user_concurrency_slots ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE user_concurrency_slots ALTER COLUMN created_at DROP DEFAULT;
-- Modify column user_concurrency_slots.created_at: default_expr: CURRENT_TIMESTAMP ->  --
-- Add/modify columns for table: tenants --
-- ALTER statements: --
ALTER TABLE tenants ALTER COLUMN created_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "tenants" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "tenants" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE tenants ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE tenants ALTER COLUMN created_at DROP DEFAULT;
-- Modify column tenants.created_at: default_expr: CURRENT_TIMESTAMP ->  --
-- ALTER statements: --
ALTER TABLE tenants ALTER COLUMN updated_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "tenants" WHERE "updated_at" IS NULL LIMIT 1) THEN
        UPDATE "tenants" SET "updated_at" = CURRENT_TIMESTAMP WHERE "updated_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE tenants ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE tenants ALTER COLUMN updated_at DROP DEFAULT;
-- Modify column tenants.updated_at: default_expr: CURRENT_TIMESTAMP ->  --
-- Add/modify columns for table: email_verifications --
-- ALTER statements: --
ALTER TABLE email_verifications ALTER COLUMN created_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "email_verifications" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "email_verifications" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE email_verifications ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE email_verifications ALTER COLUMN created_at DROP DEFAULT;
-- Modify column email_verifications.created_at: default_expr: CURRENT_TIMESTAMP ->  --
-- Add/modify columns for table: files --
-- ALTER statements: --
ALTER TABLE files ALTER COLUMN updated_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "files" WHERE "updated_at" IS NULL LIMIT 1) THEN
        UPDATE "files" SET "updated_at" = CURRENT_TIMESTAMP WHERE "updated_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE files ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE files ALTER COLUMN updated_at DROP DEFAULT;
-- Modify column files.updated_at: default_expr: CURRENT_TIMESTAMP ->  --
-- ALTER statements: --
ALTER TABLE files ALTER COLUMN created_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "files" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "files" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE files ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE files ALTER COLUMN created_at DROP DEFAULT;
-- Modify column files.created_at: default_expr: CURRENT_TIMESTAMP ->  --
-- Add/modify columns for table: exports --
-- ALTER statements: --
ALTER TABLE exports ALTER COLUMN created_date TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "exports" WHERE "created_date" IS NULL LIMIT 1) THEN
        UPDATE "exports" SET "created_date" = CURRENT_TIMESTAMP WHERE "created_date" IS NULL;
    END IF;
END
$$;
ALTER TABLE exports ALTER COLUMN created_date SET NOT NULL;
ALTER TABLE exports ALTER COLUMN created_date DROP DEFAULT;
-- Modify column exports.created_date: default_expr: CURRENT_TIMESTAMP ->  --
-- Add/modify columns for table: users --
-- ALTER statements: --
ALTER TABLE users ALTER COLUMN created_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "users" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "users" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE users ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE users ALTER COLUMN created_at DROP DEFAULT;
-- Modify column users.created_at: default_expr: CURRENT_TIMESTAMP ->  --
-- ALTER statements: --
ALTER TABLE users ALTER COLUMN updated_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "users" WHERE "updated_at" IS NULL LIMIT 1) THEN
        UPDATE "users" SET "updated_at" = CURRENT_TIMESTAMP WHERE "updated_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE users ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE users ALTER COLUMN updated_at DROP DEFAULT;
-- Modify column users.updated_at: default_expr: CURRENT_TIMESTAMP ->  --
-- Add/modify columns for table: password_resets --
-- ALTER statements: --
ALTER TABLE password_resets ALTER COLUMN created_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "password_resets" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "password_resets" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE password_resets ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE password_resets ALTER COLUMN created_at DROP DEFAULT;
-- Modify column password_resets.created_at: default_expr: CURRENT_TIMESTAMP ->  --
-- Add/modify columns for table: refresh_tokens --
-- ALTER statements: --
ALTER TABLE refresh_tokens ALTER COLUMN created_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "refresh_tokens" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "refresh_tokens" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE refresh_tokens ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE refresh_tokens ALTER COLUMN created_at DROP DEFAULT;
-- Modify column refresh_tokens.created_at: default_expr: CURRENT_TIMESTAMP ->  --
-- Add/modify columns for table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ALTER COLUMN created_date TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "restore_operations" WHERE "created_date" IS NULL LIMIT 1) THEN
        UPDATE "restore_operations" SET "created_date" = CURRENT_TIMESTAMP WHERE "created_date" IS NULL;
    END IF;
END
$$;
ALTER TABLE restore_operations ALTER COLUMN created_date SET NOT NULL;
ALTER TABLE restore_operations ALTER COLUMN created_date DROP DEFAULT;
-- Modify column restore_operations.created_date: default_expr: CURRENT_TIMESTAMP ->  --
-- Add/modify columns for table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps ALTER COLUMN created_date TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "restore_steps" WHERE "created_date" IS NULL LIMIT 1) THEN
        UPDATE "restore_steps" SET "created_date" = CURRENT_TIMESTAMP WHERE "created_date" IS NULL;
    END IF;
END
$$;
ALTER TABLE restore_steps ALTER COLUMN created_date SET NOT NULL;
ALTER TABLE restore_steps ALTER COLUMN created_date DROP DEFAULT;
-- Modify column restore_steps.created_date: default_expr: CURRENT_TIMESTAMP ->  --
-- ALTER statements: --
ALTER TABLE restore_steps ALTER COLUMN updated_date TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "restore_steps" WHERE "updated_date" IS NULL LIMIT 1) THEN
        UPDATE "restore_steps" SET "updated_date" = CURRENT_TIMESTAMP WHERE "updated_date" IS NULL;
    END IF;
END
$$;
ALTER TABLE restore_steps ALTER COLUMN updated_date SET NOT NULL;
ALTER TABLE restore_steps ALTER COLUMN updated_date DROP DEFAULT;
-- Modify column restore_steps.updated_date: default_expr: CURRENT_TIMESTAMP ->  --
-- Add/modify columns for table: audit_logs --
-- ALTER statements: --
ALTER TABLE audit_logs ALTER COLUMN timestamp TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "audit_logs" WHERE "timestamp" IS NULL LIMIT 1) THEN
        UPDATE "audit_logs" SET "timestamp" = CURRENT_TIMESTAMP WHERE "timestamp" IS NULL;
    END IF;
END
$$;
ALTER TABLE audit_logs ALTER COLUMN timestamp SET NOT NULL;
ALTER TABLE audit_logs ALTER COLUMN timestamp DROP DEFAULT;
-- Modify column audit_logs.timestamp: default_expr: CURRENT_TIMESTAMP ->  --
-- Add/modify columns for table: thumbnail_generation_jobs --
-- ALTER statements: --
ALTER TABLE thumbnail_generation_jobs ALTER COLUMN created_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "thumbnail_generation_jobs" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "thumbnail_generation_jobs" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE thumbnail_generation_jobs ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE thumbnail_generation_jobs ALTER COLUMN created_at DROP DEFAULT;
-- Modify column thumbnail_generation_jobs.created_at: default_expr: CURRENT_TIMESTAMP ->  --
-- ALTER statements: --
ALTER TABLE thumbnail_generation_jobs ALTER COLUMN updated_at TYPE timestamp without time zone;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "thumbnail_generation_jobs" WHERE "updated_at" IS NULL LIMIT 1) THEN
        UPDATE "thumbnail_generation_jobs" SET "updated_at" = CURRENT_TIMESTAMP WHERE "updated_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE thumbnail_generation_jobs ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE thumbnail_generation_jobs ALTER COLUMN updated_at DROP DEFAULT;
-- Modify column thumbnail_generation_jobs.updated_at: default_expr: CURRENT_TIMESTAMP ->  --
DROP INDEX IF EXISTS idx_areas_uuid;
DROP INDEX IF EXISTS idx_audit_logs_uuid;
DROP INDEX IF EXISTS idx_commodities_uuid;
DROP INDEX IF EXISTS idx_email_verifications_uuid;
DROP INDEX IF EXISTS idx_exports_uuid;
DROP INDEX IF EXISTS idx_files_uuid;
DROP INDEX IF EXISTS idx_images_uuid;
DROP INDEX IF EXISTS idx_invoices_uuid;
DROP INDEX IF EXISTS idx_locations_uuid;
DROP INDEX IF EXISTS idx_manuals_uuid;
DROP INDEX IF EXISTS idx_operation_slots_uuid;
DROP INDEX IF EXISTS idx_password_resets_uuid;
DROP INDEX IF EXISTS idx_refresh_tokens_uuid;
DROP INDEX IF EXISTS idx_restore_operations_uuid;
DROP INDEX IF EXISTS idx_restore_steps_uuid;
DROP INDEX IF EXISTS idx_settings_uuid;
DROP INDEX IF EXISTS idx_tenants_uuid;
DROP INDEX IF EXISTS idx_thumbnail_jobs_uuid;
DROP INDEX IF EXISTS idx_users_uuid;
-- Remove columns from table: user_concurrency_slots --
-- ALTER statements: --
ALTER TABLE user_concurrency_slots DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column user_concurrency_slots.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: tenants --
-- ALTER statements: --
ALTER TABLE tenants DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column tenants.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: operation_slots --
-- ALTER statements: --
ALTER TABLE operation_slots DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column operation_slots.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: email_verifications --
-- ALTER statements: --
ALTER TABLE email_verifications DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column email_verifications.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: files --
-- ALTER statements: --
ALTER TABLE files DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column files.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: exports --
-- ALTER statements: --
ALTER TABLE exports DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column exports.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: manuals --
-- ALTER statements: --
ALTER TABLE manuals DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column manuals.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: users --
-- ALTER statements: --
ALTER TABLE users DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column users.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: settings --
-- ALTER statements: --
ALTER TABLE settings DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column settings.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: areas --
-- ALTER statements: --
ALTER TABLE areas DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column areas.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: password_resets --
-- ALTER statements: --
ALTER TABLE password_resets DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column password_resets.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: refresh_tokens --
-- ALTER statements: --
ALTER TABLE refresh_tokens DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column refresh_tokens.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: locations --
-- ALTER statements: --
ALTER TABLE locations DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column locations.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column restore_operations.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column commodities.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column restore_steps.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: invoices --
-- ALTER statements: --
ALTER TABLE invoices DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column invoices.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: audit_logs --
-- ALTER statements: --
ALTER TABLE audit_logs DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column audit_logs.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: thumbnail_generation_jobs --
-- ALTER statements: --
ALTER TABLE thumbnail_generation_jobs DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column thumbnail_generation_jobs.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: images --
-- ALTER statements: --
ALTER TABLE images DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column images.uuid with CASCADE - This will delete data and dependent objects! --;