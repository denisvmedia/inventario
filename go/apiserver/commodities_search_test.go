package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
)

// The items list and the search endpoint share one query contract
// (services/commodityquery): the same query string has to select the same
// items on both, explain the match the same way, and refuse the same
// unsupported conditions.
func TestCommoditySearch_SharedContract(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	commodityReg := must.Must(params.FactorySet.CommodityRegistryFactory.CreateUserRegistry(ctx))
	tagReg := must.Must(params.FactorySet.TagRegistryFactory.CreateUserRegistry(ctx))

	must.Must(tagReg.Create(ctx, models.Tag{
		Kind: models.TagKindCommodity, Slug: "shed", Label: "Garden shed", Color: models.TagColorGreen,
	}))
	target := must.Must(commodityReg.Create(ctx, models.Commodity{
		Name:                  "Hedge Trimmer",
		ShortName:             "Trimmer",
		Type:                  models.CommodityTypeOther,
		OriginalPrice:         new(decimal.RequireFromString("10.00")),
		OriginalPriceCurrency: models.Currency("USD"),
		CurrentPrice:          new(decimal.RequireFromString("10.00")),
		Count:                 1,
		Status:                models.CommodityStatusInUse,
		PurchaseDate:          models.ToPDate("2023-01-01"),
		SerialNumber:          "HT-778899",
		Tags:                  models.ValuerSlice[string]{"shed"},
	}))

	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	get := func(c *qt.C, path string) (int, []byte) {
		c.Helper()
		req := must.Must(http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+path, nil))
		addTestUserAuthHeader(req, testUser.ID)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code, rr.Body.Bytes()
	}

	matchPath := `$.meta.matches["` + target.ID + `"][0]`
	for _, tc := range []struct {
		name, path, idPath string
	}{
		{"list", "/commodities?q=ht-7788", "$.data[0].id"},
		{"search", "/search?type=commodities&q=ht-7788", "$.data[0].id"},
	} {
		c.Run(tc.name+" finds an item by serial number and says so", func(c *qt.C) {
			code, body := get(c, tc.path)
			c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
			c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), 1)
			c.Check(body, checkers.JSONPathEquals(tc.idPath), target.ID)
			c.Check(body, checkers.JSONPathEquals(matchPath+".field"), "serial_number")
			c.Check(body, checkers.JSONPathEquals(matchPath+".value"), "HT-778899")
		})
	}

	c.Run("a tag label finds the item", func(c *qt.C) {
		code, body := get(c, "/search?type=commodities&q=%22garden+shed%22")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.meta.total"), float64(1))
		c.Check(body, checkers.JSONPathEquals(matchPath+".field"), "tags")
		c.Check(body, checkers.JSONPathEquals(matchPath+".value"), "shed")
	})

	c.Run("the tag filter narrows the list", func(c *qt.C) {
		code, body := get(c, "/commodities?tags=shed")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), 1)
		c.Check(body, checkers.JSONPathEquals("$.data[0].id"), target.ID)

		code, body = get(c, "/commodities?tags_exclude=shed")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathMatches("$.data[*].id", qt.Not(qt.Contains)), target.ID)
	})

	c.Run("tag counts follow the query and set its tag filter aside", func(c *qt.C) {
		code, body := get(c, "/commodities/tag-counts?q=ht-7788&tags_exclude=shed")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.data.tags.shed"), float64(1))
		c.Check(body, checkers.JSONPathEquals("$.data.untagged"), float64(0))

		code, body = get(c, "/commodities/tag-counts?q=nothing-matches-this")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathMatches("$.data.tags", qt.HasLen), 0)
	})

	c.Run("trashed=true selects the items in Trash", func(c *qt.C) {
		trashed := must.Must(commodityReg.Create(ctx, models.Commodity{
			Name:                   "Hedge Shears",
			ShortName:              "Shears",
			Type:                   models.CommodityTypeOther,
			OriginalPrice:          decimal.RequireFromString("5.00"),
			OriginalPriceCurrency:  models.Currency("USD"),
			ConvertedOriginalPrice: decimal.Zero,
			CurrentPrice:           decimal.RequireFromString("5.00"),
			Count:                  1,
			Status:                 models.CommodityStatusInUse,
			PurchaseDate:           models.ToPDate("2023-01-01"),
			Tags:                   models.ValuerSlice[string]{"shed"},
		}))
		must.Must(commodityReg.Trash(ctx, trashed.ID, testUser.ID, time.Now()))

		for _, path := range []string{"/commodities?q=hedge", "/search?type=commodities&q=hedge"} {
			code, body := get(c, path)
			c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
			c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), 1, qt.Commentf("GET %s", path))
			c.Check(body, checkers.JSONPathEquals("$.data[0].id"), target.ID, qt.Commentf("GET %s", path))

			code, body = get(c, path+"&trashed=true&tags=shed")
			c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
			c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), 1, qt.Commentf("GET %s in Trash", path))
			c.Check(body, checkers.JSONPathEquals("$.data[0].id"), trashed.ID, qt.Commentf("GET %s in Trash", path))
		}

		code, body := get(c, "/commodities/tag-counts?q=shears")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathMatches("$.data.tags", qt.HasLen), 0)

		code, body = get(c, "/commodities/tag-counts?q=shears&trashed=true")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.data.tags.shed"), float64(1))
	})

	c.Run("a list without q carries no matches", func(c *qt.C) {
		code, body := get(c, "/commodities")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(string(body), qt.Not(qt.Contains), `"matches"`)
	})

	for _, tc := range []struct {
		name, path, param string
	}{
		{"list refuses an unknown parameter", "/commodities?colour=red", "colour"},
		{"list refuses no_tags with tags", "/commodities?no_tags=true&tags=shed", "no_tags"},
		{"tag counts refuse pagination", "/commodities/tag-counts?page=2", "page"},
		{"tag counts refuse a value they cannot decode", "/commodities/tag-counts?status=%ZZ", "status"},
		{"search refuses the old tag operator", "/search?type=commodities&q=x&operator=AND", "operator"},
		{"search refuses an unknown warranty status", "/search?type=commodities&q=x&warranty_status=soon", "warranty_status"},
		{"list refuses a value it cannot decode", "/commodities?status=%ZZ", "status"},
		{"list refuses a semicolon separator", "/commodities?status=sold;lent_out=true", "status"},
		{"search refuses a value it cannot decode", "/search?type=commodities&q=x&lent_out=%G1", "lent_out"},
		{"list refuses an unreadable flag", "/commodities?unassigned=invalid", "unassigned"},
		{"list refuses an unknown tag mode without tags", "/commodities?tag_mode=some", "tag_mode"},
		{"list refuses lent_out in the trash", "/commodities?trashed=true&lent_out=true", "lent_out"},
	} {
		c.Run(tc.name, func(c *qt.C) {
			code, body := get(c, tc.path)
			c.Assert(code, qt.Equals, http.StatusBadRequest, qt.Commentf("body: %s", body))
			c.Check(body, checkers.JSONPathEquals("$.errors[0].code"), "query.invalid_parameter")
			c.Check(body, checkers.JSONPathEquals("$.errors[0].meta.parameter"), tc.param)
		})
	}
}
