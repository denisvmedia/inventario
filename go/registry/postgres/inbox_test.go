package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// The inbox on postgres, under RLS: open drafts and live items marked as
// draft, whatever their status, of the reader's group only. Promoted drafts,
// drafts being discarded, reviewed items and items in Trash are not in it.
// The count, built from ids alone, agrees with the list.
func TestInboxService_Postgres(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	regA := must.Must(fx.factorySet.CreateUserRegistrySet(fx.ctxA))
	regB := must.Must(fx.factorySet.CreateUserRegistrySet(fx.ctxB))

	item := func(name, areaID string, draft bool) models.Commodity {
		return models.Commodity{
			Name: name, ShortName: name, Type: models.CommodityTypeOther,
			Status: models.CommodityStatusInUse, Count: 1, AreaID: new(areaID), Draft: draft,
			OriginalPriceCurrency: "USD",
		}
	}
	mkDraft := func(ctx context.Context, set *registry.Set, key, name string) *models.CommodityDraft {
		d, created, err := set.CommodityDraftRegistry.CreateOrGet(ctx, models.CommodityDraft{
			ClientKey: key, Values: models.CommodityDraftValues{"name": name},
		})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsTrue)
		return d
	}

	mkDraft(fx.ctxA, regA, "a-open", "A open draft")
	promoted := mkDraft(fx.ctxA, regA, "a-promoted", "A promoted")
	_, err := regA.CommodityDraftRegistry.Promote(fx.ctxA, promoted.ID, promoted.Revision, "pk-1", item("A promoted", fx.areaAID, false))
	c.Assert(err, qt.IsNil)
	discarding := mkDraft(fx.ctxA, regA, "a-discarding", "A discarding")
	_, err = regA.CommodityDraftRegistry.MarkDiscarding(fx.ctxA, discarding.ID)
	c.Assert(err, qt.IsNil)
	sold := item("A sold draft item", fx.areaAID, true)
	sold.Status = models.CommodityStatusSold
	must.Must(regA.CommodityRegistry.Create(fx.ctxA, sold))
	must.Must(regA.CommodityRegistry.Create(fx.ctxA, item("A reviewed", fx.areaAID, false)))
	trashed := must.Must(regA.CommodityRegistry.Create(fx.ctxA, item("A trashed draft item", fx.areaAID, true)))
	_, err = regA.CommodityRegistry.Trash(fx.ctxA, trashed.ID, "test", time.Now())
	c.Assert(err, qt.IsNil)

	mkDraft(fx.ctxB, regB, "b-open", "B open draft")
	must.Must(regB.CommodityRegistry.Create(fx.ctxB, item("B draft item", fx.areaBID, true)))

	svc := services.NewInboxService()
	for _, tc := range []struct {
		name string
		ctx  context.Context
		set  *registry.Set
		want []string
	}{
		{"group A", fx.ctxA, regA, []string{"A open draft", "A sold draft item"}},
		{"group B", fx.ctxB, regB, []string{"B open draft", "B draft item"}},
	} {
		page, err := svc.List(tc.ctx, tc.set, services.InboxListOptions{Limit: 50})
		c.Assert(err, qt.IsNil, qt.Commentf(tc.name))
		titles := make([]string, 0, len(page.Entries))
		for _, e := range page.Entries {
			titles = append(titles, e.Title)
		}
		c.Check(titles, qt.DeepEquals, tc.want, qt.Commentf(tc.name))
		c.Check(page.Counts, qt.DeepEquals, services.InboxCounts{
			Total:   2,
			Reasons: map[services.InboxReason]int{services.InboxReasonDraft: 1, services.InboxReasonItemDraft: 1},
		}, qt.Commentf(tc.name))

		counts, err := svc.Count(tc.ctx, tc.set)
		c.Assert(err, qt.IsNil, qt.Commentf(tc.name))
		c.Check(counts, qt.DeepEquals, page.Counts, qt.Commentf(tc.name))
	}
}
