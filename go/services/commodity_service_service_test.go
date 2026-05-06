package services_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// serviceServiceFixture mirrors loanServiceFixture for the in-service
// surface. Same FactorySet / context / events-registry shape.
type serviceServiceFixture struct {
	ctx        context.Context
	factory    *registry.FactorySet
	serviceSvc *services.CommodityServiceService
	loanSvc    *services.CommodityLoanService
	events     *memory.CommodityEventRegistry
}

func newServiceServiceFixture(c *qt.C) *serviceServiceFixture {
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

	eventReg := factorySet.CommodityEventRegistryFactory.CreateServiceRegistry()
	concrete, ok := eventReg.(*memory.CommodityEventRegistry)
	c.Assert(ok, qt.IsTrue)

	return &serviceServiceFixture{
		ctx:        ctx,
		factory:    factorySet,
		serviceSvc: services.NewCommodityServiceService(factorySet),
		loanSvc:    services.NewCommodityLoanService(factorySet),
		events:     concrete,
	}
}

func (f *serviceServiceFixture) sendForService(c *qt.C) *models.CommodityService {
	c.Helper()
	svc := models.CommodityService{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		CommodityID:  "c-1",
		ProviderName: "Apple Service",
		Reason:       "screen replacement",
		SentAt:       models.Date("2026-05-01"),
	}
	created, existing, crossHolding, err := f.serviceSvc.StartService(f.ctx, svc)
	c.Assert(err, qt.IsNil)
	c.Assert(existing, qt.IsNil)
	c.Assert(crossHolding, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	return created
}

func TestCommodityServiceService_StartService_EmitsSentForService(t *testing.T) {
	c := qt.New(t)
	fx := newServiceServiceFixture(c)

	created := fx.sendForService(c)

	events, err := fx.events.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindSentForService)
	c.Assert(events[0].CommodityID, qt.Equals, "c-1")
	c.Assert(events[0].After["service_id"], qt.Equals, created.ID)
	c.Assert(events[0].After["provider_name"], qt.Equals, "Apple Service")
	c.Assert(events[0].After["reason"], qt.Equals, "screen replacement")
}

func TestCommodityServiceService_MarkReturned_EmitsBackFromService(t *testing.T) {
	c := qt.New(t)
	fx := newServiceServiceFixture(c)

	created := fx.sendForService(c)

	final, err := fx.serviceSvc.MarkReturned(fx.ctx, created.ID, nil, nil, nil)
	c.Assert(err, qt.IsNil)
	c.Assert(final.IsOpen(), qt.IsFalse)

	events, err := fx.events.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 2)
	kinds := []models.CommodityEventKind{events[0].Kind, events[1].Kind}
	c.Assert(kinds, qt.Contains, models.CommodityEventKindSentForService)
	c.Assert(kinds, qt.Contains, models.CommodityEventKindBackFromService)
}

func TestCommodityServiceService_MarkReturned_RecordsFinalCost(t *testing.T) {
	c := qt.New(t)
	fx := newServiceServiceFixture(c)

	created := fx.sendForService(c)

	cost := decimal.NewFromInt(245)
	currency := "EUR"
	final, err := fx.serviceSvc.MarkReturned(fx.ctx, created.ID, nil, &cost, &currency)
	c.Assert(err, qt.IsNil)
	c.Assert(final.CostAmount.Equal(cost), qt.IsTrue)
	c.Assert(final.CostCurrency, qt.Equals, "EUR")
}

func TestCommodityServiceService_UpdateService_EmitsServiceUpdated(t *testing.T) {
	c := qt.New(t)
	fx := newServiceServiceFixture(c)

	created := fx.sendForService(c)

	newReason := "diagnostic + screen"
	_, err := fx.serviceSvc.UpdateService(fx.ctx, created.ID, services.ServiceUpdate{
		Reason: &newReason,
	})
	c.Assert(err, qt.IsNil)

	events, err := fx.events.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 2)
	updates := 0
	for _, ev := range events {
		if ev.Kind == models.CommodityEventKindServiceUpdated {
			updates++
		}
	}
	c.Assert(updates, qt.Equals, 1)
}

func TestCommodityServiceService_UpdateService_NoOpDoesNotEmit(t *testing.T) {
	c := qt.New(t)
	fx := newServiceServiceFixture(c)

	created := fx.sendForService(c)

	sameReason := created.Reason
	_, err := fx.serviceSvc.UpdateService(fx.ctx, created.ID, services.ServiceUpdate{
		Reason: &sameReason,
	})
	c.Assert(err, qt.IsNil)

	events, err := fx.events.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindSentForService)
}

