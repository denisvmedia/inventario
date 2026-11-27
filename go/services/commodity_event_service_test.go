package services_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
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
		AreaID:                new("area-1"),
		Type:                  models.CommodityTypeOther,
		Status:                models.CommodityStatusInUse,
		Count:                 1,
		OriginalPriceCurrency: "USD",
		OriginalPrice:         new(decimal.NewFromInt(100)),
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
		c.AreaID = new("area-2")
		c.CurrentPrice = new(decimal.NewFromInt(50))
	})
	svc.EmitUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 2)
	kinds := []models.CommodityEventKind{events[0].Kind, events[1].Kind}
	c.Assert(kinds, qt.Contains, models.CommodityEventKindMoved)
	c.Assert(kinds, qt.Contains, models.CommodityEventKindPriceChanged)
}

func TestCommodityEventService_EmitUpdated_UnknownToZeroIsAPriceChange(t *testing.T) {
	// Learning that an item was free turns an unknown price into a known
	// zero. That is a price change, and the payload keeps the two apart.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeCommodity("c1", func(c *models.Commodity) {
		c.OriginalPrice = nil
		c.CurrentPrice = nil
	})
	after := makeCommodity("c1", func(c *models.Commodity) {
		c.OriginalPrice = new(decimal.Zero)
		c.CurrentPrice = nil
	})
	svc.EmitUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindPriceChanged)
	c.Assert(events[0].Before["original_price"], qt.IsNil)
	c.Assert(events[0].After["original_price"], qt.Equals, "0")
	c.Assert(events[0].After["current_price"], qt.IsNil)
}

func TestCommodityEventService_EmitUpdated_Unassign(t *testing.T) {
	// Issue #1986: clearing the area (A → nil) is a move and surfaces a
	// `moved` event, with the after payload carrying an empty area_id.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeCommodity("c1") // filed under "area-1"
	after := makeCommodity("c1", func(c *models.Commodity) {
		c.AreaID = nil
	})
	svc.EmitUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindMoved)
	c.Assert(events[0].After["area_id"], qt.Equals, "")
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

func TestCommodityEventService_EmitUpdated_BrandAndModel(t *testing.T) {
	// The brand and the model are ordinary fields: changing either one is a
	// generic update, and the snapshot carries both.
	for _, tc := range []struct {
		name   string
		mutate func(*models.Commodity)
	}{
		{name: "brand", mutate: func(c *models.Commodity) { c.Brand = "Bosch" }},
		{name: "model", mutate: func(c *models.Commodity) { c.Model = "GSR 12V-15" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx, svc, reg := newEventTestContext(c)

			before := makeCommodity("c1")
			after := makeCommodity("c1", tc.mutate)
			svc.EmitUpdated(ctx, before, after)

			events, err := reg.List(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(events, qt.HasLen, 1)
			c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindUpdated)
			c.Check(events[0].After["brand"], qt.Equals, after.Brand)
			c.Check(events[0].After["model"], qt.Equals, after.Model)
		})
	}
}

func TestCommodityEventService_EmitUpdated_SellerOnly(t *testing.T) {
	// Picking, changing or clearing the seller alone is a generic update,
	// and the snapshot carries the seller ids so the History can name them.
	sellerA, sellerB := "seller-a", "seller-b"
	for _, tc := range []struct {
		name          string
		before, after *string
		wantBefore    string
		wantAfter     string
	}{
		{name: "set", before: nil, after: &sellerA, wantBefore: "", wantAfter: sellerA},
		{name: "changed", before: &sellerA, after: &sellerB, wantBefore: sellerA, wantAfter: sellerB},
		{name: "cleared", before: &sellerA, after: nil, wantBefore: sellerA, wantAfter: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx, svc, reg := newEventTestContext(c)

			before := makeCommodity("c1", func(c *models.Commodity) { c.SellerID = tc.before })
			after := makeCommodity("c1", func(c *models.Commodity) { c.SellerID = tc.after })
			svc.EmitUpdated(ctx, before, after)

			events, err := reg.List(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(events, qt.HasLen, 1)
			c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindUpdated)
			c.Check(events[0].Before["seller_id"], qt.Equals, tc.wantBefore)
			c.Check(events[0].After["seller_id"], qt.Equals, tc.wantAfter)
		})
	}
}

