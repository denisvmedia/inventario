-- Rollback for tenant data migration
-- Generated manually on: 2025-08-10T18:40:00Z
-- Direction: DOWN

-- WARNING: This rollback will set all tenant_id fields to NULL
-- This should only be used if you need to completely rollback the tenant data migration

-- Step 1: Set all tenant_id fields back to NULL (this will break NOT NULL constraints if they exist)
-- Note: This rollback assumes the schema migration 1754848127 will also be rolled back
UPDATE areas SET tenant_id = NULL WHERE tenant_id = 'default-tenant';
UPDATE commodities SET tenant_id = NULL WHERE tenant_id = 'default-tenant';
UPDATE exports SET tenant_id = NULL WHERE tenant_id = 'default-tenant';
UPDATE files SET tenant_id = NULL WHERE tenant_id = 'default-tenant';
UPDATE images SET tenant_id = NULL WHERE tenant_id = 'default-tenant';
UPDATE invoices SET tenant_id = NULL WHERE tenant_id = 'default-tenant';
UPDATE locations SET tenant_id = NULL WHERE tenant_id = 'default-tenant';
UPDATE manuals SET tenant_id = NULL WHERE tenant_id = 'default-tenant';
UPDATE restore_operations SET tenant_id = NULL WHERE tenant_id = 'default-tenant';
UPDATE restore_steps SET tenant_id = NULL WHERE tenant_id = 'default-tenant';
UPDATE users SET tenant_id = NULL WHERE tenant_id = 'default-tenant';

-- Step 2: Remove the default tenant (if no other records reference it)
DELETE FROM tenants WHERE id = 'default-tenant';
