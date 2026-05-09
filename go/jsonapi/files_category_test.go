package jsonapi_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
)

func TestNewFileCategoryCountsResponse(t *testing.T) {
	t.Run("populates all four buckets and computes all", func(t *testing.T) {
		c := qt.New(t)

		counts := map[models.FileCategory]int{
			models.FileCategoryImages:    3,
			models.FileCategoryInvoices:  5,
			models.FileCategoryDocuments: 1,
			models.FileCategoryOther:     2,
		}
		bytes := map[models.FileCategory]int64{
			models.FileCategoryImages:    1024,
			models.FileCategoryInvoices:  2048,
			models.FileCategoryDocuments: 4096,
			models.FileCategoryOther:     8192,
		}
		resp := jsonapi.NewFileCategoryCountsResponse(counts, bytes)
		c.Assert(resp.Data.Images, qt.Equals, 3)
		c.Assert(resp.Data.Invoices, qt.Equals, 5)
		c.Assert(resp.Data.Documents, qt.Equals, 1)
		c.Assert(resp.Data.Other, qt.Equals, 2)
		c.Assert(resp.Data.All, qt.Equals, 11)
		c.Assert(resp.Data.Bytes.Images, qt.Equals, int64(1024))
		c.Assert(resp.Data.Bytes.Invoices, qt.Equals, int64(2048))
		c.Assert(resp.Data.Bytes.Documents, qt.Equals, int64(4096))
		c.Assert(resp.Data.Bytes.Other, qt.Equals, int64(8192))
		c.Assert(resp.Data.Bytes.All, qt.Equals, int64(1024+2048+4096+8192))
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
		c.Assert(resp.Data.Invoices, qt.Equals, 0)
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
				models.FileCategoryImages:   1,
				models.FileCategoryInvoices: 2,
			},
			map[models.FileCategory]int64{
				models.FileCategoryImages:   100,
				models.FileCategoryInvoices: 200,
			},
		)
		raw, err := json.Marshal(resp)
		c.Assert(err, qt.IsNil)

		var parsed struct {
			Data struct {
				Images    int   `json:"images"`
				Invoices  int   `json:"invoices"`
				Documents int   `json:"documents"`
				Other     int   `json:"other"`
				All       int   `json:"all"`
				Bytes     struct {
					Images    int64 `json:"images"`
					Invoices  int64 `json:"invoices"`
					Documents int64 `json:"documents"`
					Other     int64 `json:"other"`
					All       int64 `json:"all"`
				} `json:"bytes"`
			} `json:"data"`
		}
		err = json.Unmarshal(raw, &parsed)
		c.Assert(err, qt.IsNil)
		c.Assert(parsed.Data.Images, qt.Equals, 1)
		c.Assert(parsed.Data.Invoices, qt.Equals, 2)
		c.Assert(parsed.Data.Documents, qt.Equals, 0)
		c.Assert(parsed.Data.Other, qt.Equals, 0)
		c.Assert(parsed.Data.All, qt.Equals, 3)
		c.Assert(parsed.Data.Bytes.Images, qt.Equals, int64(100))
		c.Assert(parsed.Data.Bytes.Invoices, qt.Equals, int64(200))
		c.Assert(parsed.Data.Bytes.All, qt.Equals, int64(300))
	})
}