func TestCommodityEventService_EmitUpdated_SnapshotCarriesEveryComparedField(t *testing.T) {
	// The snapshot of a generic update carries every field the change check
	// compares, so a seller change made together with a date or link change
	// does not read as a seller-only edit in the timeline.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	seller := "seller-a"
	purchased := models.Date("2024-05-01")
	registered := models.Date("2024-05-02")
	link := must.Must(models.URLParse("https://shop.example/kettle"))
	before := makeCommodity("c1")
	after := makeCommodity("c1", func(c *models.Commodity) {
		c.SellerID = &seller
		c.PurchaseDate = &purchased
		c.RegisteredDate = &registered
		c.URLs = models.ValuerSlice[*models.URL]{link}
	})
	svc.EmitUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindUpdated)
	c.Check(events[0].Before["purchase_date"], qt.Equals, "")
	c.Check(events[0].After["purchase_date"], qt.Equals, "2024-05-01")
	c.Check(events[0].Before["registered_date"], qt.Equals, "")
	c.Check(events[0].After["registered_date"], qt.Equals, "2024-05-02")
	c.Check(events[0].Before["urls"], qt.IsNil)
	c.Check(events[0].After["urls"], qt.DeepEquals, []string{"https://shop.example/kettle"})
	c.Check(events[0].After["seller_id"], qt.Equals, seller)
}

func TestCommodityEventService_EmitUpdated_EmptyListsMatchMissingOnes(t *testing.T) {
	// An empty list and a missing one are the same to the change check, and
	// the snapshot stores both the same way.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeCommodity("c1", func(c *models.Commodity) { c.Tags = models.ValuerSlice[string]{} })
	after := makeCommodity("c1", func(c *models.Commodity) { c.Name = "Renamed" })
	svc.EmitUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Check(events[0].Before["tags"], qt.IsNil)
	c.Check(events[0].After["tags"], qt.IsNil)
	c.Check(events[0].Before["urls"], qt.IsNil)
}

func TestCommodityEventService_EmitUpdated_SameSeller(t *testing.T) {
	// The same seller saved again under a different pointer is no change.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	a, b := "seller-a", "seller-a"
	before := makeCommodity("c1", func(c *models.Commodity) { c.SellerID = &a })
	after := makeCommodity("c1", func(c *models.Commodity) { c.SellerID = &b })
	svc.EmitUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 0)
}

func makeLoan(id, commodityID string, mutators ...func(*models.CommodityLoan)) *models.CommodityLoan {
	due := models.Date("2026-06-01")
	l := &models.CommodityLoan{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: id},
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		CommodityID:     commodityID,
		BorrowerName:    "Alice",
		BorrowerContact: "alice@example.com",
		LentAt:          models.Date("2026-05-01"),
		DueBackAt:       &due,
	}
	for _, m := range mutators {
		m(l)
	}
	return l
}

func TestCommodityEventService_EmitLoanStarted(t *testing.T) {
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	loan := makeLoan("loan-1", "c1")
	svc.EmitLoanStarted(ctx, loan)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindLentOut)
	c.Assert(events[0].CommodityID, qt.Equals, "c1")
	c.Assert(events[0].Before, qt.IsNil)
	c.Assert(events[0].After["loan_id"], qt.Equals, "loan-1")
	c.Assert(events[0].After["borrower_name"], qt.Equals, "Alice")
	c.Assert(events[0].After["lent_at"], qt.Equals, "2026-05-01")
	c.Assert(events[0].After["due_back_at"], qt.Equals, "2026-06-01")
	c.Assert(events[0].After["borrower_contact"], qt.Equals, "alice@example.com")
}

