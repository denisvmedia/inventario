-- Migration rollback
-- Generated on: 2026-09-15T16:01:40+02:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "idx_areas_tenant_group";
DROP INDEX IF EXISTS "idx_areas_tenant_id";
DROP INDEX IF EXISTS "idx_areas_tenant_location";
DROP INDEX IF EXISTS "idx_areas_uuid";
DROP INDEX IF EXISTS "audit_logs_action_idx";
DROP INDEX IF EXISTS "audit_logs_entity_idx";
DROP INDEX IF EXISTS "audit_logs_tenant_id_idx";
DROP INDEX IF EXISTS "audit_logs_timestamp_idx";
DROP INDEX IF EXISTS "audit_logs_user_id_idx";
DROP INDEX IF EXISTS "idx_audit_logs_uuid";
DROP INDEX IF EXISTS "idx_backoffice_refresh_tokens_expires_at";
DROP INDEX IF EXISTS "idx_backoffice_refresh_tokens_token_hash";
DROP INDEX IF EXISTS "idx_backoffice_refresh_tokens_user_id";
DROP INDEX IF EXISTS "idx_backoffice_refresh_tokens_uuid";
DROP INDEX IF EXISTS "idx_backoffice_user_mfa_secrets_user";
DROP INDEX IF EXISTS "idx_backoffice_user_mfa_secrets_uuid";
DROP INDEX IF EXISTS "idx_backoffice_users_active";
DROP INDEX IF EXISTS "idx_backoffice_users_email";
DROP INDEX IF EXISTS "idx_backoffice_users_uuid";
DROP INDEX IF EXISTS "commodities_active_idx";
DROP INDEX IF EXISTS "commodities_draft_idx";
DROP INDEX IF EXISTS "commodities_extra_serial_numbers_gin_idx";
DROP INDEX IF EXISTS "commodities_name_trgm_idx";
DROP INDEX IF EXISTS "commodities_part_numbers_gin_idx";
DROP INDEX IF EXISTS "commodities_short_name_trgm_idx";
DROP INDEX IF EXISTS "commodities_tags_gin_idx";
DROP INDEX IF EXISTS "commodities_urls_gin_idx";
DROP INDEX IF EXISTS "commodities_warranty_expires_at_idx";
DROP INDEX IF EXISTS "idx_commodities_tenant_area";
DROP INDEX IF EXISTS "idx_commodities_tenant_group";
DROP INDEX IF EXISTS "idx_commodities_tenant_id";
DROP INDEX IF EXISTS "idx_commodities_tenant_status";
DROP INDEX IF EXISTS "idx_commodities_uuid";
DROP INDEX IF EXISTS "commodity_events_kind_idx";
DROP INDEX IF EXISTS "commodity_events_lookup";
DROP INDEX IF EXISTS "idx_commodity_events_tenant_group";
DROP INDEX IF EXISTS "idx_commodity_events_tenant_id";
DROP INDEX IF EXISTS "idx_commodity_events_uuid";
DROP INDEX IF EXISTS "idx_commodity_loans_active";
DROP INDEX IF EXISTS "idx_commodity_loans_commodity";
DROP INDEX IF EXISTS "idx_commodity_loans_due";
DROP INDEX IF EXISTS "idx_commodity_loans_tenant_group";
DROP INDEX IF EXISTS "idx_commodity_loans_tenant_id";
DROP INDEX IF EXISTS "idx_commodity_loans_uuid";
DROP INDEX IF EXISTS "idx_commodity_scan_audits_tenant_created";
DROP INDEX IF EXISTS "idx_commodity_scan_audits_user_created";
DROP INDEX IF EXISTS "idx_commodity_scan_audits_uuid";
DROP INDEX IF EXISTS "idx_commodity_services_active";
DROP INDEX IF EXISTS "idx_commodity_services_commodity";
DROP INDEX IF EXISTS "idx_commodity_services_due";
DROP INDEX IF EXISTS "idx_commodity_services_tenant_group";
DROP INDEX IF EXISTS "idx_commodity_services_tenant_id";
DROP INDEX IF EXISTS "idx_commodity_services_uuid";
DROP INDEX IF EXISTS "idx_supply_links_commodity";
DROP INDEX IF EXISTS "idx_supply_links_tenant_group";
DROP INDEX IF EXISTS "idx_supply_links_tenant_id";
DROP INDEX IF EXISTS "idx_supply_links_uuid";
DROP INDEX IF EXISTS "idx_currency_migration_audit_commodity";
DROP INDEX IF EXISTS "idx_currency_migration_audit_migration";
DROP INDEX IF EXISTS "idx_currency_migration_audit_tenant_group";
DROP INDEX IF EXISTS "idx_currency_migration_audit_uuid";
DROP INDEX IF EXISTS "idx_currency_migrations_group_completed";
DROP INDEX IF EXISTS "idx_currency_migrations_group_in_flight";
DROP INDEX IF EXISTS "idx_currency_migrations_group_status";
DROP INDEX IF EXISTS "idx_currency_migrations_tenant_group";
DROP INDEX IF EXISTS "idx_currency_migrations_uuid";
DROP INDEX IF EXISTS "email_verifications_email_idx";
DROP INDEX IF EXISTS "email_verifications_token_idx";
DROP INDEX IF EXISTS "email_verifications_user_id_idx";
DROP INDEX IF EXISTS "idx_email_verifications_uuid";
DROP INDEX IF EXISTS "idx_exports_tenant_group";
DROP INDEX IF EXISTS "idx_exports_tenant_id";
DROP INDEX IF EXISTS "idx_exports_tenant_status";
DROP INDEX IF EXISTS "idx_exports_tenant_type";
DROP INDEX IF EXISTS "idx_exports_uuid";
DROP INDEX IF EXISTS "files_linked_entity_idx";
DROP INDEX IF EXISTS "files_linked_entity_meta_idx";
DROP INDEX IF EXISTS "files_original_path_idx";
DROP INDEX IF EXISTS "files_path_trgm_idx";
DROP INDEX IF EXISTS "files_tags_gin_idx";
DROP INDEX IF EXISTS "files_title_trgm_idx";
DROP INDEX IF EXISTS "files_type_created_idx";
DROP INDEX IF EXISTS "idx_files_tenant_group";
DROP INDEX IF EXISTS "idx_files_tenant_group_category";
DROP INDEX IF EXISTS "idx_files_tenant_id";
DROP INDEX IF EXISTS "idx_files_tenant_linked_entity";
DROP INDEX IF EXISTS "idx_files_tenant_type";
DROP INDEX IF EXISTS "idx_files_uuid";
DROP INDEX IF EXISTS "idx_group_invites_expires_at";
DROP INDEX IF EXISTS "idx_group_invites_group_id";
DROP INDEX IF EXISTS "idx_group_invites_invitee_email";
DROP INDEX IF EXISTS "idx_group_invites_tenant_id";
DROP INDEX IF EXISTS "idx_group_invites_token";
DROP INDEX IF EXISTS "idx_group_invites_uuid";
DROP INDEX IF EXISTS "idx_group_invites_audit_archived_at";
DROP INDEX IF EXISTS "idx_group_invites_audit_original_group_id";
DROP INDEX IF EXISTS "idx_group_invites_audit_tenant_id";
DROP INDEX IF EXISTS "idx_group_invites_audit_tenant_invite";
DROP INDEX IF EXISTS "idx_group_invites_audit_used_by";
DROP INDEX IF EXISTS "idx_group_invites_audit_uuid";
DROP INDEX IF EXISTS "idx_group_memberships_group_id";
DROP INDEX IF EXISTS "idx_group_memberships_member_user_id";
DROP INDEX IF EXISTS "idx_group_memberships_tenant_id";
DROP INDEX IF EXISTS "idx_group_memberships_unique";
DROP INDEX IF EXISTS "idx_group_memberships_uuid";
DROP INDEX IF EXISTS "idx_group_notification_prefs_tenant_id";
DROP INDEX IF EXISTS "idx_group_notification_prefs_unique";
DROP INDEX IF EXISTS "idx_group_notification_prefs_user_group";
DROP INDEX IF EXISTS "idx_group_notification_prefs_uuid";
DROP INDEX IF EXISTS "idx_location_groups_status";
DROP INDEX IF EXISTS "idx_location_groups_tenant_id";
DROP INDEX IF EXISTS "idx_location_groups_tenant_slug";
DROP INDEX IF EXISTS "idx_location_groups_uuid";
DROP INDEX IF EXISTS "idx_locations_tenant_group";
DROP INDEX IF EXISTS "idx_locations_tenant_id";
DROP INDEX IF EXISTS "idx_locations_uuid";
DROP INDEX IF EXISTS "idx_login_events_created_at";
DROP INDEX IF EXISTS "idx_login_events_tenant_id";
DROP INDEX IF EXISTS "idx_login_events_user_created_at";
DROP INDEX IF EXISTS "idx_login_events_uuid";
DROP INDEX IF EXISTS "idx_magic_link_tokens_uuid";
DROP INDEX IF EXISTS "magic_link_tokens_email_idx";
DROP INDEX IF EXISTS "magic_link_tokens_token_idx";
DROP INDEX IF EXISTS "magic_link_tokens_user_id_idx";
DROP INDEX IF EXISTS "idx_maintenance_reminders_group_id";
DROP INDEX IF EXISTS "idx_maintenance_reminders_schedule_threshold";
DROP INDEX IF EXISTS "idx_maintenance_reminders_tenant_id";
DROP INDEX IF EXISTS "idx_maintenance_schedules_commodity";
DROP INDEX IF EXISTS "idx_maintenance_schedules_enabled_due";
DROP INDEX IF EXISTS "idx_maintenance_schedules_group_due";
DROP INDEX IF EXISTS "idx_maintenance_schedules_tenant_group";
DROP INDEX IF EXISTS "idx_maintenance_schedules_tenant_id";
DROP INDEX IF EXISTS "idx_maintenance_schedules_uuid";
DROP INDEX IF EXISTS "idx_operation_slots_cleanup";
DROP INDEX IF EXISTS "idx_operation_slots_operation";
DROP INDEX IF EXISTS "idx_operation_slots_unique";
DROP INDEX IF EXISTS "idx_operation_slots_user_operation";
DROP INDEX IF EXISTS "idx_operation_slots_uuid";
DROP INDEX IF EXISTS "idx_password_resets_uuid";
DROP INDEX IF EXISTS "password_resets_email_idx";
DROP INDEX IF EXISTS "password_resets_token_idx";
DROP INDEX IF EXISTS "password_resets_user_id_idx";
DROP INDEX IF EXISTS "idx_refresh_tokens_expires_at";
DROP INDEX IF EXISTS "idx_refresh_tokens_token_hash";
DROP INDEX IF EXISTS "idx_refresh_tokens_user_id";
DROP INDEX IF EXISTS "idx_refresh_tokens_uuid";
DROP INDEX IF EXISTS "idx_restore_operations_tenant_export";
DROP INDEX IF EXISTS "idx_restore_operations_tenant_group";
DROP INDEX IF EXISTS "idx_restore_operations_tenant_id";
DROP INDEX IF EXISTS "idx_restore_operations_tenant_status";
DROP INDEX IF EXISTS "idx_restore_operations_uuid";
DROP INDEX IF EXISTS "idx_restore_steps_tenant_group";
DROP INDEX IF EXISTS "idx_restore_steps_tenant_id";
DROP INDEX IF EXISTS "idx_restore_steps_tenant_operation";
DROP INDEX IF EXISTS "idx_restore_steps_tenant_result";
DROP INDEX IF EXISTS "idx_restore_steps_uuid";
DROP INDEX IF EXISTS "idx_settings_tenant_id";
DROP INDEX IF EXISTS "idx_settings_tenant_user_name";
DROP INDEX IF EXISTS "idx_settings_user_id";
DROP INDEX IF EXISTS "idx_settings_uuid";
DROP INDEX IF EXISTS "settings_value_gin_idx";
DROP INDEX IF EXISTS "idx_storage_quota_reminders_group_threshold";
DROP INDEX IF EXISTS "idx_storage_quota_reminders_tenant_id";
DROP INDEX IF EXISTS "idx_system_admin_grants_uuid";
DROP INDEX IF EXISTS "system_admin_grants_user_id_idx";
DROP INDEX IF EXISTS "idx_tags_group_kind_slug";
DROP INDEX IF EXISTS "idx_tags_tenant_group";
DROP INDEX IF EXISTS "idx_tags_tenant_id";
DROP INDEX IF EXISTS "idx_tags_uuid";
DROP INDEX IF EXISTS "tags_label_trgm_idx";
DROP INDEX IF EXISTS "idx_tenants_plan_id";
DROP INDEX IF EXISTS "idx_tenants_uuid";
DROP INDEX IF EXISTS "tenants_domain_idx";
DROP INDEX IF EXISTS "tenants_single_default_idx";
DROP INDEX IF EXISTS "tenants_slug_idx";
DROP INDEX IF EXISTS "tenants_status_idx";
DROP INDEX IF EXISTS "idx_thumbnail_jobs_cleanup";
DROP INDEX IF EXISTS "idx_thumbnail_jobs_file_id";
DROP INDEX IF EXISTS "idx_thumbnail_jobs_status_created";
DROP INDEX IF EXISTS "idx_thumbnail_jobs_tenant_id";
DROP INDEX IF EXISTS "idx_thumbnail_jobs_user_status";
DROP INDEX IF EXISTS "idx_thumbnail_jobs_uuid";
DROP INDEX IF EXISTS "idx_user_concurrency_slots_job_id";
DROP INDEX IF EXISTS "idx_user_concurrency_slots_status";
DROP INDEX IF EXISTS "idx_user_concurrency_slots_tenant_id";
DROP INDEX IF EXISTS "idx_user_concurrency_slots_user_id";
DROP INDEX IF EXISTS "idx_user_concurrency_slots_user_status";
DROP INDEX IF EXISTS "idx_user_mfa_secrets_user";
DROP INDEX IF EXISTS "idx_user_mfa_secrets_uuid";
DROP INDEX IF EXISTS "idx_oauth_identities_provider_subject";
DROP INDEX IF EXISTS "idx_oauth_identities_tenant_id";
DROP INDEX IF EXISTS "idx_oauth_identities_tenant_user";
DROP INDEX IF EXISTS "idx_oauth_identities_tenant_user_provider";
DROP INDEX IF EXISTS "idx_oauth_identities_uuid";
DROP INDEX IF EXISTS "idx_users_uuid";
DROP INDEX IF EXISTS "users_active_idx";
DROP INDEX IF EXISTS "users_tenant_email_idx";
DROP INDEX IF EXISTS "users_tenant_idx";
DROP INDEX IF EXISTS "idx_warranty_reminders_commodity_threshold";
DROP INDEX IF EXISTS "idx_warranty_reminders_group_id";
DROP INDEX IF EXISTS "idx_warranty_reminders_tenant_id";
DROP INDEX IF EXISTS "idx_worker_control_uuid";
DROP INDEX IF EXISTS "worker_control_worker_type_idx";
-- Drop RLS policy area_background_worker_access from table areas
DROP POLICY IF EXISTS "area_background_worker_access" ON "areas";
-- Drop RLS policy area_isolation from table areas
DROP POLICY IF EXISTS "area_isolation" ON "areas";
-- Drop RLS policy commodity_background_worker_access from table commodities
DROP POLICY IF EXISTS "commodity_background_worker_access" ON "commodities";
-- Drop RLS policy commodity_isolation from table commodities
DROP POLICY IF EXISTS "commodity_isolation" ON "commodities";
-- Drop RLS policy commodity_event_background_worker_access from table commodity_events
DROP POLICY IF EXISTS "commodity_event_background_worker_access" ON "commodity_events";
-- Drop RLS policy commodity_event_isolation from table commodity_events
DROP POLICY IF EXISTS "commodity_event_isolation" ON "commodity_events";
-- Drop RLS policy commodity_loan_background_worker_access from table commodity_loans
DROP POLICY IF EXISTS "commodity_loan_background_worker_access" ON "commodity_loans";
-- Drop RLS policy commodity_loan_isolation from table commodity_loans
DROP POLICY IF EXISTS "commodity_loan_isolation" ON "commodity_loans";
-- Drop RLS policy commodity_scan_audit_background_worker_access from table commodity_scan_audits
DROP POLICY IF EXISTS "commodity_scan_audit_background_worker_access" ON "commodity_scan_audits";
-- Drop RLS policy commodity_scan_audit_isolation from table commodity_scan_audits
DROP POLICY IF EXISTS "commodity_scan_audit_isolation" ON "commodity_scan_audits";
-- Drop RLS policy commodity_service_background_worker_access from table commodity_services
DROP POLICY IF EXISTS "commodity_service_background_worker_access" ON "commodity_services";
-- Drop RLS policy commodity_service_isolation from table commodity_services
DROP POLICY IF EXISTS "commodity_service_isolation" ON "commodity_services";
-- Drop RLS policy supply_link_background_worker_access from table commodity_supply_links
DROP POLICY IF EXISTS "supply_link_background_worker_access" ON "commodity_supply_links";
-- Drop RLS policy supply_link_isolation from table commodity_supply_links
DROP POLICY IF EXISTS "supply_link_isolation" ON "commodity_supply_links";
-- Drop RLS policy currency_migration_audit_background_worker_access from table currency_migration_audit_rows
DROP POLICY IF EXISTS "currency_migration_audit_background_worker_access" ON "currency_migration_audit_rows";
-- Drop RLS policy currency_migration_audit_isolation from table currency_migration_audit_rows
DROP POLICY IF EXISTS "currency_migration_audit_isolation" ON "currency_migration_audit_rows";
-- Drop RLS policy currency_migration_background_worker_access from table currency_migrations
DROP POLICY IF EXISTS "currency_migration_background_worker_access" ON "currency_migrations";
-- Drop RLS policy currency_migration_isolation from table currency_migrations
DROP POLICY IF EXISTS "currency_migration_isolation" ON "currency_migrations";
-- Drop RLS policy export_background_worker_access from table exports
DROP POLICY IF EXISTS "export_background_worker_access" ON "exports";
-- Drop RLS policy export_isolation from table exports
DROP POLICY IF EXISTS "export_isolation" ON "exports";
-- Drop RLS policy file_background_worker_access from table files
DROP POLICY IF EXISTS "file_background_worker_access" ON "files";
-- Drop RLS policy file_isolation from table files
DROP POLICY IF EXISTS "file_isolation" ON "files";
-- Drop RLS policy group_invite_background_worker_access from table group_invites
DROP POLICY IF EXISTS "group_invite_background_worker_access" ON "group_invites";
-- Drop RLS policy group_invite_tenant_isolation from table group_invites
DROP POLICY IF EXISTS "group_invite_tenant_isolation" ON "group_invites";
-- Drop RLS policy group_invite_audit_background_worker_access from table group_invites_audit
DROP POLICY IF EXISTS "group_invite_audit_background_worker_access" ON "group_invites_audit";
-- Drop RLS policy group_invite_audit_tenant_isolation from table group_invites_audit
DROP POLICY IF EXISTS "group_invite_audit_tenant_isolation" ON "group_invites_audit";
-- Drop RLS policy group_membership_background_worker_access from table group_memberships
DROP POLICY IF EXISTS "group_membership_background_worker_access" ON "group_memberships";
-- Drop RLS policy group_membership_tenant_isolation from table group_memberships
DROP POLICY IF EXISTS "group_membership_tenant_isolation" ON "group_memberships";
-- Drop RLS policy group_notification_prefs_background_worker_access from table group_notification_prefs
DROP POLICY IF EXISTS "group_notification_prefs_background_worker_access" ON "group_notification_prefs";
-- Drop RLS policy group_notification_prefs_tenant_isolation from table group_notification_prefs
DROP POLICY IF EXISTS "group_notification_prefs_tenant_isolation" ON "group_notification_prefs";
-- Drop RLS policy location_group_background_worker_access from table location_groups
DROP POLICY IF EXISTS "location_group_background_worker_access" ON "location_groups";
-- Drop RLS policy location_group_tenant_isolation from table location_groups
DROP POLICY IF EXISTS "location_group_tenant_isolation" ON "location_groups";
-- Drop RLS policy location_background_worker_access from table locations
DROP POLICY IF EXISTS "location_background_worker_access" ON "locations";
-- Drop RLS policy location_isolation from table locations
DROP POLICY IF EXISTS "location_isolation" ON "locations";
-- Drop RLS policy login_event_background_worker_access from table login_events
DROP POLICY IF EXISTS "login_event_background_worker_access" ON "login_events";
-- Drop RLS policy login_event_tenant_isolation from table login_events
DROP POLICY IF EXISTS "login_event_tenant_isolation" ON "login_events";
-- Drop RLS policy maintenance_reminder_background_worker_access from table maintenance_reminders
DROP POLICY IF EXISTS "maintenance_reminder_background_worker_access" ON "maintenance_reminders";
-- Drop RLS policy maintenance_reminder_isolation from table maintenance_reminders
DROP POLICY IF EXISTS "maintenance_reminder_isolation" ON "maintenance_reminders";
-- Drop RLS policy maintenance_schedule_background_worker_access from table maintenance_schedules
DROP POLICY IF EXISTS "maintenance_schedule_background_worker_access" ON "maintenance_schedules";
-- Drop RLS policy maintenance_schedule_isolation from table maintenance_schedules
DROP POLICY IF EXISTS "maintenance_schedule_isolation" ON "maintenance_schedules";
-- Drop RLS policy refresh_token_background_worker_access from table refresh_tokens
DROP POLICY IF EXISTS "refresh_token_background_worker_access" ON "refresh_tokens";
-- Drop RLS policy refresh_token_isolation from table refresh_tokens
DROP POLICY IF EXISTS "refresh_token_isolation" ON "refresh_tokens";
-- Drop RLS policy restore_operation_background_worker_access from table restore_operations
DROP POLICY IF EXISTS "restore_operation_background_worker_access" ON "restore_operations";
-- Drop RLS policy restore_operation_isolation from table restore_operations
DROP POLICY IF EXISTS "restore_operation_isolation" ON "restore_operations";
-- Drop RLS policy restore_step_background_worker_access from table restore_steps
DROP POLICY IF EXISTS "restore_step_background_worker_access" ON "restore_steps";
-- Drop RLS policy restore_step_isolation from table restore_steps
DROP POLICY IF EXISTS "restore_step_isolation" ON "restore_steps";
-- Drop RLS policy setting_background_worker_access from table settings
DROP POLICY IF EXISTS "setting_background_worker_access" ON "settings";
-- Drop RLS policy setting_isolation from table settings
DROP POLICY IF EXISTS "setting_isolation" ON "settings";
-- Drop RLS policy storage_quota_reminder_background_worker_access from table storage_quota_reminders
DROP POLICY IF EXISTS "storage_quota_reminder_background_worker_access" ON "storage_quota_reminders";
-- Drop RLS policy storage_quota_reminder_isolation from table storage_quota_reminders
DROP POLICY IF EXISTS "storage_quota_reminder_isolation" ON "storage_quota_reminders";
-- Drop RLS policy tag_background_worker_access from table tags
DROP POLICY IF EXISTS "tag_background_worker_access" ON "tags";
-- Drop RLS policy tag_isolation from table tags
DROP POLICY IF EXISTS "tag_isolation" ON "tags";
-- Drop RLS policy thumbnail_generation_job_background_worker_access from table thumbnail_generation_jobs
DROP POLICY IF EXISTS "thumbnail_generation_job_background_worker_access" ON "thumbnail_generation_jobs";
-- Drop RLS policy thumbnail_generation_job_isolation from table thumbnail_generation_jobs
DROP POLICY IF EXISTS "thumbnail_generation_job_isolation" ON "thumbnail_generation_jobs";
-- Drop RLS policy user_concurrency_slot_background_worker_access from table user_concurrency_slots
DROP POLICY IF EXISTS "user_concurrency_slot_background_worker_access" ON "user_concurrency_slots";
-- Drop RLS policy user_concurrency_slot_isolation from table user_concurrency_slots
DROP POLICY IF EXISTS "user_concurrency_slot_isolation" ON "user_concurrency_slots";
-- Drop RLS policy user_mfa_background_worker_access from table user_mfa_secrets
DROP POLICY IF EXISTS "user_mfa_background_worker_access" ON "user_mfa_secrets";
-- Drop RLS policy user_mfa_isolation from table user_mfa_secrets
DROP POLICY IF EXISTS "user_mfa_isolation" ON "user_mfa_secrets";
-- Drop RLS policy oauth_identity_background_worker_access from table user_oauth_identities
DROP POLICY IF EXISTS "oauth_identity_background_worker_access" ON "user_oauth_identities";
-- Drop RLS policy oauth_identity_user_isolation from table user_oauth_identities
DROP POLICY IF EXISTS "oauth_identity_user_isolation" ON "user_oauth_identities";
-- Drop RLS policy user_background_worker_access from table users
DROP POLICY IF EXISTS "user_background_worker_access" ON "users";
-- Drop RLS policy user_isolation from table users
DROP POLICY IF EXISTS "user_isolation" ON "users";
-- Drop RLS policy warranty_reminder_background_worker_access from table warranty_reminders
DROP POLICY IF EXISTS "warranty_reminder_background_worker_access" ON "warranty_reminders";
-- Drop RLS policy warranty_reminder_isolation from table warranty_reminders
DROP POLICY IF EXISTS "warranty_reminder_isolation" ON "warranty_reminders";
-- ALTER statements: --
ALTER TABLE "areas" DROP CONSTRAINT IF EXISTS "fk_area_location";
-- ALTER statements: --
ALTER TABLE "backoffice_refresh_tokens" DROP CONSTRAINT IF EXISTS "fk_backoffice_refresh_token_user";
-- ALTER statements: --
ALTER TABLE "backoffice_user_mfa_secrets" DROP CONSTRAINT IF EXISTS "fk_backoffice_mfa_user";
-- ALTER statements: --
ALTER TABLE "commodities" DROP CONSTRAINT IF EXISTS "fk_commodity_area";
-- ALTER statements: --
ALTER TABLE "commodities" DROP CONSTRAINT IF EXISTS "fk_commodity_cover_file";
-- ALTER statements: --
ALTER TABLE "commodity_events" DROP CONSTRAINT IF EXISTS "fk_commodity_event_commodity";
-- ALTER statements: --
ALTER TABLE "commodity_loans" DROP CONSTRAINT IF EXISTS "fk_commodity_loan_commodity";
-- ALTER statements: --
ALTER TABLE "commodity_services" DROP CONSTRAINT IF EXISTS "fk_commodity_service_commodity";
-- ALTER statements: --
ALTER TABLE "currency_migration_audit_rows" DROP CONSTRAINT IF EXISTS "fk_currency_migration_audit_migration";
-- ALTER statements: --
ALTER TABLE "currency_migration_audit_rows" DROP CONSTRAINT IF EXISTS "fk_currency_migration_audit_commodity";
-- ALTER statements: --
ALTER TABLE "email_verifications" DROP CONSTRAINT IF EXISTS "fk_email_verification_user";
-- ALTER statements: --
ALTER TABLE "email_verifications" DROP CONSTRAINT IF EXISTS "fk_email_verification_tenant";
-- ALTER statements: --
ALTER TABLE "exports" DROP CONSTRAINT IF EXISTS "fk_export_file";
-- ALTER statements: --
ALTER TABLE "group_invites" DROP CONSTRAINT IF EXISTS "fk_invite_group";
-- ALTER statements: --
ALTER TABLE "group_invites" DROP CONSTRAINT IF EXISTS "fk_invite_created_by";
-- ALTER statements: --
ALTER TABLE "group_invites" DROP CONSTRAINT IF EXISTS "fk_invite_used_by";
-- ALTER statements: --
ALTER TABLE "group_invites_audit" DROP CONSTRAINT IF EXISTS "fk_invite_audit_created_by";
-- ALTER statements: --
ALTER TABLE "group_invites_audit" DROP CONSTRAINT IF EXISTS "fk_invite_audit_used_by";
-- ALTER statements: --
ALTER TABLE "group_memberships" DROP CONSTRAINT IF EXISTS "fk_membership_group";
-- ALTER statements: --
ALTER TABLE "group_memberships" DROP CONSTRAINT IF EXISTS "fk_membership_user";
-- ALTER statements: --
ALTER TABLE "group_notification_prefs" DROP CONSTRAINT IF EXISTS "fk_group_notif_pref_group";
-- ALTER statements: --
ALTER TABLE "group_notification_prefs" DROP CONSTRAINT IF EXISTS "fk_group_notif_pref_user";
-- ALTER statements: --
ALTER TABLE "location_groups" DROP CONSTRAINT IF EXISTS "fk_location_group_created_by";
-- ALTER statements: --
ALTER TABLE "location_groups" DROP CONSTRAINT IF EXISTS "fk_location_group_currency_migration";
-- ALTER statements: --
ALTER TABLE "login_events" DROP CONSTRAINT IF EXISTS "fk_login_event_user";
-- ALTER statements: --
ALTER TABLE "magic_link_tokens" DROP CONSTRAINT IF EXISTS "fk_magic_link_token_user";
-- ALTER statements: --
ALTER TABLE "magic_link_tokens" DROP CONSTRAINT IF EXISTS "fk_magic_link_token_tenant";
-- ALTER statements: --
ALTER TABLE "maintenance_reminders" DROP CONSTRAINT IF EXISTS "fk_maintenance_reminder_schedule";
-- ALTER statements: --
ALTER TABLE "maintenance_schedules" DROP CONSTRAINT IF EXISTS "fk_maintenance_schedule_commodity";
-- ALTER statements: --
ALTER TABLE "user_oauth_identities" DROP CONSTRAINT IF EXISTS "fk_oauth_identity_user";
-- ALTER statements: --
ALTER TABLE "password_resets" DROP CONSTRAINT IF EXISTS "fk_password_reset_user";
-- ALTER statements: --
ALTER TABLE "password_resets" DROP CONSTRAINT IF EXISTS "fk_password_reset_tenant";
-- ALTER statements: --
ALTER TABLE "restore_operations" DROP CONSTRAINT IF EXISTS "fk_restore_operation_export";
-- ALTER statements: --
ALTER TABLE "restore_steps" DROP CONSTRAINT IF EXISTS "fk_restore_step_operation";
-- ALTER statements: --
ALTER TABLE "commodity_supply_links" DROP CONSTRAINT IF EXISTS "fk_supply_link_commodity";
-- ALTER statements: --
ALTER TABLE "system_admin_grants" DROP CONSTRAINT IF EXISTS "fk_system_admin_grants_user";
-- ALTER statements: --
ALTER TABLE "system_admin_grants" DROP CONSTRAINT IF EXISTS "fk_system_admin_grants_granted_by";
-- ALTER statements: --
ALTER TABLE "thumbnail_generation_jobs" DROP CONSTRAINT IF EXISTS "fk_thumbnail_job_file";
-- ALTER statements: --
ALTER TABLE "user_concurrency_slots" DROP CONSTRAINT IF EXISTS "fk_concurrency_slot_job";
-- ALTER statements: --
ALTER TABLE "users" DROP CONSTRAINT IF EXISTS "fk_user_default_group";
-- ALTER statements: --
ALTER TABLE "warranty_reminders" DROP CONSTRAINT IF EXISTS "fk_warranty_reminder_commodity";
-- ALTER statements: --
ALTER TABLE "areas" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "areas" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "areas" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "commodities" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "commodities" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "commodities" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "commodity_events" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "commodity_events" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "commodity_events" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "commodity_loans" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "commodity_loans" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "commodity_loans" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "commodity_scan_audits" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "commodity_scan_audits" DROP CONSTRAINT IF EXISTS "fk_entity_user";
-- ALTER statements: --
ALTER TABLE "commodity_services" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "commodity_services" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "commodity_services" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "currency_migrations" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "currency_migrations" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "currency_migrations" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "currency_migration_audit_rows" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "currency_migration_audit_rows" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "currency_migration_audit_rows" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "exports" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "exports" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "exports" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "files" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "files" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "files" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "group_invites" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "group_invites_audit" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "group_memberships" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "group_notification_prefs" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "locations" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "locations" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "locations" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "location_groups" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "login_events" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "maintenance_reminders" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "maintenance_reminders" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "maintenance_reminders" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "maintenance_schedules" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "maintenance_schedules" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "maintenance_schedules" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "user_oauth_identities" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "operation_slots" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "operation_slots" DROP CONSTRAINT IF EXISTS "fk_entity_user";
-- ALTER statements: --
ALTER TABLE "refresh_tokens" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "refresh_tokens" DROP CONSTRAINT IF EXISTS "fk_entity_user";
-- ALTER statements: --
ALTER TABLE "restore_operations" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "restore_operations" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "restore_operations" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "restore_steps" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "restore_steps" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "restore_steps" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "settings" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "settings" DROP CONSTRAINT IF EXISTS "fk_entity_user";
-- ALTER statements: --
ALTER TABLE "storage_quota_reminders" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "storage_quota_reminders" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "storage_quota_reminders" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "commodity_supply_links" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "commodity_supply_links" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "commodity_supply_links" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "tags" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "tags" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "tags" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "thumbnail_generation_jobs" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "thumbnail_generation_jobs" DROP CONSTRAINT IF EXISTS "fk_entity_user";
-- ALTER statements: --
ALTER TABLE "users" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "user_concurrency_slots" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "user_concurrency_slots" DROP CONSTRAINT IF EXISTS "fk_entity_user";
-- ALTER statements: --
ALTER TABLE "user_mfa_secrets" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "user_mfa_secrets" DROP CONSTRAINT IF EXISTS "fk_entity_user";
-- ALTER statements: --
ALTER TABLE "warranty_reminders" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "warranty_reminders" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "warranty_reminders" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "commodity_events" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "commodity_loans" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "commodity_services" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "commodity_supply_links" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "currency_migration_audit_rows" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "maintenance_reminders" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "maintenance_schedules" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "warranty_reminders" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "commodities" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "areas" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "audit_logs" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "backoffice_refresh_tokens" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "backoffice_user_mfa_secrets" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "backoffice_users" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "commodity_scan_audits" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "restore_steps" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "restore_operations" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "exports" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "user_concurrency_slots" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "thumbnail_generation_jobs" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "files" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "group_invites" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "group_memberships" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "group_notification_prefs" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "locations" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "storage_quota_reminders" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "tags" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "email_verifications" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "group_invites_audit" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "login_events" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "magic_link_tokens" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "operation_slots" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "password_resets" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "refresh_tokens" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "settings" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "system_admin_grants" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "user_mfa_secrets" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "user_oauth_identities" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "users" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "location_groups" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "currency_migrations" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "tenants" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "worker_control" CASCADE;
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS "get_current_group_id";
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS "get_current_tenant_id";
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS "get_current_user_id";
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS "set_group_context"(group_id_param text);
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS "set_tenant_context"(tenant_id_param text);
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS "set_user_context"(user_id_param text);