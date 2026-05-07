package services_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// loanServiceFixture wires an in-memory FactorySet plus a user/group
// context so the CommodityLoanService can run end-to-end. The events
// registry returned alongside is service-mode (no user filter) so the
// test can poll for whatever the service emitted regardless of who
// triggered the call.
type loanServiceFixture struct {
	ctx     context.Context
	factory *registry.FactorySet
	loanSvc *services.CommodityLoanService
	events  *memory.CommodityEventRegistry
}

func newLoanServiceFixture(c *qt.C) *loanServiceFixture {
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

	return &loanServiceFixture{
		ctx:     ctx,
		factory: factorySet,
		loanSvc: services.NewCommodityLoanService(factorySet),
		events:  concrete,
	}
}

func (f *loanServiceFixture) startLoan(c *qt.C) *models.CommodityLoan {
	c.Helper()
	loan := models.CommodityLoan{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		CommodityID:     "c-1",
		BorrowerName:    "Alice",
		BorrowerContact: "alice@example.com",
		LentAt:          models.Date("2026-05-01"),
	}
	created, existing, crossHolding, err := f.loanSvc.StartLoan(f.ctx, loan)
	c.Assert(err, qt.IsNil)
	c.Assert(existing, qt.IsNil)
	c.Assert(crossHolding, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	return created
}

// TestCommodityLoanService_StartLoan_RejectsBundle locks the issue
// #1554 invariant: a commodity with Count > 1 cannot be lent out —
// the row models a bag of interchangeable units, not a single
// instance with a borrower. StartLoan must reject with the shared
// ErrCommodityNotTrackable sentinel before touching the loan
// registry, so apiserver maps the response to 422.
func TestCommodityLoanService_StartLoan_RejectsBundle(t *testing.T) {
	c := qt.New(t)
	fx := newLoanServiceFixture(c)

	// Seed a Count > 1 commodity through the user registry. Memory
	// registries don't enforce model validation on Create, which is
	// what we want here — the legacy "bundle has warranty / loan /
	// service" rows that #1554's migration deliberately leaves alone
	// look exactly like this.
	commodityReg, err := fx.factory.CommodityRegistryFactory.CreateUserRegistry(fx.ctx)
	c.Assert(err, qt.IsNil)
	bundle, err := commodityReg.Create(fx.ctx, models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: "bundle-1"},
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		Name:      "12 light bulbs",
		ShortName: "bulbs",
		Type:      models.CommodityTypeOther,
		Status:    models.CommodityStatusInUse,
		Count:     12,
	})
	c.Assert(err, qt.IsNil)

	loan := models.CommodityLoan{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		CommodityID:  bundle.ID,
		BorrowerName: "Alice",
		LentAt:       models.Date("2026-05-01"),
	}
	created, existing, crossHolding, err := fx.loanSvc.StartLoan(fx.ctx, loan)
	c.Assert(err, qt.IsNotNil)
	c.Assert(errors.Is(err, services.ErrCommodityNotTrackable), qt.IsTrue,
		qt.Commentf("StartLoan must reject a Count>1 commodity with the shared sentinel"))
	c.Assert(created, qt.IsNil)
	c.Assert(existing, qt.IsNil)
	c.Assert(crossHolding, qt.IsNil)

	// Audit timeline must stay clean — the loan was rejected before
	// the registry insert, so no `lent_out` event leaked.
	events, err := fx.events.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 0)
}

func TestCommodityLoanService_StartLoan_EmitsLentOut(t *testing.T) {
	c := qt.New(t)
	fx := newLoanServiceFixture(c)

	created := fx.startLoan(c)

	events, err := fx.events.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindLentOut)
	c.Assert(events[0].CommodityID, qt.Equals, "c-1")
	c.Assert(events[0].After["loan_id"], qt.Equals, created.ID)
	c.Assert(events[0].After["borrower_name"], qt.Equals, "Alice")
}

func TestCommodityLoanService_MarkReturned_EmitsReturned(t *testing.T) {
	c := qt.New(t)
	fx := newLoanServiceFixture(c)

	created := fx.startLoan(c)

	final, err := fx.loanSvc.MarkReturned(fx.ctx, created.ID, nil)
	c.Assert(err, qt.IsNil)
	c.Assert(final.IsOpen(), qt.IsFalse)

	events, err := fx.events.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 2)
	kinds := []models.CommodityEventKind{events[0].Kind, events[1].Kind}
	c.Assert(kinds, qt.Contains, models.CommodityEventKindLentOut)
	c.Assert(kinds, qt.Contains, models.CommodityEventKindReturned)
}

func TestCommodityLoanService_UpdateLoan_EmitsLoanUpdated(t *testing.T) {
	c := qt.New(t)
	fx := newLoanServiceFixture(c)

	created := fx.startLoan(c)

	newContact := "alice@new.example.com"
	_, err := fx.loanSvc.UpdateLoan(fx.ctx, created.ID, services.LoanUpdate{
		BorrowerContact: &newContact,
	})
	c.Assert(err, qt.IsNil)

	events, err := fx.events.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 2)
	updates := 0
	for _, ev := range events {
		if ev.Kind == models.CommodityEventKindLoanUpdated {
			updates++
		}
	}
	c.Assert(updates, qt.Equals, 1)
}

func TestCommodityLoanService_UpdateLoan_NoOpDoesNotEmit(t *testing.T) {
	// PATCH with no-op (a present pointer that matches the current
	// value) must not pollute the timeline. Locks the
	// loanFieldsChanged gate.
	c := qt.New(t)
	fx := newLoanServiceFixture(c)

	created := fx.startLoan(c)

	sameContact := created.BorrowerContact
	_, err := fx.loanSvc.UpdateLoan(fx.ctx, created.ID, services.LoanUpdate{
		BorrowerContact: &sameContact,
	})
	c.Assert(err, qt.IsNil)

	events, err := fx.events.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	// Only the original lent_out emit; no loan_updated row.
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindLentOut)
}
