-- Migration generated from schema differences
-- Generated on: 2025-08-12T07:37:24Z
-- Direction: UP

-- Add/modify columns for table: areas --
-- ALTER statements: --
ALTER TABLE areas ADD COLUMN user_id TEXT NOT NULL;
-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE commodities ADD COLUMN user_id TEXT NOT NULL;
-- Add/modify columns for table: files --
-- ALTER statements: --
ALTER TABLE files ADD COLUMN user_id TEXT NOT NULL;
-- Add/modify columns for table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ADD COLUMN user_id TEXT NOT NULL;
-- Add/modify columns for table: exports --
-- ALTER statements: --
ALTER TABLE exports ADD COLUMN user_id TEXT NOT NULL;
-- Add/modify columns for table: images --
-- ALTER statements: --
ALTER TABLE images ADD COLUMN user_id TEXT NOT NULL;
-- Add/modify columns for table: invoices --
-- ALTER statements: --
ALTER TABLE invoices ADD COLUMN user_id TEXT NOT NULL;
-- Add/modify columns for table: locations --
-- ALTER statements: --
ALTER TABLE locations ADD COLUMN user_id TEXT NOT NULL;
-- Add/modify columns for table: manuals --
-- ALTER statements: --
ALTER TABLE manuals ADD COLUMN user_id TEXT NOT NULL;
-- Add/modify columns for table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps ADD COLUMN user_id TEXT NOT NULL;
-- Add/modify columns for table: users --
-- ALTER statements: --
ALTER TABLE users ADD COLUMN user_id TEXT NOT NULL;
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
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());