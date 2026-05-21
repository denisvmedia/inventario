//go:build integration

package seeddata_test

import (
	"context"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/denisvmedia/inventario/debug/seeddata"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// postgresTestDSN resolves the integration-test database DSN. CI always
// sets POSTGRES_TEST_DSN (see .github/workflows/go-test-postgres.yml).
//
// When it is unset the test skips rather than falling back to a hard-coded
// local DSN: this matches the prevailing convention in the repo's other
// integration tests (registry/postgres/posgres_utils_test.go's
// skipIfNoPostgreSQL, which also has its local-DSN fallback deliberately
// commented out). A silent fallback is risky — a developer running the
// integration tag without POSTGRES_TEST_DSN could unknowingly point the
// destructive cleanup at a real local database. Skipping makes the
// requirement explicit instead.
func postgresTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("Skipping PostgreSQL tests: POSTGRES_TEST_DSN environment variable not set")
	}
	return dsn
}

func TestSeedDataPostgreSQL(t *testing.T) {
	c := qt.New(t)

	// Connect to test database
	dsn := postgresTestDSN(t)
	db, err := sqlx.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	// Test connection
	err = db.Ping()
	c.Assert(err, qt.IsNil)

	// Create factory set with PostgreSQL
	factorySet := postgres.NewFactorySet(db)

	// Wipe any pre-existing test-org object graph (including the tenant
	// row) so the seed below runs against a known-clean slate. The
	// test-postgres CI job runs many integration packages against one
	// shared database, so a sibling test may have left a stale `test-org`
	// tenant whose users own locations (and other child rows) — without
	// this cleanup the seed would attach to that stale tenant and the
	// fixture-user assertions would fail.
	cleanupTestData(c, db)

	// Re-create the test-org tenant deterministically. SeedData's
	// empty-slug path returns existingTenants[0], which — with other
	// packages' tenants possibly still present in the shared DB — is not
	// guaranteed to be test-org. Passing TenantSlug routes
	// findOrCreateTenant through GetBySlug, which errors if the tenant is
	// absent, so the test must ensure it exists first. Creating it here
	// (rather than leaving it in cleanup) keeps cleanupTestData purely
	// destructive and the seeding path deterministic.
	registrySet := factorySet.CreateServiceRegistrySet()
	_, err = registrySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Test Organization",
		Slug:   "test-org",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	// Pin the tenant explicitly so findOrCreateTenant takes the
	// deterministic GetBySlug path.
	//
	// SeedSystemAdmin opts into the sysadmin fixture (#1758) so the
	// is_system_admin round-trip is exercised against Postgres.
	_, err = seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug:      "test-org",
		SeedSystemAdmin: true,
	})
	c.Assert(err, qt.IsNil)

	// Verify that the tenant is present
	tenants, err := registrySet.TenantRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(len(tenants) >= 1, qt.IsTrue, qt.Commentf("Expected at least 1 tenant, got %d", len(tenants)))

	// Find the test tenant
	var testTenant *models.Tenant
	for _, tenant := range tenants {
		if tenant.Slug == "test-org" {
			testTenant = tenant
			break
		}
	}
	c.Assert(testTenant, qt.IsNotNil, qt.Commentf("Test tenant with slug 'test-org' not found"))
	c.Assert(testTenant.Name, qt.Equals, "Test Organization")
	c.Assert(testTenant.Status, qt.Equals, models.TenantStatusActive)

	// Verify that users were created with the correct tenant ID.
	// Seven well-known fixture users land in test-org after #1758:
	// admin, user2, orphan, family (owner of the secondary group),
	// teammate (second member of admin's primary group), sysadmin
	// (platform system admin) and blocktarget (block/unblock fixture).
	// Scoped to the test tenant so rows left by sibling integration
	// packages in the shared DB do not skew the count.
	users := usersForTenant(c, registrySet, testTenant.ID)
	c.Assert(len(users) >= 7, qt.IsTrue, qt.Commentf("Expected at least 7 users, got %d", len(users)))

	// Find the test users
	var adminUser, regularUser, orphanUser, familyUser, sysadminUser, blockTargetUser *models.User
	for _, user := range users {
		switch user.Email {
		case "admin@test-org.com":
			adminUser = user
		case "user2@test-org.com":
			regularUser = user
		case "orphan@test-org.com":
			orphanUser = user
		case "family@test-org.com":
			familyUser = user
		case "sysadmin@test-org.com":
			sysadminUser = user
		case "blocktarget@test-org.com":
			blockTargetUser = user
		}
	}

	c.Assert(adminUser, qt.IsNotNil, qt.Commentf("Admin user not found"))
	c.Assert(adminUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(adminUser.Name, qt.Equals, "Test Administrator")
	c.Assert(adminUser.IsActive, qt.Equals, true)

	c.Assert(regularUser, qt.IsNotNil, qt.Commentf("Regular user not found"))
	c.Assert(regularUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(regularUser.Name, qt.Equals, "Test User 2")
	c.Assert(regularUser.IsActive, qt.Equals, true)

	// Orphan: active so it can authenticate, zero memberships so e2e tests
	// hit the real `/api/v1/groups` empty-collection response (issue #1277).
	c.Assert(orphanUser, qt.IsNotNil, qt.Commentf("Orphan user not found"))
	c.Assert(orphanUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(orphanUser.IsActive, qt.Equals, true)
	memberships, err := registrySet.GroupMembershipRegistry.ListByUser(context.Background(), testTenant.ID, orphanUser.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(memberships, qt.HasLen, 0)

	// Family user owns the secondary group (Family).
	c.Assert(familyUser, qt.IsNotNil, qt.Commentf("family user not found"))
	c.Assert(familyUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(familyUser.IsActive, qt.IsTrue)

	// Sysadmin: the is_system_admin flag must round-trip through the
	// Postgres INSERT/SELECT path (issue #1758).
	c.Assert(sysadminUser, qt.IsNotNil, qt.Commentf("sysadmin user not found"))
	c.Assert(sysadminUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(sysadminUser.IsActive, qt.IsTrue)
	c.Assert(sysadminUser.IsSystemAdmin, qt.IsTrue)

	// Block-target: a plain active fixture — asserted explicitly so a
	// broken insert can't be masked by unrelated rows (issue #1758).
	c.Assert(blockTargetUser, qt.IsNotNil, qt.Commentf("blocktarget user not found"))
	c.Assert(blockTargetUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(blockTargetUser.IsActive, qt.IsTrue)
	c.Assert(blockTargetUser.IsSystemAdmin, qt.IsFalse)
}

// usersForTenant returns the users belonging to a single tenant. The
// test-postgres CI job shares one database across many integration
// packages, so a global UserRegistry.List would mix in unrelated rows;
// scoping by tenant keeps the fixture-user count assertion deterministic.
func usersForTenant(c *qt.C, registrySet *registry.Set, tenantID string) []*models.User {
	all, err := registrySet.UserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	scoped := make([]*models.User, 0, len(all))
	for _, u := range all {
		if u.TenantID == tenantID {
			scoped = append(scoped, u)
		}
	}
	return scoped
}

// cleanupTestData wipes the `test-org` tenant's entire object graph —
// including the tenant row itself — so TestSeedDataPostgreSQL always
// starts from a clean slate.
//
// It is deliberately scoped to the test-org tenant only. The test-postgres
// CI job runs integration packages in parallel against one shared
// database, so a global TRUNCATE — or deleting other tenants' rows — would
// corrupt a concurrently-running package. Every table in the schema
// (except `tenants` itself) carries a tenant_id column, so each child
// DELETE is filtered by the resolved test-org tenant id(s); the tenant
// rows are then deleted by slug.
//
// Deletes run inside a single transaction in FK-dependency order
// (children before parents). The three nullable self-referential /
// cyclic columns (commodities.cover_file_id, users.default_group_id,
// location_groups.currency_migration_id) are NULLed first so the cycles
// break cleanly. Errors are asserted, not swallowed: if cleanup silently
// fails the next run rots exactly the way this fix is repairing.
func cleanupTestData(c *qt.C, db *sqlx.DB) {
	tx, err := db.Beginx()
	c.Assert(err, qt.IsNil)
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Resolve the test-org tenant id(s). If none exist there is nothing
	// to clean — the seed will create the tenant from scratch.
	var tenantIDs []string
	err = tx.Select(&tenantIDs, "SELECT id FROM tenants WHERE slug = 'test-org'")
	c.Assert(err, qt.IsNil)
	if len(tenantIDs) == 0 {
		c.Assert(tx.Commit(), qt.IsNil)
		committed = true
		return
	}

	// Break the cyclic FKs first by NULLing the nullable referencing
	// columns, so the ordered DELETEs below never trip a cycle.
	cyclicNulls := []string{
		"UPDATE commodities SET cover_file_id = NULL WHERE tenant_id = ANY($1)",
		"UPDATE location_groups SET currency_migration_id = NULL WHERE tenant_id = ANY($1)",
		"UPDATE users SET default_group_id = NULL WHERE tenant_id = ANY($1)",
	}
	for _, q := range cyclicNulls {
		_, err = tx.Exec(q, tenantIDs)
		c.Assert(err, qt.IsNil, qt.Commentf("cleanup pre-step failed: %s", q))
	}

	// Tables in FK-dependency order: every table must be deleted before
	// any table it references. Leaf/child tables first, parents last.
	// `tenants` is omitted on purpose (see the doc comment above).
	orderedTables := []string{
		// Leaf tables — reference commodities / files / schedules / etc.
		"user_concurrency_slots",
		"commodity_events",
		"commodity_loans",
		"commodity_services",
		"warranty_reminders",
		"currency_migration_audit_rows",
		"maintenance_reminders",
		"commodity_supply_links",
		"restore_steps",
		"thumbnail_generation_jobs",
		"group_memberships",
		"group_invites",
		"group_invites_audit",
		"group_notification_prefs",
		"storage_quota_reminders",
		"tags",
		"login_events",
		"email_verifications",
		"password_resets",
		"refresh_tokens",
		"user_mfa_secrets",
		"operation_slots",
		"settings",
		"audit_logs",
		// Mid-tier — reference commodities / exports.
		"maintenance_schedules",
		"restore_operations",
		// Inventory tree — commodities -> areas -> locations.
		"commodities",
		"areas",
		"locations",
		// File-owning + currency rows.
		"exports",
		"files",
		"currency_migrations",
		// Parents last. location_groups before users: location_groups.created_by
		// points at users (and users.default_group_id was NULLed above), so the
		// dependency now runs one way — location_groups -> users — and the
		// groups must be deleted first.
		"location_groups",
		"users",
	}
	for _, table := range orderedTables {
		_, err = tx.Exec("DELETE FROM "+table+" WHERE tenant_id = ANY($1)", tenantIDs)
		c.Assert(err, qt.IsNil, qt.Commentf("cleanup DELETE FROM %s failed", table))
	}

	// Finally drop the tenant row(s) themselves. All tenant_id-bearing
	// children are gone, so the fk_entity_tenant constraints are satisfied.
	_, err = tx.Exec("DELETE FROM tenants WHERE id = ANY($1)", tenantIDs)
	c.Assert(err, qt.IsNil, qt.Commentf("cleanup DELETE FROM tenants failed"))

	c.Assert(tx.Commit(), qt.IsNil)
	committed = true
}
