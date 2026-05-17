package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestCommodityRegistry_Create(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of CommodityRegistry
	r, _ := getCommodityRegistry(c) // will create the commodity

	// Verify the count of commodities in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestCommodityRegistry_Delete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of CommodityRegistry
	r, createdCommodity := getCommodityRegistry(c)

	// Delete the commodity from the registry
	err := r.Delete(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the commodity is no longer present in the registry
	_, err = r.Get(ctx, createdCommodity.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)

	// Verify the count of commodities in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestCommodityRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
		Email: "test@example.com",
		Name:  "Test User",
	}

	// Create user in the system first
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create a test commodity without required fields
	commodity := models.Commodity{}

	// Create the commodity - should succeed (no validation in memory registry)
	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)
	c.Assert(createdCommodity, qt.IsNotNil)
}

func TestCommodityRegistry_Create_AreaNotFound(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
		Email: "test@example.com",
		Name:  "Test User",
	}

	// Create user in the system first
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create a test commodity with an invalid area ID
	commodity := models.Commodity{
		AreaID:    "invalid",
		Name:      "test",
		Status:    models.CommodityStatusInUse,
		Type:      models.CommodityTypeEquipment,
		Count:     1,
		ShortName: "test",
	}

	// Create the commodity - should succeed (no validation in memory registry)
	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)
	c.Assert(createdCommodity, qt.IsNotNil)
}

func TestCommodityRegistry_Delete_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
		Email: "test@example.com",
		Name:  "Test User",
	}

	// Create user in the system first
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Attempt to delete a non-existing commodity from the registry and expect a not found error
	err = registrySet.CommodityRegistry.Delete(ctx, "nonexistent")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestCommodityRegistry_List_SortedByPurchaseDate(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-sort"},
			TenantID: "test-tenant-id",
		},
		Email: "sort@example.com",
		Name:  "Sort User",
	}

	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	location, err := registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"})
	c.Assert(err, qt.IsNil)

	area, err := registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Area", LocationID: location.ID})
	c.Assert(err, qt.IsNil)

	testCases := []struct {
		name         string
		purchaseDate *models.Date
	}{
		{name: "older", purchaseDate: models.ToPDate("2021-06-15")},
		{name: "newest", purchaseDate: models.ToPDate("2023-12-01")},
		{name: "middle", purchaseDate: models.ToPDate("2022-03-20")},
		{name: "no_date", purchaseDate: nil},
	}

	for _, tc := range testCases {
		_, err = registrySet.CommodityRegistry.Create(ctx, models.Commodity{
			AreaID:       area.ID,
			Name:         tc.name,
			ShortName:    tc.name,
			Status:       models.CommodityStatusInUse,
			Type:         models.CommodityTypeElectronics,
			Count:        1,
			PurchaseDate: tc.purchaseDate,
		})
		c.Assert(err, qt.IsNil)
	}

	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 4)

	// Expect descending order: newest → middle → older → no_date (nil last)
	c.Assert(commodities[0].Name, qt.Equals, "newest")
	c.Assert(commodities[1].Name, qt.Equals, "middle")
	c.Assert(commodities[2].Name, qt.Equals, "older")
	c.Assert(commodities[3].Name, qt.Equals, "no_date")
	c.Assert(commodities[3].PurchaseDate, qt.IsNil)
}

// TestCommodityRegistry_GetMany_BatchFetch locks the batched primitive
// added under issue #1512: many ids, one round-trip's worth of work,
// arbitrary result order, callers responsible for any re-ordering. The
// memory backend doesn't have round-trips to count, but the contract
// (set semantics, missing ids dropped, duplicates collapsed) is the
// same as postgres so both backends share this shape of test.
func TestCommodityRegistry_GetMany_BatchFetch(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-getmany"},
			TenantID: "test-tenant-id",
		},
		Email: "getmany@example.com",
		Name:  "GetMany User",
	}
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	location, err := registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"})
	c.Assert(err, qt.IsNil)
	area, err := registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Area", LocationID: location.ID})
	c.Assert(err, qt.IsNil)

	mkCommodity := func(name string) *models.Commodity {
		created, cerr := registrySet.CommodityRegistry.Create(ctx, models.Commodity{
			AreaID:    area.ID,
			Name:      name,
			ShortName: name,
			Status:    models.CommodityStatusInUse,
			Type:      models.CommodityTypeElectronics,
			Count:     1,
		})
		c.Assert(cerr, qt.IsNil)
		return created
	}
	c1 := mkCommodity("alpha")
	c2 := mkCommodity("beta")
	c3 := mkCommodity("gamma")

	c.Run("returns requested commodities; order is not the caller's", func(c *qt.C) {
		got, gerr := registrySet.CommodityRegistry.GetMany(ctx, []string{c2.ID, c1.ID, c3.ID})
		c.Assert(gerr, qt.IsNil)
		c.Assert(got, qt.HasLen, 3)
		byID := map[string]string{}
		for _, com := range got {
			byID[com.ID] = com.Name
		}
		c.Assert(byID, qt.DeepEquals, map[string]string{
			c1.ID: "alpha",
			c2.ID: "beta",
			c3.ID: "gamma",
		})
	})

	c.Run("empty ids returns nil without error", func(c *qt.C) {
		got, gerr := registrySet.CommodityRegistry.GetMany(ctx, nil)
		c.Assert(gerr, qt.IsNil)
		c.Assert(got, qt.IsNil)
	})

	c.Run("unknown ids are silently dropped", func(c *qt.C) {
		got, gerr := registrySet.CommodityRegistry.GetMany(ctx, []string{c1.ID, "no-such-id", c3.ID})
		c.Assert(gerr, qt.IsNil)
		c.Assert(got, qt.HasLen, 2)
	})

	c.Run("duplicate ids collapse to one result", func(c *qt.C) {
		got, gerr := registrySet.CommodityRegistry.GetMany(ctx, []string{c1.ID, c1.ID, c1.ID})
		c.Assert(gerr, qt.IsNil)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].ID, qt.Equals, c1.ID)
	})

	c.Run("empty-string ids are ignored", func(c *qt.C) {
		got, gerr := registrySet.CommodityRegistry.GetMany(ctx, []string{"", c2.ID, ""})
		c.Assert(gerr, qt.IsNil)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].ID, qt.Equals, c2.ID)
	})
}

