package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// setupTestDatabase creates a test database connection and returns cleanup function.
func setupTestDatabase(t *testing.T) (*registry.FactorySet, func()) {
	t.Helper()
	dsn := mustTestDSN(t)

	// Set up fresh database with bootstrap and migrations.
	err := setupFreshDatabase(dsn)
	if err != nil {
		t.Fatalf("Failed to setup fresh database: %v", err)
	}

	registrySetFunc, cleanupFunc := postgres.NewPostgresRegistrySet()
	factorySet, err := registrySetFunc(registry.Config(dsn))
	if err != nil {
		t.Fatalf("Failed to create registry set: %v", err)
	}

	return factorySet, func() {
		cleanupFunc()
	}
}

// mustTestDSN returns the Postgres DSN for the integration suite, skipping the
// test when it is unset (the CI gate that does set it is issue #2094).
func mustTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN environment variable not set")
	}
	return dsn
}

// createTestTenant creates a real, active tenant row. location_groups and users
// FK to tenants, so the isolation tests need a genuine tenant id rather than the
// pre-groups era's hardcoded "test-tenant-id" string.
func createIsolationTenant(c *qt.C, fs *registry.FactorySet) *models.Tenant {
	c.Helper()
	uniq := fmt.Sprintf("%d", time.Now().UnixNano())
	tenant := must.Must(fs.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Isolation Tenant " + uniq,
		Slug:   "isolation-tenant-" + uniq,
		Status: models.TenantStatusActive,
	}))
	return tenant
}

// createTestUser creates a test user in the given tenant. The id is minted
// server-side; the email is made unique so repeated calls within a suite never
// collide on the unique-email constraint.
func createTestUser(c *qt.C, userRegistry registry.UserRegistry, tenantID, email string) *models.User {
	c.Helper()
	uniqueEmail := fmt.Sprintf("%s-%d", email, time.Now().UnixNano())
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
		},
		Email:    uniqueEmail,
		Name:     "Test User",
		IsActive: true,
	}

	err := user.SetPassword("TestPassword123")
	c.Assert(err, qt.IsNil)

	created, err := userRegistry.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)

	return created
}

// createTestGroup creates an active USD location group owned by the given user.
// RLS for locations/areas/commodities/files/exports is GROUP-scoped
// (tenant_id AND group_id), so every isolation fixture needs its own group, and
// the group MUST carry GroupCurrency (commodity validation reads it off the
// group in context).
func createTestGroup(c *qt.C, fs *registry.FactorySet, tenantID, userID, name string) *models.LocationGroup {
	c.Helper()
	group := must.Must(fs.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                name,
		Status:              models.LocationGroupStatusActive,
		GroupCurrency:       models.Currency("USD"),
		CreatedBy:           userID,
	}))
	return group
}

// userGroupContext builds the request context the user-aware registries expect:
// it carries BOTH the user (→ get_current_tenant_id / get_current_user_id) AND
// the group (→ get_current_group_id). Without the group, group-scoped RLS sees
// an empty get_current_group_id() and every create/read fails closed.
func userGroupContext(ctx context.Context, user *models.User, group *models.LocationGroup) context.Context {
	return appctx.WithGroup(appctx.WithUser(ctx, user), group)
}

// isolationFixture is a fully-wired single-user tenant/group/context bundle used
// by the isolation tests. Two fixtures in DIFFERENT groups model the two parties
// whose data must stay mutually invisible under group-scoped RLS.
type isolationFixture struct {
	tenant *models.Tenant
	user   *models.User
	group  *models.LocationGroup
	ctx    context.Context
}

// newIsolationPair builds two users in two SEPARATE groups within ONE tenant.
// Same tenant + different group is exactly what makes the negative assertions
// meaningful: isolation here is enforced by the group dimension of RLS, not by
// the tenant dimension (which is held constant) and not by user id.
func newIsolationPair(c *qt.C, fs *registry.FactorySet) (user1, user2 isolationFixture) {
	c.Helper()
	tenant := createIsolationTenant(c, fs)

	u1 := createTestUser(c, fs.UserRegistry, tenant.ID, "user1@example.com")
	g1 := createTestGroup(c, fs, tenant.ID, u1.ID, "User1 Group")

	u2 := createTestUser(c, fs.UserRegistry, tenant.ID, "user2@example.com")
	g2 := createTestGroup(c, fs, tenant.ID, u2.ID, "User2 Group")

	user1 = isolationFixture{tenant: tenant, user: u1, group: g1, ctx: userGroupContext(context.Background(), u1, g1)}
	user2 = isolationFixture{tenant: tenant, user: u2, group: g2, ctx: userGroupContext(context.Background(), u2, g2)}
	return user1, user2
}