func TestCommodityServiceService_StartService_RejectsSecondOpen(t *testing.T) {
	c := qt.New(t)
	fx := newServiceServiceFixture(c)

	first := fx.sendForService(c)

	second := models.CommodityService{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		CommodityID:  "c-1",
		ProviderName: "Bob's Repair Shop",
		SentAt:       models.Date("2026-05-05"),
	}
	created, existing, crossHolding, err := fx.serviceSvc.StartService(fx.ctx, second)
	c.Assert(errors.Is(err, services.ErrServiceAlreadyOpen), qt.IsTrue)
	c.Assert(created, qt.IsNil)
	c.Assert(crossHolding, qt.IsNil)
	c.Assert(existing, qt.IsNotNil)
	c.Assert(existing.ID, qt.Equals, first.ID)
}

// TestCrossKindInvariant_LendBlocksService locks the cross-kind
// invariant: if a commodity is currently lent out, a send-for-service
// must fail with ErrCommodityAlreadyOut + the existing loan as the
// holding payload (so the FE can render "already lent to X").
func TestCrossKindInvariant_LendBlocksService(t *testing.T) {
	c := qt.New(t)
	fx := newServiceServiceFixture(c)

	loan := models.CommodityLoan{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		CommodityID:  "c-1",
		BorrowerName: "Alice",
		LentAt:       models.Date("2026-05-01"),
	}
	createdLoan, _, _, err := fx.loanSvc.StartLoan(fx.ctx, loan)
	c.Assert(err, qt.IsNil)

	svc := models.CommodityService{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		CommodityID:  "c-1",
		ProviderName: "Apple Service",
		SentAt:       models.Date("2026-05-02"),
	}
	created, existing, crossHolding, err := fx.serviceSvc.StartService(fx.ctx, svc)
	c.Assert(errors.Is(err, services.ErrCommodityAlreadyOut), qt.IsTrue)
	c.Assert(created, qt.IsNil)
	c.Assert(existing, qt.IsNil)
	c.Assert(crossHolding, qt.IsNotNil)
	c.Assert(crossHolding.Kind, qt.Equals, services.HoldingKindLoan)
	c.Assert(crossHolding.ID, qt.Equals, createdLoan.ID)
	c.Assert(crossHolding.PartyName, qt.Equals, "Alice")
}

// TestCrossKindInvariant_ServiceBlocksLend is the symmetric path: a
// commodity currently in service must reject a Lend with the open
// service as the cross-holding payload.
func TestCrossKindInvariant_ServiceBlocksLend(t *testing.T) {
	c := qt.New(t)
	fx := newServiceServiceFixture(c)

	createdSvc := fx.sendForService(c)

	loan := models.CommodityLoan{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		CommodityID:  "c-1",
		BorrowerName: "Alice",
		LentAt:       models.Date("2026-05-02"),
	}
	created, existing, crossHolding, err := fx.loanSvc.StartLoan(fx.ctx, loan)
	c.Assert(errors.Is(err, services.ErrCommodityAlreadyOut), qt.IsTrue)
	c.Assert(created, qt.IsNil)
	c.Assert(existing, qt.IsNil)
	c.Assert(crossHolding, qt.IsNotNil)
	c.Assert(crossHolding.Kind, qt.Equals, services.HoldingKindService)
	c.Assert(crossHolding.ID, qt.Equals, createdSvc.ID)
	c.Assert(crossHolding.PartyName, qt.Equals, "Apple Service")
}

// TestCrossKindInvariant_AllowedAfterClose verifies that closing one
// holding releases the commodity for the other kind. The cross-kind
// guard must NOT permanently block — it only matters while a row is
// open.
func TestCrossKindInvariant_AllowedAfterClose(t *testing.T) {
	c := qt.New(t)
	fx := newServiceServiceFixture(c)

	createdSvc := fx.sendForService(c)

	// Close the service.
	_, err := fx.serviceSvc.MarkReturned(fx.ctx, createdSvc.ID, nil, nil, nil)
	c.Assert(err, qt.IsNil)

	// Now a lend should succeed.
	loan := models.CommodityLoan{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		CommodityID:  "c-1",
		BorrowerName: "Alice",
		LentAt:       models.Date("2026-05-15"),
	}
	created, _, _, err := fx.loanSvc.StartLoan(fx.ctx, loan)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
}
