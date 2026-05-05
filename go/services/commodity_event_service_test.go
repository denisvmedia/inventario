package services_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// newEventTestContext builds a user + group context against an in-memory
// FactorySet so the CommodityEventService can construct an RLS-scoped
// registry on each emit. Returns the ctx, the factory set, and a
// service-mode events registry the test can poll for the rows that
// landed.
func newEventTestContext(c *qt.C) (context.Context, *services.CommodityEventService, *memory.CommodityEventRegistry) {
	c.Helper()

	factorySet := memory.NewFactorySet()
	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
		Email: "u@example.com",
		Name:  "Tester",
	}
	ctx := appctx.WithUser(context.Background(), user)
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "tenant-1",
		},
	})

	svc := services.NewCommodityEventService(factorySet)

	// Service-mode read avoids the user-aware filter so the test can see
	// every event regardless of who emitted it.
	reg := factorySet.CommodityEventRegistryFactory.CreateServiceRegistry()
	concrete, ok := reg.(*memory.CommodityEventRegistry)
	c.Assert(ok, qt.IsTrue)
	return ctx, svc, concrete
}

func makeCommodity(id string, mutators ...func(*models.Commodity)) *models.Commodity {
	c := &models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: id},
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		Name:                  "Test Item",
		AreaID:                "area-1",
		Type:                  models.CommodityTypeOther,
		Status:                models.CommodityStatusInUse,
		Count:                 1,
		OriginalPriceCurrency: "USD",
		OriginalPrice:         decimal.NewFromInt(100),
	}
	for _, m := range mutators {
		m(c)
	}
	return c
}

func TestCommodityEventService_EmitCreated(t *testing.T) {
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	commodity := makeCommodity("c1")
	svc.EmitCreated(ctx, commodity)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindCreated)
	c.Assert(events[0].CommodityID, qt.Equals, "c1")
	c.Assert(events[0].Before, qt.IsNil)
	c.Assert(events[0].After["name"], qt.Equals, "Test Item")
}

func TestCommodityEventService_EmitDeleted(t *testing.T) {
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	commodity := makeCommodity("c1")
	svc.EmitDeleted(ctx, commodity)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindDeleted)
	c.Assert(events[0].After, qt.IsNil)
}

func TestCommodityEventService_EmitUpdated_NoChange(t *testing.T) {
	// When the before / after snapshots are identical (the user saved a
	// row without editing anything), the service must not emit anything.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeCommodity("c1")
	after := makeCommodity("c1")
	svc.EmitUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 0)
}

func TestCommodityEventService_EmitUpdated_StatusOnly(t *testing.T) {
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeCommodity("c1")
	after := makeCommodity("c1", func(c *models.Commodity) {
		c.Status = models.CommodityStatusSold
	})
	svc.EmitUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindStatusChanged)
	c.Assert(events[0].Before["status"], qt.Equals, "in_use")
	c.Assert(events[0].After["status"], qt.Equals, "sold")
}

func TestCommodityEventService_EmitUpdated_MovedAndPrice(t *testing.T) {
	// Multiple aspects shifting in a single write fan out to multiple
	// events so the timeline keeps each meaningful change as its own row.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeCommodity("c1")
	after := makeCommodity("c1", func(c *models.Commodity) {
		c.AreaID = "area-2"
		c.CurrentPrice = decimal.NewFromInt(50)
	})
	svc.EmitUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 2)
	kinds := []models.CommodityEventKind{events[0].Kind, events[1].Kind}
	c.Assert(kinds, qt.Contains, models.CommodityEventKindMoved)
	c.Assert(kinds, qt.Contains, models.CommodityEventKindPriceChanged)
}

func TestCommodityEventService_EmitUpdated_GenericUpdate(t *testing.T) {
	// Editing fields outside the specific-kind set (name, comments, etc.)
	// surfaces as a generic "updated" row.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeCommodity("c1")
	after := makeCommodity("c1", func(c *models.Commodity) {
		c.Name = "Renamed"
		c.Comments = "now with notes"
	})
	svc.EmitUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindUpdated)
}

func TestCommodityEventService_EmitUpdated_CoverChangedTakesPrecedence(t *testing.T) {
	// Cover-only edits emit a `cover_changed` row, not a generic update.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	id := "f1"
	before := makeCommodity("c1")
	after := makeCommodity("c1", func(c *models.Commodity) {
		c.CoverFileID = &id
	})
	svc.EmitUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindCoverChanged)
	c.Assert(events[0].After["cover_file_id"], qt.Equals, "f1")
}
