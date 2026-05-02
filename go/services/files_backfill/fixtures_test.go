package files_backfill_test

import (
	"context"
	"database/sql"
	"fmt"

	qt "github.com/frankban/quicktest"
)

// LegacyFixture is what seedLegacyFixtures returns: the IDs callers need
// to scope their assertions to this fixture's rows (the test DB is shared,
// so global COUNT(*) would race against other suites) plus a Cleanup that
// wipes every row this fixture wrote.
type LegacyFixture struct {
	TenantID string
	GroupID  string
	Counts   legacyCounts
	Cleanup  func()
}

// legacyCounts controls how many rows seedLegacyFixturesN seeds per legacy
// source. Asymmetric defaults let per-source counter assertions
// disambiguate without fixing arithmetic to a single value.
type legacyCounts struct {
	Images   int
	Invoices int
	Manuals  int
}

func (c legacyCounts) total() int { return c.Images + c.Invoices + c.Manuals }

func defaultLegacyCounts() legacyCounts {
	return legacyCounts{Images: 3, Invoices: 2, Manuals: 1}
}

// seedLegacyFixtures is the small-scale default — 3 images, 2 invoices, 1
// manual — kept as a thin wrapper so existing happy-path / mapping tests
// don't churn. New tests that need a different volume or want two tenants
// in flight should call seedLegacyFixturesN directly.
//
// The DB is shared across test runs, so the returned Cleanup must always
// run on test exit — otherwise the next run's COUNT(*) checks would
// double-count.
func seedLegacyFixtures(c *qt.C, db *sql.DB) LegacyFixture {
	c.Helper()
	return seedLegacyFixturesN(c, db, defaultLegacyCounts())
}

