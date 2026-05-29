package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

type serviceFixture struct {
	ctx     context.Context
	svcReg  registry.CommodityServiceRegistry
	groupID string
}

func newServiceFixture(c *qt.C, groupID string) serviceFixture {
	c.Helper()

	svcFactory := memory.NewCommodityServiceRegistryFactory()
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: groupID},
			TenantID: "tenant-1",
		},
		Slug: groupID,
	})

	return serviceFixture{
		ctx:     ctx,
		svcReg:  svcFactory.MustCreateUserRegistry(ctx),
		groupID: groupID,
	}
}

func makeService(commodityID, sentAt string, expectedReturn *string) models.CommodityService {
	svc := models.CommodityService{
		CommodityID:  commodityID,
		ProviderName: "Apple Service",
		SentAt:       models.Date(sentAt),
	}
	if expectedReturn != nil {
		d := models.Date(*expectedReturn)
		svc.ExpectedReturnAt = &d
	}
	return svc
}

func TestCommodityServiceRegistry_Memory_CreateAndGet(t *testing.T) {
	c := qt.New(t)
	fx := newServiceFixture(c, "group-1")

	created, err := fx.svcReg.Create(fx.ctx, makeService("commodity-1", "2026-05-01", nil))
	c.Assert(err, qt.IsNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.IsOpen(), qt.IsTrue)
	c.Assert(created.GroupID, qt.Equals, "group-1")

	got, err := fx.svcReg.Get(fx.ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, created.ID)
	c.Assert(got.ProviderName, qt.Equals, "Apple Service")
}

func TestCommodityServiceRegistry_Memory_GetOpenForCommodity(t *testing.T) {
	c := qt.New(t)
	fx := newServiceFixture(c, "group-1")

	_, err := fx.svcReg.GetOpenForCommodity(fx.ctx, "commodity-1")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	created, err := fx.svcReg.Create(fx.ctx, makeService("commodity-1", "2026-05-01", nil))
	c.Assert(err, qt.IsNil)
	open, err := fx.svcReg.GetOpenForCommodity(fx.ctx, "commodity-1")
	c.Assert(err, qt.IsNil)
	c.Assert(open.ID, qt.Equals, created.ID)

	closed := *created
	closed.ReturnedAt = func() models.PDate { d := models.Date("2026-05-10"); return &d }()
	_, err = fx.svcReg.Update(fx.ctx, closed)
	c.Assert(err, qt.IsNil)

	_, err = fx.svcReg.GetOpenForCommodity(fx.ctx, "commodity-1")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestCommodityServiceRegistry_Memory_ListByCommodity_OrdersDesc(t *testing.T) {
	c := qt.New(t)
	fx := newServiceFixture(c, "group-1")

	for _, d := range []string{"2026-04-01", "2026-04-15", "2026-05-01"} {
		_, err := fx.svcReg.Create(fx.ctx, makeService("commodity-1", d, nil))
		c.Assert(err, qt.IsNil)
	}
	_, err := fx.svcReg.Create(fx.ctx, makeService("commodity-2", "2026-04-20", nil))
	c.Assert(err, qt.IsNil)

	got, err := fx.svcReg.ListByCommodity(fx.ctx, "commodity-1")
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.HasLen, 3)
	c.Assert(string(got[0].SentAt), qt.Equals, "2026-05-01")
	c.Assert(string(got[1].SentAt), qt.Equals, "2026-04-15")
	c.Assert(string(got[2].SentAt), qt.Equals, "2026-04-01")
}