func TestCommodityEventService_EmitLoanStarted_DropsEmptyOptionals(t *testing.T) {
	// Empty-string contact / note / nil due-back must not appear in the
	// JSONB payload — keeps the row minimal and avoids "" sneaking into
	// FE conditionals that test for presence.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	loan := makeLoan("loan-1", "c1", func(l *models.CommodityLoan) {
		l.BorrowerContact = ""
		l.BorrowerNote = ""
		l.DueBackAt = nil
	})
	svc.EmitLoanStarted(ctx, loan)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	_, hasContact := events[0].After["borrower_contact"]
	_, hasNote := events[0].After["borrower_note"]
	_, hasDue := events[0].After["due_back_at"]
	c.Assert(hasContact, qt.IsFalse)
	c.Assert(hasNote, qt.IsFalse)
	c.Assert(hasDue, qt.IsFalse)
}

func TestCommodityEventService_EmitLoanReturned(t *testing.T) {
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	returned := models.Date("2026-05-15")
	loan := makeLoan("loan-1", "c1", func(l *models.CommodityLoan) {
		l.ReturnedAt = &returned
	})
	svc.EmitLoanReturned(ctx, loan)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindReturned)
	c.Assert(events[0].After["loan_id"], qt.Equals, "loan-1")
	c.Assert(events[0].After["returned_at"], qt.Equals, "2026-05-15")
	c.Assert(events[0].Before, qt.IsNil)
}

func TestCommodityEventService_EmitLoanUpdated_NoChange(t *testing.T) {
	// Saving a loan with the same field values must not emit — same
	// idempotency gate as commodity EmitUpdated.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeLoan("loan-1", "c1")
	after := makeLoan("loan-1", "c1")
	svc.EmitLoanUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 0)
}

func TestCommodityEventService_EmitLoanUpdated_FieldChange(t *testing.T) {
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeLoan("loan-1", "c1")
	after := makeLoan("loan-1", "c1", func(l *models.CommodityLoan) {
		l.BorrowerContact = "alice@new.example.com"
		l.BorrowerNote = "back office"
	})
	svc.EmitLoanUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindLoanUpdated)
	c.Assert(events[0].Before["borrower_contact"], qt.Equals, "alice@example.com")
	c.Assert(events[0].After["borrower_contact"], qt.Equals, "alice@new.example.com")
	c.Assert(events[0].After["borrower_note"], qt.Equals, "back office")
}

func TestCommodityEventService_EmitLoanUpdated_DropsEmptyOptionals(t *testing.T) {
	// Sparse-payload guarantee: empty borrower_contact / borrower_note
	// and nil due_back_at must NOT appear as keys on either side of
	// the diff. Same shape as snapshotLoanLifecycle so the FE's
	// "key in payload" semantics stay consistent across kinds.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeLoan("loan-1", "c1", func(l *models.CommodityLoan) {
		l.BorrowerContact = ""
		l.BorrowerNote = ""
		l.DueBackAt = nil
	})
	after := makeLoan("loan-1", "c1", func(l *models.CommodityLoan) {
		l.BorrowerContact = ""
		l.BorrowerNote = "back office"
		l.DueBackAt = nil
	})
	svc.EmitLoanUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)

	_, beforeHasContact := events[0].Before["borrower_contact"]
	_, beforeHasNote := events[0].Before["borrower_note"]
	_, beforeHasDue := events[0].Before["due_back_at"]
	c.Assert(beforeHasContact, qt.IsFalse)
	c.Assert(beforeHasNote, qt.IsFalse)
	c.Assert(beforeHasDue, qt.IsFalse)

	_, afterHasContact := events[0].After["borrower_contact"]
	_, afterHasDue := events[0].After["due_back_at"]
	c.Assert(afterHasContact, qt.IsFalse)
	c.Assert(afterHasDue, qt.IsFalse)
	// borrower_note is non-empty on the `after` side and must be present.
	c.Assert(events[0].After["borrower_note"], qt.Equals, "back office")
}

func TestCommodityEventService_EmitLoanUpdated_DueBackChange(t *testing.T) {
	// due_back_at shifting between two non-nil dates must trigger an
	// event — covers the equalPDate path used by loanFieldsChanged.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeLoan("loan-1", "c1")
	newDue := models.Date("2026-07-01")
	after := makeLoan("loan-1", "c1", func(l *models.CommodityLoan) {
		l.DueBackAt = &newDue
	})
	svc.EmitLoanUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindLoanUpdated)
	c.Assert(events[0].Before["due_back_at"], qt.Equals, "2026-06-01")
	c.Assert(events[0].After["due_back_at"], qt.Equals, "2026-07-01")
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

