package services_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

func (f *bulkFixture) file(c *qt.C, title string, tags ...string) *models.FileEntity {
	c.Helper()
	return must.Must(f.set.FileRegistry.Create(f.ctx, models.FileEntity{
		Title:    title,
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		Tags:     tags,
		File: &models.File{
			Path:         title,
			OriginalPath: "blobs/" + title + ".pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))
}

func (f *bulkFixture) fileTags(c *qt.C, id string) []string {
	c.Helper()
	return []string(must.Must(f.set.FileRegistry.Get(f.ctx, id)).Tags)
}

func TestParseFileTagPatch_Refuses(t *testing.T) {
	tooMany := make([]string, services.MaxBulkTags+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("tag-%d", i)
	}
	tests := []struct {
		name        string
		add, remove []string
	}{
		{"no tags", nil, nil},
		{"a tag both added and removed", []string{"Receipt"}, []string{"receipt"}},
		{"a tag that is no tag", []string{"###"}, nil},
		{"too many tags", tooMany, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			_, err := services.ParseFileTagPatch(tt.add, tt.remove)
			errs, ok := errors.AsType[validation.Errors](err)
			c.Assert(ok, qt.IsTrue, qt.Commentf("err = %v", err))
			c.Assert(errs["tags"], qt.IsNotNil, qt.Commentf("errs = %v", errs))
		})
	}
}

func TestFileTagPatch_Apply(t *testing.T) {
	c := qt.New(t)
	patch := must.Must(services.ParseFileTagPatch([]string{"Receipt", "warranty"}, []string{"Old"}))
	c.Assert(patch.AddTags, qt.DeepEquals, []string{"receipt", "warranty"})
	c.Assert(patch.RemoveTags, qt.DeepEquals, []string{"old"})

	got, changed := patch.Apply(models.FileEntity{Tags: models.StringSlice{"manual", "old", "warranty"}})
	c.Assert(changed, qt.IsTrue)
	c.Assert([]string(got.Tags), qt.DeepEquals, []string{"manual", "warranty", "receipt"})

	_, changed = patch.Apply(models.FileEntity{Tags: models.StringSlice{"receipt", "warranty"}})
	c.Assert(changed, qt.IsFalse)
}

func TestFileBulkService_UpdateTags(t *testing.T) {
	c := qt.New(t)
	f := newBulkFixture(c)
	svc := services.NewFileBulkService(services.NewTagService(f.fs))
	manual := f.file(c, "manual", "manual", "old")
	photo := f.file(c, "photo")
	receipt := f.file(c, "receipt", "receipt")
	trashed := f.file(c, "trashed")
	c.Assert(f.set.FileRegistry.Trash(f.ctx, trashed.ID, "user-1", "", time.Now()), qt.IsNil)

	patch := must.Must(services.ParseFileTagPatch([]string{"Receipt"}, []string{"old"}))
	c.Assert(svc.EnsureTags(f.ctx, &patch), qt.IsNil)
	ids := []string{manual.ID, photo.ID, receipt.ID, trashed.ID, "missing", manual.ID}
	out := svc.UpdateTags(f.ctx, f.set.FileRegistry, ids, patch)

	c.Assert(out.Succeeded, qt.DeepEquals, []string{manual.ID, photo.ID})
	c.Assert(out.Skipped, qt.DeepEquals, []services.BulkSkip{{ID: receipt.ID, Name: "receipt"}})
	c.Assert(out.Failed, qt.HasLen, 2)
	c.Assert(out.Failed[0].ID, qt.Equals, trashed.ID)
	c.Assert(out.Failed[0].Err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(out.Failed[1].ID, qt.Equals, "missing")

	c.Assert(f.fileTags(c, manual.ID), qt.DeepEquals, []string{"manual", "receipt"})
	c.Assert(f.fileTags(c, photo.ID), qt.DeepEquals, []string{"receipt"})

	// A tag the group did not have is created as a file tag, with the label
	// as typed.
	tag, err := f.set.TagRegistry.GetBySlug(f.ctx, models.TagKindFile, "receipt")
	c.Assert(err, qt.IsNil)
	c.Assert(tag.Label, qt.Equals, "Receipt")
}

func TestFileBulkService_StopsWhenTheRequestEnds(t *testing.T) {
	c := qt.New(t)
	f := newBulkFixture(c)
	svc := services.NewFileBulkService(services.NewTagService(f.fs))
	a := f.file(c, "a")
	ctx, cancel := context.WithCancel(f.ctx)
	cancel()

	patch := must.Must(services.ParseFileTagPatch([]string{"x"}, nil))
	out := svc.UpdateTags(ctx, f.set.FileRegistry, []string{a.ID}, patch)
	c.Assert(out.Succeeded, qt.HasLen, 0)
	c.Assert(out.Failed, qt.HasLen, 1)
	c.Assert(f.fileTags(c, a.ID), qt.HasLen, 0)
}
