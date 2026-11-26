package registrytest

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// DraftEnv is what RunCommodityDraftContract needs from a backend:
// registries scoped to one empty group, the context that carries the group
// scope, and an area items can be placed in.
type DraftEnv struct {
	Ctx         context.Context
	Drafts      registry.CommodityDraftRegistry
	Commodities registry.CommodityRegistry
	Files       registry.FileRegistry
	Sellers     registry.SellerRegistry
	Fields      registry.CustomFieldRegistry
	AreaID      string
}

// RunCommodityDraftContract checks the draft lifecycle a backend's
// CommodityDraftRegistry implements (#2812): idempotent create by client
// key, revision-checked saves, the open-drafts list, and a promotion that
// creates exactly one item and hands it the draft's files. newEnv is called
// once per subtest and must return an empty group.
func RunCommodityDraftContract(t *testing.T, newEnv func(t *testing.T) DraftEnv) {
	t.Run("create is idempotent by client key", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		first, created, err := env.Drafts.CreateOrGet(env.Ctx, models.CommodityDraft{
			ClientKey: "key-1",
			Values:    models.CommodityDraftValues{"name": "Drill"},
		})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsTrue)
		c.Assert(first.ID, qt.Not(qt.Equals), "")
		c.Assert(first.Status, qt.Equals, models.CommodityDraftOpen)
		c.Assert(first.Revision, qt.Equals, 1)
		c.Assert(first.Values.Title(), qt.Equals, "Drill")

		again, created, err := env.Drafts.CreateOrGet(env.Ctx, models.CommodityDraft{
			ClientKey: "key-1",
			Values:    models.CommodityDraftValues{"name": "Something else"},
		})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsFalse)
		c.Assert(again.ID, qt.Equals, first.ID)
		c.Assert(again.Values.Title(), qt.Equals, "Drill", qt.Commentf("a repeated create leaves the draft unchanged"))

		other, created, err := env.Drafts.CreateOrGet(env.Ctx, models.CommodityDraft{ClientKey: "key-2"})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsTrue)
		c.Assert(other.ID, qt.Not(qt.Equals), first.ID)
		c.Assert(other.Values, qt.DeepEquals, models.CommodityDraftValues{})

		byKey, err := env.Drafts.GetByClientKey(env.Ctx, "key-2")
		c.Assert(err, qt.IsNil)
		c.Assert(byKey.ID, qt.Equals, other.ID)
		_, err = env.Drafts.GetByClientKey(env.Ctx, "missing")
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

		count, err := env.Drafts.Count(env.Ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 2)
	})

	t.Run("save checks the revision", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		draft := createContractDraft(c, env, "key-1", "Drill")

		saved, err := env.Drafts.SaveValues(env.Ctx, draft.ID, 1, models.CommodityDraftValues{
			"name":   "Cordless drill",
			"count":  float64(2),
			"nested": map[string]any{"a": "b"},
		})
		c.Assert(err, qt.IsNil)
		c.Assert(saved.Revision, qt.Equals, 2)
		c.Assert(saved.Values.Title(), qt.Equals, "Cordless drill")
		c.Assert(saved.UpdatedAt.Before(draft.UpdatedAt), qt.IsFalse)

		got, err := env.Drafts.Get(env.Ctx, draft.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Revision, qt.Equals, 2)
		c.Assert(got.Values["count"], qt.Equals, float64(2))
		c.Assert(got.Values["nested"], qt.DeepEquals, map[string]any{"a": "b"})

		// A tab that loaded revision 1 must not overwrite revision 2.
		_, err = env.Drafts.SaveValues(env.Ctx, draft.ID, 1, models.CommodityDraftValues{"name": "Stale"})
		c.Assert(err, qt.ErrorIs, registry.ErrDraftRevisionConflict)
		got, err = env.Drafts.Get(env.Ctx, draft.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Values.Title(), qt.Equals, "Cordless drill")

		_, err = env.Drafts.SaveValues(env.Ctx, "missing", 1, models.CommodityDraftValues{})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})

	t.Run("list open drafts", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		a := createContractDraft(c, env, "a", "A")
		b := createContractDraft(c, env, "b", "B")
		promoted := createContractDraft(c, env, "p", "P")
		_, err := env.Drafts.Promote(env.Ctx, promoted.ID, 1, "promotion-1", contractItem(env, "P"))
		c.Assert(err, qt.IsNil)
		// Saving a moves it to the top.
		_, err = env.Drafts.SaveValues(env.Ctx, a.ID, 1, models.CommodityDraftValues{"name": "A2"})
		c.Assert(err, qt.IsNil)

		page, total, err := env.Drafts.ListOpen(env.Ctx, 0, 10)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 2)
		c.Assert(draftIDs(page), qt.DeepEquals, []string{a.ID, b.ID})

		second, total, err := env.Drafts.ListOpen(env.Ctx, 1, 1)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 2)
		c.Assert(draftIDs(second), qt.DeepEquals, []string{b.ID})

		ids, err := env.Drafts.OpenIDs(env.Ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(ids, qt.DeepEquals, []string{a.ID, b.ID})
	})

	t.Run("promote creates one item and moves the files", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		draft := createContractDraft(c, env, "key-1", "Drill")
		photo := contractFile(c, env, "photo.jpg")
		_, created, err := env.Files.AddLink(env.Ctx, models.FileLink{
			FileID: photo.ID, EntityType: models.FileLinkDraft, EntityID: draft.ID, Meta: models.FileLinkMetaImages,
		})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsTrue)
		counts, err := env.Files.CountLinkedTo(env.Ctx, models.FileLinkDraft, []string{draft.ID, "other-draft"})
		c.Assert(err, qt.IsNil)
		c.Assert(counts, qt.DeepEquals, map[string]int{draft.ID: 1})
		before, err := env.Commodities.Count(env.Ctx)
		c.Assert(err, qt.IsNil)

		// A stale revision promotes nothing.
		_, err = env.Drafts.Promote(env.Ctx, draft.ID, 7, "promotion-1", contractItem(env, "Drill"))
		c.Assert(err, qt.ErrorIs, registry.ErrDraftRevisionConflict)

		promotion, err := env.Drafts.Promote(env.Ctx, draft.ID, 1, "promotion-1", contractItem(env, "Drill"))
		c.Assert(err, qt.IsNil)
		result, item := promotion.Draft, promotion.Commodity
		c.Assert(item, qt.IsNotNil)
		c.Assert(item.Name, qt.Equals, "Drill")
		c.Assert(item.AssetNumber, qt.IsNotNil, qt.Commentf("the item gets an asset number like any new item"))
		c.Assert(result.Status, qt.Equals, models.CommodityDraftPromoted)
		c.Assert(result.PromotedCommodityID, qt.IsNotNil)
		c.Assert(*result.PromotedCommodityID, qt.Equals, item.ID)
		c.Assert(result.PromotedAt, qt.IsNotNil)
		c.Assert(result.Values, qt.DeepEquals, models.CommodityDraftValues{})
		c.Assert(result.Revision, qt.Equals, 2)

		stored, err := env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(stored.Name, qt.Equals, "Drill")

		moved, err := env.Files.Get(env.Ctx, photo.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(moved.Links, qt.HasLen, 1)
		c.Assert(moved.Links[0].EntityType, qt.Equals, models.FileLinkCommodity)
		c.Assert(moved.Links[0].EntityID, qt.Equals, item.ID)
		c.Assert(moved.Links[0].Meta, qt.Equals, models.FileLinkMetaImages)
		onDraft, err := env.Files.ListByLinkedEntity(env.Ctx, models.FileLinkDraft, draft.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(onDraft, qt.HasLen, 0)
		counts, err = env.Files.CountLinkedTo(env.Ctx, models.FileLinkDraft, []string{draft.ID})
		c.Assert(err, qt.IsNil)
		c.Assert(counts, qt.HasLen, 0)

		// A retry with the same key after a lost response creates nothing
		// and finds the item; another key is told the draft is an item.
		again, err := env.Drafts.Promote(env.Ctx, draft.ID, 1, "promotion-1", contractItem(env, "Drill"))
		c.Assert(err, qt.IsNil)
		c.Assert(again.Commodity, qt.IsNil)
		c.Assert(*again.Draft.PromotedCommodityID, qt.Equals, item.ID)
		_, err = env.Drafts.Promote(env.Ctx, draft.ID, 1, "promotion-2", contractItem(env, "Drill"))
		c.Assert(err, qt.ErrorIs, registry.ErrDraftPromoted)
		_, err = env.Drafts.Promote(env.Ctx, draft.ID, 2, "promotion-2", contractItem(env, "Drill"))
		c.Assert(err, qt.ErrorIs, registry.ErrDraftPromoted)
		after, err := env.Commodities.Count(env.Ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(after, qt.Equals, before+1)

		// A promoted draft cannot be saved, and no longer appears as open.
		_, err = env.Drafts.SaveValues(env.Ctx, draft.ID, 2, models.CommodityDraftValues{"name": "Late"})
		c.Assert(err, qt.ErrorIs, registry.ErrDraftPromoted)
		_, total, err := env.Drafts.ListOpen(env.Ctx, 0, 10)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 0)
	})

	t.Run("promote keeps the seller and refuses one the group lacks", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		seller, err := env.Sellers.Create(env.Ctx, models.Seller{Name: "Hardware store"})
		c.Assert(err, qt.IsNil)

		draft := createContractDraft(c, env, "key-1", "Drill")
		item := contractItem(env, "Drill")
		item.SellerID = new(seller.ID)
		promotion, err := env.Drafts.Promote(env.Ctx, draft.ID, 1, "promotion-1", item)
		c.Assert(err, qt.IsNil)
		c.Assert(promotion.Commodity.SellerID, qt.IsNotNil)
		c.Assert(*promotion.Commodity.SellerID, qt.Equals, seller.ID)

		// A seller deleted while the draft was open: nothing is created and
		// the draft stays open.
		stale := createContractDraft(c, env, "key-2", "Saw")
		item = contractItem(env, "Saw")
		item.SellerID = new("no-such-seller")
		_, err = env.Drafts.Promote(env.Ctx, stale.ID, 1, "promotion-1", item)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		got, err := env.Drafts.Get(env.Ctx, stale.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.IsOpen(), qt.IsTrue)
		count, err := env.Commodities.Count(env.Ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 1)
	})

	t.Run("promote of an unknown draft", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		_, err := env.Drafts.Promote(env.Ctx, "missing", 1, "promotion-1", contractItem(env, "Drill"))
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})

	t.Run("drafts are not items", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		createContractDraft(c, env, "key-1", "Drill")
		items, err := env.Commodities.List(env.Ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(items, qt.HasLen, 0)
		count, err := env.Commodities.Count(env.Ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 0)
	})

	t.Run("delete removes the draft", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		draft := createContractDraft(c, env, "key-1", "Drill")
		c.Assert(env.Drafts.Delete(env.Ctx, draft.ID), qt.IsNil)
		_, err := env.Drafts.Get(env.Ctx, draft.ID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		// The key is free again.
		_, created, err := env.Drafts.CreateOrGet(env.Ctx, models.CommodityDraft{ClientKey: "key-1"})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsTrue)
	})

	runCommodityDraftLifecycleContract(t, newEnv)
}

