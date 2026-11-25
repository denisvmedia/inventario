package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// A discard can stop after it took the draft out of play: the server dies,
// the database fails, or the client goes away. The draft is then hidden from
// every list but keeps its files and its client key.

// failTrashFactory makes every move of a draft file to Trash fail.
type failTrashFactory struct {
	registry.FileRegistryFactory
}

type failTrashRegistry struct {
	registry.FileRegistry
}

func (f *failTrashFactory) CreateUserRegistry(ctx context.Context) (registry.FileRegistry, error) {
	inner, err := f.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}
	return &failTrashRegistry{FileRegistry: inner}, nil
}

func (*failTrashRegistry) TrashUnlessHeld(context.Context, string, string, string, string, time.Time) (bool, error) {
	return false, errors.New("connection reset")
}

func TestCommodityDraftDiscard_StoppedHalfWayIsFinishedLater(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)
	reg := fx.factorySet.CommodityDraftRegistryFactory.MustCreateUserRegistry(fx.ctxA)
	draft, photo := raceDraft(c, fx, "form:half")

	broken := *fx.factorySet
	broken.FileRegistryFactory = &failTrashFactory{FileRegistryFactory: fx.factorySet.FileRegistryFactory}
	c.Assert(services.NewCommodityDraftService(&broken).Discard(fx.ctxA, draft.ID), qt.IsNotNil)

	// Hidden, but the row, the photo's link and the client key remain.
	open, total, err := reg.ListOpen(fx.ctxA, 0, 100)
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 0)
	c.Assert(open, qt.HasLen, 0)
	row, err := reg.Get(fx.ctxA, draft.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(row.IsDiscarding(), qt.IsTrue)
	live, err := fx.groupASet.FileRegistry.Get(fx.ctxA, photo.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(live.Links, qt.HasLen, 1)
	svc := services.NewCommodityDraftService(fx.factorySet)
	_, _, err = svc.Create(fx.ctxA, "form:half", models.CommodityDraftValues{"name": "Half again"})
	c.Assert(err, qt.ErrorIs, registry.ErrDraftDiscarded)

	// A discard that may still be running is left alone.
	finished, err := svc.FinishStaleDiscards(fx.ctxA)
	c.Assert(err, qt.IsNil)
	c.Assert(finished, qt.Equals, 0)

	// Ten minutes on it is finished: the row is gone, the photo is in Trash
	// and the key makes a new draft.
	later := services.NewCommodityDraftService(fx.factorySet,
		services.WithCommodityDraftClock(func() time.Time { return time.Now().Add(11 * time.Minute) }))
	finished, err = later.FinishStaleDiscards(fx.ctxA)
	c.Assert(err, qt.IsNil)
	c.Assert(finished, qt.Equals, 1)
	_, err = reg.Get(fx.ctxA, draft.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = fx.groupASet.FileRegistry.GetTrashed(fx.ctxA, photo.ID)
	c.Assert(err, qt.IsNil)
	again, created, err := svc.Create(fx.ctxA, "form:half", models.CommodityDraftValues{"name": "Half again"})
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	c.Assert(again.ID, qt.Not(qt.Equals), draft.ID)
}

// TestCommodityDraftDiscard_ClientGoneAfterTheDraftIsShut: the request is
// canceled once the draft is taken out of play. Run the way the handler runs
// it, on a context the client cannot cancel, the discard still finishes.
func TestCommodityDraftDiscard_ClientGoneAfterTheDraftIsShut(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)
	reg := fx.factorySet.CommodityDraftRegistryFactory.MustCreateUserRegistry(fx.ctxA)
	draft, photo := raceDraft(c, fx, "form:gone")

	reqCtx, cancel := context.WithCancel(fx.ctxA)
	defer cancel()
	hooked := *fx.factorySet
	hooked.FileRegistryFactory = &hookFileFactory{
		FileRegistryFactory: fx.factorySet.FileRegistryFactory,
		at:                  "list",
		once:                &sync.Once{},
		hook:                cancel,
	}
	c.Assert(services.NewCommodityDraftService(&hooked).Discard(context.WithoutCancel(reqCtx), draft.ID), qt.IsNil)
	c.Assert(reqCtx.Err(), qt.IsNotNil)

	_, err := reg.Get(fx.ctxA, draft.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = fx.groupASet.FileRegistry.GetTrashed(fx.ctxA, photo.ID)
	c.Assert(err, qt.IsNil)
}
