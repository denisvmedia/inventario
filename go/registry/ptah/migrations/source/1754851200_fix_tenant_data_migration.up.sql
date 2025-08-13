-- Data migration to fix tenant_id NULL constraint violations
-- Generated manually on: 2025-08-10T18:40:00Z
-- Direction: UP
-- This migration assumes the schema migration 1754848127 has already been applied

-- Step 1: Insert default tenant for existing data (if not exists)
INSERT INTO tenants (id, name, slug, status, created_at, updated_at)
SELECT 'default-tenant-id', 'Default Tenant', 'default-tenant', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
    WHERE NOT EXISTS (SELECT 1 FROM tenants WHERE id = 'default-tenant-id');

-- Step 2: Update all existing records with NULL tenant_id to use the default tenant
UPDATE areas SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE commodities SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE exports SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE files SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE images SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE invoices SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE locations SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE manuals SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE restore_operations SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE restore_steps SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE users SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
