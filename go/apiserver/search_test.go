package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
)

// The header search groups its results by kind, so /search answers for
// locations, areas and tags as well as for items and files.
func TestSearch_LocationsAreasAndTags(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	locationReg := must.Must(params.FactorySet.LocationRegistryFactory.CreateUserRegistry(ctx))
	areaReg := must.Must(params.FactorySet.AreaRegistryFactory.CreateUserRegistry(ctx))
	tagReg := must.Must(params.FactorySet.TagRegistryFactory.CreateUserRegistry(ctx))

	cottage := must.Must(locationReg.Create(ctx, models.Location{Name: "Weekend Cottage", Address: "Lakeside Road 7"}))
	boathouse := must.Must(locationReg.Create(ctx, models.Location{Name: "Boathouse", Address: "Lakeside Road 9"}))
	loft := must.Must(areaReg.Create(ctx, models.Area{Name: "Cottage Loft", LocationID: cottage.ID}))
	must.Must(areaReg.Create(ctx, models.Area{Name: "Boat Shed", LocationID: boathouse.ID}))
	// Ends in the final sigma ς, which strings.ToLower keeps apart from the
	// σ that Σ lowers to.
	street := must.Must(areaReg.Create(ctx, models.Area{Name: "Οδός", LocationID: boathouse.ID}))
	itemTag := must.Must(tagReg.Create(ctx, models.Tag{
		Kind: models.TagKindCommodity, Slug: "lake-gear", Label: "Lake gear", Color: models.TagColorBlue,
	}))
	fileTag := must.Must(tagReg.Create(ctx, models.Tag{
		Kind: models.TagKindFile, Slug: "lake-permits", Label: "Lake permits", Color: models.TagColorGreen,
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

	c.Run("locations match on name and address, sorted by name", func(c *qt.C) {
		code, body := get(c, "/search?type=locations&q=lakeside")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.meta.entity_type"), "locations")
		c.Check(body, checkers.JSONPathEquals("$.meta.total"), float64(2))
		c.Check(body, checkers.JSONPathEquals("$.data[0].id"), boathouse.ID)
		c.Check(body, checkers.JSONPathEquals("$.data[1].id"), cottage.ID)
	})

	c.Run("every term has to match a location", func(c *qt.C) {
		code, body := get(c, "/search?type=locations&q=cottage+road")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.meta.total"), float64(1))
		c.Check(body, checkers.JSONPathEquals("$.data[0].id"), cottage.ID)
		c.Check(body, checkers.JSONPathEquals("$.data[0].name"), "Weekend Cottage")
	})

	c.Run("areas match on name", func(c *qt.C) {
		code, body := get(c, "/search?type=areas&q=LOFT")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.meta.entity_type"), "areas")
		c.Check(body, checkers.JSONPathEquals("$.meta.total"), float64(1))
		c.Check(body, checkers.JSONPathEquals("$.data[0].id"), loft.ID)
		c.Check(body, checkers.JSONPathEquals("$.data[0].location_id"), cottage.ID)
	})

	c.Run("case is folded, final sigma included", func(c *qt.C) {
		for _, q := range []string{"ΟΔΌΣ", "οδόσ", "ΔΌΣ", "Οδός"} {
			code, body := get(c, "/search?type=areas&q="+url.QueryEscape(q))
			c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("q %s, body: %s", q, body))
			c.Check(body, checkers.JSONPathEquals("$.meta.total"), float64(1), qt.Commentf("q %s", q))
			c.Check(body, checkers.JSONPathEquals("$.data[0].id"), street.ID, qt.Commentf("q %s", q))
		}
	})

	c.Run("an area does not match on its location's address", func(c *qt.C) {
		code, body := get(c, "/search?type=areas&q=lakeside")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.meta.total"), float64(0))
		c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), 0)
	})

	c.Run("the total counts every match, not the page", func(c *qt.C) {
		code, body := get(c, "/search?type=locations&q=lakeside&limit=1&offset=1")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.meta.total"), float64(2))
		c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), 1)
		c.Check(body, checkers.JSONPathEquals("$.data[0].id"), cottage.ID)
	})

	c.Run("tags of both kinds match on label or slug", func(c *qt.C) {
		code, body := get(c, "/search?type=tags&q=lake")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.meta.entity_type"), "tags")
		c.Check(body, checkers.JSONPathEquals("$.meta.total"), float64(2))
		c.Check(body, checkers.JSONPathEquals("$.data[0].id"), itemTag.ID)
		c.Check(body, checkers.JSONPathEquals("$.data[0].kind"), "commodity")
		c.Check(body, checkers.JSONPathEquals("$.data[1].id"), fileTag.ID)
		c.Check(body, checkers.JSONPathEquals("$.data[1].kind"), "file")

		code, body = get(c, "/search?type=tags&q=lake-permits")
		c.Assert(code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.meta.total"), float64(1))
		c.Check(body, checkers.JSONPathEquals("$.data[0].slug"), "lake-permits")
	})

	c.Run("an unknown entity type is refused", func(c *qt.C) {
		code, body := get(c, "/search?type=members&q=lake")
		c.Assert(code, qt.Equals, http.StatusBadRequest, qt.Commentf("body: %s", body))
	})
}
