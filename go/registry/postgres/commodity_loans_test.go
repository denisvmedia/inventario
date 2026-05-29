package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/services"
)

// loanPGFixture mirrors tagPGFixture for the loan registry. Two groups
// so cross-group isolation can be exercised; one commodity per group so
// loans have something to attach to.
type loanPGFixture struct {
	factorySet *registry.FactorySet
	groupASet  *registry.Set
	groupBSet  *registry.Set
	ctxA       context.Context
	ctxB       context.Context
	user       *models.User
	groupAID   string
	groupBID   string
	commAID    string
	commBID    string
}

func newLoanPGFixture(t *testing.T) loanPGFixture {
	t.Helper()
	c := qt.New(t)

	groupASet, _ := setupTestRegistrySet(t)

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	factorySet := postgres.NewFactorySet(dbx)

	user := getTestUser(c, groupASet)

	serviceSet := factorySet.CreateServiceRegistrySet()
	groups, err := serviceSet.LocationGroupRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(groups, qt.HasLen, 1)
	groupAID := groups[0].ID

	groupBSlug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	groupB, err := serviceSet.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Name:                "Test Group B",
		Slug:                groupBSlug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
		GroupCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)
	groupBID := groupB.ID

	groupBSet := postgres.NewRegistrySetWithUserAndGroupID(dbx, user.ID, user.TenantID, groupBID)

	ctxA := loanCtxFor(user, groupAID)
	ctxB := loanCtxFor(user, groupBID)

	areaAID := seedTagArea(c, groupASet, ctxA)
	areaBID := seedTagArea(c, groupBSet, ctxB)

	commAID := seedTagCommodity(c, groupASet, ctxA, areaAID, "Drill A")
	commBID := seedTagCommodity(c, groupBSet, ctxB, areaBID, "Drill B")

	return loanPGFixture{
		factorySet: factorySet,
		groupASet:  groupASet,
		groupBSet:  groupBSet,
		ctxA:       ctxA,
		ctxB:       ctxB,
		user:       user,
		groupAID:   groupAID,
		groupBID:   groupBID,
		commAID:    commAID,
		commBID:    commBID,
	}
}

func loanCtxFor(user *models.User, groupID string) context.Context {
	ctx := appctx.WithUser(context.Background(), user)
	return appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: groupID},
			TenantID: user.TenantID,
		},
	})
}

func TestCommodityLoanRegistry_Postgres_CreateAndGet(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)

	created, err := fx.groupASet.CommodityLoanRegistry.Create(fx.ctxA, models.CommodityLoan{
		CommodityID:  fx.commAID,
		BorrowerName: "Alice",
		LentAt:       models.Date("2026-05-01"),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.GroupID, qt.Equals, fx.groupAID)
	c.Assert(created.IsOpen(), qt.IsTrue)

	got, err := fx.groupASet.CommodityLoanRegistry.Get(fx.ctxA, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.BorrowerName, qt.Equals, "Alice")
}