// seedLocation creates a location owned by the fixture's user/group via the
// user-aware (RLS-scoped) registry.
func seedLocation(c *qt.C, fs *registry.FactorySet, f isolationFixture, name string) *models.Location {
	c.Helper()
	reg := must.Must(fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	return must.Must(reg.Create(f.ctx, models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        f.tenant.ID,
			GroupID:         f.group.ID,
			CreatedByUserID: f.user.ID,
		},
		Name:    name,
		Address: "123 " + name + " Street",
	}))
}

// seedArea creates an area under the given location, owned by the fixture.
func seedArea(c *qt.C, fs *registry.FactorySet, f isolationFixture, locationID, name string) *models.Area {
	c.Helper()
	reg := must.Must(fs.AreaRegistryFactory.CreateUserRegistry(f.ctx))
	return must.Must(reg.Create(f.ctx, models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        f.tenant.ID,
			GroupID:         f.group.ID,
			CreatedByUserID: f.user.ID,
		},
		Name:       name,
		LocationID: locationID,
	}))
}

// seedCommodity creates a non-draft commodity in the given area, owned by the
// fixture. Currency is USD to match the group currency (so ConvertedOriginalPrice
// stays zero).
func seedCommodity(c *qt.C, fs *registry.FactorySet, f isolationFixture, areaID, name, shortName string) *models.Commodity {
	c.Helper()
	reg := must.Must(fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	return must.Must(reg.Create(f.ctx, models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        f.tenant.ID,
			GroupID:         f.group.ID,
			CreatedByUserID: f.user.ID,
		},
		Name:                   name,
		ShortName:              shortName,
		AreaID:                 new(areaID),
		Type:                   models.CommodityTypeElectronics,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(100.00),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.NewFromFloat(90.00),
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           models.ToPDate("2023-01-01"),
		RegisteredDate:         models.ToPDate("2023-01-02"),
		LastModifiedDate:       models.ToPDate("2023-01-03"),
		Draft:                  false,
	}))
}

// TestUserIsolation_Commodities tests that a user in one group cannot access a
// commodity created by a user in a different group (group-scoped RLS).
func TestUserIsolation_Commodities(t *testing.T) {
	c := qt.New(t)
	fs, cleanup := setupTestDatabase(t)
	defer cleanup()

	user1, user2 := newIsolationPair(c, fs)

	// User1 creates location → area → commodity in group1.
	loc1 := seedLocation(c, fs, user1, "User1 Location")
	area1 := seedArea(c, fs, user1, loc1.ID, "User1 Area")
	created1 := seedCommodity(c, fs, user1, area1.ID, "User1 Commodity", "UC1")
	c.Assert(created1.GetCreatedByUserID(), qt.Equals, user1.user.ID)

	// User2 (different group) cannot Get user1's commodity.
	reg2 := must.Must(fs.CommodityRegistryFactory.CreateUserRegistry(user2.ctx))
	_, err := reg2.Get(user2.ctx, created1.ID)
	c.Assert(err, qt.IsNotNil)

	// User2 cannot see user1's commodity in their list.
	commodities2, err := reg2.List(user2.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities2, qt.HasLen, 0)

	// User1 can see their own commodity.
	reg1 := must.Must(fs.CommodityRegistryFactory.CreateUserRegistry(user1.ctx))
	commodities1, err := reg1.List(user1.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities1, qt.HasLen, 1)
	c.Assert(commodities1[0].ID, qt.Equals, created1.ID)
}

