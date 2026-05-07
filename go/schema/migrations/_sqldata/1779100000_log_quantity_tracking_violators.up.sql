-- Issue #1554: log (don't auto-strip) commodities that already violate
-- the new "no per-instance tracking on bundles" constraint. The write-time
-- guard (model + service layer) blocks every NEW write that would create
-- such a row, so legacy data is the only source of violations from now
-- on. We deliberately leave the existing rows alone — splitting a bundle
-- into per-unit rows is a user decision, not something a migration
-- should auto-perform.
--
-- This is a hand-written data migration (not Ptah-generated). It only
-- emits PostgreSQL NOTICEs — no DDL, no UPDATE — so it's safe to re-run.

DO $$
DECLARE
  warranty_count int;
  loan_count int;
  service_count int;
BEGIN
  SELECT COUNT(*) INTO warranty_count
  FROM commodities
  WHERE count > 1
    AND (warranty_expires_at IS NOT NULL OR coalesce(warranty_notes, '') <> '');
  IF warranty_count > 0 THEN
    RAISE NOTICE '#1554: % commodities have count > 1 with warranty fields set; clear warranty fields or split the row to enable warranty tracking', warranty_count;
  END IF;

  SELECT COUNT(*) INTO loan_count
  FROM commodity_loans l
  JOIN commodities c ON c.id = l.commodity_id
  WHERE c.count > 1;
  IF loan_count > 0 THEN
    RAISE NOTICE '#1554: % loan rows reference a commodity with count > 1; future StartLoan calls will be rejected for those rows', loan_count;
  END IF;

  SELECT COUNT(*) INTO service_count
  FROM commodity_services s
  JOIN commodities c ON c.id = s.commodity_id
  WHERE c.count > 1;
  IF service_count > 0 THEN
    RAISE NOTICE '#1554: % service rows reference a commodity with count > 1; future StartService calls will be rejected for those rows', service_count;
  END IF;
END $$;
