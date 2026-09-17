package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// TestOperationSlotRegistry_TenantIsolation asserts that a user registry sees
// only its own tenant's slots and a service registry sees every tenant's.
//
// Both halves of the mechanism have to hold for that. The table needs its RLS
// policies, and every query needs to run inside the repository transaction: a
// query issued on the pool skips the role switch and runs as the login, which
// inherits inventario_background_worker and its USING (true) policy, so it sees
// every tenant whatever the policies say. Either half alone fails this test.
//
// It connects as a non-superuser login that is merely a member of the service
// roles, like a real deployment. The default harness connects as a superuser,
// which bypasses RLS and would report a pass either way.
func TestOperationSlotRegistry_TenantIsolation(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	userA := getTestUser(c, registrySet)

	tenantB, err := registrySet.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "Slot Tenant B",
		Slug:   "slot-tenant-b",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	userB, err := registrySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantB.ID},
		Email:               "owner@slot-tenant-b.com",
		Name:                "Slot Owner B",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	factory := newNonBypassAppFactory(c, dsn)

	// Seed through the service registry, which preserves the tenant and user on
	// the entity — one live slot and one expired slot per tenant.
	service := factory.OperationSlotRegistryFactory.CreateServiceRegistry()

	// UTC deliberately: created_at and expires_at are TIMESTAMP WITHOUT TIME
	// ZONE, so a local-time value is stored verbatim and compared against a
	// NOW() the server reads in its own zone. Seeding in local time makes the
	// expired rows look live wherever the two differ.
	now := time.Now().UTC()
	for _, seed := range []struct {
		tenantID, userID string
		slotID           int
		expiresAt        time.Time
	}{
		{userA.TenantID, userA.ID, 1, now.Add(time.Hour)},
		{userA.TenantID, userA.ID, 2, now.Add(-time.Hour)},
		{tenantB.ID, userB.ID, 1, now.Add(time.Hour)},
		{tenantB.ID, userB.ID, 2, now.Add(-time.Hour)},
	} {
		_, err = service.Create(ctx, models.OperationSlot{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{
				TenantID: seed.tenantID,
				UserID:   seed.userID,
			},
			SlotID:        seed.slotID,
			OperationName: "upload",
			CreatedAt:     now.Add(-2 * time.Hour),
			ExpiresAt:     seed.expiresAt,
		})
		c.Assert(err, qt.IsNil)
	}

	ctxA := appctx.WithUser(ctx, userA)
	userRegistry, err := factory.OperationSlotRegistryFactory.CreateUserRegistry(ctxA)
	c.Assert(err, qt.IsNil)

	c.Run("repository reads are confined to the caller's tenant", func(c *qt.C) {
		count, err := userRegistry.Count(ctxA)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 2)

		slots, err := userRegistry.List(ctxA)
		c.Assert(err, qt.IsNil)
		c.Assert(slots, qt.HasLen, 2)
		for _, slot := range slots {
			c.Assert(slot.TenantID, qt.Equals, userA.TenantID)
		}
	})

	// The methods below issue their own SQL and carry no tenant predicate, so the
	// policy is the only thing confining them — and it only reaches them because
	// they run inside the repository transaction.
	c.Run("hand-written queries are confined too", func(c *qt.C) {
		foreign, err := userRegistry.GetUserSlotStats(ctxA, userB.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(foreign, qt.HasLen, 0,
			qt.Commentf("tenant A must not see tenant B's slot statistics"))

		own, err := userRegistry.GetUserSlotStats(ctxA, userA.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(own["upload"], qt.Equals, 1, qt.Commentf("only the live slot counts"))

		active, err := userRegistry.GetActiveSlotCount(ctxA, userB.ID, "upload")
		c.Assert(err, qt.IsNil)
		c.Assert(active, qt.Equals, 0)

		_, err = userRegistry.GetSlot(ctxA, userB.ID, "upload", 1)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

		err = userRegistry.ReleaseSlot(ctxA, userB.ID, "upload", 1)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound,
			qt.Commentf("tenant A must not be able to release tenant B's slot"))
	})

	c.Run("the service registry still spans every tenant", func(c *qt.C) {
		slots, err := service.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(slots, qt.HasLen, 4)

		expired, err := service.GetExpiredSlots(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(expired, qt.HasLen, 2)

		removed, err := service.CleanupExpiredSlots(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(removed, qt.Equals, 2,
			qt.Commentf("the sweep must cross tenants, as its worker role allows"))

		remaining, err := service.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(remaining, qt.HasLen, 2)
	})
}
