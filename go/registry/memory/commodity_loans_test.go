package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

type loanFixture struct {
	ctx     context.Context
	loanReg registry.CommodityLoanRegistry
	groupID string
}

func newLoanFixture(c *qt.C, groupID string) loanFixture {
	c.Helper()

	loanFactory := memory.NewCommodityLoanRegistryFactory()
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

	return loanFixture{
		ctx:     ctx,
		loanReg: loanFactory.MustCreateUserRegistry(ctx),
		groupID: groupID,
	}
}

func makeLoan(commodityID, lentAt string, dueBack *string) models.CommodityLoan {
	loan := models.CommodityLoan{
		CommodityID:  commodityID,
		BorrowerName: "Alice",
		LentAt:       models.Date(lentAt),
	}
	if dueBack != nil {
		d := models.Date(*dueBack)
		loan.DueBackAt = &d
	}
	return loan
}

func TestCommodityLoanRegistry_Memory_CreateAndGet(t *testing.T) {
	c := qt.New(t)
	fx := newLoanFixture(c, "group-1")

	created, err := fx.loanReg.Create(fx.ctx, makeLoan("commodity-1", "2026-05-01", nil))
	c.Assert(err, qt.IsNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.IsOpen(), qt.IsTrue)
	c.Assert(created.GroupID, qt.Equals, "group-1") // populated by CreateWithUser

	got, err := fx.loanReg.Get(fx.ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, created.ID)
	c.Assert(got.BorrowerName, qt.Equals, "Alice")
}

func TestCommodityLoanRegistry_Memory_GetOpenForCommodity(t *testing.T) {
	c := qt.New(t)
	fx := newLoanFixture(c, "group-1")

	// No loans yet → ErrNotFound
	_, err := fx.loanReg.GetOpenForCommodity(fx.ctx, "commodity-1")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)

	// Open loan → returned
	created, err := fx.loanReg.Create(fx.ctx, makeLoan("commodity-1", "2026-05-01", nil))
	c.Assert(err, qt.IsNil)
	open, err := fx.loanReg.GetOpenForCommodity(fx.ctx, "commodity-1")
	c.Assert(err, qt.IsNil)
	c.Assert(open.ID, qt.Equals, created.ID)

	// Close it → ErrNotFound again
	closed := *created
	closed.ReturnedAt = func() models.PDate { d := models.Date("2026-05-10"); return &d }()
	_, err = fx.loanReg.Update(fx.ctx, closed)
	c.Assert(err, qt.IsNil)

	_, err = fx.loanReg.GetOpenForCommodity(fx.ctx, "commodity-1")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestCommodityLoanRegistry_Memory_ListByCommodity_OrdersDesc(t *testing.T) {
	c := qt.New(t)
	fx := newLoanFixture(c, "group-1")

	// Three loans on the same commodity, two on a different one.
	for _, d := range []string{"2026-04-01", "2026-04-15", "2026-05-01"} {
		_, err := fx.loanReg.Create(fx.ctx, makeLoan("commodity-1", d, nil))
		c.Assert(err, qt.IsNil)
	}
	_, err := fx.loanReg.Create(fx.ctx, makeLoan("commodity-2", "2026-04-20", nil))
	c.Assert(err, qt.IsNil)

	got, err := fx.loanReg.ListByCommodity(fx.ctx, "commodity-1")
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.HasLen, 3)
	c.Assert(string(got[0].LentAt), qt.Equals, "2026-05-01")
	c.Assert(string(got[1].LentAt), qt.Equals, "2026-04-15")
	c.Assert(string(got[2].LentAt), qt.Equals, "2026-04-01")
}

func TestCommodityLoanRegistry_Memory_ListPaginated_StateFilter(t *testing.T) {
	c := qt.New(t)
	fx := newLoanFixture(c, "group-1")

	// Past due (will be overdue when scanned at 2026-05-10).
	pastDue := "2026-04-15"
	overdueLoan, err := fx.loanReg.Create(fx.ctx, makeLoan("commodity-1", "2026-04-01", &pastDue))
	c.Assert(err, qt.IsNil)

	// Open, due in the future.
	future := "2026-06-01"
	_, err = fx.loanReg.Create(fx.ctx, makeLoan("commodity-2", "2026-05-01", &future))
	c.Assert(err, qt.IsNil)

	// Returned.
	closed, err := fx.loanReg.Create(fx.ctx, makeLoan("commodity-3", "2026-04-15", nil))
	c.Assert(err, qt.IsNil)
	closedRet := *closed
	closedRet.ReturnedAt = func() models.PDate { d := models.Date("2026-05-05"); return &d }()
	_, err = fx.loanReg.Update(fx.ctx, closedRet)
	c.Assert(err, qt.IsNil)

	now := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

	all, total, err := fx.loanReg.ListPaginated(fx.ctx, 0, 10, registry.LoanListOptions{Now: now})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 3)
	c.Assert(all, qt.HasLen, 3)

	open, total, err := fx.loanReg.ListPaginated(fx.ctx, 0, 10, registry.LoanListOptions{State: registry.LoanStateOpen, Now: now})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 2)
	c.Assert(open, qt.HasLen, 2)

	overdue, total, err := fx.loanReg.ListPaginated(fx.ctx, 0, 10, registry.LoanListOptions{State: registry.LoanStateOverdue, Now: now})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 1)
	c.Assert(overdue, qt.HasLen, 1)
	c.Assert(overdue[0].ID, qt.Equals, overdueLoan.ID)

	returned, total, err := fx.loanReg.ListPaginated(fx.ctx, 0, 10, registry.LoanListOptions{State: registry.LoanStateReturned, Now: now})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 1)
	c.Assert(returned, qt.HasLen, 1)
	c.Assert(returned[0].ID, qt.Equals, closed.ID)
}

func TestCommodityLoanRegistry_Memory_CountOpenByCommodity(t *testing.T) {
	c := qt.New(t)
	fx := newLoanFixture(c, "group-1")

	_, err := fx.loanReg.Create(fx.ctx, makeLoan("commodity-1", "2026-05-01", nil))
	c.Assert(err, qt.IsNil)

	// Already-returned loan on commodity-2 — should NOT count.
	closed, err := fx.loanReg.Create(fx.ctx, makeLoan("commodity-2", "2026-04-15", nil))
	c.Assert(err, qt.IsNil)
	closed.ReturnedAt = func() models.PDate { d := models.Date("2026-04-30"); return &d }()
	_, err = fx.loanReg.Update(fx.ctx, *closed)
	c.Assert(err, qt.IsNil)

	counts, err := fx.loanReg.CountOpenByCommodity(fx.ctx, []string{"commodity-1", "commodity-2", "missing"})
	c.Assert(err, qt.IsNil)
	c.Assert(counts["commodity-1"], qt.Equals, 1)
	c.Assert(counts["commodity-2"], qt.Equals, 0)
	c.Assert(counts["missing"], qt.Equals, 0)
}

func TestCommodityLoanRegistry_Memory_GroupIsolation(t *testing.T) {
	c := qt.New(t)

	a := newLoanFixture(c, "group-A")
	b := newLoanFixture(c, "group-B")

	_, err := a.loanReg.Create(a.ctx, makeLoan("commodity-1", "2026-05-01", nil))
	c.Assert(err, qt.IsNil)

	bList, err := b.loanReg.List(b.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(bList, qt.HasLen, 0, qt.Commentf("loans created in group-A must not be visible to group-B"))
}