// TestUserIsolation_Locations tests that a user in one group cannot access a
// location created by a user in a different group.
func TestUserIsolation_Locations(t *testing.T) {
	c := qt.New(t)
	fs, cleanup := setupTestDatabase(t)
	defer cleanup()

	user1, user2 := newIsolationPair(c, fs)

	created1 := seedLocation(c, fs, user1, "User1 Location")
	c.Assert(created1.GetCreatedByUserID(), qt.Equals, user1.user.ID)

	// User2 (different group) cannot Get user1's location.
	reg2 := must.Must(fs.LocationRegistryFactory.CreateUserRegistry(user2.ctx))
	_, err := reg2.Get(user2.ctx, created1.ID)
	c.Assert(err, qt.IsNotNil)

	// User2 cannot see user1's location in their list.
	locations2, err := reg2.List(user2.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locations2, qt.HasLen, 0)

	// User1 can see their own location.
	reg1 := must.Must(fs.LocationRegistryFactory.CreateUserRegistry(user1.ctx))
	locations1, err := reg1.List(user1.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locations1, qt.HasLen, 1)
	c.Assert(locations1[0].ID, qt.Equals, created1.ID)
}

// TestUserIsolation_Files tests that a user in one group cannot access a file
// created by a user in a different group.
func TestUserIsolation_Files(t *testing.T) {
	c := qt.New(t)
	fs, cleanup := setupTestDatabase(t)
	defer cleanup()

	user1, user2 := newIsolationPair(c, fs)

	reg1 := must.Must(fs.FileRegistryFactory.CreateUserRegistry(user1.ctx))
	created1 := must.Must(reg1.Create(user1.ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        user1.tenant.ID,
			GroupID:         user1.group.ID,
			CreatedByUserID: user1.user.ID,
		},
		Title:       "User1 File",
		Description: "A file created by user1",
		Type:        models.FileTypeDocument,
		File: &models.File{
			OriginalPath: "/uploads/user1-file.txt",
			Path:         "user1-file",
			Ext:          ".txt",
			MIMEType:     "text/plain",
		},
	}))
	c.Assert(created1.GetCreatedByUserID(), qt.Equals, user1.user.ID)

	// User2 (different group) cannot Get user1's file.
	reg2 := must.Must(fs.FileRegistryFactory.CreateUserRegistry(user2.ctx))
	_, err := reg2.Get(user2.ctx, created1.ID)
	c.Assert(err, qt.IsNotNil)

	// User2 cannot see user1's file in their list.
	files2, err := reg2.List(user2.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(files2, qt.HasLen, 0)

	// User1 can see their own file.
	files1, err := reg1.List(user1.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(files1, qt.HasLen, 1)
	c.Assert(files1[0].ID, qt.Equals, created1.ID)
}

// TestUserIsolation_Exports tests that a user in one group cannot access an
// export created by a user in a different group. Exports are GROUP-scoped just
// like the other entities (tenant_id AND group_id in the RLS policy).
func TestUserIsolation_Exports(t *testing.T) {
	c := qt.New(t)
	fs, cleanup := setupTestDatabase(t)
	defer cleanup()

	user1, user2 := newIsolationPair(c, fs)

	reg1 := must.Must(fs.ExportRegistryFactory.CreateUserRegistry(user1.ctx))
	created1 := must.Must(reg1.Create(user1.ctx, models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        user1.tenant.ID,
			GroupID:         user1.group.ID,
			CreatedByUserID: user1.user.ID,
		},
		Type:        models.ExportTypeFullDatabase,
		Description: "An export created by user1",
		Status:      models.ExportStatusPending,
	}))
	c.Assert(created1.GetCreatedByUserID(), qt.Equals, user1.user.ID)

	// User2 (different group) cannot Get user1's export.
	reg2 := must.Must(fs.ExportRegistryFactory.CreateUserRegistry(user2.ctx))
	_, err := reg2.Get(user2.ctx, created1.ID)
	c.Assert(err, qt.IsNotNil)

	// User2 cannot see user1's export in their list.
	exports2, err := reg2.List(user2.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(exports2, qt.HasLen, 0)

	// User1 can see their own export.
	exports1, err := reg1.List(user1.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(exports1, qt.HasLen, 1)
	c.Assert(exports1[0].ID, qt.Equals, created1.ID)
}