func TestCommodityServiceRegistry_Memory_ListPaginated_StateFilter(t *testing.T) {
	c := qt.New(t)
	fx := newServiceFixture(c, "group-1")

	pastDue := "2026-04-15"
	overdueSvc, err := fx.svcReg.Create(fx.ctx, makeService("commodity-1", "2026-04-01", &pastDue))
	c.Assert(err, qt.IsNil)

	future := "2026-06-01"
	_, err = fx.svcReg.Create(fx.ctx, makeService("commodity-2", "2026-05-01", &future))
	c.Assert(err, qt.IsNil)

	completed, err := fx.svcReg.Create(fx.ctx, makeService("commodity-3", "2026-04-15", nil))
	c.Assert(err, qt.IsNil)
	completedRet := *completed
	completedRet.ReturnedAt = func() models.PDate { d := models.Date("2026-05-05"); return &d }()
	_, err = fx.svcReg.Update(fx.ctx, completedRet)
	c.Assert(err, qt.IsNil)

	now := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

	all, total, err := fx.svcReg.ListPaginated(fx.ctx, 0, 10, registry.ServiceListOptions{Now: now})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 3)
	c.Assert(all, qt.HasLen, 3)

	open, total, err := fx.svcReg.ListPaginated(fx.ctx, 0, 10, registry.ServiceListOptions{State: registry.ServiceStateOpen, Now: now})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 2)
	c.Assert(open, qt.HasLen, 2)

	overdue, total, err := fx.svcReg.ListPaginated(fx.ctx, 0, 10, registry.ServiceListOptions{State: registry.ServiceStateOverdue, Now: now})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 1)
	c.Assert(overdue, qt.HasLen, 1)
	c.Assert(overdue[0].ID, qt.Equals, overdueSvc.ID)

	doneRows, total, err := fx.svcReg.ListPaginated(fx.ctx, 0, 10, registry.ServiceListOptions{State: registry.ServiceStateCompleted, Now: now})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 1)
	c.Assert(doneRows, qt.HasLen, 1)
	c.Assert(doneRows[0].ID, qt.Equals, completed.ID)
}

func TestCommodityServiceRegistry_Memory_CountOpenByCommodity(t *testing.T) {
	c := qt.New(t)
	fx := newServiceFixture(c, "group-1")

	_, err := fx.svcReg.Create(fx.ctx, makeService("commodity-1", "2026-05-01", nil))
	c.Assert(err, qt.IsNil)

	completed, err := fx.svcReg.Create(fx.ctx, makeService("commodity-2", "2026-04-15", nil))
	c.Assert(err, qt.IsNil)
	completed.ReturnedAt = func() models.PDate { d := models.Date("2026-04-30"); return &d }()
	_, err = fx.svcReg.Update(fx.ctx, *completed)
	c.Assert(err, qt.IsNil)

	counts, err := fx.svcReg.CountOpenByCommodity(fx.ctx, []string{"commodity-1", "commodity-2", "missing"})
	c.Assert(err, qt.IsNil)
	c.Assert(counts["commodity-1"], qt.Equals, 1)
	c.Assert(counts["commodity-2"], qt.Equals, 0)
	c.Assert(counts["missing"], qt.Equals, 0)
}

func TestCommodityServiceRegistry_Memory_GroupIsolation(t *testing.T) {
	c := qt.New(t)

	a := newServiceFixture(c, "group-A")
	b := newServiceFixture(c, "group-B")

	_, err := a.svcReg.Create(a.ctx, makeService("commodity-1", "2026-05-01", nil))
	c.Assert(err, qt.IsNil)

	bList, err := b.svcReg.List(b.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(bList, qt.HasLen, 0, qt.Commentf("services created in group-A must not be visible to group-B"))
}

func TestCommodityServiceModel_CostPairValidation(t *testing.T) {
	c := qt.New(t)

	// Both unset → valid.
	svc := models.CommodityService{
		CommodityID:  "c-1",
		ProviderName: "Apple Service",
		SentAt:       models.Date("2026-05-01"),
	}
	c.Assert(svc.ValidateWithContext(c.Context()), qt.IsNil)

	// Both set → valid.
	svc.CostAmount = decimal.NewFromInt(245)
	svc.CostCurrency = "EUR"
	c.Assert(svc.ValidateWithContext(c.Context()), qt.IsNil)

	// Amount only → invalid.
	svc.CostCurrency = ""
	c.Assert(svc.ValidateWithContext(c.Context()), qt.IsNotNil)

	// Currency only → invalid.
	svc.CostAmount = decimal.Zero
	svc.CostCurrency = "EUR"
	c.Assert(svc.ValidateWithContext(c.Context()), qt.IsNotNil)

	// Bogus currency → invalid (not ISO 4217).
	svc.CostAmount = decimal.NewFromInt(100)
	svc.CostCurrency = "XYZ"
	c.Assert(svc.ValidateWithContext(c.Context()), qt.IsNotNil)
}
