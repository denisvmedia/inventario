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
			models.FileCategoryPhotos:    3,
			models.FileCategoryInvoices:  5,
			models.FileCategoryDocuments: 1,
			models.FileCategoryOther:     2,
		}
		resp := jsonapi.NewFileCategoryCountsResponse(counts)
		c.Assert(resp.Data.Photos, qt.Equals, 3)
		c.Assert(resp.Data.Invoices, qt.Equals, 5)
		c.Assert(resp.Data.Documents, qt.Equals, 1)
		c.Assert(resp.Data.Other, qt.Equals, 2)
		c.Assert(resp.Data.All, qt.Equals, 11)
	})

	t.Run("missing buckets default to zero", func(t *testing.T) {
		c := qt.New(t)

		resp := jsonapi.NewFileCategoryCountsResponse(map[models.FileCategory]int{
			models.FileCategoryPhotos: 4,
		})
		c.Assert(resp.Data.Photos, qt.Equals, 4)
		c.Assert(resp.Data.Invoices, qt.Equals, 0)
		c.Assert(resp.Data.Documents, qt.Equals, 0)
		c.Assert(resp.Data.Other, qt.Equals, 0)
		c.Assert(resp.Data.All, qt.Equals, 4)
	})

	t.Run("JSON shape matches the FE tile renderer contract", func(t *testing.T) {
		c := qt.New(t)

		resp := jsonapi.NewFileCategoryCountsResponse(map[models.FileCategory]int{
			models.FileCategoryPhotos:   1,
			models.FileCategoryInvoices: 2,
		})
		raw, err := json.Marshal(resp)
		c.Assert(err, qt.IsNil)

		var parsed map[string]map[string]int
		err = json.Unmarshal(raw, &parsed)
		c.Assert(err, qt.IsNil)
		c.Assert(parsed["data"]["photos"], qt.Equals, 1)
		c.Assert(parsed["data"]["invoices"], qt.Equals, 2)
		c.Assert(parsed["data"]["documents"], qt.Equals, 0)
		c.Assert(parsed["data"]["other"], qt.Equals, 0)
		c.Assert(parsed["data"]["all"], qt.Equals, 3)
	})
}