// seedLegacyFixturesN stamps a self-contained tenant + group + commodity
// graph plus the requested number of legacy rows directly via SQL.
// Bypassing the registry layer keeps the test focused on the backfill
// itself and avoids pulling registry helpers into a service-level test
// package. Each invocation uses fresh uniqueIDs, so calling this multiple
// times in one test produces independent tenants for cross-tenant
// assertions.
func seedLegacyFixturesN(c *qt.C, db *sql.DB, counts legacyCounts) LegacyFixture {
	c.Helper()

	ctx := context.Background()
	tenantID := uniqueID("t")
	groupID := uniqueID("g")
	userID := uniqueID("u")
	locationID := uniqueID("loc")
	areaID := uniqueID("area")
	commodityID := uniqueID("com")

	// Tenant + user + group are the minimum scaffolding needed for
	// commodity FKs and RLS group_id columns. We bypass the validator and
	// status enums because we only need referential integrity, not real
	// product semantics.
	exec(c, db, `
		INSERT INTO tenants (id, name, slug, status, registration_mode)
		VALUES ($1, 'backfill-test', $2, 'active', 'closed')`,
		tenantID, "backfill-test-"+tenantID[:8])
	exec(c, db, `
		INSERT INTO users (id, tenant_id, email, password_hash, name, is_active, created_at)
		VALUES ($1, $2, $3, '', 'backfill', true, NOW())`,
		userID, tenantID, "u-"+userID+"@example.com")
	exec(c, db, `
		INSERT INTO location_groups (id, tenant_id, slug, name, status, created_by, main_currency, created_at, updated_at)
		VALUES ($1, $2, $3, 'Backfill', 'active', $4, 'USD', NOW(), NOW())`,
		groupID, tenantID, "g-"+groupID[:8], userID)
	exec(c, db, `
		INSERT INTO group_memberships (id, tenant_id, group_id, member_user_id, role, joined_at)
		VALUES ($1, $2, $3, $4, 'admin', NOW())`,
		uniqueID("gm"), tenantID, groupID, userID)
	exec(c, db, `
		INSERT INTO locations (id, tenant_id, group_id, created_by_user_id, name, address)
		VALUES ($1, $2, $3, $4, 'Loc', '')`,
		locationID, tenantID, groupID, userID)
	exec(c, db, `
		INSERT INTO areas (id, tenant_id, group_id, created_by_user_id, location_id, name)
		VALUES ($1, $2, $3, $4, $5, 'Area')`,
		areaID, tenantID, groupID, userID, locationID)
	exec(c, db, `
		INSERT INTO commodities (
			id, tenant_id, group_id, created_by_user_id, area_id, name, short_name,
			type, status, count, original_price, original_price_currency,
			converted_original_price, current_price, draft
		) VALUES (
			$1, $2, $3, $4, $5, 'Backfill commodity', 'BC',
			'electronics', 'in_use', 1, 0, 'USD', 0, 0, false
		)`,
		commodityID, tenantID, groupID, userID, areaID)

	// Cycle through a few representative MIMEs for images so we still
	// exercise the FileTypeFromMIME branch even at scale; invoices and
	// manuals are PDF-only by historical bucket convention.
	imageMIMEs := []string{"image/jpeg", "image/png", "image/heic", "image/webp"}
	for i := range counts.Images {
		mime := imageMIMEs[i%len(imageMIMEs)]
		path := fmt.Sprintf("img%d", i)
		exec(c, db, `
			INSERT INTO images (
				id, uuid, tenant_id, group_id, created_by_user_id, commodity_id,
				path, original_path, ext, mime_type
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10
			)`,
			uniqueID(fmt.Sprintf("img%d", i)), uniqueID(fmt.Sprintf("imgU%d", i)),
			tenantID, groupID, userID, commodityID,
			path, path+".jpg", ".jpg", mime)
	}
	for i := range counts.Invoices {
		path := fmt.Sprintf("inv%d", i)
		exec(c, db, `
			INSERT INTO invoices (
				id, uuid, tenant_id, group_id, created_by_user_id, commodity_id,
				path, original_path, ext, mime_type
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, '.pdf', 'application/pdf'
			)`,
			uniqueID(fmt.Sprintf("inv%d", i)), uniqueID(fmt.Sprintf("invU%d", i)),
			tenantID, groupID, userID, commodityID,
			path, path+".pdf")
	}
	for i := range counts.Manuals {
		path := fmt.Sprintf("man%d", i)
		exec(c, db, `
			INSERT INTO manuals (
				id, uuid, tenant_id, group_id, created_by_user_id, commodity_id,
				path, original_path, ext, mime_type
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, '.pdf', 'application/pdf'
			)`,
			uniqueID(fmt.Sprintf("man%d", i)), uniqueID(fmt.Sprintf("manU%d", i)),
			tenantID, groupID, userID, commodityID,
			path, path+".pdf")
	}

	cleanup := func() {
		// Cleanup order: dependents first. files rows that the backfill
		// produced FK-by-uuid back to legacy rows, but the schema's FKs
		// are commodity-keyed, so deleting the legacy + files rows by
		// our generated tenant_id is enough.
		exec(c, db, `DELETE FROM files WHERE tenant_id = $1`, tenantID)
		exec(c, db, `DELETE FROM images WHERE tenant_id = $1`, tenantID)
		exec(c, db, `DELETE FROM invoices WHERE tenant_id = $1`, tenantID)
		exec(c, db, `DELETE FROM manuals WHERE tenant_id = $1`, tenantID)
		exec(c, db, `DELETE FROM commodities WHERE tenant_id = $1`, tenantID)
		exec(c, db, `DELETE FROM areas WHERE tenant_id = $1`, tenantID)
		exec(c, db, `DELETE FROM locations WHERE tenant_id = $1`, tenantID)
		exec(c, db, `DELETE FROM group_memberships WHERE tenant_id = $1`, tenantID)
		exec(c, db, `DELETE FROM location_groups WHERE tenant_id = $1`, tenantID)
		exec(c, db, `DELETE FROM users WHERE tenant_id = $1`, tenantID)
		exec(c, db, `DELETE FROM tenants WHERE id = $1`, tenantID)
		_ = ctx // kept around in case future cleanups need it
	}
	return LegacyFixture{
		TenantID: tenantID,
		GroupID:  groupID,
		Counts:   counts,
		Cleanup:  cleanup,
	}
}

func exec(c *qt.C, db *sql.DB, query string, args ...any) {
	c.Helper()
	_, err := db.Exec(query, args...)
	c.Assert(err, qt.IsNil, qt.Commentf("query: %s", query))
}

// uniqueID returns a per-test-run identifier. Combining a prefix with a
// random suffix keeps fixtures from colliding across parallel test runs
// against the same shared Postgres instance.
func uniqueID(prefix string) string {
	return prefix + "-" + randomHex(16)
}
