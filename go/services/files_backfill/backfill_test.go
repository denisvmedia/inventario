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
//
// CI runs this package via `make test-go-postgres`, which is wired to
// include `./services/files_backfill/...` alongside `./registry/postgres/...`.
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
	fx := seedLegacyFixtures(c, db)
	defer fx.Cleanup()

	mgr := files_backfill.NewManager(db)

	// Dry run: must report all pending and write nothing. Inserted is
	// zeroed on dry-run so callers can't mistake "would insert" for
	// "did insert".
	plan, err := mgr.PreviewOnly(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(plan.DryRun, qt.IsTrue)
	c.Assert(rowsBySource(plan.Sources, "images").Pending, qt.Equals, 3)
	c.Assert(rowsBySource(plan.Sources, "invoices").Pending, qt.Equals, 2)
	c.Assert(rowsBySource(plan.Sources, "manuals").Pending, qt.Equals, 1)
	c.Assert(plan.TotalInserted(), qt.Equals, 0)
	// Transaction was rolled back, so the row counts in `files` for our
	// fixture UUIDs must be zero.
	c.Assert(legacyFilesCount(c, db, fx.TenantID), qt.Equals, 0)

	// Live run: every pending row should land in `files` and the
	// per-source counters must match what the dry run reported.
	plan, err = mgr.Apply(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(plan.DryRun, qt.IsFalse)
	c.Assert(plan.TotalInserted(), qt.Equals, 6)
	c.Assert(rowsBySource(plan.Sources, "images").Inserted, qt.Equals, 3)
	c.Assert(rowsBySource(plan.Sources, "invoices").Inserted, qt.Equals, 2)
	c.Assert(rowsBySource(plan.Sources, "manuals").Inserted, qt.Equals, 1)
	c.Assert(legacyFilesCount(c, db, fx.TenantID), qt.Equals, 6)

	// Idempotency: re-run produces zero new rows and reports every
	// legacy row as already migrated.
	plan, err = mgr.Apply(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(plan.TotalInserted(), qt.Equals, 0)
	c.Assert(plan.TotalPending(), qt.Equals, 0)
	c.Assert(legacyFilesCount(c, db, fx.TenantID), qt.Equals, 6)
}

func TestBackfill_PreservesLegacyTablesAndCategoryMapping(t *testing.T) {
	c := qt.New(t)
	dsn := skipIfNoPostgreSQL(t)

	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()
	c.Assert(db.Ping(), qt.IsNil)

	ctx := context.Background()
	fx := seedLegacyFixtures(c, db)
	defer fx.Cleanup()

	_, err = files_backfill.NewManager(db).Apply(ctx)
	c.Assert(err, qt.IsNil)

	// Legacy tables must remain populated — cutover (#1421) is the only
	// place that drops them. This is the contract for safe rollback.
	// Filtering by tenant_id keeps the assertion robust against rows
	// from other tests sharing this DB.
	c.Assert(tenantRowCount(c, db, "images", fx.TenantID), qt.Equals, 3)
	c.Assert(tenantRowCount(c, db, "invoices", fx.TenantID), qt.Equals, 2)
	c.Assert(tenantRowCount(c, db, "manuals", fx.TenantID), qt.Equals, 1)

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
			WHERE tenant_id = $1
			  AND linked_entity_type = 'commodity'
			  AND linked_entity_meta = $2
			  AND category = $3`,
			fx.TenantID, tc.linkedEntityMeta, tc.category).Scan(&n)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, tenantRowCount(c, db, tc.legacyTable, fx.TenantID),
			qt.Commentf("backfilled %s rows must match legacy count", tc.legacyTable))
	}
}

// TestBackfill_CrossTenantIsolation seeds two independent tenants with
// asymmetric legacy row counts and verifies that a single global backfill
// run produces files rows correctly attributed per tenant — no rows leak
// from one tenant's commodity_id / tenant_id space into the other's. This
// exercises the contract that `INSERT … SELECT s.tenant_id FROM <legacy>`
// preserves tenant attribution on every backfilled row, which is what the
// existing `files` RLS policy (file_isolation, keyed on tenant_id +
// group_id) relies on after the data lands.
func TestBackfill_CrossTenantIsolation(t *testing.T) {
	c := qt.New(t)
	dsn := skipIfNoPostgreSQL(t)

	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()
	c.Assert(db.Ping(), qt.IsNil)

	ctx := context.Background()

	// Two tenants with deliberately different shapes so a count-based
	// assertion can prove neither leaks into the other.
	fxA := seedLegacyFixturesN(c, db, legacyCounts{Images: 4, Invoices: 3, Manuals: 2})
	defer fxA.Cleanup()
	fxB := seedLegacyFixturesN(c, db, legacyCounts{Images: 1, Invoices: 5, Manuals: 7})
	defer fxB.Cleanup()
	c.Assert(fxA.TenantID, qt.Not(qt.Equals), fxB.TenantID)

	plan, err := files_backfill.NewManager(db).Apply(ctx)
	c.Assert(err, qt.IsNil)
	// Inserted >= our combined fixture total — other parallel suites may
	// have seeded their own legacy rows in this shared DB. Tenant-scoped
	// assertions below carry the actual isolation contract.
	c.Assert(plan.TotalInserted() >= fxA.Counts.total()+fxB.Counts.total(),
		qt.IsTrue, qt.Commentf("got %d inserted", plan.TotalInserted()))

	// Per-tenant `files` totals must match each fixture's legacy total
	// exactly — not the combined sum, not zero.
	c.Assert(legacyFilesCount(c, db, fxA.TenantID), qt.Equals, fxA.Counts.total())
	c.Assert(legacyFilesCount(c, db, fxB.TenantID), qt.Equals, fxB.Counts.total())

	// No `files` row produced for tenant A may carry tenant B's tenant_id
	// (and vice versa). This is the cross-tenant leak check.
	c.Assert(filesForTenantSeededBy(c, db, fxA.TenantID, fxB.TenantID), qt.Equals, 0,
		qt.Commentf("tenant A's legacy rows must not produce files rows attributed to tenant B"))
	c.Assert(filesForTenantSeededBy(c, db, fxB.TenantID, fxA.TenantID), qt.Equals, 0,
		qt.Commentf("tenant B's legacy rows must not produce files rows attributed to tenant A"))

	// Per-source per-tenant counts mirror each fixture's input.
	for _, fx := range []LegacyFixture{fxA, fxB} {
		assertPerSourceMapping(c, db, ctx, fx)
	}
}

// TestBackfill_ScaleManyRows exercises the backfill against a fixture
// large enough that the SQL has to do real work — covering the AC bound
// of "100+ mixed rows" and exercising the same code path that will run
// against production volumes.
func TestBackfill_ScaleManyRows(t *testing.T) {
	c := qt.New(t)
	dsn := skipIfNoPostgreSQL(t)

	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()
	c.Assert(db.Ping(), qt.IsNil)

	ctx := context.Background()
	counts := legacyCounts{Images: 60, Invoices: 40, Manuals: 25} // 125 rows
	fx := seedLegacyFixturesN(c, db, counts)
	defer fx.Cleanup()

	plan, err := files_backfill.NewManager(db).Apply(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(rowsBySource(plan.Sources, "images").Inserted >= counts.Images, qt.IsTrue)
	c.Assert(rowsBySource(plan.Sources, "invoices").Inserted >= counts.Invoices, qt.IsTrue)
	c.Assert(rowsBySource(plan.Sources, "manuals").Inserted >= counts.Manuals, qt.IsTrue)
	c.Assert(legacyFilesCount(c, db, fx.TenantID), qt.Equals, counts.total())

	// Re-run on the same fixture must be a no-op — at scale the
	// `WHERE NOT EXISTS` filter is the load-bearing piece, so prove it
	// holds when the candidate set is non-trivial. Tenant-scoped count
	// stays exact even though other parallel suites may have rows in
	// flight against the shared DB.
	_, err = files_backfill.NewManager(db).Apply(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(legacyFilesCount(c, db, fx.TenantID), qt.Equals, counts.total())
}

// assertPerSourceMapping asserts that the requested fixture's legacy rows
// map to files rows with the expected category + linked_entity_meta per
// source. Used by the cross-tenant test to apply the same per-source
// shape check to each tenant.
func assertPerSourceMapping(c *qt.C, db *sql.DB, ctx context.Context, fx LegacyFixture) {
	c.Helper()
	cases := []struct {
		legacyTable      string
		linkedEntityMeta string
		category         string
		expected         int
	}{
		{"images", "images", "photos", fx.Counts.Images},
		{"invoices", "invoices", "invoices", fx.Counts.Invoices},
		{"manuals", "manuals", "documents", fx.Counts.Manuals},
	}
	for _, tc := range cases {
		var n int
		err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM files
			WHERE tenant_id = $1
			  AND linked_entity_type = 'commodity'
			  AND linked_entity_meta = $2
			  AND category = $3`,
			fx.TenantID, tc.linkedEntityMeta, tc.category).Scan(&n)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, tc.expected,
			qt.Commentf("tenant %s %s rows must match legacy count", fx.TenantID, tc.legacyTable))
	}
}

