-- Migration generated from schema differences
-- Generated on: 2025-08-12T15:59:18Z
-- Direction: UP

-- ============================================================================
-- CUSTOM INJECTION START: Proper transition handling for user isolation
-- WARNING: If this migration is re-generated, this custom logic will be lost!
--
-- This custom logic is required because:
-- 1. The users table was created without an 'id' primary key in earlier migrations
-- 2. We're adding user_id columns with NOT NULL constraints to all tables
-- 3. This creates a chicken-and-egg problem: we need users to exist before we can
--    reference them, but we can't create users without the proper table structure
--
-- The solution is to:
-- 1. Fix the users table structure first
-- 2. Create a default user for existing data
-- 3. Add user_id columns as nullable first
-- 4. Update existing records to reference the default user
-- 5. Then add NOT NULL constraints and foreign keys
-- ============================================================================

-- Step 1: Create a default tenant if it doesn't exist
INSERT INTO tenants (id, name, slug, status, created_at, updated_at)
SELECT 'default-tenant-id', 'Default Tenant', 'default-tenant', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM tenants WHERE id = 'default-tenant-id');

-- Step 2: Fix users table - add missing id primary key
ALTER TABLE users ADD COLUMN id TEXT;
UPDATE users SET id = gen_random_uuid()::text WHERE id IS NULL;
ALTER TABLE users ALTER COLUMN id SET NOT NULL;
ALTER TABLE users ADD PRIMARY KEY (id);

-- Step 3: Add user_id column to users table (for self-reference)
ALTER TABLE users ADD COLUMN user_id TEXT;

-- Step 4: Create a default user for existing data
INSERT INTO users (id, email, password_hash, name, role, is_active, tenant_id, user_id, created_at, updated_at)
SELECT 'default-user-id', 'admin@example.com', '$2a$10$default.hash.for.migration.purposes.only', 'Default Admin', 'admin', true, 'default-tenant-id', 'default-user-id', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM users WHERE id = 'default-user-id');

-- Step 5: Update existing users to set user_id to their own ID (self-reference)
UPDATE users SET user_id = id WHERE user_id IS NULL;

-- Step 6: Add NOT NULL constraint to users.user_id
ALTER TABLE users ALTER COLUMN user_id SET NOT NULL;

-- Step 7: Add user_id columns as nullable first to other tables
ALTER TABLE areas ADD COLUMN user_id TEXT;
ALTER TABLE restore_operations ADD COLUMN user_id TEXT;
ALTER TABLE invoices ADD COLUMN user_id TEXT;
ALTER TABLE manuals ADD COLUMN user_id TEXT;
ALTER TABLE locations ADD COLUMN user_id TEXT;
ALTER TABLE files ADD COLUMN user_id TEXT;
ALTER TABLE exports ADD COLUMN user_id TEXT;
ALTER TABLE commodities ADD COLUMN user_id TEXT;
ALTER TABLE images ADD COLUMN user_id TEXT;
ALTER TABLE restore_steps ADD COLUMN user_id TEXT;

-- Step 8: Update existing records to reference the default user
UPDATE areas SET user_id = 'default-user-id' WHERE user_id IS NULL;
UPDATE restore_operations SET user_id = 'default-user-id' WHERE user_id IS NULL;
UPDATE invoices SET user_id = 'default-user-id' WHERE user_id IS NULL;
UPDATE manuals SET user_id = 'default-user-id' WHERE user_id IS NULL;
UPDATE locations SET user_id = 'default-user-id' WHERE user_id IS NULL;
UPDATE files SET user_id = 'default-user-id' WHERE user_id IS NULL;
UPDATE exports SET user_id = 'default-user-id' WHERE user_id IS NULL;
UPDATE commodities SET user_id = 'default-user-id' WHERE user_id IS NULL;
UPDATE images SET user_id = 'default-user-id' WHERE user_id IS NULL;
UPDATE restore_steps SET user_id = 'default-user-id' WHERE user_id IS NULL;

-- Step 9: Add NOT NULL constraints to other tables
ALTER TABLE areas ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE restore_operations ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE invoices ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE manuals ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE locations ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE files ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE exports ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE commodities ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE images ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE restore_steps ALTER COLUMN user_id SET NOT NULL;

-- Step 10: Add foreign key constraints
ALTER TABLE users ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE areas ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE restore_operations ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE invoices ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE manuals ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE locations ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE files ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE exports ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE commodities ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE images ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE restore_steps ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);

-- CUSTOM INJECTION END
-- ============================================================================
-- Ensures areas can only be accessed and modified by their user
DROP POLICY IF EXISTS area_user_isolation ON areas;
CREATE POLICY area_user_isolation ON areas FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures commodities can only be accessed and modified by their user
DROP POLICY IF EXISTS commodity_user_isolation ON commodities;
CREATE POLICY commodity_user_isolation ON commodities FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures exports can only be accessed and modified by their user
DROP POLICY IF EXISTS export_user_isolation ON exports;
CREATE POLICY export_user_isolation ON exports FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures files can only be accessed and modified by their user
DROP POLICY IF EXISTS file_user_isolation ON files;
CREATE POLICY file_user_isolation ON files FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures images can only be accessed and modified by their user
DROP POLICY IF EXISTS image_user_isolation ON images;
CREATE POLICY image_user_isolation ON images FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures invoices can only be accessed and modified by their user
DROP POLICY IF EXISTS invoice_user_isolation ON invoices;
CREATE POLICY invoice_user_isolation ON invoices FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures locations can only be accessed and modified by their user
DROP POLICY IF EXISTS location_user_isolation ON locations;
CREATE POLICY location_user_isolation ON locations FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures manuals can only be accessed and modified by their user
DROP POLICY IF EXISTS manual_user_isolation ON manuals;
CREATE POLICY manual_user_isolation ON manuals FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures restore operations can only be accessed and modified by their user
DROP POLICY IF EXISTS restore_operation_user_isolation ON restore_operations;
CREATE POLICY restore_operation_user_isolation ON restore_operations FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures restore steps can only be accessed and modified by their user
DROP POLICY IF EXISTS restore_step_user_isolation ON restore_steps;
CREATE POLICY restore_step_user_isolation ON restore_steps FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures users can only access and modify their own data
DROP POLICY IF EXISTS user_user_isolation ON users;
CREATE POLICY user_user_isolation ON users FOR ALL TO inventario_app
    USING (id = get_current_user_id())
    WITH CHECK (id = get_current_user_id());