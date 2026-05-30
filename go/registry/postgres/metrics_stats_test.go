package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// TestSystemStats_CrossTenantTotals is the security-relevant regression
// test for the business-metrics source (#843). It seeds entities in TWO
// tenants and asserts that FactorySet.SystemStats reports the COMBINED
// totals — proving the source runs under the RLS-bypassing
// background-worker role and sees every tenant's rows, not just the
// caller's. If a future change accidentally scoped SystemStats to a
// single tenant (e.g. by routing through inventario_app), the
// cross-tenant counts would drop and this test would fail.
//
// Self-skips when POSTGRES_TEST_DSN is unset (via skipIfNoPostgreSQL),
// so it is inert under the default `make test-go`.
func TestSystemStats_CrossTenantTotals(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)
	ctx := context.Background()

	// Tenant A: setupTestRegistrySet bootstraps the schema and returns a
	// user+group-aware set already scoped to tenant A's seeded user/group.
	tenantASet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	const imageSize = int64(1024)

	// Seed tenant A: location → area → commodity, plus one image file.
	locA := createTestLocation(c, tenantASet)
	areaA := createTestArea(c, tenantASet, locA.ID)
	createTestCommodity(c, tenantASet, areaA.ID)
	seedImageFile(c, tenantASet, imageSize)

	// Tenant B: a second, fully independent tenant + user + group seeded
	// via the service (RLS-bypass) registry set, then a user+group-aware
	// set built for it so the per-entity registries pass their tenant /
	// group context checks.
	serviceSet := postgres.NewFactorySet(newSystemStatsDBX(c, dsn)).CreateServiceRegistrySet()

	tenantB, err := serviceSet.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "Metrics Tenant B",
		Slug:   "metrics-tenant-b",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	userB, err := serviceSet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantB.ID},
		Email:               "owner@metrics-tenant-b.com",
		Name:                "Metrics Owner B",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	groupBSlug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	groupB, err := serviceSet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantB.ID},
		Name:                "Metrics Group B",
		Slug:                groupBSlug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           userB.ID,
		GroupCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)

	tenantBSet := postgres.NewRegistrySetWithUserAndGroupID(
		newSystemStatsDBX(c, dsn), userB.ID, tenantB.ID, groupB.ID,
	)

	locB := createTestLocation(c, tenantBSet)
	areaB := createTestArea(c, tenantBSet, locB.ID)
	createTestCommodity(c, tenantBSet, areaB.ID)
	seedImageFile(c, tenantBSet, imageSize)

	// Collect installation-wide stats through the wired SystemStats source.
	statsFn := postgres.NewFactorySet(newSystemStatsDBX(c, dsn)).SystemStats
	c.Assert(statsFn, qt.IsNotNil)

	stats, err := statsFn(ctx)
	c.Assert(err, qt.IsNil)

	// Both tenants contributed: setupTestRegistrySet seeds tenant A
	// (1 tenant, 1 user, 1 group) and we added tenant B (1/1/1), plus one
	// location/area/commodity/image-file in each. The DB is shared across
	// tests in a run, so assert lower bounds (>=) rather than exact counts
	// — the point is that BOTH tenants are visible, not that the table is
	// pristine.
	c.Assert(stats.Tenants >= 2, qt.IsTrue, qt.Commentf("tenants=%d", stats.Tenants))
	c.Assert(stats.Users >= 2, qt.IsTrue, qt.Commentf("users=%d", stats.Users))
	c.Assert(stats.LocationGroups >= 2, qt.IsTrue, qt.Commentf("groups=%d", stats.LocationGroups))
	c.Assert(stats.Locations >= 2, qt.IsTrue, qt.Commentf("locations=%d", stats.Locations))
	c.Assert(stats.Areas >= 2, qt.IsTrue, qt.Commentf("areas=%d", stats.Areas))
	c.Assert(stats.Commodities >= 2, qt.IsTrue, qt.Commentf("commodities=%d", stats.Commodities))
	c.Assert(stats.Files >= 2, qt.IsTrue, qt.Commentf("files=%d", stats.Files))

	// Storage breakdown is summed across tenants too: two image files of
	// imageSize each land in the images bucket regardless of tenant.
	c.Assert(stats.StorageImages >= 2*imageSize, qt.IsTrue,
		qt.Commentf("storage_images=%d", stats.StorageImages))
}

// newSystemStatsDBX opens a fresh *sqlx.DB over the shared test pool. A
// distinct *sql.DB handle per FactorySet keeps the test from sharing
// transaction state across the several factory sets it builds.
func newSystemStatsDBX(c *qt.C, dsn string) *sqlx.DB {
	c.Helper()
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	sqlDB := stdlib.OpenDBFromPool(pool)
	c.Cleanup(func() { _ = sqlDB.Close() })
	return sqlx.NewDb(sqlDB, "pgx")
}

// seedImageFile creates one image FileEntity with an explicit byte size
// so the storage-breakdown SUM is non-zero and provably cross-tenant.
func seedImageFile(c *qt.C, set *registry.Set, size int64) {
	c.Helper()
	now := time.Now()
	_, err := set.FileRegistry.Create(c.Context(), models.FileEntity{
		Title:     "metrics-photo",
		Type:      models.FileTypeFromMIME("image/jpeg"),
		Category:  models.FileCategoryImages,
		CreatedAt: now,
		UpdatedAt: now,
		File: &models.File{
			Path:         "metrics-photo",
			OriginalPath: "metrics-photo.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
			SizeBytes:    size,
		},
	})
	c.Assert(err, qt.IsNil)
}
