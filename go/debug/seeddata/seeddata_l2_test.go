package seeddata_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/debug/seeddata"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

// The public, unauthenticated /api/v1/seed endpoint must not be coaxed into
// seeding into a pre-existing production tenant matched by slug (#2113, L-2).
// The refusal itself is asserted by
// TestSeedDataRefusesFreshSeedIntoPreExistingNonTestTenant in seeddata_test.go;
// the cases below pin the EXEMPTIONS that must keep working.

func TestSeedData_AllowsPreExistingTestOrgTenant(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()
	registrySet := factorySet.CreateServiceRegistrySet()

	_, err := registrySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Test Organization",
		Slug:   "test-org",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	// Seeding into the well-known sentinel is allowed and idempotent.
	_, err = seeddata.SeedData(factorySet, seeddata.SeedOptions{TenantSlug: "test-org"})
	c.Assert(err, qt.IsNil)
}

func TestSeedData_CreateTenantIfMissingOverridesGuardForExistingTenant(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()
	registrySet := factorySet.CreateServiceRegistrySet()

	// A pre-existing non-test-org tenant the trusted e2e fixture targets.
	_, err := registrySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Tenant B",
		Slug:   "tenant-b",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	// CreateTenantIfMissing is the explicit env-gated opt-in (#1851); with it
	// set, re-seeding an already-created fixture tenant is allowed.
	_, err = seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug:            "tenant-b",
		CreateTenantIfMissing: true,
	})
	c.Assert(err, qt.IsNil)
}

func TestSeedData_CreateTenantIfMissingCreatesNewTenant(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()

	// The slug does not exist yet; CreateTenantIfMissing provisions it.
	_, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug:            "fresh-tenant",
		CreateTenantIfMissing: true,
	})
	c.Assert(err, qt.IsNil)

	registrySet := factorySet.CreateServiceRegistrySet()
	created, err := registrySet.TenantRegistry.GetBySlug(context.Background(), "fresh-tenant")
	c.Assert(err, qt.IsNil)
	c.Assert(created.Slug, qt.Equals, "fresh-tenant")
}
