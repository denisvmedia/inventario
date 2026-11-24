package postgres_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
)

// The Files page and its record-to-record walk page through the same list
// several times. Files created in the same instant must still come back in
// one fixed order, or a file repeats on two pages or is skipped.
func TestFileRegistry_Postgres_OrderNewestFirstThenID(t *testing.T) {
	c := qt.New(t)

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	user := getTestUser(c, registrySet)
	ctx := appctx.WithUser(c.Context(), user)
	reg := registrySet.FileRegistry

	older := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	same := older.Add(time.Hour)
	newer := same.Add(time.Hour)

	create := func(title string, at time.Time) string {
		created, err := reg.Create(ctx, models.FileEntity{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID:        user.TenantID,
				CreatedByUserID: user.ID,
			},
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
		cat := models.FileCategoryDocuments
		var walked []string
		for offset := 0; offset < len(want); offset += 2 {
			page, total, err := reg.ListPaginated(ctx, offset, 2, nil, &cat, nil, nil)
			c.Assert(err, qt.IsNil)
			c.Assert(total, qt.Equals, len(want))
			walked = append(walked, ids(page)...)
		}
		c.Assert(walked, qt.DeepEquals, want)
	})
}