// filesForTenantSeededBy counts files rows whose tenant_id is `victim`
// but whose uuid was actually seeded by `attacker`'s legacy rows. A
// non-zero result proves a cross-tenant leak in the backfill SQL.
func filesForTenantSeededBy(c *qt.C, db *sql.DB, victim, attacker string) int {
	c.Helper()
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM files f
		WHERE f.tenant_id = $1
		  AND (
		    EXISTS (SELECT 1 FROM images   i WHERE i.uuid = f.uuid AND i.tenant_id = $2)
		 OR EXISTS (SELECT 1 FROM invoices i WHERE i.uuid = f.uuid AND i.tenant_id = $2)
		 OR EXISTS (SELECT 1 FROM manuals  m WHERE m.uuid = f.uuid AND m.tenant_id = $2)
		  )`,
		victim, attacker).Scan(&n)
	c.Assert(err, qt.IsNil)
	return n
}

func rowsBySource(rows []files_backfill.SourceStats, source string) files_backfill.SourceStats {
	for _, r := range rows {
		if r.Source == source {
			return r
		}
	}
	return files_backfill.SourceStats{}
}

// tenantRowCount counts rows in `table` scoped to `tenantID`. The DB is
// shared across test suites, so a global COUNT(*) would race; tenant
// filtering keeps the assertion bounded to the fixture this test seeded.
// `table` is one of a fixed set of legacy table names asserted by the
// caller — never user input — so the gosec G202 warning on string
// concatenation is suppressed.
func tenantRowCount(c *qt.C, db *sql.DB, table, tenantID string) int {
	c.Helper()
	switch table {
	case "images", "invoices", "manuals":
	default:
		c.Fatalf("tenantRowCount: unsupported table %q (only legacy tables allowed)", table)
	}
	var n int
	// #nosec G202 -- table name is range-checked above against a closed allow-list
	err := db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE tenant_id = $1", tenantID).Scan(&n)
	c.Assert(err, qt.IsNil)
	return n
}

// legacyFilesCount counts `files` rows for the given tenant whose uuid is
// also present in any legacy table — i.e. the rows backfill is
// responsible for. Tenant filtering keeps the assertion bounded to the
// fixture this test seeded.
func legacyFilesCount(c *qt.C, db *sql.DB, tenantID string) int {
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM files f
		WHERE f.tenant_id = $1
		  AND (EXISTS (SELECT 1 FROM images   WHERE uuid = f.uuid)
		    OR EXISTS (SELECT 1 FROM invoices WHERE uuid = f.uuid)
		    OR EXISTS (SELECT 1 FROM manuals  WHERE uuid = f.uuid))`,
		tenantID).Scan(&n)
	c.Assert(err, qt.IsNil)
	return n
}