func TestCommodityLoanRegistry_Postgres_StartLoan_RejectsSecondOpen(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)
	svc := services.NewCommodityLoanService(fx.factorySet)

	first, _, _, err := svc.StartLoan(fx.ctxA, models.CommodityLoan{
		CommodityID:  fx.commAID,
		BorrowerName: "Alice",
		LentAt:       models.Date("2026-05-01"),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(first, qt.IsNotNil)

	// Second open loan on the same commodity → ErrLoanAlreadyOpen + the existing.
	_, existing, _, err := svc.StartLoan(fx.ctxA, models.CommodityLoan{
		CommodityID:  fx.commAID,
		BorrowerName: "Bob",
		LentAt:       models.Date("2026-05-02"),
	})
	c.Assert(err, qt.ErrorIs, services.ErrLoanAlreadyOpen)
	c.Assert(existing, qt.IsNotNil)
	c.Assert(existing.ID, qt.Equals, first.ID)

	// Mark it returned, then a fresh Start should succeed.
	_, err = svc.MarkReturned(fx.ctxA, first.ID, nil)
	c.Assert(err, qt.IsNil)

	third, _, _, err := svc.StartLoan(fx.ctxA, models.CommodityLoan{
		CommodityID:  fx.commAID,
		BorrowerName: "Charlie",
		LentAt:       models.Date("2026-05-10"),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(third.ID, qt.Not(qt.Equals), first.ID)
}

func TestCommodityLoanRegistry_Postgres_StateFilter(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)

	now := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

	// Overdue: lent 2026-04-01, due 2026-04-15.
	pastDue := models.Date("2026-04-15")
	overdue, err := fx.groupASet.CommodityLoanRegistry.Create(fx.ctxA, models.CommodityLoan{
		CommodityID:  fx.commAID,
		BorrowerName: "Alice",
		LentAt:       models.Date("2026-04-01"),
		DueBackAt:    &pastDue,
	})
	c.Assert(err, qt.IsNil)

	open, total, err := fx.groupASet.CommodityLoanRegistry.ListPaginated(fx.ctxA, 0, 10, registry.LoanListOptions{State: registry.LoanStateOpen, Now: now})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 1)
	c.Assert(open, qt.HasLen, 1)
	c.Assert(open[0].ID, qt.Equals, overdue.ID)

	overdueList, total, err := fx.groupASet.CommodityLoanRegistry.ListPaginated(fx.ctxA, 0, 10, registry.LoanListOptions{State: registry.LoanStateOverdue, Now: now})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 1)
	c.Assert(overdueList, qt.HasLen, 1)

	returnedList, total, err := fx.groupASet.CommodityLoanRegistry.ListPaginated(fx.ctxA, 0, 10, registry.LoanListOptions{State: registry.LoanStateReturned, Now: now})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 0)
	c.Assert(returnedList, qt.HasLen, 0)
}

func TestCommodityLoanRegistry_Postgres_GroupIsolation(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)

	_, err := fx.groupASet.CommodityLoanRegistry.Create(fx.ctxA, models.CommodityLoan{
		CommodityID:  fx.commAID,
		BorrowerName: "Alice",
		LentAt:       models.Date("2026-05-01"),
	})
	c.Assert(err, qt.IsNil)

	bList, err := fx.groupBSet.CommodityLoanRegistry.List(fx.ctxB)
	c.Assert(err, qt.IsNil)
	c.Assert(bList, qt.HasLen, 0, qt.Commentf("loans in group A must not be visible to group B via RLS"))
}

func TestCommodityLoanRegistry_Postgres_CountOpenByCommodity(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)

	_, err := fx.groupASet.CommodityLoanRegistry.Create(fx.ctxA, models.CommodityLoan{
		CommodityID:  fx.commAID,
		BorrowerName: "Alice",
		LentAt:       models.Date("2026-05-01"),
	})
	c.Assert(err, qt.IsNil)

	counts, err := fx.groupASet.CommodityLoanRegistry.CountOpenByCommodity(fx.ctxA, []string{fx.commAID, "ghost"})
	c.Assert(err, qt.IsNil)
	c.Assert(counts[fx.commAID], qt.Equals, 1)
	c.Assert(counts["ghost"], qt.Equals, 0)
}

// TestCommodityLoanRegistry_Postgres_FKCascade verifies that hard-deleting
// the parent commodity drops every loan row pointing at it. Mirrors the
// `ON DELETE CASCADE` clause we hand-add to the migration.
func TestCommodityLoanRegistry_Postgres_FKCascade(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)

	loan, err := fx.groupASet.CommodityLoanRegistry.Create(fx.ctxA, models.CommodityLoan{
		CommodityID:  fx.commAID,
		BorrowerName: "Alice",
		LentAt:       models.Date("2026-05-01"),
	})
	c.Assert(err, qt.IsNil)

	err = fx.groupASet.CommodityRegistry.Delete(fx.ctxA, fx.commAID)
	c.Assert(err, qt.IsNil)

	_, err = fx.groupASet.CommodityLoanRegistry.Get(fx.ctxA, loan.ID)
	c.Assert(err, qt.IsNotNil, qt.Commentf("loan should be cascade-deleted with the parent commodity"))
}
