package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// Discard and promote of one draft run on different devices. Whichever comes
// second must see what the first did: a promote never makes an item of a
// draft being discarded, and a discard never deletes the record of a promoted
// draft or sends the item's photos to Trash.

func raceDraft(c *qt.C, fx loanPGFixture, key string) (*models.CommodityDraft, *models.FileEntity) {
	c.Helper()
	reg := fx.factorySet.CommodityDraftRegistryFactory.MustCreateUserRegistry(fx.ctxA)
	draft, created, err := reg.CreateOrGet(fx.ctxA, models.CommodityDraft{ClientKey: key, Values: models.CommodityDraftValues{"name": "Drill"}})
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	photo, err := fx.groupASet.FileRegistry.Create(fx.ctxA, models.FileEntity{
		Title:    key + "-photo",
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		Links:    []models.FileLink{{EntityType: models.FileLinkDraft, EntityID: draft.ID, Meta: models.FileLinkMetaImages}},
		File:     &models.File{Path: key, OriginalPath: "files/" + key + ".jpg", Ext: ".jpg", MIMEType: "image/jpeg", SizeBytes: 10},
	})
	c.Assert(err, qt.IsNil)
	return draft, photo
}

func raceItem(name string) models.Commodity {
	return models.Commodity{
		Name:      name,
		ShortName: name,
		Type:      models.CommodityTypeOther,
		Count:     1,
		Status:    models.CommodityStatusInUse,
	}
}

// hookFileFactory runs a hook once, when the discard reaches the named step
// of its file work.
type hookFileFactory struct {
	registry.FileRegistryFactory
	at   string
	once *sync.Once
	hook func()
}

type hookFileRegistry struct {
	registry.FileRegistry
	f *hookFileFactory
}

func (f *hookFileFactory) CreateUserRegistry(ctx context.Context) (registry.FileRegistry, error) {
	inner, err := f.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}
	return &hookFileRegistry{FileRegistry: inner, f: f}, nil
}

func (r *hookFileRegistry) fire(step, entityType string) {
	if step == r.f.at && entityType == models.FileLinkDraft {
		r.f.once.Do(r.f.hook)
	}
}

func (r *hookFileRegistry) ListByLinkedEntity(ctx context.Context, entityType, entityID string) ([]*models.FileEntity, error) {
	r.fire("list", entityType)
	return r.FileRegistry.ListByLinkedEntity(ctx, entityType, entityID)
}

func (r *hookFileRegistry) RemoveLinksToEntity(ctx context.Context, entityType, entityID string) ([]string, error) {
	r.fire("unlink", entityType)
	return r.FileRegistry.RemoveLinksToEntity(ctx, entityType, entityID)
}

func TestCommodityDraftDiscard_PromoteInTheMiddleIsRefused(t *testing.T) {
	for _, at := range []string{"list", "unlink"} {
		t.Run(at, func(t *testing.T) {
			c := qt.New(t)
			fx := newLoanPGFixture(t)
			draft, photo := raceDraft(c, fx, "race-"+at)
			itemsBefore, err := fx.groupASet.CommodityRegistry.Count(fx.ctxA)
			c.Assert(err, qt.IsNil)

			promoteSvc := services.NewCommodityDraftService(fx.factorySet)
			var promoteErr error
			hooked := *fx.factorySet
			hooked.FileRegistryFactory = &hookFileFactory{
				FileRegistryFactory: fx.factorySet.FileRegistryFactory,
				at:                  at,
				once:                &sync.Once{},
				hook: func() {
					// Another device finishes the draft while the discard is
					// between its first step and the row delete.
					_, _, promoteErr = promoteSvc.Promote(fx.ctxA, draft.ID, 1, "other-device", raceItem("Raced"))
				},
			}
			c.Assert(services.NewCommodityDraftService(&hooked).Discard(fx.ctxA, draft.ID), qt.IsNil)

			c.Assert(promoteErr, qt.ErrorIs, registry.ErrDraftDiscarded)
			itemsAfter, err := fx.groupASet.CommodityRegistry.Count(fx.ctxA)
			c.Assert(err, qt.IsNil)
			c.Assert(itemsAfter, qt.Equals, itemsBefore, qt.Commentf("the promote made an item of a discarded draft"))
			_, err = fx.factorySet.CommodityDraftRegistryFactory.MustCreateUserRegistry(fx.ctxA).Get(fx.ctxA, draft.ID)
			c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
			_, err = fx.groupASet.FileRegistry.Get(fx.ctxA, photo.ID)
			c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
			trashed, err := fx.groupASet.FileRegistry.GetTrashed(fx.ctxA, photo.ID)
			c.Assert(err, qt.IsNil)
			c.Assert(trashed.Links, qt.HasLen, 0)
		})
	}
}

