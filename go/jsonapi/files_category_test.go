package jsonapi_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
)

func TestNewFileCategoryCountsResponse(t *testing.T) {
	t.Run("populates the three buckets and computes all (#1622)", func(t *testing.T) {
		c := qt.New(t)

		counts := map[models.FileCategory]int{
			models.FileCategoryImages:    3,
			models.FileCategoryDocuments: 6,
			models.FileCategoryOther:     2,
		}
		bytes := map[models.FileCategory]int64{
			models.FileCategoryImages:    1024,
			models.FileCategoryDocuments: 4096,
			models.FileCategoryOther:     8192,
		}
		resp := jsonapi.NewFileCategoryCountsResponse(counts, bytes)
		c.Assert(resp.Data.Images, qt.Equals, 3)
		c.Assert(resp.Data.Documents, qt.Equals, 6)
		c.Assert(resp.Data.Other, qt.Equals, 2)
		c.Assert(resp.Data.All, qt.Equals, 11)
		c.Assert(resp.Data.Bytes.Images, qt.Equals, int64(1024))
		c.Assert(resp.Data.Bytes.Documents, qt.Equals, int64(4096))
		c.Assert(resp.Data.Bytes.Other, qt.Equals, int64(8192))
		c.Assert(resp.Data.Bytes.All, qt.Equals, int64(1024+4096+8192))
	})

	t.Run("missing buckets default to zero", func(t *testing.T) {
		c := qt.New(t)

		resp := jsonapi.NewFileCategoryCountsResponse(
			map[models.FileCategory]int{
				models.FileCategoryImages: 4,
			},
			map[models.FileCategory]int64{
				models.FileCategoryImages: 1024,
			},
		)
		c.Assert(resp.Data.Images, qt.Equals, 4)
		c.Assert(resp.Data.Documents, qt.Equals, 0)
		c.Assert(resp.Data.Other, qt.Equals, 0)
		c.Assert(resp.Data.All, qt.Equals, 4)
		c.Assert(resp.Data.Bytes.Images, qt.Equals, int64(1024))
		c.Assert(resp.Data.Bytes.All, qt.Equals, int64(1024))
	})

	t.Run("JSON shape matches the FE tile renderer contract", func(t *testing.T) {
		c := qt.New(t)

		resp := jsonapi.NewFileCategoryCountsResponse(
			map[models.FileCategory]int{
				models.FileCategoryImages:    1,
				models.FileCategoryDocuments: 2,
			},
			map[models.FileCategory]int64{
				models.FileCategoryImages:    100,
				models.FileCategoryDocuments: 200,
			},
		)
		raw, err := json.Marshal(resp)
		c.Assert(err, qt.IsNil)

		var parsed struct {
			Data struct {
				Images    int `json:"images"`
				Documents int `json:"documents"`
				Other     int `json:"other"`
				All       int `json:"all"`
				Bytes     struct {
					Images    int64 `json:"images"`
					Documents int64 `json:"documents"`
					Other     int64 `json:"other"`
					All       int64 `json:"all"`
				} `json:"bytes"`
			} `json:"data"`
		}
		err = json.Unmarshal(raw, &parsed)
		c.Assert(err, qt.IsNil)
		c.Assert(parsed.Data.Images, qt.Equals, 1)
		c.Assert(parsed.Data.Documents, qt.Equals, 2)
		c.Assert(parsed.Data.Other, qt.Equals, 0)
		c.Assert(parsed.Data.All, qt.Equals, 3)
		c.Assert(parsed.Data.Bytes.Images, qt.Equals, int64(100))
		c.Assert(parsed.Data.Bytes.Documents, qt.Equals, int64(200))
		c.Assert(parsed.Data.Bytes.All, qt.Equals, int64(300))
	})

	t.Run("legacy `invoices` map key is ignored (post-#1622)", func(t *testing.T) {
		c := qt.New(t)

		// If a stale caller still hands us a map keyed by the dropped
		// FileCategory("invoices"), it should be silently ignored — not
		// folded into another bucket and not crash the response builder.
		resp := jsonapi.NewFileCategoryCountsResponse(
			map[models.FileCategory]int{
				models.FileCategoryImages:       1,
				models.FileCategory("invoices"): 99,
			},
			map[models.FileCategory]int64{
				models.FileCategoryImages:       100,
				models.FileCategory("invoices"): 999,
			},
		)
		c.Assert(resp.Data.Images, qt.Equals, 1)
		c.Assert(resp.Data.Documents, qt.Equals, 0)
		c.Assert(resp.Data.Other, qt.Equals, 0)
		c.Assert(resp.Data.All, qt.Equals, 1)
		c.Assert(resp.Data.Bytes.Images, qt.Equals, int64(100))
		c.Assert(resp.Data.Bytes.All, qt.Equals, int64(100))
	})
}
