-- Migration generated from schema differences
-- Generated on: 2026-03-02T16:30:39+01:00
-- Direction: UP

-- Add/modify columns for table: user_concurrency_slots --
-- ALTER statements: --
ALTER TABLE user_concurrency_slots ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- ALTER statements: --
ALTER TABLE user_concurrency_slots ALTER COLUMN updated_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "user_concurrency_slots" WHERE "updated_at" IS NULL LIMIT 1) THEN
        UPDATE "user_concurrency_slots" SET "updated_at" = CURRENT_TIMESTAMP WHERE "updated_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE user_concurrency_slots ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE user_concurrency_slots ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column user_concurrency_slots.updated_at: default_expr:  -> CURRENT_TIMESTAMP --
-- ALTER statements: --
ALTER TABLE user_concurrency_slots ALTER COLUMN created_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "user_concurrency_slots" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "user_concurrency_slots" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE user_concurrency_slots ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE user_concurrency_slots ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column user_concurrency_slots.created_at: default_expr:  -> CURRENT_TIMESTAMP --
-- Add/modify columns for table: tenants --
-- ALTER statements: --
ALTER TABLE tenants ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- ALTER statements: --
ALTER TABLE tenants ALTER COLUMN created_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "tenants" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "tenants" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE tenants ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE tenants ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column tenants.created_at: default_expr:  -> CURRENT_TIMESTAMP --
-- ALTER statements: --
ALTER TABLE tenants ALTER COLUMN updated_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "tenants" WHERE "updated_at" IS NULL LIMIT 1) THEN
        UPDATE "tenants" SET "updated_at" = CURRENT_TIMESTAMP WHERE "updated_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE tenants ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE tenants ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column tenants.updated_at: default_expr:  -> CURRENT_TIMESTAMP --
-- Add/modify columns for table: operation_slots --
-- ALTER statements: --
ALTER TABLE operation_slots ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- Add/modify columns for table: email_verifications --
-- ALTER statements: --
ALTER TABLE email_verifications ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- ALTER statements: --
ALTER TABLE email_verifications ALTER COLUMN created_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "email_verifications" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "email_verifications" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE email_verifications ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE email_verifications ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column email_verifications.created_at: default_expr:  -> CURRENT_TIMESTAMP --
-- Add/modify columns for table: files --
-- ALTER statements: --
ALTER TABLE files ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- ALTER statements: --
ALTER TABLE files ALTER COLUMN updated_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "files" WHERE "updated_at" IS NULL LIMIT 1) THEN
        UPDATE "files" SET "updated_at" = CURRENT_TIMESTAMP WHERE "updated_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE files ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE files ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column files.updated_at: default_expr:  -> CURRENT_TIMESTAMP --
-- ALTER statements: --
ALTER TABLE files ALTER COLUMN created_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "files" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "files" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE files ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE files ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column files.created_at: default_expr:  -> CURRENT_TIMESTAMP --
-- Add/modify columns for table: exports --
-- ALTER statements: --
ALTER TABLE exports ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- ALTER statements: --
ALTER TABLE exports ALTER COLUMN created_date TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "exports" WHERE "created_date" IS NULL LIMIT 1) THEN
        UPDATE "exports" SET "created_date" = CURRENT_TIMESTAMP WHERE "created_date" IS NULL;
    END IF;
END
$$;
ALTER TABLE exports ALTER COLUMN created_date SET NOT NULL;
ALTER TABLE exports ALTER COLUMN created_date SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column exports.created_date: default_expr:  -> CURRENT_TIMESTAMP --
-- Add/modify columns for table: manuals --
-- ALTER statements: --
ALTER TABLE manuals ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- Add/modify columns for table: users --
-- ALTER statements: --
ALTER TABLE users ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- ALTER statements: --
ALTER TABLE users ALTER COLUMN created_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "users" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "users" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE users ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE users ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column users.created_at: default_expr:  -> CURRENT_TIMESTAMP --
-- ALTER statements: --
ALTER TABLE users ALTER COLUMN updated_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "users" WHERE "updated_at" IS NULL LIMIT 1) THEN
        UPDATE "users" SET "updated_at" = CURRENT_TIMESTAMP WHERE "updated_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE users ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE users ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column users.updated_at: default_expr:  -> CURRENT_TIMESTAMP --
-- Add/modify columns for table: settings --
-- ALTER statements: --
ALTER TABLE settings ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- Add/modify columns for table: areas --
-- ALTER statements: --
ALTER TABLE areas ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- Add/modify columns for table: password_resets --
-- ALTER statements: --
ALTER TABLE password_resets ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- ALTER statements: --
ALTER TABLE password_resets ALTER COLUMN created_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "password_resets" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "password_resets" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE password_resets ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE password_resets ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column password_resets.created_at: default_expr:  -> CURRENT_TIMESTAMP --
-- Add/modify columns for table: refresh_tokens --
-- ALTER statements: --
ALTER TABLE refresh_tokens ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- ALTER statements: --
ALTER TABLE refresh_tokens ALTER COLUMN created_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "refresh_tokens" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "refresh_tokens" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE refresh_tokens ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE refresh_tokens ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column refresh_tokens.created_at: default_expr:  -> CURRENT_TIMESTAMP --
-- Add/modify columns for table: locations --
-- ALTER statements: --
ALTER TABLE locations ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- Add/modify columns for table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- ALTER statements: --
ALTER TABLE restore_operations ALTER COLUMN created_date TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "restore_operations" WHERE "created_date" IS NULL LIMIT 1) THEN
        UPDATE "restore_operations" SET "created_date" = CURRENT_TIMESTAMP WHERE "created_date" IS NULL;
    END IF;