// makeService returns a fully-populated CommodityService fixture so the
// EmitService* tests can mutate just the field they're exercising.
// Mirrors makeLoan above.
func makeService(id, commodityID string, mutators ...func(*models.CommodityService)) *models.CommodityService {
	due := models.Date("2026-06-01")
	svc := &models.CommodityService{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: id},
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		CommodityID:      commodityID,
		ProviderName:     "Apple Service",
		ProviderContact:  "+1 800-275-2273",
		Reason:           "screen replacement",
		SentAt:           models.Date("2026-05-01"),
		ExpectedReturnAt: &due,
	}
	for _, m := range mutators {
		m(svc)
	}
	return svc
}

func TestCommodityEventService_EmitServiceStarted(t *testing.T) {
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	row := makeService("svc-1", "c1")
	svc.EmitServiceStarted(ctx, row)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindSentForService)
	c.Assert(events[0].CommodityID, qt.Equals, "c1")
	c.Assert(events[0].Before, qt.IsNil)
	c.Assert(events[0].After["service_id"], qt.Equals, "svc-1")
	c.Assert(events[0].After["provider_name"], qt.Equals, "Apple Service")
	c.Assert(events[0].After["sent_at"], qt.Equals, "2026-05-01")
	c.Assert(events[0].After["expected_return_at"], qt.Equals, "2026-06-01")
	c.Assert(events[0].After["provider_contact"], qt.Equals, "+1 800-275-2273")
	c.Assert(events[0].After["reason"], qt.Equals, "screen replacement")
}

func TestCommodityEventService_EmitServiceStarted_DropsEmptyOptionals(t *testing.T) {
	// Empty contact / reason / nil expected-return must not appear in
	// the JSONB payload — same sparse-payload discipline as
	// EmitLoanStarted_DropsEmptyOptionals.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	row := makeService("svc-1", "c1", func(s *models.CommodityService) {
		s.ProviderContact = ""
		s.Reason = ""
		s.ExpectedReturnAt = nil
	})
	svc.EmitServiceStarted(ctx, row)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	_, hasContact := events[0].After["provider_contact"]
	_, hasReason := events[0].After["reason"]
	_, hasExpected := events[0].After["expected_return_at"]
	c.Assert(hasContact, qt.IsFalse)
	c.Assert(hasReason, qt.IsFalse)
	c.Assert(hasExpected, qt.IsFalse)
}

func TestCommodityEventService_EmitServiceStarted_IncludesCostWhenSet(t *testing.T) {
	// When the caller records a cost on the lifecycle event, both
	// cost_amount and cost_currency must land in the JSONB payload so
	// the timeline can render "Cost: 245 EUR" without a join.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	row := makeService("svc-1", "c1", func(s *models.CommodityService) {
		s.CostAmount = decimal.NewFromInt(245)
		s.CostCurrency = "EUR"
	})
	svc.EmitServiceStarted(ctx, row)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].After["cost_amount"], qt.Equals, "245")
	c.Assert(events[0].After["cost_currency"], qt.Equals, "EUR")
}

func TestCommodityEventService_EmitServiceReturned(t *testing.T) {
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	returned := models.Date("2026-05-15")
	row := makeService("svc-1", "c1", func(s *models.CommodityService) {
		s.ReturnedAt = &returned
	})
	svc.EmitServiceReturned(ctx, row)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindBackFromService)
	c.Assert(events[0].After["service_id"], qt.Equals, "svc-1")
	c.Assert(events[0].After["returned_at"], qt.Equals, "2026-05-15")
	c.Assert(events[0].Before, qt.IsNil)
}

func TestCommodityEventService_EmitServiceUpdated_NoChange(t *testing.T) {
	// Saving a service row with the same field values must not emit —
	// same idempotency gate as EmitLoanUpdated_NoChange.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeService("svc-1", "c1")
	after := makeService("svc-1", "c1")
	svc.EmitServiceUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 0)
}

