-- Migration generated from schema differences
-- Generated on: 2026-09-15T16:01:40+02:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Gets the current group ID from session for RLS policies
CREATE OR REPLACE FUNCTION "get_current_group_id"() RETURNS text AS $$
BEGIN RETURN current_setting('app.current_group_id', true); END;
$$
LANGUAGE plpgsql SECURITY INVOKER STABLE;
-- Gets the current tenant ID from session for RLS policies
CREATE OR REPLACE FUNCTION "get_current_tenant_id"() RETURNS text AS $$
BEGIN RETURN current_setting('app.current_tenant_id', true); END;
$$
LANGUAGE plpgsql SECURITY INVOKER STABLE;
-- Gets the current user ID from session for RLS policies
CREATE OR REPLACE FUNCTION "get_current_user_id"() RETURNS text AS $$
BEGIN RETURN current_setting('app.current_user_id', true); END;
$$
LANGUAGE plpgsql SECURITY INVOKER STABLE;
-- Sets the current group context for RLS policies (transaction-local)
CREATE OR REPLACE FUNCTION "set_group_context"(group_id_param text) RETURNS void AS $$
BEGIN PERFORM set_config('app.current_group_id', group_id_param, true); END;
$$
LANGUAGE plpgsql SECURITY INVOKER VOLATILE;
-- Sets the current tenant context for RLS policies (transaction-local)
CREATE OR REPLACE FUNCTION "set_tenant_context"(tenant_id_param text) RETURNS void AS $$
BEGIN PERFORM set_config('app.current_tenant_id', tenant_id_param, true); END;
$$
LANGUAGE plpgsql SECURITY INVOKER VOLATILE;
-- Sets the current user context for RLS policies (transaction-local)
CREATE OR REPLACE FUNCTION "set_user_context"(user_id_param text) RETURNS void AS $$
BEGIN PERFORM set_config('app.current_user_id', user_id_param, true); END;
$$
LANGUAGE plpgsql SECURITY INVOKER VOLATILE;
-- POSTGRES TABLE: audit_logs --
CREATE TABLE "audit_logs" (
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text,
  "timestamp" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "user_id" TEXT,
  "tenant_id" TEXT,
  "action" TEXT NOT NULL,
  "entity_type" TEXT,
  "entity_id" TEXT,
  "ip_address" TEXT,
  "user_agent" TEXT,
  "success" BOOLEAN NOT NULL DEFAULT true,
  "error_message" TEXT,
  "impersonated_by" TEXT
);
-- POSTGRES TABLE: backoffice_users --
CREATE TABLE "backoffice_users" (
  "email" TEXT NOT NULL,
  "name" TEXT NOT NULL,
  "password_hash" TEXT NOT NULL,
  "role" TEXT NOT NULL,
  "is_active" BOOLEAN NOT NULL DEFAULT true,
  "mfa_enforced" BOOLEAN NOT NULL DEFAULT true,
  "last_login_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: backoffice_refresh_tokens --
CREATE TABLE "backoffice_refresh_tokens" (
  "backoffice_user_id" TEXT NOT NULL,
  "token_hash" VARCHAR(128) NOT NULL,
  "expires_at" TIMESTAMP NOT NULL,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "last_used_at" TIMESTAMP,
  "ip_address" VARCHAR(45),
  "user_agent" TEXT,
  "revoked_at" TIMESTAMP,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: backoffice_user_mfa_secrets --
CREATE TABLE "backoffice_user_mfa_secrets" (
  "backoffice_user_id" TEXT NOT NULL,
  "secret_encrypted" TEXT NOT NULL,
  "enabled_at" TIMESTAMP,
  "backup_codes_hashed" JSONB NOT NULL DEFAULT '[]',
  "last_used_at" TIMESTAMP,
  "last_used_step" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: tenants --
CREATE TABLE "tenants" (
  "name" TEXT NOT NULL,
  "slug" TEXT UNIQUE NOT NULL,
  "domain" TEXT,
  "status" TEXT NOT NULL DEFAULT 'active',
  "is_default" BOOLEAN NOT NULL DEFAULT false,
  "registration_mode" TEXT NOT NULL DEFAULT 'closed',
  "settings" JSONB,
  "plan_id" TEXT NOT NULL DEFAULT 'unlimited',
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: worker_control --
CREATE TABLE "worker_control" (
  "worker_type" TEXT NOT NULL,
  "paused" BOOLEAN NOT NULL DEFAULT false,
  "paused_by" TEXT,
  "paused_at" TIMESTAMP,
  "reason" TEXT,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: areas --
CREATE TABLE "areas" (
  "name" TEXT NOT NULL,
  "location_id" TEXT NOT NULL,
  "icon" TEXT NOT NULL DEFAULT '',
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: commodities --
CREATE TABLE "commodities" (
  "name" TEXT NOT NULL,
  "short_name" TEXT,
  "type" TEXT NOT NULL,
  "area_id" TEXT,
  "count" INTEGER NOT NULL DEFAULT 1,
  "original_price" DECIMAL(15,2),
  "original_price_currency" TEXT,
  "converted_original_price" DECIMAL(15,2),
  "current_price" DECIMAL(15,2),
  "serial_number" TEXT,
  "extra_serial_numbers" JSONB,
  "part_numbers" JSONB,
  "tags" JSONB,
  "status" TEXT NOT NULL,
  "purchase_date" TEXT,
  "registered_date" TEXT,
  "last_modified_date" TEXT,
  "urls" JSONB,
  "comments" TEXT,
  "draft" BOOLEAN NOT NULL DEFAULT false,
  "cover_file_id" TEXT,
  "warranty_expires_at" TEXT,
  "warranty_notes" TEXT,
  "acquisition_price" DECIMAL(15,2),
  "acquisition_currency" TEXT,
  "status_date" TEXT,
  "status_note" TEXT,
  "sale_price" DECIMAL(15,2),
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: commodity_events --
CREATE TABLE "commodity_events" (
  "commodity_id" TEXT NOT NULL,
  "kind" TEXT NOT NULL,
  "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
  "before" JSONB,
  "after" JSONB,
  "note" TEXT,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: commodity_loans --
CREATE TABLE "commodity_loans" (
  "commodity_id" TEXT NOT NULL,
  "borrower_name" TEXT NOT NULL,
  "borrower_contact" TEXT,
  "borrower_note" TEXT,
  "lent_at" TEXT NOT NULL,
  "due_back_at" TEXT,
  "returned_at" TEXT,
  "reminder_sent_overdue" BOOLEAN NOT NULL DEFAULT false,
  "reminder_sent_due_soon" BOOLEAN NOT NULL DEFAULT false,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: commodity_scan_audits --
CREATE TABLE "commodity_scan_audits" (
  "provider" VARCHAR(32) NOT NULL,
  "model" VARCHAR(64) NOT NULL,
  "photo_count" SMALLINT NOT NULL,
  "total_photo_bytes" INTEGER NOT NULL,
  "status" VARCHAR(16) NOT NULL,
  "error_code" VARCHAR(64),
  "latency_ms" INTEGER NOT NULL,
  "tokens_used" INTEGER NOT NULL DEFAULT 0,
  "result_json" JSONB,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: commodity_services --
CREATE TABLE "commodity_services" (
  "commodity_id" TEXT NOT NULL,
  "provider_name" TEXT NOT NULL,
  "provider_contact" TEXT,
  "reason" TEXT,
  "sent_at" TEXT NOT NULL,
  "expected_return_at" TEXT,
  "returned_at" TEXT,
  "cost_amount" DECIMAL(14,2),
  "cost_currency" TEXT,
  "reminder_sent_overdue" BOOLEAN NOT NULL DEFAULT false,
  "reminder_sent_due_soon" BOOLEAN NOT NULL DEFAULT false,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: commodity_supply_links --
CREATE TABLE "commodity_supply_links" (
  "commodity_id" TEXT NOT NULL,
  "label" TEXT NOT NULL,
  "url" TEXT NOT NULL,
  "notes" TEXT,
  "sort_order" INTEGER NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: currency_migration_audit_rows --
CREATE TABLE "currency_migration_audit_rows" (
  "migration_id" TEXT NOT NULL,
  "commodity_id" TEXT,
  "original_price_before" DECIMAL(15,2),
  "original_price_after" DECIMAL(15,2),
  "original_currency_before" TEXT,
  "original_currency_after" TEXT,
  "converted_before" DECIMAL(15,2),
  "converted_after" DECIMAL(15,2),
  "current_before" DECIMAL(15,2),
  "current_after" DECIMAL(15,2),
  "acquisition_filled_in_this_run" BOOLEAN NOT NULL DEFAULT false,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: currency_migrations --
CREATE TABLE "currency_migrations" (
  "status" TEXT NOT NULL,
  "from_currency" TEXT NOT NULL,
  "to_currency" TEXT NOT NULL,
  "exchange_rate" DECIMAL(20,10) NOT NULL,
  "commodity_count" INTEGER NOT NULL DEFAULT 0,
  "total_before" DECIMAL(20,2),
  "total_after" DECIMAL(20,2),
  "preview_token" TEXT,
  "preview_expires_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "started_at" TIMESTAMP,
  "completed_at" TIMESTAMP,
  "error_message" TEXT,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: email_verifications --
CREATE TABLE "email_verifications" (
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text,
  "user_id" TEXT NOT NULL,
  "tenant_id" TEXT NOT NULL,
  "email" TEXT NOT NULL,
  "token" TEXT NOT NULL,
  "expires_at" TIMESTAMP NOT NULL,
  "verified_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- POSTGRES TABLE: exports --
CREATE TABLE "exports" (
  "type" TEXT NOT NULL,
  "status" TEXT NOT NULL,
  "include_file_data" BOOLEAN NOT NULL DEFAULT false,
  "selected_items" JSONB,
  "file_id" TEXT,
  "file_path" TEXT,
  "created_date" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "completed_date" TIMESTAMP,
  "deleted_at" TIMESTAMP,
  "error_message" TEXT,
  "description" TEXT,
  "imported" BOOLEAN NOT NULL DEFAULT false,
  "file_size" BIGINT DEFAULT 0,
  "location_count" INTEGER DEFAULT 0,
  "area_count" INTEGER DEFAULT 0,
  "commodity_count" INTEGER DEFAULT 0,
  "image_count" INTEGER DEFAULT 0,
  "invoice_count" INTEGER DEFAULT 0,
  "manual_count" INTEGER DEFAULT 0,
  "file_count" INTEGER DEFAULT 0,
  "binary_data_size" BIGINT DEFAULT 0,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: files --
CREATE TABLE "files" (
  "title" TEXT,
  "description" TEXT,
  "type" TEXT NOT NULL,
  "category" TEXT NOT NULL DEFAULT 'other',
  "tags" JSONB,
  "linked_entity_type" TEXT,
  "linked_entity_id" TEXT,
  "linked_entity_meta" TEXT,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text,
  "path" TEXT NOT NULL,
  "original_path" TEXT NOT NULL,
  "ext" TEXT NOT NULL,
  "mime_type" TEXT NOT NULL,
  "size_bytes" BIGINT NOT NULL DEFAULT 0
);
-- POSTGRES TABLE: group_invites --
CREATE TABLE "group_invites" (
  "group_id" TEXT NOT NULL,
  "token" TEXT NOT NULL,
  "created_by" TEXT NOT NULL,
  "expires_at" TIMESTAMP NOT NULL,
  "used_by" TEXT,
  "used_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "invitee_email" TEXT,
  "role" TEXT NOT NULL DEFAULT 'user',
  "tenant_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: group_invites_audit --
CREATE TABLE "group_invites_audit" (
  "original_invite_id" TEXT NOT NULL,
  "original_invite_uuid" TEXT NOT NULL,
  "original_group_id" TEXT NOT NULL,
  "original_group_slug" TEXT NOT NULL,
  "original_group_name" TEXT NOT NULL,
  "token" TEXT NOT NULL,
  "created_by" TEXT NOT NULL,
  "used_by" TEXT NOT NULL,
  "original_created_at" TIMESTAMP NOT NULL,
  "original_expires_at" TIMESTAMP NOT NULL,
  "used_at" TIMESTAMP NOT NULL,
  "archived_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: group_memberships --
CREATE TABLE "group_memberships" (
  "group_id" TEXT NOT NULL,
  "member_user_id" TEXT NOT NULL,
  "role" TEXT NOT NULL DEFAULT 'user',
  "joined_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: group_notification_prefs --
CREATE TABLE "group_notification_prefs" (
  "group_id" TEXT NOT NULL,
  "user_id" TEXT NOT NULL,
  "category" TEXT NOT NULL,
  "enabled" BOOLEAN NOT NULL,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: location_groups --
CREATE TABLE "location_groups" (
  "slug" TEXT NOT NULL,
  "name" TEXT NOT NULL,
  "icon" TEXT,
  "description" TEXT NOT NULL DEFAULT '',
  "status" TEXT NOT NULL DEFAULT 'active',
  "created_by" TEXT NOT NULL,
  "group_currency" TEXT NOT NULL DEFAULT 'USD',
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "currency_migration_id" TEXT,
  "tenant_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: locations --
CREATE TABLE "locations" (
  "name" TEXT NOT NULL,
  "address" TEXT NOT NULL,
  "icon" TEXT NOT NULL DEFAULT '',
  "description" TEXT NOT NULL DEFAULT '',
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: login_events --
CREATE TABLE "login_events" (
  "user_id" TEXT,
  "email" TEXT NOT NULL,
  "outcome" TEXT NOT NULL,
  "method" TEXT NOT NULL DEFAULT 'password',
  "ip_address" VARCHAR(64),
  "user_agent" TEXT,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: magic_link_tokens --
CREATE TABLE "magic_link_tokens" (
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text,
  "user_id" TEXT NOT NULL,
  "tenant_id" TEXT NOT NULL,
  "email" TEXT NOT NULL,
  "token" TEXT NOT NULL,
  "expires_at" TIMESTAMP NOT NULL,
  "claimed_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- POSTGRES TABLE: maintenance_reminders --
CREATE TABLE "maintenance_reminders" (
  "schedule_id" TEXT NOT NULL,
  "threshold_days" INTEGER NOT NULL,
  "sent_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: maintenance_schedules --
CREATE TABLE "maintenance_schedules" (
  "commodity_id" TEXT NOT NULL,
  "title" TEXT NOT NULL,
  "interval_days" INTEGER NOT NULL,
  "next_due_at" TEXT NOT NULL,
  "last_done_at" TEXT,
  "notes" TEXT,
  "enabled" BOOLEAN NOT NULL DEFAULT true,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: operation_slots --
CREATE TABLE "operation_slots" (
  "slot_id" INTEGER NOT NULL,
  "operation_name" TEXT NOT NULL DEFAULT 'upload',
  "created_at" TIMESTAMP NOT NULL,
  "expires_at" TIMESTAMP NOT NULL,
  "tenant_id" TEXT NOT NULL,
  "user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: password_resets --
CREATE TABLE "password_resets" (
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text,
  "user_id" TEXT NOT NULL,
  "tenant_id" TEXT NOT NULL,
  "email" TEXT NOT NULL,
  "token" TEXT NOT NULL,
  "expires_at" TIMESTAMP NOT NULL,
  "used_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- POSTGRES TABLE: refresh_tokens --
CREATE TABLE "refresh_tokens" (
  "token_hash" VARCHAR(128) NOT NULL,
  "expires_at" TIMESTAMP NOT NULL,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "last_used_at" TIMESTAMP,
  "ip_address" VARCHAR(45),
  "user_agent" TEXT,
  "revoked_at" TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: restore_operations --
CREATE TABLE "restore_operations" (
  "export_id" TEXT NOT NULL,
  "description" TEXT NOT NULL,
  "status" TEXT NOT NULL,
  "options" JSONB NOT NULL,
  "created_date" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "started_date" TIMESTAMP,
  "completed_date" TIMESTAMP,
  "error_message" TEXT,
  "location_count" INTEGER DEFAULT 0,
  "area_count" INTEGER DEFAULT 0,
  "commodity_count" INTEGER DEFAULT 0,
  "image_count" INTEGER DEFAULT 0,
  "invoice_count" INTEGER DEFAULT 0,
  "manual_count" INTEGER DEFAULT 0,
  "file_count" INTEGER DEFAULT 0,
  "binary_data_size" BIGINT DEFAULT 0,
  "error_count" INTEGER DEFAULT 0,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: restore_steps --
CREATE TABLE "restore_steps" (
  "restore_operation_id" TEXT NOT NULL,
  "name" TEXT NOT NULL,
  "result" TEXT NOT NULL,
  "duration" BIGINT,
  "reason" TEXT,
  "created_date" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_date" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: settings --
CREATE TABLE "settings" (
  "name" TEXT NOT NULL,
  "value" JSONB NOT NULL,
  "tenant_id" TEXT NOT NULL,
  "user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: storage_quota_reminders --
CREATE TABLE "storage_quota_reminders" (
  "threshold_percent" INTEGER NOT NULL,
  "sent_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: system_admin_grants --
CREATE TABLE "system_admin_grants" (
  "user_id" TEXT NOT NULL,
  "granted_by" TEXT,
  "granted_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: tags --
CREATE TABLE "tags" (
  "kind" TEXT NOT NULL DEFAULT 'commodity',
  "slug" TEXT NOT NULL,
  "label" TEXT NOT NULL,
  "color" TEXT NOT NULL DEFAULT 'muted',
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: thumbnail_generation_jobs --
CREATE TABLE "thumbnail_generation_jobs" (
  "file_id" TEXT NOT NULL,
  "status" TEXT NOT NULL DEFAULT 'pending',
  "attempt_count" INTEGER NOT NULL DEFAULT 0,
  "max_attempts" INTEGER NOT NULL DEFAULT 3,
  "error_message" TEXT,
  "processing_started_at" TIMESTAMP,
  "processing_completed_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: user_concurrency_slots --
CREATE TABLE "user_concurrency_slots" (
  "job_id" TEXT NOT NULL,
  "status" TEXT NOT NULL DEFAULT 'active',
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: user_mfa_secrets --
CREATE TABLE "user_mfa_secrets" (
  "secret_encrypted" TEXT NOT NULL,
  "enabled_at" TIMESTAMP,
  "backup_codes_hashed" JSONB NOT NULL DEFAULT '[]',
  "last_used_at" TIMESTAMP,
  "last_used_step" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: user_oauth_identities --
CREATE TABLE "user_oauth_identities" (
  "user_id" TEXT NOT NULL,
  "provider" TEXT NOT NULL,
  "provider_user_id" TEXT NOT NULL,
  "email" TEXT NOT NULL,
  "linked_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: users --
CREATE TABLE "users" (
  "email" TEXT NOT NULL,
  "password_hash" TEXT NOT NULL,
  "name" TEXT NOT NULL,
  "is_active" BOOLEAN NOT NULL DEFAULT true,
  "last_login_at" TIMESTAMP,
  "default_group_id" TEXT,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: warranty_reminders --
CREATE TABLE "warranty_reminders" (
  "commodity_id" TEXT NOT NULL,
  "threshold_days" INTEGER NOT NULL,
  "sent_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Enable RLS for areas table
ALTER TABLE "areas" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for commodities table
ALTER TABLE "commodities" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for commodity_events table
ALTER TABLE "commodity_events" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for commodity_loans table
ALTER TABLE "commodity_loans" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for commodity_scan_audits table
ALTER TABLE "commodity_scan_audits" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for commodity_services table
ALTER TABLE "commodity_services" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for commodity_supply_links table
ALTER TABLE "commodity_supply_links" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for currency_migration_audit_rows table
ALTER TABLE "currency_migration_audit_rows" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for currency_migrations table
ALTER TABLE "currency_migrations" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for exports table
ALTER TABLE "exports" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for files table
ALTER TABLE "files" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for group_invites table
ALTER TABLE "group_invites" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for group_invites_audit table
ALTER TABLE "group_invites_audit" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for group_memberships table
ALTER TABLE "group_memberships" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for group_notification_prefs table
ALTER TABLE "group_notification_prefs" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for location_groups table
ALTER TABLE "location_groups" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for locations table
ALTER TABLE "locations" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for login_events table
ALTER TABLE "login_events" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for maintenance_reminders table
ALTER TABLE "maintenance_reminders" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for maintenance_schedules table
ALTER TABLE "maintenance_schedules" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for refresh_tokens table
ALTER TABLE "refresh_tokens" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for restore_operations table
ALTER TABLE "restore_operations" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for restore_steps table
ALTER TABLE "restore_steps" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for settings table
ALTER TABLE "settings" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for storage_quota_reminders table
ALTER TABLE "storage_quota_reminders" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for tags table
ALTER TABLE "tags" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for thumbnail_generation_jobs table
ALTER TABLE "thumbnail_generation_jobs" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for user_concurrency_slots table
ALTER TABLE "user_concurrency_slots" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for user_mfa_secrets table
ALTER TABLE "user_mfa_secrets" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for user_oauth_identities table
ALTER TABLE "user_oauth_identities" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for users table
ALTER TABLE "users" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for warranty_reminders table
ALTER TABLE "warranty_reminders" ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all areas for processing
DROP POLICY IF EXISTS "area_background_worker_access" ON "areas";
CREATE POLICY "area_background_worker_access" ON "areas" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures areas can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "area_isolation" ON "areas";
CREATE POLICY "area_isolation" ON "areas" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all commodities for processing
DROP POLICY IF EXISTS "commodity_background_worker_access" ON "commodities";
CREATE POLICY "commodity_background_worker_access" ON "commodities" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures commodities can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "commodity_isolation" ON "commodities";
CREATE POLICY "commodity_isolation" ON "commodities" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all commodity events for processing
DROP POLICY IF EXISTS "commodity_event_background_worker_access" ON "commodity_events";
CREATE POLICY "commodity_event_background_worker_access" ON "commodity_events" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures commodity events can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "commodity_event_isolation" ON "commodity_events";
CREATE POLICY "commodity_event_isolation" ON "commodity_events" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all commodity loans for processing
DROP POLICY IF EXISTS "commodity_loan_background_worker_access" ON "commodity_loans";
CREATE POLICY "commodity_loan_background_worker_access" ON "commodity_loans" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures commodity loans can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "commodity_loan_isolation" ON "commodity_loans";
CREATE POLICY "commodity_loan_isolation" ON "commodity_loans" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all commodity scan audit rows for retention/analytics
DROP POLICY IF EXISTS "commodity_scan_audit_background_worker_access" ON "commodity_scan_audits";
CREATE POLICY "commodity_scan_audit_background_worker_access" ON "commodity_scan_audits" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures commodity scan audit rows can only be accessed and modified by the owning user within their tenant
DROP POLICY IF EXISTS "commodity_scan_audit_isolation" ON "commodity_scan_audits";
CREATE POLICY "commodity_scan_audit_isolation" ON "commodity_scan_audits" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
-- Allows background workers to access all commodity services for processing
DROP POLICY IF EXISTS "commodity_service_background_worker_access" ON "commodity_services";
CREATE POLICY "commodity_service_background_worker_access" ON "commodity_services" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures commodity services can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "commodity_service_isolation" ON "commodity_services";
CREATE POLICY "commodity_service_isolation" ON "commodity_services" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all supply links for processing
DROP POLICY IF EXISTS "supply_link_background_worker_access" ON "commodity_supply_links";
CREATE POLICY "supply_link_background_worker_access" ON "commodity_supply_links" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures supply links can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "supply_link_isolation" ON "commodity_supply_links";
CREATE POLICY "supply_link_isolation" ON "commodity_supply_links" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows the background worker to insert audit rows in TX2
DROP POLICY IF EXISTS "currency_migration_audit_background_worker_access" ON "currency_migration_audit_rows";
CREATE POLICY "currency_migration_audit_background_worker_access" ON "currency_migration_audit_rows" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures currency migration audit rows are isolated by tenant and group
DROP POLICY IF EXISTS "currency_migration_audit_isolation" ON "currency_migration_audit_rows";
CREATE POLICY "currency_migration_audit_isolation" ON "currency_migration_audit_rows" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to claim, advance, and recover currency migration rows
DROP POLICY IF EXISTS "currency_migration_background_worker_access" ON "currency_migrations";
CREATE POLICY "currency_migration_background_worker_access" ON "currency_migrations" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures currency migration rows can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "currency_migration_isolation" ON "currency_migrations";
CREATE POLICY "currency_migration_isolation" ON "currency_migrations" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all exports for processing
DROP POLICY IF EXISTS "export_background_worker_access" ON "exports";
CREATE POLICY "export_background_worker_access" ON "exports" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures exports can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "export_isolation" ON "exports";
CREATE POLICY "export_isolation" ON "exports" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all files for processing
DROP POLICY IF EXISTS "file_background_worker_access" ON "files";
CREATE POLICY "file_background_worker_access" ON "files" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures files can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "file_isolation" ON "files";
CREATE POLICY "file_isolation" ON "files" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all group invites for cleanup
DROP POLICY IF EXISTS "group_invite_background_worker_access" ON "group_invites";
CREATE POLICY "group_invite_background_worker_access" ON "group_invites" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures group invites are isolated by tenant
DROP POLICY IF EXISTS "group_invite_tenant_isolation" ON "group_invites";
CREATE POLICY "group_invite_tenant_isolation" ON "group_invites" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');
-- Allows background workers to insert audit rows during group purge
DROP POLICY IF EXISTS "group_invite_audit_background_worker_access" ON "group_invites_audit";
CREATE POLICY "group_invite_audit_background_worker_access" ON "group_invites_audit" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures group invite audit records are isolated by tenant
DROP POLICY IF EXISTS "group_invite_audit_tenant_isolation" ON "group_invites_audit";
CREATE POLICY "group_invite_audit_tenant_isolation" ON "group_invites_audit" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');
-- Allows background workers to access all group memberships for processing
DROP POLICY IF EXISTS "group_membership_background_worker_access" ON "group_memberships";
CREATE POLICY "group_membership_background_worker_access" ON "group_memberships" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures group memberships are isolated by tenant; user-level filtering happens in application logic
DROP POLICY IF EXISTS "group_membership_tenant_isolation" ON "group_memberships";
CREATE POLICY "group_membership_tenant_isolation" ON "group_memberships" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');
-- Allows background workers to read per-group prefs when deciding whether to enqueue a reminder
DROP POLICY IF EXISTS "group_notification_prefs_background_worker_access" ON "group_notification_prefs";
CREATE POLICY "group_notification_prefs_background_worker_access" ON "group_notification_prefs" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures per-group notification prefs are isolated by tenant; user-level filtering happens in application logic
DROP POLICY IF EXISTS "group_notification_prefs_tenant_isolation" ON "group_notification_prefs";
CREATE POLICY "group_notification_prefs_tenant_isolation" ON "group_notification_prefs" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');
-- Allows background workers to access all location groups for processing
DROP POLICY IF EXISTS "location_group_background_worker_access" ON "location_groups";
CREATE POLICY "location_group_background_worker_access" ON "location_groups" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures location groups are isolated by tenant; group-level access is enforced in application logic via memberships
DROP POLICY IF EXISTS "location_group_tenant_isolation" ON "location_groups";
CREATE POLICY "location_group_tenant_isolation" ON "location_groups" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');
-- Allows background workers to access all locations for processing
DROP POLICY IF EXISTS "location_background_worker_access" ON "locations";
CREATE POLICY "location_background_worker_access" ON "locations" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures locations can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "location_isolation" ON "locations";
CREATE POLICY "location_isolation" ON "locations" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows the retention worker and login flow to insert/sweep events outside any user context
DROP POLICY IF EXISTS "login_event_background_worker_access" ON "login_events";
CREATE POLICY "login_event_background_worker_access" ON "login_events" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Login events are tenant-isolated; per-user filtering happens in application logic so a user only sees their own attempts
DROP POLICY IF EXISTS "login_event_tenant_isolation" ON "login_events";
CREATE POLICY "login_event_tenant_isolation" ON "login_events" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');
-- Allows background workers to record reminder emissions across all groups
DROP POLICY IF EXISTS "maintenance_reminder_background_worker_access" ON "maintenance_reminders";
CREATE POLICY "maintenance_reminder_background_worker_access" ON "maintenance_reminders" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures maintenance reminders are accessible only by their tenant and group
DROP POLICY IF EXISTS "maintenance_reminder_isolation" ON "maintenance_reminders";
CREATE POLICY "maintenance_reminder_isolation" ON "maintenance_reminders" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all maintenance schedules for processing
DROP POLICY IF EXISTS "maintenance_schedule_background_worker_access" ON "maintenance_schedules";
CREATE POLICY "maintenance_schedule_background_worker_access" ON "maintenance_schedules" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures maintenance schedules can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "maintenance_schedule_isolation" ON "maintenance_schedules";
CREATE POLICY "maintenance_schedule_isolation" ON "maintenance_schedules" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all refresh tokens for cleanup
DROP POLICY IF EXISTS "refresh_token_background_worker_access" ON "refresh_tokens";
CREATE POLICY "refresh_token_background_worker_access" ON "refresh_tokens" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures refresh tokens can only be accessed and modified by the owning user within their tenant
DROP POLICY IF EXISTS "refresh_token_isolation" ON "refresh_tokens";
CREATE POLICY "refresh_token_isolation" ON "refresh_tokens" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
-- Allows background workers to access all restore operations for processing
DROP POLICY IF EXISTS "restore_operation_background_worker_access" ON "restore_operations";
CREATE POLICY "restore_operation_background_worker_access" ON "restore_operations" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures restore operations can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "restore_operation_isolation" ON "restore_operations";
CREATE POLICY "restore_operation_isolation" ON "restore_operations" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all restore steps for processing
DROP POLICY IF EXISTS "restore_step_background_worker_access" ON "restore_steps";
CREATE POLICY "restore_step_background_worker_access" ON "restore_steps" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures restore steps can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "restore_step_isolation" ON "restore_steps";
CREATE POLICY "restore_step_isolation" ON "restore_steps" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all settings for processing
DROP POLICY IF EXISTS "setting_background_worker_access" ON "settings";
CREATE POLICY "setting_background_worker_access" ON "settings" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures settings can only be accessed and modified by their tenant and user with required contexts
DROP POLICY IF EXISTS "setting_isolation" ON "settings";
CREATE POLICY "setting_isolation" ON "settings" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
-- Allows background workers to record reminder emissions across all groups
DROP POLICY IF EXISTS "storage_quota_reminder_background_worker_access" ON "storage_quota_reminders";
CREATE POLICY "storage_quota_reminder_background_worker_access" ON "storage_quota_reminders" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures storage quota reminders are accessible only by their tenant and group
DROP POLICY IF EXISTS "storage_quota_reminder_isolation" ON "storage_quota_reminders";
CREATE POLICY "storage_quota_reminder_isolation" ON "storage_quota_reminders" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all tags for processing
DROP POLICY IF EXISTS "tag_background_worker_access" ON "tags";
CREATE POLICY "tag_background_worker_access" ON "tags" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures tags can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "tag_isolation" ON "tags";
CREATE POLICY "tag_isolation" ON "tags" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all thumbnail generation jobs for processing
DROP POLICY IF EXISTS "thumbnail_generation_job_background_worker_access" ON "thumbnail_generation_jobs";
CREATE POLICY "thumbnail_generation_job_background_worker_access" ON "thumbnail_generation_jobs" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures thumbnail generation jobs can only be accessed and modified by their tenant and user with required contexts
DROP POLICY IF EXISTS "thumbnail_generation_job_isolation" ON "thumbnail_generation_jobs";
CREATE POLICY "thumbnail_generation_job_isolation" ON "thumbnail_generation_jobs" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
-- Allows background workers to access all user concurrency slots for coordination
DROP POLICY IF EXISTS "user_concurrency_slot_background_worker_access" ON "user_concurrency_slots";
CREATE POLICY "user_concurrency_slot_background_worker_access" ON "user_concurrency_slots" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures user concurrency slots can only be accessed and modified by their tenant and user with required contexts
DROP POLICY IF EXISTS "user_concurrency_slot_isolation" ON "user_concurrency_slots";
CREATE POLICY "user_concurrency_slot_isolation" ON "user_concurrency_slots" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
-- Allows the login flow + management endpoints to read the row before RLS context is established on the connection
DROP POLICY IF EXISTS "user_mfa_background_worker_access" ON "user_mfa_secrets";
CREATE POLICY "user_mfa_background_worker_access" ON "user_mfa_secrets" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures MFA secrets can only be accessed and modified by the owning user within their tenant
DROP POLICY IF EXISTS "user_mfa_isolation" ON "user_mfa_secrets";
CREATE POLICY "user_mfa_isolation" ON "user_mfa_secrets" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
-- OAuth callback is the only background-worker writer; runs before any user session exists and looks up identities by (provider, provider_user_id) — no scheduled job touches this table
DROP POLICY IF EXISTS "oauth_identity_background_worker_access" ON "user_oauth_identities";
CREATE POLICY "oauth_identity_background_worker_access" ON "user_oauth_identities" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Users can read and modify only their own OAuth identities
DROP POLICY IF EXISTS "oauth_identity_user_isolation" ON "user_oauth_identities";
CREATE POLICY "oauth_identity_user_isolation" ON "user_oauth_identities" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
-- Allows background workers to access all users for processing
DROP POLICY IF EXISTS "user_background_worker_access" ON "users";
CREATE POLICY "user_background_worker_access" ON "users" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures users can only access and modify their own data within their tenant with required contexts
DROP POLICY IF EXISTS "user_isolation" ON "users";
CREATE POLICY "user_isolation" ON "users" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
-- Allows background workers to record reminder emissions across all groups
DROP POLICY IF EXISTS "warranty_reminder_background_worker_access" ON "warranty_reminders";
CREATE POLICY "warranty_reminder_background_worker_access" ON "warranty_reminders" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures warranty reminders are accessible only by their tenant and group
DROP POLICY IF EXISTS "warranty_reminder_isolation" ON "warranty_reminders";
CREATE POLICY "warranty_reminder_isolation" ON "warranty_reminders" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS "idx_areas_tenant_group" ON "areas" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_areas_tenant_id" ON "areas" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_areas_tenant_location" ON "areas" ("tenant_id", "location_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_areas_uuid" ON "areas" ("uuid");
CREATE INDEX IF NOT EXISTS "audit_logs_action_idx" ON "audit_logs" ("action");
CREATE INDEX IF NOT EXISTS "audit_logs_entity_idx" ON "audit_logs" ("entity_type", "entity_id");
CREATE INDEX IF NOT EXISTS "audit_logs_tenant_id_idx" ON "audit_logs" ("tenant_id");
CREATE INDEX IF NOT EXISTS "audit_logs_timestamp_idx" ON "audit_logs" ("timestamp");
CREATE INDEX IF NOT EXISTS "audit_logs_user_id_idx" ON "audit_logs" ("user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_audit_logs_uuid" ON "audit_logs" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_backoffice_refresh_tokens_expires_at" ON "backoffice_refresh_tokens" ("expires_at");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_backoffice_refresh_tokens_token_hash" ON "backoffice_refresh_tokens" ("token_hash");
CREATE INDEX IF NOT EXISTS "idx_backoffice_refresh_tokens_user_id" ON "backoffice_refresh_tokens" ("backoffice_user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_backoffice_refresh_tokens_uuid" ON "backoffice_refresh_tokens" ("uuid");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_backoffice_user_mfa_secrets_user" ON "backoffice_user_mfa_secrets" ("backoffice_user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_backoffice_user_mfa_secrets_uuid" ON "backoffice_user_mfa_secrets" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_backoffice_users_active" ON "backoffice_users" ("is_active");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_backoffice_users_email" ON "backoffice_users" ("email");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_backoffice_users_uuid" ON "backoffice_users" ("uuid");
CREATE INDEX IF NOT EXISTS "commodities_active_idx" ON "commodities" ("status", "area_id") WHERE draft = false;
CREATE INDEX IF NOT EXISTS "commodities_draft_idx" ON "commodities" ("last_modified_date") WHERE draft = true;
CREATE INDEX IF NOT EXISTS "commodities_extra_serial_numbers_gin_idx" ON "commodities" USING GIN ("extra_serial_numbers");
CREATE INDEX IF NOT EXISTS "commodities_name_trgm_idx" ON "commodities" USING GIN ("name" gin_trgm_ops);
CREATE INDEX IF NOT EXISTS "commodities_part_numbers_gin_idx" ON "commodities" USING GIN ("part_numbers");
CREATE INDEX IF NOT EXISTS "commodities_short_name_trgm_idx" ON "commodities" USING GIN ("short_name" gin_trgm_ops);
CREATE INDEX IF NOT EXISTS "commodities_tags_gin_idx" ON "commodities" USING GIN ("tags");
CREATE INDEX IF NOT EXISTS "commodities_urls_gin_idx" ON "commodities" USING GIN ("urls");
CREATE INDEX IF NOT EXISTS "commodities_warranty_expires_at_idx" ON "commodities" ("warranty_expires_at") WHERE warranty_expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS "idx_commodities_tenant_area" ON "commodities" ("tenant_id", "area_id");
CREATE INDEX IF NOT EXISTS "idx_commodities_tenant_group" ON "commodities" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_commodities_tenant_id" ON "commodities" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_commodities_tenant_status" ON "commodities" ("tenant_id", "status");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_commodities_uuid" ON "commodities" ("uuid");
CREATE INDEX IF NOT EXISTS "commodity_events_kind_idx" ON "commodity_events" ("commodity_id", "kind");
CREATE INDEX IF NOT EXISTS "commodity_events_lookup" ON "commodity_events" ("group_id", "commodity_id", "occurred_at");
CREATE INDEX IF NOT EXISTS "idx_commodity_events_tenant_group" ON "commodity_events" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_commodity_events_tenant_id" ON "commodity_events" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_commodity_events_uuid" ON "commodity_events" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_commodity_loans_active" ON "commodity_loans" ("group_id", "due_back_at") WHERE returned_at IS NULL;
CREATE INDEX IF NOT EXISTS "idx_commodity_loans_commodity" ON "commodity_loans" ("commodity_id", "lent_at");
CREATE INDEX IF NOT EXISTS "idx_commodity_loans_due" ON "commodity_loans" ("due_back_at") WHERE returned_at IS NULL AND due_back_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS "idx_commodity_loans_tenant_group" ON "commodity_loans" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_commodity_loans_tenant_id" ON "commodity_loans" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_commodity_loans_uuid" ON "commodity_loans" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_commodity_scan_audits_tenant_created" ON "commodity_scan_audits" ("tenant_id", "created_at");
CREATE INDEX IF NOT EXISTS "idx_commodity_scan_audits_user_created" ON "commodity_scan_audits" ("user_id", "created_at");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_commodity_scan_audits_uuid" ON "commodity_scan_audits" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_commodity_services_active" ON "commodity_services" ("group_id", "expected_return_at") WHERE returned_at IS NULL;
CREATE INDEX IF NOT EXISTS "idx_commodity_services_commodity" ON "commodity_services" ("commodity_id", "sent_at");
CREATE INDEX IF NOT EXISTS "idx_commodity_services_due" ON "commodity_services" ("expected_return_at") WHERE returned_at IS NULL AND expected_return_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS "idx_commodity_services_tenant_group" ON "commodity_services" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_commodity_services_tenant_id" ON "commodity_services" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_commodity_services_uuid" ON "commodity_services" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_supply_links_commodity" ON "commodity_supply_links" ("commodity_id", "sort_order");
CREATE INDEX IF NOT EXISTS "idx_supply_links_tenant_group" ON "commodity_supply_links" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_supply_links_tenant_id" ON "commodity_supply_links" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_supply_links_uuid" ON "commodity_supply_links" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_currency_migration_audit_commodity" ON "currency_migration_audit_rows" ("commodity_id");
CREATE INDEX IF NOT EXISTS "idx_currency_migration_audit_migration" ON "currency_migration_audit_rows" ("migration_id");
CREATE INDEX IF NOT EXISTS "idx_currency_migration_audit_tenant_group" ON "currency_migration_audit_rows" ("tenant_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_currency_migration_audit_uuid" ON "currency_migration_audit_rows" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_currency_migrations_group_completed" ON "currency_migrations" ("group_id", "completed_at") WHERE status = 'completed';
CREATE UNIQUE INDEX IF NOT EXISTS "idx_currency_migrations_group_in_flight" ON "currency_migrations" ("group_id") WHERE status IN ('pending', 'running');
CREATE INDEX IF NOT EXISTS "idx_currency_migrations_group_status" ON "currency_migrations" ("group_id", "status");
CREATE INDEX IF NOT EXISTS "idx_currency_migrations_tenant_group" ON "currency_migrations" ("tenant_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_currency_migrations_uuid" ON "currency_migrations" ("uuid");
CREATE INDEX IF NOT EXISTS "email_verifications_email_idx" ON "email_verifications" ("email");
CREATE UNIQUE INDEX IF NOT EXISTS "email_verifications_token_idx" ON "email_verifications" ("token");
CREATE INDEX IF NOT EXISTS "email_verifications_user_id_idx" ON "email_verifications" ("user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_email_verifications_uuid" ON "email_verifications" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_exports_tenant_group" ON "exports" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_exports_tenant_id" ON "exports" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_exports_tenant_status" ON "exports" ("tenant_id", "status");
CREATE INDEX IF NOT EXISTS "idx_exports_tenant_type" ON "exports" ("tenant_id", "type");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_exports_uuid" ON "exports" ("uuid");
CREATE INDEX IF NOT EXISTS "files_linked_entity_idx" ON "files" ("linked_entity_type", "linked_entity_id");
CREATE INDEX IF NOT EXISTS "files_linked_entity_meta_idx" ON "files" ("linked_entity_type", "linked_entity_id", "linked_entity_meta");
CREATE INDEX IF NOT EXISTS "files_original_path_idx" ON "files" ("original_path");
CREATE INDEX IF NOT EXISTS "files_path_trgm_idx" ON "files" USING GIN ("path" gin_trgm_ops);
CREATE INDEX IF NOT EXISTS "files_tags_gin_idx" ON "files" USING GIN ("tags");
CREATE INDEX IF NOT EXISTS "files_title_trgm_idx" ON "files" USING GIN ("title" gin_trgm_ops);
CREATE INDEX IF NOT EXISTS "files_type_created_idx" ON "files" ("type", "created_at");
CREATE INDEX IF NOT EXISTS "idx_files_tenant_group" ON "files" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_files_tenant_group_category" ON "files" ("tenant_id", "group_id", "category");
CREATE INDEX IF NOT EXISTS "idx_files_tenant_id" ON "files" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_files_tenant_linked_entity" ON "files" ("tenant_id", "linked_entity_type", "linked_entity_id");
CREATE INDEX IF NOT EXISTS "idx_files_tenant_type" ON "files" ("tenant_id", "type");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_files_uuid" ON "files" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_group_invites_expires_at" ON "group_invites" ("expires_at");
CREATE INDEX IF NOT EXISTS "idx_group_invites_group_id" ON "group_invites" ("group_id");
CREATE INDEX IF NOT EXISTS "idx_group_invites_invitee_email" ON "group_invites" ("invitee_email") WHERE invitee_email IS NOT NULL;
CREATE INDEX IF NOT EXISTS "idx_group_invites_tenant_id" ON "group_invites" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_group_invites_token" ON "group_invites" ("token");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_group_invites_uuid" ON "group_invites" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_group_invites_audit_archived_at" ON "group_invites_audit" ("archived_at");
CREATE INDEX IF NOT EXISTS "idx_group_invites_audit_original_group_id" ON "group_invites_audit" ("original_group_id");
CREATE INDEX IF NOT EXISTS "idx_group_invites_audit_tenant_id" ON "group_invites_audit" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_group_invites_audit_tenant_invite" ON "group_invites_audit" ("tenant_id", "original_invite_id");
CREATE INDEX IF NOT EXISTS "idx_group_invites_audit_used_by" ON "group_invites_audit" ("used_by");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_group_invites_audit_uuid" ON "group_invites_audit" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_group_memberships_group_id" ON "group_memberships" ("group_id");
CREATE INDEX IF NOT EXISTS "idx_group_memberships_member_user_id" ON "group_memberships" ("member_user_id");
CREATE INDEX IF NOT EXISTS "idx_group_memberships_tenant_id" ON "group_memberships" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_group_memberships_unique" ON "group_memberships" ("tenant_id", "group_id", "member_user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_group_memberships_uuid" ON "group_memberships" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_group_notification_prefs_tenant_id" ON "group_notification_prefs" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_group_notification_prefs_unique" ON "group_notification_prefs" ("tenant_id", "group_id", "user_id", "category");
CREATE INDEX IF NOT EXISTS "idx_group_notification_prefs_user_group" ON "group_notification_prefs" ("user_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_group_notification_prefs_uuid" ON "group_notification_prefs" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_location_groups_status" ON "location_groups" ("status");
CREATE INDEX IF NOT EXISTS "idx_location_groups_tenant_id" ON "location_groups" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_location_groups_tenant_slug" ON "location_groups" ("tenant_id", "slug");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_location_groups_uuid" ON "location_groups" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_locations_tenant_group" ON "locations" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_locations_tenant_id" ON "locations" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_locations_uuid" ON "locations" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_login_events_created_at" ON "login_events" ("created_at");
CREATE INDEX IF NOT EXISTS "idx_login_events_tenant_id" ON "login_events" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_login_events_user_created_at" ON "login_events" ("user_id", "created_at");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_login_events_uuid" ON "login_events" ("uuid");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_magic_link_tokens_uuid" ON "magic_link_tokens" ("uuid");
CREATE INDEX IF NOT EXISTS "magic_link_tokens_email_idx" ON "magic_link_tokens" ("email");
CREATE UNIQUE INDEX IF NOT EXISTS "magic_link_tokens_token_idx" ON "magic_link_tokens" ("token");
CREATE INDEX IF NOT EXISTS "magic_link_tokens_user_id_idx" ON "magic_link_tokens" ("user_id");
CREATE INDEX IF NOT EXISTS "idx_maintenance_reminders_group_id" ON "maintenance_reminders" ("group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_maintenance_reminders_schedule_threshold" ON "maintenance_reminders" ("schedule_id", "threshold_days");
CREATE INDEX IF NOT EXISTS "idx_maintenance_reminders_tenant_id" ON "maintenance_reminders" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_maintenance_schedules_commodity" ON "maintenance_schedules" ("commodity_id", "next_due_at");
CREATE INDEX IF NOT EXISTS "idx_maintenance_schedules_enabled_due" ON "maintenance_schedules" ("next_due_at") WHERE enabled = true;
CREATE INDEX IF NOT EXISTS "idx_maintenance_schedules_group_due" ON "maintenance_schedules" ("group_id", "next_due_at");
CREATE INDEX IF NOT EXISTS "idx_maintenance_schedules_tenant_group" ON "maintenance_schedules" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_maintenance_schedules_tenant_id" ON "maintenance_schedules" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_maintenance_schedules_uuid" ON "maintenance_schedules" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_operation_slots_cleanup" ON "operation_slots" ("expires_at");
CREATE INDEX IF NOT EXISTS "idx_operation_slots_operation" ON "operation_slots" ("operation_name", "expires_at");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_operation_slots_unique" ON "operation_slots" ("tenant_id", "user_id", "operation_name", "slot_id");
CREATE INDEX IF NOT EXISTS "idx_operation_slots_user_operation" ON "operation_slots" ("tenant_id", "user_id", "operation_name", "expires_at");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_operation_slots_uuid" ON "operation_slots" ("uuid");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_password_resets_uuid" ON "password_resets" ("uuid");
CREATE INDEX IF NOT EXISTS "password_resets_email_idx" ON "password_resets" ("email");
CREATE UNIQUE INDEX IF NOT EXISTS "password_resets_token_idx" ON "password_resets" ("token");
CREATE INDEX IF NOT EXISTS "password_resets_user_id_idx" ON "password_resets" ("user_id");
CREATE INDEX IF NOT EXISTS "idx_refresh_tokens_expires_at" ON "refresh_tokens" ("expires_at");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_refresh_tokens_token_hash" ON "refresh_tokens" ("token_hash");
CREATE INDEX IF NOT EXISTS "idx_refresh_tokens_user_id" ON "refresh_tokens" ("user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_refresh_tokens_uuid" ON "refresh_tokens" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_restore_operations_tenant_export" ON "restore_operations" ("tenant_id", "export_id");
CREATE INDEX IF NOT EXISTS "idx_restore_operations_tenant_group" ON "restore_operations" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_restore_operations_tenant_id" ON "restore_operations" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_restore_operations_tenant_status" ON "restore_operations" ("tenant_id", "status");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_restore_operations_uuid" ON "restore_operations" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_restore_steps_tenant_group" ON "restore_steps" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_restore_steps_tenant_id" ON "restore_steps" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_restore_steps_tenant_operation" ON "restore_steps" ("tenant_id", "restore_operation_id");
CREATE INDEX IF NOT EXISTS "idx_restore_steps_tenant_result" ON "restore_steps" ("tenant_id", "result");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_restore_steps_uuid" ON "restore_steps" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_settings_tenant_id" ON "settings" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_settings_tenant_user_name" ON "settings" ("tenant_id", "user_id", "name");
CREATE INDEX IF NOT EXISTS "idx_settings_user_id" ON "settings" ("user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_settings_uuid" ON "settings" ("uuid");
CREATE INDEX IF NOT EXISTS "settings_value_gin_idx" ON "settings" USING GIN ("value");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_storage_quota_reminders_group_threshold" ON "storage_quota_reminders" ("group_id", "threshold_percent");
CREATE INDEX IF NOT EXISTS "idx_storage_quota_reminders_tenant_id" ON "storage_quota_reminders" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_system_admin_grants_uuid" ON "system_admin_grants" ("uuid");
CREATE UNIQUE INDEX IF NOT EXISTS "system_admin_grants_user_id_idx" ON "system_admin_grants" ("user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_tags_group_kind_slug" ON "tags" ("group_id", "kind", "slug");
CREATE INDEX IF NOT EXISTS "idx_tags_tenant_group" ON "tags" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_tags_tenant_id" ON "tags" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_tags_uuid" ON "tags" ("uuid");
CREATE INDEX IF NOT EXISTS "tags_label_trgm_idx" ON "tags" USING GIN ("label" gin_trgm_ops);
CREATE INDEX IF NOT EXISTS "idx_tenants_plan_id" ON "tenants" ("plan_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_tenants_uuid" ON "tenants" ("uuid");
CREATE INDEX IF NOT EXISTS "tenants_domain_idx" ON "tenants" ("domain");
CREATE UNIQUE INDEX IF NOT EXISTS "tenants_single_default_idx" ON "tenants" ("is_default") WHERE is_default = true;
CREATE UNIQUE INDEX IF NOT EXISTS "tenants_slug_idx" ON "tenants" ("slug");
CREATE INDEX IF NOT EXISTS "tenants_status_idx" ON "tenants" ("status");
CREATE INDEX IF NOT EXISTS "idx_thumbnail_jobs_cleanup" ON "thumbnail_generation_jobs" ("status", "processing_completed_at");
CREATE INDEX IF NOT EXISTS "idx_thumbnail_jobs_file_id" ON "thumbnail_generation_jobs" ("file_id");
CREATE INDEX IF NOT EXISTS "idx_thumbnail_jobs_status_created" ON "thumbnail_generation_jobs" ("status", "created_at");
CREATE INDEX IF NOT EXISTS "idx_thumbnail_jobs_tenant_id" ON "thumbnail_generation_jobs" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_thumbnail_jobs_user_status" ON "thumbnail_generation_jobs" ("user_id", "status");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_thumbnail_jobs_uuid" ON "thumbnail_generation_jobs" ("uuid");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_user_concurrency_slots_job_id" ON "user_concurrency_slots" ("job_id");
CREATE INDEX IF NOT EXISTS "idx_user_concurrency_slots_status" ON "user_concurrency_slots" ("status");
CREATE INDEX IF NOT EXISTS "idx_user_concurrency_slots_tenant_id" ON "user_concurrency_slots" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_user_concurrency_slots_user_id" ON "user_concurrency_slots" ("user_id");
CREATE INDEX IF NOT EXISTS "idx_user_concurrency_slots_user_status" ON "user_concurrency_slots" ("user_id", "status");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_user_mfa_secrets_user" ON "user_mfa_secrets" ("tenant_id", "user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_user_mfa_secrets_uuid" ON "user_mfa_secrets" ("uuid");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_oauth_identities_provider_subject" ON "user_oauth_identities" ("provider", "provider_user_id");
CREATE INDEX IF NOT EXISTS "idx_oauth_identities_tenant_id" ON "user_oauth_identities" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_oauth_identities_tenant_user" ON "user_oauth_identities" ("tenant_id", "user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_oauth_identities_tenant_user_provider" ON "user_oauth_identities" ("tenant_id", "user_id", "provider");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_oauth_identities_uuid" ON "user_oauth_identities" ("uuid");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_users_uuid" ON "users" ("uuid");
CREATE INDEX IF NOT EXISTS "users_active_idx" ON "users" ("is_active");
CREATE UNIQUE INDEX IF NOT EXISTS "users_tenant_email_idx" ON "users" ("tenant_id", "email");
CREATE INDEX IF NOT EXISTS "users_tenant_idx" ON "users" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_warranty_reminders_commodity_threshold" ON "warranty_reminders" ("commodity_id", "threshold_days");
CREATE INDEX IF NOT EXISTS "idx_warranty_reminders_group_id" ON "warranty_reminders" ("group_id");
CREATE INDEX IF NOT EXISTS "idx_warranty_reminders_tenant_id" ON "warranty_reminders" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_worker_control_uuid" ON "worker_control" ("uuid");
CREATE UNIQUE INDEX IF NOT EXISTS "worker_control_worker_type_idx" ON "worker_control" ("worker_type");
-- ALTER statements: --
ALTER TABLE "backoffice_refresh_tokens" ADD CONSTRAINT "fk_backoffice_refresh_token_user" FOREIGN KEY ("backoffice_user_id") REFERENCES "backoffice_users"("id");
-- ALTER statements: --
ALTER TABLE "backoffice_user_mfa_secrets" ADD CONSTRAINT "fk_backoffice_mfa_user" FOREIGN KEY ("backoffice_user_id") REFERENCES "backoffice_users"("id");
-- ALTER statements: --
ALTER TABLE "areas" ADD CONSTRAINT "fk_area_location" FOREIGN KEY ("location_id") REFERENCES "locations"("id");
-- ALTER statements: --
ALTER TABLE "areas" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "areas" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "areas" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "commodities" ADD CONSTRAINT "fk_commodity_area" FOREIGN KEY ("area_id") REFERENCES "areas"("id");
-- ALTER statements: --
ALTER TABLE "commodities" ADD CONSTRAINT "fk_commodity_cover_file" FOREIGN KEY ("cover_file_id") REFERENCES "files"("id") ON DELETE SET NULL;
-- ALTER statements: --
ALTER TABLE "commodities" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "commodities" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "commodities" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "commodity_events" ADD CONSTRAINT "fk_commodity_event_commodity" FOREIGN KEY ("commodity_id") REFERENCES "commodities"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "commodity_events" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "commodity_events" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "commodity_events" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "commodity_loans" ADD CONSTRAINT "fk_commodity_loan_commodity" FOREIGN KEY ("commodity_id") REFERENCES "commodities"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "commodity_loans" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "commodity_loans" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "commodity_loans" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "commodity_scan_audits" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "commodity_scan_audits" ADD CONSTRAINT "fk_entity_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "commodity_services" ADD CONSTRAINT "fk_commodity_service_commodity" FOREIGN KEY ("commodity_id") REFERENCES "commodities"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "commodity_services" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "commodity_services" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "commodity_services" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "commodity_supply_links" ADD CONSTRAINT "fk_supply_link_commodity" FOREIGN KEY ("commodity_id") REFERENCES "commodities"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "commodity_supply_links" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "commodity_supply_links" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "commodity_supply_links" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "currency_migration_audit_rows" ADD CONSTRAINT "fk_currency_migration_audit_migration" FOREIGN KEY ("migration_id") REFERENCES "currency_migrations"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "currency_migration_audit_rows" ADD CONSTRAINT "fk_currency_migration_audit_commodity" FOREIGN KEY ("commodity_id") REFERENCES "commodities"("id") ON DELETE SET NULL;
-- ALTER statements: --
ALTER TABLE "currency_migration_audit_rows" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "currency_migration_audit_rows" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "currency_migration_audit_rows" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "currency_migrations" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "currency_migrations" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "currency_migrations" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "email_verifications" ADD CONSTRAINT "fk_email_verification_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "email_verifications" ADD CONSTRAINT "fk_email_verification_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "exports" ADD CONSTRAINT "fk_export_file" FOREIGN KEY ("file_id") REFERENCES "files"("id") ON DELETE SET NULL;
-- ALTER statements: --
ALTER TABLE "exports" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "exports" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "exports" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "files" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "files" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "files" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "group_invites" ADD CONSTRAINT "fk_invite_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "group_invites" ADD CONSTRAINT "fk_invite_created_by" FOREIGN KEY ("created_by") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "group_invites" ADD CONSTRAINT "fk_invite_used_by" FOREIGN KEY ("used_by") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "group_invites" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "group_invites_audit" ADD CONSTRAINT "fk_invite_audit_created_by" FOREIGN KEY ("created_by") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "group_invites_audit" ADD CONSTRAINT "fk_invite_audit_used_by" FOREIGN KEY ("used_by") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "group_invites_audit" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "group_memberships" ADD CONSTRAINT "fk_membership_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "group_memberships" ADD CONSTRAINT "fk_membership_user" FOREIGN KEY ("member_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "group_memberships" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "group_notification_prefs" ADD CONSTRAINT "fk_group_notif_pref_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "group_notification_prefs" ADD CONSTRAINT "fk_group_notif_pref_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "group_notification_prefs" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "location_groups" ADD CONSTRAINT "fk_location_group_created_by" FOREIGN KEY ("created_by") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "location_groups" ADD CONSTRAINT "fk_location_group_currency_migration" FOREIGN KEY ("currency_migration_id") REFERENCES "currency_migrations"("id") ON DELETE SET NULL;
-- ALTER statements: --
ALTER TABLE "location_groups" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "locations" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "locations" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "locations" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "login_events" ADD CONSTRAINT "fk_login_event_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "login_events" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "magic_link_tokens" ADD CONSTRAINT "fk_magic_link_token_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "magic_link_tokens" ADD CONSTRAINT "fk_magic_link_token_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "maintenance_reminders" ADD CONSTRAINT "fk_maintenance_reminder_schedule" FOREIGN KEY ("schedule_id") REFERENCES "maintenance_schedules"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "maintenance_reminders" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "maintenance_reminders" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "maintenance_reminders" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "maintenance_schedules" ADD CONSTRAINT "fk_maintenance_schedule_commodity" FOREIGN KEY ("commodity_id") REFERENCES "commodities"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "maintenance_schedules" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "maintenance_schedules" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "maintenance_schedules" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "operation_slots" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "operation_slots" ADD CONSTRAINT "fk_entity_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "password_resets" ADD CONSTRAINT "fk_password_reset_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "password_resets" ADD CONSTRAINT "fk_password_reset_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "refresh_tokens" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "refresh_tokens" ADD CONSTRAINT "fk_entity_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "restore_operations" ADD CONSTRAINT "fk_restore_operation_export" FOREIGN KEY ("export_id") REFERENCES "exports"("id");
-- ALTER statements: --
ALTER TABLE "restore_operations" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "restore_operations" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "restore_operations" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "restore_steps" ADD CONSTRAINT "fk_restore_step_operation" FOREIGN KEY ("restore_operation_id") REFERENCES "restore_operations"("id");
-- ALTER statements: --
ALTER TABLE "restore_steps" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "restore_steps" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "restore_steps" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "settings" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "settings" ADD CONSTRAINT "fk_entity_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "storage_quota_reminders" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "storage_quota_reminders" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "storage_quota_reminders" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "system_admin_grants" ADD CONSTRAINT "fk_system_admin_grants_user" FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "system_admin_grants" ADD CONSTRAINT "fk_system_admin_grants_granted_by" FOREIGN KEY ("granted_by") REFERENCES "users"("id") ON DELETE SET NULL;
-- ALTER statements: --
ALTER TABLE "tags" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "tags" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "tags" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "thumbnail_generation_jobs" ADD CONSTRAINT "fk_thumbnail_job_file" FOREIGN KEY ("file_id") REFERENCES "files"("id");
-- ALTER statements: --
ALTER TABLE "thumbnail_generation_jobs" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "thumbnail_generation_jobs" ADD CONSTRAINT "fk_entity_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "user_concurrency_slots" ADD CONSTRAINT "fk_concurrency_slot_job" FOREIGN KEY ("job_id") REFERENCES "thumbnail_generation_jobs"("id");
-- ALTER statements: --
ALTER TABLE "user_concurrency_slots" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "user_concurrency_slots" ADD CONSTRAINT "fk_entity_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "user_mfa_secrets" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "user_mfa_secrets" ADD CONSTRAINT "fk_entity_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "user_oauth_identities" ADD CONSTRAINT "fk_oauth_identity_user" FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "user_oauth_identities" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "users" ADD CONSTRAINT "fk_user_default_group" FOREIGN KEY ("default_group_id") REFERENCES "location_groups"("id") ON DELETE SET NULL;
-- ALTER statements: --
ALTER TABLE "users" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "warranty_reminders" ADD CONSTRAINT "fk_warranty_reminder_commodity" FOREIGN KEY ("commodity_id") REFERENCES "commodities"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "warranty_reminders" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "warranty_reminders" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "warranty_reminders" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");