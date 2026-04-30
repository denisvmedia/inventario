package files_backfill_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/lib/pq"

	"github.com/denisvmedia/inventario/services/files_backfill"
)

// We exercise the backfill end-to-end against a real Postgres instance —
// the SQL is the whole behaviour, so a memory mock would only re-implement
// what we're testing. Skipping is deliberate when no DSN is available so
// the suite stays runnable on machines without a local Postgres.
func skipIfNoPostgreSQL(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("Skipping PostgreSQL tests: POSTGRES_TEST_DSN environment variable not set")
	}
	if _, err := url.Parse(dsn); err != nil {
		t.Fatalf("invalid POSTGRES_TEST_DSN: %v", err)
	}
	return dsn
}

func TestBackfill_HappyPath(t *testing.T) {
	c := qt.New(t)
	dsn := skipIfNoPostgreSQL(t)

	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()
	c.Assert(db.Ping(), qt.IsNil)

	ctx := context.Background()
	cleanup := seedLegacyFixtures(c, db)
	defer cleanup()

	mgr := files_backfill.NewManager(db)

	// Dry run: must report all pending and write nothing.
	plan, err := mgr.PreviewOnly(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(plan.DryRun, qt.IsTrue)
	c.Assert(rowsBySource(plan.Sources, "images").Pending, qt.Equals, 3)
	c.Assert(rowsBySource(plan.Sources, "invoices").Pending, qt.Equals, 2)
	c.Assert(rowsBySource(plan.Sources, "manuals").Pending, qt.Equals, 1)
	// Inserted reflects what the SQL would do; for a dry run the
	// transaction was rolled back, so the row counts in `files` for our
	// fixture UUIDs must be zero.
	c.Assert(legacyFilesCount(c, db), qt.Equals, 0)

	// Live run: every pending row should land in `files` and the
	// per-source counters must match what the dry run reported.
	plan, err = mgr.Apply(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(plan.DryRun, qt.IsFalse)
	c.Assert(plan.TotalInserted(), qt.Equals, 6)
	c.Assert(rowsBySource(plan.Sources, "images").Inserted, qt.Equals, 3)
	c.Assert(rowsBySource(plan.Sources, "invoices").Inserted, qt.Equals, 2)
	c.Assert(rowsBySource(plan.Sources, "manuals").Inserted, qt.Equals, 1)
	c.Assert(legacyFilesCount(c, db), qt.Equals, 6)

	// Idempotency: re-run produces zero new rows and reports every
	// legacy row as already migrated.
	plan, err = mgr.Apply(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(plan.TotalInserted(), qt.Equals, 0)
	c.Assert(plan.TotalPending(), qt.Equals, 0)
	c.Assert(legacyFilesCount(c, db), qt.Equals, 6)
}

func TestBackfill_PreservesLegacyTablesAndCategoryMapping(t *testing.T) {
	c := qt.New(t)
	dsn := skipIfNoPostgreSQL(t)

	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()
	c.Assert(db.Ping(), qt.IsNil)

	ctx := context.Background()
	cleanup := seedLegacyFixtures(c, db)
	defer cleanup()

	_, err = files_backfill.NewManager(db).Apply(ctx)
	c.Assert(err, qt.IsNil)

	// Legacy tables must remain populated — cutover (#1421) is the only
	// place that drops them. This is the contract for safe rollback.
	c.Assert(rowCount(c, db, "images"), qt.Equals, 3)
	c.Assert(rowCount(c, db, "invoices"), qt.Equals, 2)
	c.Assert(rowCount(c, db, "manuals"), qt.Equals, 1)

	// Each legacy row must produce exactly one files row whose
	// linked_entity + category match the bucket mapping in the issue.
	cases := []struct {
		legacyTable      string
		linkedEntityMeta string
		category         string
	}{
		{"images", "images", "photos"},
		{"invoices", "invoices", "invoices"},
		{"manuals", "manuals", "documents"},
	}
	for _, tc := range cases {
		var n int
		err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM files
			WHERE linked_entity_type = 'commodity'
			  AND linked_entity_meta = $1
			  AND category = $2`,
			tc.linkedEntityMeta, tc.category).Scan(&n)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, rowCount(c, db, tc.legacyTable),
			qt.Commentf("backfilled %s rows must match legacy count", tc.legacyTable))
	}
}

func rowsBySource(rows []files_backfill.SourceStats, source string) files_backfill.SourceStats {
	for _, r := range rows {
		if r.Source == source {
			return r
		}
	}
	return files_backfill.SourceStats{}
}

func rowCount(c *qt.C, db *sql.DB, table string) int {
	var n int
	err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n)
	c.Assert(err, qt.IsNil)
	return n
}

// legacyFilesCount counts files rows whose uuid is also present in any
// legacy table — i.e. the rows backfill is responsible for. This is
// stricter than rowCount("files") because the test DB may carry rows from
// other tests/seed flows.
func legacyFilesCount(c *qt.C, db *sql.DB) int {
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM files f
		WHERE EXISTS (SELECT 1 FROM images   WHERE uuid = f.uuid)
		   OR EXISTS (SELECT 1 FROM invoices WHERE uuid = f.uuid)
		   OR EXISTS (SELECT 1 FROM manuals  WHERE uuid = f.uuid)`).Scan(&n)
	c.Assert(err, qt.IsNil)
	return n
}