func TestCommodityEventService_EmitServiceUpdated_FieldChange(t *testing.T) {
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeService("svc-1", "c1")
	after := makeService("svc-1", "c1", func(s *models.CommodityService) {
		s.Reason = "diagnostic + screen"
		s.ProviderContact = "service@example.com"
	})
	svc.EmitServiceUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindServiceUpdated)
	c.Assert(events[0].Before["reason"], qt.Equals, "screen replacement")
	c.Assert(events[0].After["reason"], qt.Equals, "diagnostic + screen")
	c.Assert(events[0].After["provider_contact"], qt.Equals, "service@example.com")
}

func TestCommodityEventService_EmitServiceUpdated_CostPairChange(t *testing.T) {
	// A change to either half of the cost pair (amount or currency)
	// triggers the diff gate. Verifies serviceFieldsChanged sees the
	// pair as a unit.
	c := qt.New(t)
	ctx, svc, reg := newEventTestContext(c)

	before := makeService("svc-1", "c1")
	after := makeService("svc-1", "c1", func(s *models.CommodityService) {
		s.CostAmount = decimal.NewFromInt(245)
		s.CostCurrency = "EUR"
	})
	svc.EmitServiceUpdated(ctx, before, after)

	events, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindServiceUpdated)
	c.Assert(events[0].After["cost_amount"], qt.Equals, "245")
	c.Assert(events[0].After["cost_currency"], qt.Equals, "EUR")
}

// specificEventFields are the payload keys of the specific events. The
// "updated" event of the same save leaves them out, so a change is never
// recorded twice.
var specificEventFields = []string{
	"status", "area_id", "original_price", "original_price_currency",
	"converted_original_price", "current_price", "cover_file_id",
}

// A save that changes a field a specific event covers and a generic field
// records both: the specific event and an "updated" event with the generic
// fields.
func TestCommodityEventService_EmitUpdated_SpecificAndGenericInOneSave(t *testing.T) {
	cover := "f1"
	tests := []struct {
		name         string
		mutate       func(*models.Commodity)
		wantSpecific models.CommodityEventKind
		field        string
		before       any
		after        any
	}{
		{
			name: "status and type",
			mutate: func(c *models.Commodity) {
				c.Status = models.CommodityStatusSold
				c.Type = models.CommodityTypeElectronics
			},
			wantSpecific: models.CommodityEventKindStatusChanged,
			field:        "type", before: "other", after: "electronics",
		},
		{
			name: "area and name",
			mutate: func(c *models.Commodity) {
				c.AreaID = new("area-2")
				c.Name = "Renamed Item"
			},
			wantSpecific: models.CommodityEventKindMoved,
			field:        "name", before: "Test Item", after: "Renamed Item",
		},
		{
			name: "price and brand",
			mutate: func(c *models.Commodity) {
				c.CurrentPrice = new(decimal.NewFromInt(50))
				c.Brand = "Acme"
			},
			wantSpecific: models.CommodityEventKindPriceChanged,
			field:        "brand", before: "", after: "Acme",
		},
		{
			name: "cover and comments",
			mutate: func(c *models.Commodity) {
				c.CoverFileID = &cover
				c.Comments = "kept in the box"
			},
			wantSpecific: models.CommodityEventKindCoverChanged,
			field:        "comments", before: "", after: "kept in the box",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx, svc, reg := newEventTestContext(c)

			svc.EmitUpdated(ctx, makeCommodity("c1"), makeCommodity("c1", tc.mutate))

			events, err := reg.List(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(events, qt.HasLen, 2)
			byKind := map[models.CommodityEventKind]*models.CommodityEvent{}
			for _, e := range events {
				byKind[e.Kind] = e
			}
			c.Assert(byKind[tc.wantSpecific], qt.IsNotNil)
			updated := byKind[models.CommodityEventKindUpdated]
			c.Assert(updated, qt.IsNotNil)
			c.Assert(updated.Before[tc.field], qt.Equals, tc.before)
			c.Assert(updated.After[tc.field], qt.Equals, tc.after)
			for _, key := range specificEventFields {
				_, inBefore := updated.Before[key]
				_, inAfter := updated.After[key]
				c.Assert(inBefore || inAfter, qt.IsFalse, qt.Commentf("key %s", key))
			}
		})
	}
}
