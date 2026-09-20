package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

// The RLS repository stamps tenant ownership differently depending on which
// registry created it, and that difference is the backbone of tenant
// isolation:
//
//   - a USER registry overwrites whatever tenant id the caller supplied with
//     its own, so a request cannot write into another tenant by populating a
//     field;
//   - a SERVICE registry preserves the entity's tenant id, because the
//     callers that need one — the seed, the purgers, the workers — operate
//     across tenants by design and have nowhere else to put the value.
//
// The test that claimed to cover this asserted `true == true` with a comment
// saying the fix was applied (#2114 N1). That is worse than no test: it
// counts as coverage and reads as maintained. This drives the real
// repositories against Postgres instead. Self-skips without
// POSTGRES_TEST_DSN; CI sets it (go-test-postgres.yml).

// TestRLSGroupRepository_UserRegistryOverridesSuppliedTenant covers the
// group-scoped repository (locations, areas, commodities, files) — the one
// most user data goes through.
func TestRLSGroupRepository_UserRegistryOverridesSuppliedTenant(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	ctx := context.Background()
	tenants, err := registrySet.TenantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(tenants) > 0, qt.IsTrue)
	ownTenant := tenants[0].ID

	created, err := registrySet.LocationRegistry.Create(ctx, models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			// A tenant this caller has no business writing into.
			TenantID: "tenant-belonging-to-someone-else",
		},
		Name:    "Smuggled",
		Address: "nowhere",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created.TenantID, qt.Equals, ownTenant,
		qt.Commentf("a user registry must stamp its OWN tenant, not the one the caller supplied"))
}

// And the same property on the plain RLSRepository, which backs the
// tenant-user-scoped tables (settings, operation slots, thumbnail jobs). Two
// repositories implement this stamping independently, so a test of one says
// nothing about the other.
func TestRLSRepository_UserRegistryOverridesSuppliedTenant(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	ctx := context.Background()
	tenants, err := registrySet.TenantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(tenants) > 0, qt.IsTrue)
	ownTenant := tenants[0].ID

	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(users) > 0, qt.IsTrue)

	now := time.Now().UTC()
	created, err := registrySet.OperationSlotRegistry.Create(ctx, models.OperationSlot{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: "tenant-belonging-to-someone-else",
			UserID:   users[0].ID,
		},
		SlotID:        1,
		OperationName: "upload",
		CreatedAt:     now,
		ExpiresAt:     now.Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created.TenantID, qt.Equals, ownTenant,
		qt.Commentf("a user registry must stamp its OWN tenant, not the one the caller supplied"))
}

// And the other half: a service registry keeps what it was given, which is
// the only way a cross-tenant caller can write on behalf of a tenant that is
// not in its context.
func TestRLSRepository_ServiceRegistryPreservesSuppliedTenant(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	ctx := context.Background()
	tenants, err := registrySet.TenantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(tenants) > 0, qt.IsTrue)
	existing := tenants[0]

	// A second tenant, so "preserved" means something other than "happened
	// to match the only tenant there is".
	second, err := registrySet.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "Second Organization",
		Slug:   "second-org",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(second.ID, qt.Not(qt.Equals), existing.ID)

	// TenantRegistry itself comes from the service set used by the harness,
	// so a row created through it carries the tenant it was handed.
	user, err := registrySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: second.ID},
		Email:               "service-created@second.example",
		Name:                "Service Created",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(user.TenantID, qt.Equals, second.ID,
		qt.Commentf("a service registry must preserve the tenant it was handed"))
}
