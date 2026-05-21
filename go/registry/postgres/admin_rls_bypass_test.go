package postgres_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// TestAdminCrossTenantQueries_NonBypassAppRole is the regression test for
// issue #1787. The cross-tenant admin endpoints (admin groups list/detail,
// admin tenants list/detail, admin users list, admin group members list)
// used to issue `SET LOCAL row_security = off` while connected as a
// non-BYPASSRLS role. PostgreSQL answers any query that WOULD be filtered
// by an RLS policy with SQLSTATE 42501 in that situation, so every one of
// those endpoints returned HTTP 500 on a standard deployment.
//
// The fix routes the admin registry methods through store.DoAsAdmin, which
// runs the transaction under the dedicated `inventario_admin` role created
// by the bootstrap SQL with the BYPASSRLS attribute.
//
// The default test harness connects as a superuser, which bypasses RLS
// regardless and would therefore mask the bug. This test deliberately
// connects as a freshly-created, non-superuser, non-BYPASSRLS login role
// that is merely a *member* of inventario_app / inventario_background_worker
// / inventario_admin — exactly the production role model — so it exercises
// the genuinely broken scenario.
func TestAdminCrossTenantQueries_NonBypassAppRole(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)

	// Seed two tenants, each with a user and a group, via the standard
	// (superuser) harness so the schema is bootstrapped and migrated.
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	seededUser := getTestUser(c, registrySet)
	tenantA := seededUser.TenantID

	// A second tenant + user + group so the cross-tenant listings have
	// rows the admin caller is not a member of.
	secondTenant, err := registrySet.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "Second Organization",
		Slug:   "second-org",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	secondUser, err := registrySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: secondTenant.ID},
		Email:               "owner@second-org.com",
		Name:                "Second Owner",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	secondSlug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	secondGroup, err := registrySet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: secondTenant.ID},
		Name:                "Second Group",
		Slug:                secondSlug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           secondUser.ID,
		GroupCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)

	// A membership in the second tenant's group so ListByGroupWithUsersAdmin
	// has a cross-tenant row to join.
	_, err = registrySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: secondTenant.ID},
		GroupID:             secondGroup.ID,
		MemberUserID:        secondUser.ID,
		Role:                models.GroupRoleOwner,
	})
	c.Assert(err, qt.IsNil)

	// Build a FactorySet on a connection that authenticates as a
	// non-BYPASSRLS login role — the production role model.
	adminFactory := newNonBypassAppFactory(c, dsn)

	// Each of these calls used to fail with SQLSTATE 42501. Under the
	// inventario_admin role they must succeed and see BOTH tenants.

	c.Run("groups list crosses tenants", func(c *qt.C) {
		items, total, err := adminFactory.LocationGroupRegistry.ListAdmin(ctx, registry.AdminGroupListOptions{})
		c.Assert(err, qt.IsNil)
		c.Assert(total >= 2, qt.IsTrue,
			qt.Commentf("expected groups from both tenants, got total=%d", total))
		c.Assert(len(items) >= 2, qt.IsTrue)
	})

	c.Run("group detail loads a foreign-tenant group", func(c *qt.C) {
		detail, err := adminFactory.LocationGroupRegistry.GetAdmin(ctx, secondGroup.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(detail, qt.IsNotNil)
		c.Assert(detail.Group.ID, qt.Equals, secondGroup.ID)
	})

	c.Run("tenants list crosses tenants", func(c *qt.C) {
		items, total, err := adminFactory.TenantRegistry.ListAdmin(ctx, registry.AdminTenantListOptions{})
		c.Assert(err, qt.IsNil)
		c.Assert(total >= 2, qt.IsTrue,
			qt.Commentf("expected at least 2 tenants, got total=%d", total))
		c.Assert(len(items) >= 2, qt.IsTrue)
	})

	c.Run("tenant detail loads computed counts", func(c *qt.C) {
		detail, err := adminFactory.TenantRegistry.GetAdmin(ctx, secondTenant.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(detail, qt.IsNotNil)
		c.Assert(detail.UserCount, qt.Equals, 1)
		c.Assert(detail.GroupCount, qt.Equals, 1)
	})

	c.Run("users list crosses tenants", func(c *qt.C) {
		items, total, err := adminFactory.UserRegistry.ListAdminByTenant(ctx, secondTenant.ID, registry.AdminUserListOptions{})
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 1)
		c.Assert(items, qt.HasLen, 1)
		c.Assert(items[0].User.ID, qt.Equals, secondUser.ID)
	})

	c.Run("active session count crosses tenants", func(c *qt.C) {
		count, err := adminFactory.UserRegistry.CountSessionsByUser(ctx, secondUser.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 0)
	})

	c.Run("group members list crosses tenants", func(c *qt.C) {
		members, err := adminFactory.GroupMembershipRegistry.ListByGroupWithUsersAdmin(ctx, secondGroup.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(members, qt.HasLen, 1)
		c.Assert(members[0].User.ID, qt.Equals, secondUser.ID)
	})

	// Sanity check: tenant A's group is also visible — the admin surface
	// is genuinely cross-tenant, not just scoped to the second tenant.
	c.Run("group detail loads tenant A group too", func(c *qt.C) {
		groups, err := registrySet.LocationGroupRegistry.ListByTenant(ctx, tenantA)
		c.Assert(err, qt.IsNil)
		c.Assert(len(groups) >= 1, qt.IsTrue)
		detail, err := adminFactory.LocationGroupRegistry.GetAdmin(ctx, groups[0].ID)
		c.Assert(err, qt.IsNil)
		c.Assert(detail.Group.ID, qt.Equals, groups[0].ID)
	})
}

// newNonBypassAppFactory creates a PostgreSQL FactorySet whose connection
// authenticates as a non-superuser, non-BYPASSRLS login role. The role is a
// member of inventario_app, inventario_background_worker and
// inventario_admin — i.e. it can SET ROLE to any of them but bypasses RLS
// only while inventario_admin is the *active* role. This mirrors the
// production deployment and is the only configuration in which the #1787
// bug reproduces.
func newNonBypassAppFactory(c *qt.C, superuserDSN string) *registry.FactorySet {
	c.Helper()

	const (
		appLoginRole = "inventario_app_test_login"
		appLoginPass = "app_test_login_pw"
	)

	// Create / refresh the non-bypass login role via the superuser pool.
	adminPool, err := getOrCreatePool(superuserDSN)
	c.Assert(err, qt.IsNil)

	setupSQL := fmt.Sprintf(`
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%[1]s') THEN
        CREATE ROLE %[1]s WITH LOGIN PASSWORD '%[2]s'
            NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION;
    ELSE
        ALTER ROLE %[1]s WITH LOGIN PASSWORD '%[2]s'
            NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION;
    END IF;
    -- Defensive: this login role MUST NOT carry BYPASSRLS itself,
    -- otherwise the test would silently stop reproducing #1787.
    ALTER ROLE %[1]s WITH NOBYPASSRLS;
END $$;
GRANT inventario_app TO %[1]s;
GRANT inventario_background_worker TO %[1]s;
GRANT inventario_admin TO %[1]s;
`, appLoginRole, appLoginPass)

	_, err = adminPool.Exec(c.Context(), setupSQL)
	c.Assert(err, qt.IsNil)

	// Build a DSN for the non-bypass login role, reusing host / database /
	// query params from the superuser DSN.
	u, err := url.Parse(superuserDSN)
	c.Assert(err, qt.IsNil)
	u.User = url.UserPassword(appLoginRole, appLoginPass)

	pool, err := pgxpool.New(c.Context(), u.String())
	c.Assert(err, qt.IsNil)
	c.Cleanup(pool.Close)

	sqlDB := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	c.Cleanup(func() { _ = sqlDB.Close() })

	return postgres.NewFactorySet(sqlDB)
}