END
$$;
ALTER TABLE restore_operations ALTER COLUMN created_date SET NOT NULL;
ALTER TABLE restore_operations ALTER COLUMN created_date SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column restore_operations.created_date: default_expr:  -> CURRENT_TIMESTAMP --
-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE commodities ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- Add/modify columns for table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- ALTER statements: --
ALTER TABLE restore_steps ALTER COLUMN created_date TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "restore_steps" WHERE "created_date" IS NULL LIMIT 1) THEN
        UPDATE "restore_steps" SET "created_date" = CURRENT_TIMESTAMP WHERE "created_date" IS NULL;
    END IF;
END
$$;
ALTER TABLE restore_steps ALTER COLUMN created_date SET NOT NULL;
ALTER TABLE restore_steps ALTER COLUMN created_date SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column restore_steps.created_date: default_expr:  -> CURRENT_TIMESTAMP --
-- ALTER statements: --
ALTER TABLE restore_steps ALTER COLUMN updated_date TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "restore_steps" WHERE "updated_date" IS NULL LIMIT 1) THEN
        UPDATE "restore_steps" SET "updated_date" = CURRENT_TIMESTAMP WHERE "updated_date" IS NULL;
    END IF;
END
$$;
ALTER TABLE restore_steps ALTER COLUMN updated_date SET NOT NULL;
ALTER TABLE restore_steps ALTER COLUMN updated_date SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column restore_steps.updated_date: default_expr:  -> CURRENT_TIMESTAMP --
-- Add/modify columns for table: invoices --
-- ALTER statements: --
ALTER TABLE invoices ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- Add/modify columns for table: audit_logs --
-- ALTER statements: --
ALTER TABLE audit_logs ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- ALTER statements: --
ALTER TABLE audit_logs ALTER COLUMN timestamp TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "audit_logs" WHERE "timestamp" IS NULL LIMIT 1) THEN
        UPDATE "audit_logs" SET "timestamp" = CURRENT_TIMESTAMP WHERE "timestamp" IS NULL;
    END IF;
END
$$;
ALTER TABLE audit_logs ALTER COLUMN timestamp SET NOT NULL;
ALTER TABLE audit_logs ALTER COLUMN timestamp SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column audit_logs.timestamp: default_expr:  -> CURRENT_TIMESTAMP --
-- Add/modify columns for table: thumbnail_generation_jobs --
-- ALTER statements: --
ALTER TABLE thumbnail_generation_jobs ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
-- ALTER statements: --
ALTER TABLE thumbnail_generation_jobs ALTER COLUMN created_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "thumbnail_generation_jobs" WHERE "created_at" IS NULL LIMIT 1) THEN
        UPDATE "thumbnail_generation_jobs" SET "created_at" = CURRENT_TIMESTAMP WHERE "created_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE thumbnail_generation_jobs ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE thumbnail_generation_jobs ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column thumbnail_generation_jobs.created_at: default_expr:  -> CURRENT_TIMESTAMP --
-- ALTER statements: --
ALTER TABLE thumbnail_generation_jobs ALTER COLUMN updated_at TYPE TIMESTAMP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "thumbnail_generation_jobs" WHERE "updated_at" IS NULL LIMIT 1) THEN
        UPDATE "thumbnail_generation_jobs" SET "updated_at" = CURRENT_TIMESTAMP WHERE "updated_at" IS NULL;
    END IF;
END
$$;
ALTER TABLE thumbnail_generation_jobs ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE thumbnail_generation_jobs ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
-- Modify column thumbnail_generation_jobs.updated_at: default_expr:  -> CURRENT_TIMESTAMP --
-- Add/modify columns for table: images --
-- ALTER statements: --
ALTER TABLE images ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_areas_uuid ON areas (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_audit_logs_uuid ON audit_logs (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_commodities_uuid ON commodities (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_verifications_uuid ON email_verifications (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_exports_uuid ON exports (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_files_uuid ON files (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_images_uuid ON images (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_invoices_uuid ON invoices (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_locations_uuid ON locations (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_manuals_uuid ON manuals (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_operation_slots_uuid ON operation_slots (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_password_resets_uuid ON password_resets (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_uuid ON refresh_tokens (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_restore_operations_uuid ON restore_operations (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_restore_steps_uuid ON restore_steps (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_settings_uuid ON settings (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_uuid ON tenants (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_thumbnail_jobs_uuid ON thumbnail_generation_jobs (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_uuid ON users (uuid);