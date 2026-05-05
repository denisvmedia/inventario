-- Migration rollback
-- Generated on: 2026-05-05T07:41:38+02:00
-- Direction: DOWN

DROP INDEX IF EXISTS idx_commodity_loans_active;
DROP INDEX IF EXISTS idx_commodity_loans_commodity;
DROP INDEX IF EXISTS idx_commodity_loans_due;
DROP INDEX IF EXISTS idx_commodity_loans_tenant_group;
DROP INDEX IF EXISTS idx_commodity_loans_tenant_id;
DROP INDEX IF EXISTS idx_commodity_loans_uuid;
-- Drop RLS policy commodity_loan_background_worker_access from table commodity_loans
DROP POLICY IF EXISTS commodity_loan_background_worker_access ON commodity_loans;
-- Drop RLS policy commodity_loan_isolation from table commodity_loans
DROP POLICY IF EXISTS commodity_loan_isolation ON commodity_loans;
-- NOTE: RLS policies were removed from table commodity_loans - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS commodity_loans CASCADE;