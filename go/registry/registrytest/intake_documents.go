package registrytest

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// IntakeDocumentEnv is what RunIntakeDocumentContract needs from a backend:
// the user registry of delivered documents of one empty group with the
// context that carries the group scope, the service registry, and the
// group's drafts with an area an item can be promoted into.
type IntakeDocumentEnv struct {
	Ctx       context.Context
	Documents registry.IntakeDocumentRegistry
	Service   registry.IntakeDocumentRegistry
	Drafts    registry.CommodityDraftRegistry
	AreaID    string
}

// RunIntakeDocumentContract checks the record of delivered documents a
// backend's IntakeDocumentRegistry implements (#2809): one document per
// external identity in the group, progress recorded once, the lookups by
// draft and by item, the item a promotion records, and the refusal of the
// service registry. newEnv is called once per subtest and must return an
// empty group.
func RunIntakeDocumentContract(t *testing.T, newEnv func(t *testing.T) IntakeDocumentEnv) {
	tests := []struct {
		name string
		run  func(t *testing.T, env IntakeDocumentEnv)
	}{
		{"create or get keeps one document per identity", intakeDocumentContractCreateOrGet},
		{"concurrent creates store one document", intakeDocumentContractConcurrentCreate},
		{"progress is recorded once", intakeDocumentContractProgress},
		{"documents list by draft and by item", intakeDocumentContractLists},
		{"a promotion records the item", intakeDocumentContractPromotion},
		{"recording a promoted draft records its item", intakeDocumentContractPromotedFirst},
		{"the service registry is refused", intakeDocumentContractNeedsGroup},
		{"restored timestamps are kept", intakeDocumentContractTimestamps},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, newEnv(t))
		})
	}
}

func contractIntakeDocument(id string) models.IntakeDocument {
	return models.IntakeDocument{
		ExternalSystem:     "paperless-ngx",
		ExternalInstance:   "home",
		ExternalDocumentID: id,
		SourceName:         "Paperless-ngx",
		SourceURL:          "https://paperless.example/documents/" + id + "/",
		PayloadHash:        "hash-" + id,
	}
}

func intakeDocumentContractCreateOrGet(t *testing.T, env IntakeDocumentEnv) {
	c := qt.New(t)
	doc, created, err := env.Documents.CreateOrGet(env.Ctx, contractIntakeDocument("7"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	c.Assert(doc.ID, qt.Not(qt.Equals), "")
	c.Assert(doc.GroupID, qt.Not(qt.Equals), "")
	c.Assert(doc.CreatedAt.IsZero(), qt.IsFalse)
	c.Assert(doc.DraftID, qt.IsNil)

	again := contractIntakeDocument("7")
	again.PayloadHash = "another-hash"
	same, created, err := env.Documents.CreateOrGet(env.Ctx, again)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsFalse)
	c.Assert(same.ID, qt.Equals, doc.ID)
	c.Assert(same.PayloadHash, qt.Equals, "hash-7")

	// Another instance or document id is another document.
	other := contractIntakeDocument("7")
	other.ExternalInstance = "office"
	_, created, err = env.Documents.CreateOrGet(env.Ctx, other)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)

	found, err := env.Documents.GetByExternalID(env.Ctx, "paperless-ngx", "home", "7")
	c.Assert(err, qt.IsNil)
	c.Assert(found.ID, qt.Equals, doc.ID)
	_, err = env.Documents.GetByExternalID(env.Ctx, "paperless-ngx", "home", "8")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	_, err = env.Documents.Create(env.Ctx, contractIntakeDocument("7"))
	c.Assert(err, qt.ErrorIs, registry.ErrAlreadyExists)

	bad := contractIntakeDocument("9")
	bad.SourceURL = "ftp://paperless.example/9"
	_, _, err = env.Documents.CreateOrGet(env.Ctx, bad)
	c.Assert(err, qt.IsNotNil)
}

func intakeDocumentContractConcurrentCreate(t *testing.T, env IntakeDocumentEnv) {
	c := qt.New(t)
	const n = 8
	ids := make([]string, n)
	created := make([]bool, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			doc, isNew, err := env.Documents.CreateOrGet(env.Ctx, contractIntakeDocument("race"))
			errs[i], created[i] = err, isNew
			if doc != nil {
				ids[i] = doc.ID
			}
		})
	}
	wg.Wait()
	made := 0
	for i := range n {
		c.Assert(errs[i], qt.IsNil)
		c.Assert(ids[i], qt.Equals, ids[0])
		if created[i] {
			made++
		}
	}
	c.Assert(made, qt.Equals, 1)
}

