package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register file:// driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// seededTenant bundles the ids a purge test needs to assert against one tenant.
type seededTenant struct {
	tenantID string
}

// seedTenantWithDependents creates a tenant + admin user + active group, then a
// representative spread of dependent rows across the FK graph: the inventory
// hierarchy (location/area/commodity), a file, a tag, and the two tenant-only
// auth tables (refresh_tokens, login_events). It returns the ids so the test
// can prove they vanish (or survive) after a purge.
func seedTenantWithDependents(c *qt.C, ctx context.Context, fs *registry.FactorySet, dbx *sqlx.DB, slug, email string) seededTenant {
	c.Helper()

	svc := fs.CreateServiceRegistrySet()

	tenant, err := fs.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "Tenant " + slug,
		Slug:   slug,
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               email,
		Name:                "Admin " + slug,
		IsActive:            true,
	}
	c.Assert(user.SetPassword("Password123"), qt.IsNil)
	createdUser, err := fs.UserRegistry.Create(ctx, user)
	c.Assert(err, qt.IsNil)

	groupSlug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	group, err := svc.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Name:                "Group " + slug,
		Slug:                groupSlug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           createdUser.ID,
		GroupCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)

	// User+group-aware set so the inventory/file/tag seeds land with the right
	// tenant + group + created_by stamped from context. The seed helpers read
	// the user + group (incl. currency) off the context for validation.
	userSet := postgres.NewRegistrySetWithUserAndGroupID(dbx, createdUser.ID, tenant.ID, group.ID)
	userCtx := appctx.WithUser(ctx, createdUser)
	userCtx = appctx.WithGroup(userCtx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: group.ID},
			TenantID: tenant.ID,
		},
		GroupCurrency: models.Currency("USD"),
	})
	areaID := seedTagArea(c, userSet, userCtx)
	seedTagCommodity(c, userSet, userCtx, areaID, "Widget "+slug)
	seedTagFile(c, userSet, userCtx, "file-"+slug)
	mustCreateTag(c, userSet.TagRegistry, userCtx, models.TagKindCommodity, "tag-"+slug)

	// Tenant-only auth rows.
	_, err = svc.RefreshTokenRegistry.Create(ctx, models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: tenant.ID,
			UserID:   createdUser.ID,
		},
		TokenHash: "hash-" + slug,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	})
	c.Assert(err, qt.IsNil)

	uid := createdUser.ID
	_, err = svc.LoginEventRegistry.Create(ctx, models.LoginEvent{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		UserID:              &uid,
		Email:               email,
		Outcome:             models.LoginOutcomeOK,
		Method:              models.LoginMethodPassword,
	})
	c.Assert(err, qt.IsNil)

	return seededTenant{tenantID: tenant.ID}
}

// countRowsForTenant counts rows in a table for one tenant, reading under the
// background-worker role so RLS doesn't hide foreign-tenant rows (the same
// bypass the purger itself relies on).
func countRowsForTenant(c *qt.C, dbx *sqlx.DB, table, tenantID string) int {
	c.Helper()
	var n int
	err := store.DoAsBackgroundWorker(context.Background(), dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &n, "SELECT COUNT(*) FROM "+table+" WHERE tenant_id = $1", tenantID)
	})
	c.Assert(err, qt.IsNil)
	return n
}

// TestTenantPurger_Postgres_PurgesAllTenantDependents is the #2115 regression:
// a tenant carrying data across the FK graph (inventory hierarchy, file, tag,
// group, user, refresh_token, login_event) must be fully cleared by
// PurgeTenantDependents so the subsequent tenants DELETE succeeds. A second
// tenant's rows must survive untouched, and a repeat purge must be a no-op.
func TestTenantPurger_Postgres_PurgesAllTenantDependents(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")

	a := seedTenantWithDependents(c, ctx, fs, dbx, "tenant-a", "admin@a.example")
	b := seedTenantWithDependents(c, ctx, fs, dbx, "tenant-b", "admin@b.example")

	purger := postgres.NewTenantPurger(dbx)

	err = purger.PurgeTenantDependents(ctx, a.tenantID)
	c.Assert(err, qt.IsNil)

	// Every dependent table is empty for tenant A.
	for _, table := range []string{
		"locations", "areas", "commodities", "files", "tags",
		"location_groups", "users", "refresh_tokens", "login_events",
	} {
		c.Assert(countRowsForTenant(c, dbx, table, a.tenantID), qt.Equals, 0,
			qt.Commentf("table %s should be empty for purged tenant A", table))
	}

	// The final tenants DELETE is what was broken before the fix: any orphaned
	// dependent would trip a NO ACTION FK here. It succeeding is the regression.
	err = fs.TenantRegistry.Delete(ctx, a.tenantID)
	c.Assert(err, qt.IsNil)
	_, err = fs.TenantRegistry.Get(ctx, a.tenantID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Tenant B is untouched — same table set still carries its rows.
	for _, table := range []string{
		"locations", "areas", "commodities", "files", "tags",
		"location_groups", "users", "refresh_tokens", "login_events",
	} {
		c.Assert(countRowsForTenant(c, dbx, table, b.tenantID) > 0, qt.IsTrue,
			qt.Commentf("table %s must still carry tenant B rows", table))
	}

	// Idempotent: a second purge after the rows are gone is a clean no-op.
	err = purger.PurgeTenantDependents(ctx, a.tenantID)
	c.Assert(err, qt.IsNil)
}
