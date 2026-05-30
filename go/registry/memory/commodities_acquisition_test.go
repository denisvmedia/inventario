package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func bootstrapForAcquisitionTest(c *qt.C) (context.Context, *registry.Set) {
	c.Helper()
	factorySet := memory.NewFactorySet()

	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
		Email: "u@example.com",
		Name:  "User",
	}
	u, err := factorySet.CreateServiceRegistrySet().UserRegistry.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	group := &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: u.TenantID,
		},
		Slug:          "default-group-default-slug-22",
		Name:          "Default",
		GroupCurrency: "USD",
	}
	ctx := appctx.WithUser(context.Background(), u)
	ctx = appctx.WithGroup(ctx, group)

	regSet := must.Must(factorySet.CreateUserRegistrySet(ctx))
	return ctx, regSet
}

// TestCommodityRegistry_Create_DropsAcquisitionPayload — issue #1550 / #202.
// API-facing CommodityRegistry.Create must never persist user-supplied
// acquisition columns regardless of payload.
func TestCommodityRegistry_Create_DropsAcquisitionPayload(t *testing.T) {
	c := qt.New(t)

	ctx, set := bootstrapForAcquisitionTest(c)

	location, err := set.LocationRegistry.Create(ctx, models.Location{Name: "L1"})
	c.Assert(err, qt.IsNil)
	area, err := set.AreaRegistry.Create(ctx, models.Area{Name: "A1", LocationID: location.ID})
	c.Assert(err, qt.IsNil)

	dec := decimal.NewFromInt(123)
	cur := models.Currency("USD")

	created, err := set.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID:              area.ID,
		Name:                "thing",
		ShortName:           "t",
		Status:              models.CommodityStatusInUse,
		Type:                models.CommodityTypeOther,
		Count:               1,
		AcquisitionPrice:    &dec,
		AcquisitionCurrency: &cur,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created.AcquisitionPrice, qt.IsNil)
	c.Assert(created.AcquisitionCurrency, qt.IsNil)

	got, err := set.CommodityRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.AcquisitionPrice, qt.IsNil)
	c.Assert(got.AcquisitionCurrency, qt.IsNil)
}

// TestCommodityRegistry_Create_RestoreAcquisitionContext — issue #534. A Create
// carrying the trusted registry.WithRestoreAcquisition signal persists the
// acquisition pair onto the fresh row — the only seam by which a
// signature-verified backup restore reconstructs #202 provenance. A normal
// Create (no signal) still nils it; the bypass is NOT a registry method, so it
// is never reachable from the API write path.
func TestCommodityRegistry_Create_RestoreAcquisitionContext(t *testing.T) {
	c := qt.New(t)

	ctx, set := bootstrapForAcquisitionTest(c)

	location, err := set.LocationRegistry.Create(ctx, models.Location{Name: "L1"})
	c.Assert(err, qt.IsNil)
	area, err := set.AreaRegistry.Create(ctx, models.Area{Name: "A1", LocationID: location.ID})
	c.Assert(err, qt.IsNil)

	price := decimal.RequireFromString("499.99")
	currency := models.Currency("USD")
	restoreCtx := registry.WithRestoreAcquisition(ctx, price, currency)

	created, err := set.CommodityRegistry.Create(restoreCtx, models.Commodity{
		AreaID:    area.ID,
		Name:      "thing",
		ShortName: "t",
		Status:    models.CommodityStatusInUse,
		Type:      models.CommodityTypeOther,
		Count:     1,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created.AcquisitionPrice, qt.IsNotNil)
	c.Assert(created.AcquisitionPrice.String(), qt.Equals, "499.99")
	c.Assert(created.AcquisitionCurrency, qt.IsNotNil)
	c.Assert(string(*created.AcquisitionCurrency), qt.Equals, "USD")

	got, err := set.CommodityRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.AcquisitionPrice, qt.IsNotNil)
	c.Assert(got.AcquisitionPrice.String(), qt.Equals, "499.99")
	c.Assert(got.AcquisitionCurrency, qt.IsNotNil)
	c.Assert(string(*got.AcquisitionCurrency), qt.Equals, "USD")
}

// TestCommodityRegistry_Update_PreservesAcquisition — once the
// migration worker has filled the acquisition columns (simulated here
// via the underlying memory store's UpdateWithUser path), subsequent
// API Update calls must not be able to change them.
func TestCommodityRegistry_Update_PreservesAcquisition(t *testing.T) {
	c := qt.New(t)

	ctx, set := bootstrapForAcquisitionTest(c)

	location, err := set.LocationRegistry.Create(ctx, models.Location{Name: "L1"})
	c.Assert(err, qt.IsNil)
	area, err := set.AreaRegistry.Create(ctx, models.Area{Name: "A1", LocationID: location.ID})
	c.Assert(err, qt.IsNil)

	created, err := set.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID:    area.ID,
		Name:      "thing",
		ShortName: "t",
		Status:    models.CommodityStatusInUse,
		Type:      models.CommodityTypeOther,
		Count:     1,
	})
	c.Assert(err, qt.IsNil)

	// Plant the acquisition columns the way the worker eventually will,
	// bypassing the API guard via the registry's internal UpdateWithUser
	// path.
	planted := decimal.NewFromInt(100)
	plantedCur := models.Currency("USD")
	memReg, ok := set.CommodityRegistry.(*memory.CommodityRegistry)
	c.Assert(ok, qt.IsTrue)
	stored, err := memReg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	stored.AcquisitionPrice = &planted
	stored.AcquisitionCurrency = &plantedCur
	_, err = memReg.Registry.UpdateWithUser(ctx, *stored)
	c.Assert(err, qt.IsNil)

	// User Update with a payload that tries to clear them.
	stored.Name = "renamed"
	stored.AcquisitionPrice = nil
	stored.AcquisitionCurrency = nil
	updated, err := set.CommodityRegistry.Update(ctx, *stored)
	c.Assert(err, qt.IsNil)
	c.Assert(updated.Name, qt.Equals, "renamed")
	c.Assert(updated.AcquisitionPrice, qt.IsNotNil)
	c.Assert(updated.AcquisitionPrice.String(), qt.Equals, "100")
	c.Assert(updated.AcquisitionCurrency, qt.IsNotNil)
	c.Assert(string(*updated.AcquisitionCurrency), qt.Equals, "USD")

	// User Update with a payload that tries to overwrite them with
	// something else: also ignored.
	bogus := decimal.NewFromInt(999)
	bogusCur := models.Currency("EUR")
	stored.AcquisitionPrice = &bogus
	stored.AcquisitionCurrency = &bogusCur
	updated2, err := set.CommodityRegistry.Update(ctx, *stored)
	c.Assert(err, qt.IsNil)
	c.Assert(updated2.AcquisitionPrice.String(), qt.Equals, "100")
	c.Assert(string(*updated2.AcquisitionCurrency), qt.Equals, "USD")
}