// TestCommodityRegistry_GetMany_GroupScoped pins down that the batched
// fetch never leaks rows the calling user's group context shouldn't see
// — the same isItemVisible filter that gates Get applies to GetMany.
// Without this, a cross-group id passed in the IN-list would silently
// surface in the result.
func TestCommodityRegistry_GetMany_GroupScoped(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()

	makeUser := func(id, email string) *models.User {
		u, uerr := factorySet.CreateServiceRegistrySet().UserRegistry.Create(context.Background(), models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: id},
				TenantID: "test-tenant-id",
			},
			Email: email,
			Name:  email,
		})
		c.Assert(uerr, qt.IsNil)
		return u
	}
	uA := makeUser("user-a", "a@example.com")
	uB := makeUser("user-b", "b@example.com")

	ctxA := appctx.WithUser(context.Background(), uA)
	ctxB := appctx.WithUser(context.Background(), uB)
	regA := must.Must(factorySet.CreateUserRegistrySet(ctxA))
	regB := must.Must(factorySet.CreateUserRegistrySet(ctxB))

	locA, err := regA.LocationRegistry.Create(ctxA, models.Location{Name: "LocA"})
	c.Assert(err, qt.IsNil)
	areaA, err := regA.AreaRegistry.Create(ctxA, models.Area{Name: "AreaA", LocationID: locA.ID})
	c.Assert(err, qt.IsNil)
	mineA, err := regA.CommodityRegistry.Create(ctxA, models.Commodity{
		AreaID: areaA.ID, Name: "a-only", ShortName: "ao",
		Status: models.CommodityStatusInUse, Type: models.CommodityTypeElectronics, Count: 1,
	})
	c.Assert(err, qt.IsNil)

	// User B asks for user A's commodity id — must return empty, never
	// the row.
	got, gerr := regB.CommodityRegistry.GetMany(ctxB, []string{mineA.ID})
	c.Assert(gerr, qt.IsNil)
	c.Assert(got, qt.HasLen, 0)
}

func getCommodityRegistry(c *qt.C) (*memory.CommodityRegistry, *models.Commodity) {
	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
		Email: "test@example.com",
		Name:  "Test User",
	}

	// Create user in the system first
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	location1, err := registrySet.LocationRegistry.Create(ctx, models.Location{
		Name: "Location 1",
	})
	c.Assert(err, qt.IsNil)

	area1, err := registrySet.AreaRegistry.Create(ctx, models.Area{
		Name:       "Area 1",
		LocationID: location1.ID,
	})
	c.Assert(err, qt.IsNil)

	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID:    area1.ID,
		Name:      "commodity1",
		ShortName: "commodity1",
		Status:    models.CommodityStatusInUse,
		Type:      models.CommodityTypeWhiteGoods,
		Count:     1,
		// Note: ID will be generated server-side for security
	})
	c.Assert(err, qt.IsNil)
	c.Assert(createdCommodity, qt.IsNotNil)
	// Verify that a valid UUID was generated (36 characters with hyphens)
	c.Assert(createdCommodity.ID, qt.Not(qt.Equals), "")
	c.Assert(createdCommodity.ID, qt.HasLen, 36)
	c.Assert(createdCommodity.Name, qt.Equals, "commodity1")
	c.Assert(createdCommodity.AreaID, qt.Equals, area1.ID)

	return registrySet.CommodityRegistry.(*memory.CommodityRegistry), createdCommodity
}