func intakeDocumentContractProgress(t *testing.T, env IntakeDocumentEnv) {
	c := qt.New(t)
	doc, _, err := env.Documents.CreateOrGet(env.Ctx, contractIntakeDocument("7"))
	c.Assert(err, qt.IsNil)

	got, err := env.Documents.RecordProgress(env.Ctx, doc.ID, registry.IntakeDocumentProgress{SessionID: new("session-1")})
	c.Assert(err, qt.IsNil)
	c.Assert(*got.SessionID, qt.Equals, "session-1")
	c.Assert(got.DraftID, qt.IsNil)

	got, err = env.Documents.RecordProgress(env.Ctx, doc.ID, registry.IntakeDocumentProgress{
		SessionID: new("session-2"),
		DraftID:   new("draft-1"),
		FileID:    new("file-1"),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(*got.SessionID, qt.Equals, "session-1")
	c.Assert(*got.DraftID, qt.Equals, "draft-1")
	c.Assert(*got.FileID, qt.Equals, "file-1")

	recorded := got.UpdatedAt
	got, err = env.Documents.RecordProgress(env.Ctx, doc.ID, registry.IntakeDocumentProgress{DraftID: new("draft-2")})
	c.Assert(err, qt.IsNil)
	c.Assert(*got.DraftID, qt.Equals, "draft-1")
	// A repeat that changes nothing leaves the update time.
	c.Assert(got.UpdatedAt.Equal(recorded), qt.IsTrue, qt.Commentf("updated_at %v, recorded %v", got.UpdatedAt, recorded))

	_, err = env.Documents.RecordProgress(env.Ctx, "missing", registry.IntakeDocumentProgress{DraftID: new("draft-1")})
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	got, err = env.Documents.RecordProgress(env.Ctx, doc.ID, registry.IntakeDocumentProgress{CommodityID: new("item-1")})
	c.Assert(err, qt.IsNil)
	c.Assert(*got.CommodityID, qt.Equals, "item-1")
	got, err = env.Documents.RecordProgress(env.Ctx, doc.ID, registry.IntakeDocumentProgress{CommodityID: new("item-2")})
	c.Assert(err, qt.IsNil)
	c.Assert(*got.CommodityID, qt.Equals, "item-1")
}

func intakeDocumentContractLists(t *testing.T, env IntakeDocumentEnv) {
	c := qt.New(t)
	first, _, err := env.Documents.CreateOrGet(env.Ctx, contractIntakeDocument("1"))
	c.Assert(err, qt.IsNil)
	second, _, err := env.Documents.CreateOrGet(env.Ctx, contractIntakeDocument("2"))
	c.Assert(err, qt.IsNil)
	_, err = env.Documents.RecordProgress(env.Ctx, first.ID, registry.IntakeDocumentProgress{DraftID: new("draft-a")})
	c.Assert(err, qt.IsNil)
	_, err = env.Documents.RecordProgress(env.Ctx, second.ID, registry.IntakeDocumentProgress{DraftID: new("draft-b")})
	c.Assert(err, qt.IsNil)

	docs, err := env.Documents.ListByDrafts(env.Ctx, "draft-b", "draft-a", "draft-c")
	c.Assert(err, qt.IsNil)
	c.Assert(docs, qt.HasLen, 2)
	c.Assert(docs[0].ID, qt.Equals, first.ID)
	c.Assert(docs[1].ID, qt.Equals, second.ID)

	docs, err = env.Documents.ListByDrafts(env.Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(docs, qt.HasLen, 0)
	docs, err = env.Documents.ListByCommodities(env.Ctx, "item-x")
	c.Assert(err, qt.IsNil)
	c.Assert(docs, qt.HasLen, 0)
}

func intakeDocumentContractPromotion(t *testing.T, env IntakeDocumentEnv) {
	c := qt.New(t)
	draft, _, err := env.Drafts.CreateOrGet(env.Ctx, models.CommodityDraft{
		ClientKey: "intake:candidate-1",
		Values:    models.CommodityDraftValues{"name": "Drill"},
	})
	c.Assert(err, qt.IsNil)
	doc, _, err := env.Documents.CreateOrGet(env.Ctx, contractIntakeDocument("7"))
	c.Assert(err, qt.IsNil)
	_, err = env.Documents.RecordProgress(env.Ctx, doc.ID, registry.IntakeDocumentProgress{DraftID: &draft.ID})
	c.Assert(err, qt.IsNil)
	// A document of another draft is left alone.
	other, _, err := env.Documents.CreateOrGet(env.Ctx, contractIntakeDocument("8"))
	c.Assert(err, qt.IsNil)
	_, err = env.Documents.RecordProgress(env.Ctx, other.ID, registry.IntakeDocumentProgress{DraftID: new("draft-other")})
	c.Assert(err, qt.IsNil)

	promotion, err := env.Drafts.Promote(env.Ctx, draft.ID, draft.Revision, "promotion-1", models.Commodity{
		Name:      "Drill",
		ShortName: "Drill",
		Type:      models.CommodityTypeOther,
		AreaID:    new(env.AreaID),
		Count:     1,
		Status:    models.CommodityStatusInUse,
		Tags:      models.ValuerSlice[string]{},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(promotion.Commodity, qt.IsNotNil)

	docs, err := env.Documents.ListByCommodities(env.Ctx, promotion.Commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(docs, qt.HasLen, 1)
	c.Assert(docs[0].ID, qt.Equals, doc.ID)
	c.Assert(*docs[0].DraftID, qt.Equals, draft.ID)

	untouched, err := env.Documents.Get(env.Ctx, other.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(untouched.CommodityID, qt.IsNil)
}

// intakeDocumentContractPromotedFirst: the draft became an item before the
// delivery recorded it, so the promotion found no document to update.
// Recording the draft records the item with it; a document that already
// names another draft is left alone.
func intakeDocumentContractPromotedFirst(t *testing.T, env IntakeDocumentEnv) {
	c := qt.New(t)
	draft, _, err := env.Drafts.CreateOrGet(env.Ctx, models.CommodityDraft{
		ClientKey: "intake:candidate-1",
		Values:    models.CommodityDraftValues{"name": "Drill"},
	})
	c.Assert(err, qt.IsNil)
	promotion, err := env.Drafts.Promote(env.Ctx, draft.ID, draft.Revision, "promotion-1", models.Commodity{
		Name:      "Drill",
		ShortName: "Drill",
		Type:      models.CommodityTypeOther,
		AreaID:    new(env.AreaID),
		Count:     1,
		Status:    models.CommodityStatusInUse,
		Tags:      models.ValuerSlice[string]{},
	})
	c.Assert(err, qt.IsNil)

	doc, _, err := env.Documents.CreateOrGet(env.Ctx, contractIntakeDocument("7"))
	c.Assert(err, qt.IsNil)
	got, err := env.Documents.RecordProgress(env.Ctx, doc.ID, registry.IntakeDocumentProgress{DraftID: &draft.ID})
	c.Assert(err, qt.IsNil)
	c.Assert(*got.DraftID, qt.Equals, draft.ID)
	c.Assert(got.CommodityID, qt.IsNotNil)
	c.Assert(*got.CommodityID, qt.Equals, promotion.Commodity.ID)

	other, _, err := env.Documents.CreateOrGet(env.Ctx, contractIntakeDocument("8"))
	c.Assert(err, qt.IsNil)
	_, err = env.Documents.RecordProgress(env.Ctx, other.ID, registry.IntakeDocumentProgress{DraftID: new("draft-other")})
	c.Assert(err, qt.IsNil)
	got, err = env.Documents.RecordProgress(env.Ctx, other.ID, registry.IntakeDocumentProgress{DraftID: &draft.ID})
	c.Assert(err, qt.IsNil)
	c.Assert(*got.DraftID, qt.Equals, "draft-other")
	c.Assert(got.CommodityID, qt.IsNil)
}

func intakeDocumentContractNeedsGroup(t *testing.T, env IntakeDocumentEnv) {
	c := qt.New(t)
	doc, _, err := env.Documents.CreateOrGet(env.Ctx, contractIntakeDocument("7"))
	c.Assert(err, qt.IsNil)

	_, _, err = env.Service.CreateOrGet(env.Ctx, contractIntakeDocument("8"))
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
	_, err = env.Service.GetByExternalID(env.Ctx, "paperless-ngx", "home", "7")
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
	_, err = env.Service.RecordProgress(env.Ctx, doc.ID, registry.IntakeDocumentProgress{DraftID: new("d")})
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
	_, err = env.Service.ListByDrafts(env.Ctx, "d")
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
	_, err = env.Service.ListByCommodities(env.Ctx, "i")
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)

	// The embedded registry serves the purges across groups.
	all, err := env.Service.List(env.Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 1)
	c.Assert(env.Service.Delete(env.Ctx, doc.ID), qt.IsNil)
	_, err = env.Documents.Get(env.Ctx, doc.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func intakeDocumentContractTimestamps(t *testing.T, env IntakeDocumentEnv) {
	c := qt.New(t)
	at := time.Date(2025, 3, 14, 10, 0, 0, 0, time.UTC)
	doc := contractIntakeDocument("7")
	doc.CreatedAt = at
	doc.UpdatedAt = at.Add(time.Hour)
	stored, _, err := env.Documents.CreateOrGet(env.Ctx, doc)
	c.Assert(err, qt.IsNil)
	got, err := env.Documents.Get(env.Ctx, stored.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.CreatedAt.Equal(at), qt.IsTrue, qt.Commentf("created_at %s", got.CreatedAt))
	c.Assert(got.UpdatedAt.Equal(at.Add(time.Hour)), qt.IsTrue, qt.Commentf("updated_at %s", got.UpdatedAt))
}
