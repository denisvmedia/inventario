package memory_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
)

// The Files page and its record-to-record walk page through the same list
// several times. Files created in the same instant must still come back in
// one fixed order, or a file repeats on two pages or is skipped.
func TestFileRegistry_Memory_OrderNewestFirstThenID(t *testing.T) {
	c := qt.New(t)

	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "tenant-1",
		},
		Slug: "g1",
	})
	reg := memory.NewFileRegistryFactory().MustCreateUserRegistry(ctx)

	older := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	same := older.Add(time.Hour)
	newer := same.Add(time.Hour)

	create := func(title string, at time.Time) string {
		created, err := reg.Create(ctx, models.FileEntity{
			Title:     title,
			Category:  models.FileCategoryDocuments,
			Tags:      models.StringSlice{"walk"},
			CreatedAt: at,
			UpdatedAt: at,
			File:      &models.File{Path: title, OriginalPath: title + ".pdf", Ext: ".pdf", MIMEType: "application/pdf"},
		})
		c.Assert(err, qt.IsNil)
		return created.ID
	}
	// Insertion order differs from the expected order on purpose.
	tie := []string{create("tie-a", same), create("tie-b", same)}
	oldID := create("old", older)
	tie = append(tie, create("tie-c", same))
	newID := create("new", newer)

	slices.SortFunc(tie, func(a, b string) int { return strings.Compare(b, a) })
	want := append(append([]string{newID}, tie...), oldID)

	ids := func(files []*models.FileEntity) []string {
		out := make([]string, 0, len(files))
		for _, f := range files {
			out = append(out, f.ID)
		}
		return out
	}

	c.Run("Search", func(c *qt.C) {
		got, err := reg.Search(ctx, "", nil, nil, []string{"walk"}, nil, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(ids(got), qt.DeepEquals, want)
	})

	c.Run("ListPaginated walks every file once", func(c *qt.C) {
		var walked []string
		for offset := 0; offset < len(want); offset += 2 {
			page, total, err := reg.ListPaginated(ctx, offset, 2, nil, nil, nil, nil)
			c.Assert(err, qt.IsNil)
			c.Assert(total, qt.Equals, len(want))
			walked = append(walked, ids(page)...)
		}
		c.Assert(walked, qt.DeepEquals, want)
	})
}