// runCommodityDraftLifecycleContract covers the promotion key, the two-step
// discard, attaching files and the fate of a discarded draft's files.
func runCommodityDraftLifecycleContract(t *testing.T, newEnv func(t *testing.T) DraftEnv) {
	t.Helper()
	t.Run("promote writes the item's custom field values", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		field, err := env.Fields.Create(env.Ctx, models.CustomFieldDefinition{Name: "Frame number", Type: models.CustomFieldTypeText})
		c.Assert(err, qt.IsNil)
		draft := createContractDraft(c, env, "key-1", "Bike")

		// A value its field refuses promotes nothing.
		refused := contractItem(env, "Bike")
		refused.CustomFields = models.CustomFieldValues{field.ID: json.RawMessage(`42`)}
		_, err = env.Drafts.Promote(env.Ctx, draft.ID, 1, "promotion-1", refused)
		c.Assert(err, qt.ErrorIs, registry.ErrInvalidCustomFieldValue)
		got, err := env.Drafts.Get(env.Ctx, draft.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.IsOpen(), qt.IsTrue)
		count, err := env.Commodities.Count(env.Ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 0)

		item := contractItem(env, "Bike")
		item.CustomFields = models.CustomFieldValues{field.ID: json.RawMessage(`"FR-100"`)}
		result, err := env.Drafts.Promote(env.Ctx, draft.ID, 1, "promotion-2", item)
		c.Assert(err, qt.IsNil)
		c.Assert(string(result.Commodity.CustomFields[field.ID]), qt.Equals, `"FR-100"`)
		stored, err := env.Commodities.Get(env.Ctx, result.Commodity.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(string(stored.CustomFields[field.ID]), qt.Equals, `"FR-100"`)
	})

	t.Run("promote needs a promotion key", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		draft := createContractDraft(c, env, "key-1", "Drill")
		_, err := env.Drafts.Promote(env.Ctx, draft.ID, 1, "", contractItem(env, "Drill"))
		c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
		got, err := env.Drafts.Get(env.Ctx, draft.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.IsOpen(), qt.IsTrue)
	})

	t.Run("a discard shuts the draft before it is deleted", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		draft := createContractDraft(c, env, "key-1", "Drill")
		other := createContractDraft(c, env, "key-2", "Saw")
		photo := contractFile(c, env, "photo.jpg")

		// Only a draft being discarded can be deleted this way.
		c.Assert(env.Drafts.DeleteDiscarding(env.Ctx, draft.ID), qt.ErrorIs, registry.ErrNotFound)

		marked, err := env.Drafts.MarkDiscarding(env.Ctx, draft.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(marked.Status, qt.Equals, models.CommodityDraftDiscarding)

		page, total, err := env.Drafts.ListOpen(env.Ctx, 0, 10)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 1)
		c.Assert(draftIDs(page), qt.DeepEquals, []string{other.ID})
		ids, err := env.Drafts.OpenIDs(env.Ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(ids, qt.DeepEquals, []string{other.ID})

		_, err = env.Drafts.SaveValues(env.Ctx, draft.ID, 1, models.CommodityDraftValues{"name": "Late"})
		c.Assert(err, qt.ErrorIs, registry.ErrDraftDiscarded)
		_, err = env.Drafts.Promote(env.Ctx, draft.ID, 1, "promotion-1", contractItem(env, "Drill"))
		c.Assert(err, qt.ErrorIs, registry.ErrDraftDiscarded)
		_, _, err = env.Drafts.AttachFile(env.Ctx, draft.ID, models.FileLink{FileID: photo.ID, Meta: models.FileLinkMetaImages})
		c.Assert(err, qt.ErrorIs, registry.ErrDraftDiscarded)
		count, err := env.Commodities.Count(env.Ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 0)

		// A discard that stopped half way can be taken up again.
		again, err := env.Drafts.MarkDiscarding(env.Ctx, draft.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(again.Status, qt.Equals, models.CommodityDraftDiscarding)

		c.Assert(env.Drafts.DeleteDiscarding(env.Ctx, draft.ID), qt.IsNil)
		_, err = env.Drafts.Get(env.Ctx, draft.ID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		c.Assert(env.Drafts.DeleteDiscarding(env.Ctx, draft.ID), qt.ErrorIs, registry.ErrNotFound)
		_, err = env.Drafts.MarkDiscarding(env.Ctx, draft.ID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})

	t.Run("list the discards that began before a time", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		first := createContractDraft(c, env, "key-1", "Drill")
		second := createContractDraft(c, env, "key-2", "Saw")
		createContractDraft(c, env, "key-3", "Hammer")
		before := time.Now()
		_, err := env.Drafts.MarkDiscarding(env.Ctx, first.ID)
		c.Assert(err, qt.IsNil)
		_, err = env.Drafts.MarkDiscarding(env.Ctx, second.ID)
		c.Assert(err, qt.IsNil)

		none, err := env.Drafts.ListDiscarding(env.Ctx, before, 10)
		c.Assert(err, qt.IsNil)
		c.Assert(none, qt.HasLen, 0)

		later := time.Now().Add(time.Minute)
		both, err := env.Drafts.ListDiscarding(env.Ctx, later, 10)
		c.Assert(err, qt.IsNil)
		c.Assert(draftIDs(both), qt.DeepEquals, []string{first.ID, second.ID})
		oldest, err := env.Drafts.ListDiscarding(env.Ctx, later, 1)
		c.Assert(err, qt.IsNil)
		c.Assert(draftIDs(oldest), qt.DeepEquals, []string{first.ID})
	})

	t.Run("a promoted draft cannot be discarded", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		draft := createContractDraft(c, env, "key-1", "Drill")
		_, err := env.Drafts.Promote(env.Ctx, draft.ID, 1, "promotion-1", contractItem(env, "Drill"))
		c.Assert(err, qt.IsNil)
		_, err = env.Drafts.MarkDiscarding(env.Ctx, draft.ID)
		c.Assert(err, qt.ErrorIs, registry.ErrDraftPromoted)
		got, err := env.Drafts.Get(env.Ctx, draft.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Status, qt.Equals, models.CommodityDraftPromoted)
	})

	t.Run("attach a file to an open draft", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		draft := createContractDraft(c, env, "key-1", "Drill")
		photo := contractFile(c, env, "photo.jpg")

		link, created, err := env.Drafts.AttachFile(env.Ctx, draft.ID, models.FileLink{FileID: photo.ID, Meta: models.FileLinkMetaImages})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsTrue)
		c.Assert(link.EntityType, qt.Equals, models.FileLinkDraft)
		c.Assert(link.EntityID, qt.Equals, draft.ID)
		_, created, err = env.Drafts.AttachFile(env.Ctx, draft.ID, models.FileLink{FileID: photo.ID, Meta: models.FileLinkMetaImages})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsFalse)
		onDraft, err := env.Files.ListByLinkedEntity(env.Ctx, models.FileLinkDraft, draft.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(onDraft, qt.HasLen, 1)

		_, _, err = env.Drafts.AttachFile(env.Ctx, "missing", models.FileLink{FileID: photo.ID, Meta: models.FileLinkMetaImages})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		_, _, err = env.Drafts.AttachFile(env.Ctx, draft.ID, models.FileLink{FileID: "missing", Meta: models.FileLinkMetaImages})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		binned := contractFile(c, env, "binned.jpg")
		trashed, err := env.Files.TrashUnlessHeld(env.Ctx, binned.ID, models.FileLinkDraft, draft.ID, "someone", time.Now())
		c.Assert(err, qt.IsNil)
		c.Assert(trashed, qt.IsTrue)
		_, _, err = env.Drafts.AttachFile(env.Ctx, draft.ID, models.FileLink{FileID: binned.ID, Meta: models.FileLinkMetaImages})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound, qt.Commentf("a file in Trash takes no new links"))

		_, err = env.Drafts.Promote(env.Ctx, draft.ID, 1, "promotion-1", contractItem(env, "Drill"))
		c.Assert(err, qt.IsNil)
		late := contractFile(c, env, "late.jpg")
		_, _, err = env.Drafts.AttachFile(env.Ctx, draft.ID, models.FileLink{FileID: late.ID, Meta: models.FileLinkMetaImages})
		c.Assert(err, qt.ErrorIs, registry.ErrDraftPromoted)
	})

	t.Run("a draft file goes to Trash unless something else holds it", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		draft := createContractDraft(c, env, "key-1", "Drill")
		live, err := env.Commodities.Create(env.Ctx, contractItem(env, "Live"))
		c.Assert(err, qt.IsNil)
		gone, err := env.Commodities.Create(env.Ctx, contractItem(env, "Gone"))
		c.Assert(err, qt.IsNil)
		_, err = env.Commodities.Trash(env.Ctx, gone.ID, "someone", time.Now())
		c.Assert(err, qt.IsNil)

		link := func(file *models.FileEntity, entityType, entityID string) {
			c.Helper()
			_, _, err := env.Files.AddLink(env.Ctx, models.FileLink{FileID: file.ID, EntityType: entityType, EntityID: entityID, Meta: models.FileLinkMetaImages})
			c.Assert(err, qt.IsNil)
		}
		only := contractFile(c, env, "only.jpg")
		link(only, models.FileLinkDraft, draft.ID)
		shared := contractFile(c, env, "shared.jpg")
		link(shared, models.FileLinkDraft, draft.ID)
		link(shared, models.FileLinkCommodity, live.ID)
		withTrashed := contractFile(c, env, "with-trashed.jpg")
		link(withTrashed, models.FileLinkDraft, draft.ID)
		link(withTrashed, models.FileLinkCommodity, gone.ID)

		now := time.Now()
		trashed, err := env.Files.TrashUnlessHeld(env.Ctx, only.ID, models.FileLinkDraft, draft.ID, "someone", now)
		c.Assert(err, qt.IsNil)
		c.Assert(trashed, qt.IsTrue)
		got, err := env.Files.GetTrashed(env.Ctx, only.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.DeletedWithCommodityID, qt.IsNil)

		trashed, err = env.Files.TrashUnlessHeld(env.Ctx, shared.ID, models.FileLinkDraft, draft.ID, "someone", now)
		c.Assert(err, qt.IsNil)
		c.Assert(trashed, qt.IsFalse)
		_, err = env.Files.Get(env.Ctx, shared.ID)
		c.Assert(err, qt.IsNil)

		trashed, err = env.Files.TrashUnlessHeld(env.Ctx, withTrashed.ID, models.FileLinkDraft, draft.ID, "someone", now)
		c.Assert(err, qt.IsNil)
		c.Assert(trashed, qt.IsTrue)
		got, err = env.Files.GetTrashed(env.Ctx, withTrashed.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.DeletedWithCommodityID, qt.IsNotNil)
		c.Assert(*got.DeletedWithCommodityID, qt.Equals, gone.ID)

		// A file already in Trash is left as it is.
		trashed, err = env.Files.TrashUnlessHeld(env.Ctx, only.ID, models.FileLinkDraft, draft.ID, "someone", now)
		c.Assert(err, qt.IsNil)
		c.Assert(trashed, qt.IsFalse)
	})
}

func createContractDraft(c *qt.C, env DraftEnv, key, name string) *models.CommodityDraft {
	c.Helper()
	draft, created, err := env.Drafts.CreateOrGet(env.Ctx, models.CommodityDraft{
		ClientKey: key,
		Values:    models.CommodityDraftValues{"name": name},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	return draft
}

func contractItem(env DraftEnv, name string) models.Commodity {
	return models.Commodity{
		Name:      name,
		ShortName: name,
		Type:      models.CommodityTypeOther,
		AreaID:    new(env.AreaID),
		Count:     1,
		Status:    models.CommodityStatusInUse,
		Tags:      models.ValuerSlice[string]{},
	}
}

func contractFile(c *qt.C, env DraftEnv, name string) *models.FileEntity {
	c.Helper()
	file, err := env.Files.Create(env.Ctx, models.FileEntity{
		Title:    name,
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		File: &models.File{
			Path:         name,
			OriginalPath: "files/" + name,
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	})
	c.Assert(err, qt.IsNil)
	return file
}

func draftIDs(drafts []*models.CommodityDraft) []string {
	ids := make([]string, 0, len(drafts))
	for _, d := range drafts {
		ids = append(ids, d.ID)
	}
	return ids
}
