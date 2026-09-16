-- Migration rollback
-- Generated on: 2026-09-15T22:29:28+02:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Drop RLS policy operation_slot_background_worker_access from table operation_slots
DROP POLICY IF EXISTS "operation_slot_background_worker_access" ON "operation_slots";
-- Drop RLS policy operation_slot_isolation from table operation_slots
DROP POLICY IF EXISTS "operation_slot_isolation" ON "operation_slots";
-- Disable RLS for operation_slots table
ALTER TABLE "operation_slots" DISABLE ROW LEVEL SECURITY;