func TestCommodityDraftDiscard_AfterPromoteKeepsTheItem(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)
	draft, photo := raceDraft(c, fx, "promoted-first")
	svc := services.NewCommodityDraftService(fx.factorySet)

	item, created, err := svc.Promote(fx.ctxA, draft.ID, 1, "device-a", raceItem("Drill"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)

	c.Assert(svc.Discard(fx.ctxA, draft.ID), qt.ErrorIs, registry.ErrDraftPromoted)

	stored, err := fx.factorySet.CommodityDraftRegistryFactory.MustCreateUserRegistry(fx.ctxA).Get(fx.ctxA, draft.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Status, qt.Equals, models.CommodityDraftPromoted)
	c.Assert(*stored.PromotedCommodityID, qt.Equals, item.ID)
	live, err := fx.groupASet.FileRegistry.Get(fx.ctxA, photo.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(live.Links, qt.HasLen, 1)
	c.Assert(live.Links[0].EntityType, qt.Equals, models.FileLinkCommodity)
	c.Assert(live.Links[0].EntityID, qt.Equals, item.ID)
}

// TestCommodityDraftDiscard_RacesPromote runs both at once, many times, and
// checks that each run ends in one of the two clean states.
func TestCommodityDraftDiscard_RacesPromote(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)
	svc := services.NewCommodityDraftService(fx.factorySet)
	drafts := fx.factorySet.CommodityDraftRegistryFactory.MustCreateUserRegistry(fx.ctxA)

	promotedRuns, discardedRuns := 0, 0
	for i := range 12 {
		draft, photo := raceDraft(c, fx, fmt.Sprintf("race-%d", i))
		start := make(chan struct{})
		var wg sync.WaitGroup
		var item *models.Commodity
		var promoteErr, discardErr error
		wg.Go(func() {
			<-start
			item, _, promoteErr = svc.Promote(fx.ctxA, draft.ID, 1, "device-a", raceItem(fmt.Sprintf("Raced item %d", i)))
		})
		wg.Go(func() {
			<-start
			discardErr = svc.Discard(fx.ctxA, draft.ID)
		})
		close(start)
		wg.Wait()

		switch {
		case promoteErr == nil:
			promotedRuns++
			c.Assert(discardErr, qt.ErrorIs, registry.ErrDraftPromoted, qt.Commentf("run %d", i))
			stored, err := drafts.Get(fx.ctxA, draft.ID)
			c.Assert(err, qt.IsNil, qt.Commentf("run %d: the promoted draft's record is gone", i))
			c.Assert(*stored.PromotedCommodityID, qt.Equals, item.ID)
			live, err := fx.groupASet.FileRegistry.Get(fx.ctxA, photo.ID)
			c.Assert(err, qt.IsNil, qt.Commentf("run %d: the item's photo went to Trash", i))
			c.Assert(live.Links, qt.HasLen, 1)
			c.Assert(live.Links[0].EntityID, qt.Equals, item.ID)
		case discardErr == nil:
			discardedRuns++
			c.Assert(errors.Is(promoteErr, registry.ErrDraftDiscarded) || errors.Is(promoteErr, registry.ErrNotFound), qt.IsTrue,
				qt.Commentf("run %d: promote error %v", i, promoteErr))
			_, err := drafts.Get(fx.ctxA, draft.ID)
			c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
			_, err = fx.groupASet.FileRegistry.GetTrashed(fx.ctxA, photo.ID)
			c.Assert(err, qt.IsNil, qt.Commentf("run %d: the draft's photo is not in Trash", i))
		default:
			c.Fatalf("run %d: both failed: promote %v, discard %v", i, promoteErr, discardErr)
		}
	}
	c.Logf("promoted %d, discarded %d", promotedRuns, discardedRuns)

	items, err := fx.groupASet.CommodityRegistry.List(fx.ctxA)
	c.Assert(err, qt.IsNil)
	made := 0
	for _, item := range items {
		if strings.HasPrefix(item.Name, "Raced item ") {
			made++
		}
	}
	c.Assert(made, qt.Equals, promotedRuns, qt.Commentf("an item exists for a discarded draft"))
}

func TestCommodityDraftPromote_ConcurrentRetries(t *testing.T) {
	t.Run("same key", func(t *testing.T) {
		c := qt.New(t)
		fx := newLoanPGFixture(t)
		draft, _ := raceDraft(c, fx, "same-key")
		svc := services.NewCommodityDraftService(fx.factorySet)

		const n = 8
		type result struct {
			id      string
			created bool
			err     error
		}
		out := make([]result, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				item, created, err := svc.Promote(fx.ctxA, draft.ID, 1, "device-a", raceItem("Once"))
				out[i] = result{err: err, created: created}
				if err == nil {
					out[i].id = item.ID
				}
			})
		}
		wg.Wait()
		createdCount, ids := 0, map[string]bool{}
		for _, r := range out {
			c.Assert(r.err, qt.IsNil)
			if r.created {
				createdCount++
			}
			ids[r.id] = true
		}
		c.Assert(createdCount, qt.Equals, 1)
		c.Assert(ids, qt.HasLen, 1)
	})

	t.Run("different keys", func(t *testing.T) {
		c := qt.New(t)
		fx := newLoanPGFixture(t)
		draft, _ := raceDraft(c, fx, "other-keys")
		svc := services.NewCommodityDraftService(fx.factorySet)
		itemsBefore, err := fx.groupASet.CommodityRegistry.Count(fx.ctxA)
		c.Assert(err, qt.IsNil)

		const n = 8
		errs := make([]error, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				_, _, errs[i] = svc.Promote(fx.ctxA, draft.ID, 1, fmt.Sprintf("device-%d", i), raceItem("Once"))
			})
		}
		wg.Wait()
		won := 0
		for _, err := range errs {
			if err == nil {
				won++
				continue
			}
			c.Assert(err, qt.ErrorIs, registry.ErrDraftPromoted)
		}
		c.Assert(won, qt.Equals, 1)
		itemsAfter, err := fx.groupASet.CommodityRegistry.Count(fx.ctxA)
		c.Assert(err, qt.IsNil)
		c.Assert(itemsAfter, qt.Equals, itemsBefore+1)
	})
}
