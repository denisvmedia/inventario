package apiserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// tagFixture seeds one tag of the given kind and returns it along with a
// ready-to-serve handler.
func tagFixture(c *qt.C, kind models.TagKind, slug, label string) (apiserver.Params, *models.User, *models.LocationGroup, *models.Tag, http.Handler) {
	c.Helper()

	params, testUser, testGroup := newParams()
	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))

	tag := must.Must(registrySet.TagRegistry.Create(ctx, models.Tag{
		Kind:  kind,
		Slug:  slug,
		Label: label,
		Color: models.TagColorBlue,
	}))

	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	return params, testUser, testGroup, tag, handler
}

func tagsRequest(c *qt.C, handler http.Handler, method, url, userID string, body any) *httptest.ResponseRecorder {
	c.Helper()

	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(must.Must(json.Marshal(body)))
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, url, reader)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, userID)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// kind separates item-tags from file-tags, which are distinct entities
// sharing a slug namespace. There is no "all", so a request that omits it
// has to be refused rather than answered with one of the two.
func TestTagsList_KindIsRequired(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{name: "commodity", query: "?kind=commodity", want: http.StatusOK},
		{name: "file", query: "?kind=file", want: http.StatusOK},
		{name: "omitted", query: "", want: http.StatusUnprocessableEntity},
		{name: "empty", query: "?kind=", want: http.StatusUnprocessableEntity},
		{name: "unknown", query: "?kind=everything", want: http.StatusUnprocessableEntity},
		{name: "whitespace only", query: "?kind=%20", want: http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			_, testUser, testGroup, _, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

			rr := tagsRequest(c, handler, "GET", "/api/v1/g/"+testGroup.Slug+"/tags"+tt.query, testUser.ID, nil)
			c.Assert(rr.Code, qt.Equals, tt.want)
		})
	}
}

// Two tags with the same slug and different kinds are separate entities;
// a list of one kind must not return the other.
func TestTagsList_KindSeparatesTheNamespaces(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup, _, handler := tagFixture(c, models.TagKindCommodity, "invoice", "Invoice")

	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	must.Must(registrySet.TagRegistry.Create(ctx, models.Tag{
		Kind:  models.TagKindFile,
		Slug:  "invoice",
		Label: "Invoice (file)",
		Color: models.TagColorAmber,
	}))

	rr := tagsRequest(c, handler, "GET", "/api/v1/g/"+testGroup.Slug+"/tags?kind=commodity", testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 1)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].label"), "Invoice")

	rr = tagsRequest(c, handler, "GET", "/api/v1/g/"+testGroup.Slug+"/tags?kind=file", testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 1)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].label"), "Invoice (file)")
}

// The usage block costs an extra query per listing, so it is opt-in. The
// Tags page asks for it; the tag picker does not.
func TestTagsList_UsageIsOptIn(t *testing.T) {
	c := qt.New(t)

	_, testUser, testGroup, _, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

	rr := tagsRequest(c, handler, "GET", "/api/v1/g/"+testGroup.Slug+"/tags?kind=commodity", testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var withoutInclude struct {
		Data []struct {
			Meta *struct {
				Usage *registry.TagUsage `json:"usage"`
			} `json:"meta"`
		} `json:"data"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &withoutInclude), qt.IsNil)
	c.Assert(withoutInclude.Data, qt.HasLen, 1)
	c.Check(withoutInclude.Data[0].Meta, qt.IsNil)

	rr = tagsRequest(c, handler, "GET", "/api/v1/g/"+testGroup.Slug+"/tags?kind=commodity&include=usage", testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var withInclude struct {
		Data []struct {
			Meta *struct {
				Usage *registry.TagUsage `json:"usage"`
			} `json:"meta"`
		} `json:"data"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &withInclude), qt.IsNil)
	c.Assert(withInclude.Data, qt.HasLen, 1)
	c.Assert(withInclude.Data[0].Meta, qt.IsNotNil)
	c.Assert(withInclude.Data[0].Meta.Usage, qt.IsNotNil)
}

// `include` is a comma-separated list, so the match has to be on a whole
// token. A value that merely contains "usage" as a substring is a
// different extra and must not turn this one on.
func TestTagsList_IncludeMatchesWholeTokens(t *testing.T) {
	tests := []struct {
		include   string
		wantUsage bool
	}{
		{include: "usage", wantUsage: true},
		{include: "stats,usage", wantUsage: true},
		{include: " usage ", wantUsage: true},
		{include: "usages", wantUsage: false},
		{include: "disk-usage", wantUsage: false},
		{include: "", wantUsage: false},
	}

	for _, tt := range tests {
		t.Run(tt.include, func(t *testing.T) {
			c := qt.New(t)

			_, testUser, testGroup, _, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

			rr := tagsRequest(c, handler, "GET",
				"/api/v1/g/"+testGroup.Slug+"/tags?kind=commodity&include="+tt.include, testUser.ID, nil)
			c.Assert(rr.Code, qt.Equals, http.StatusOK)

			var body struct {
				Data []struct {
					Meta *struct {
						Usage *registry.TagUsage `json:"usage"`
					} `json:"meta"`
				} `json:"data"`
			}
			c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
			c.Assert(body.Data, qt.HasLen, 1)
			if tt.wantUsage {
				c.Check(body.Data[0].Meta, qt.IsNotNil)
				return
			}
			c.Check(body.Data[0].Meta, qt.IsNil)
		})
	}
}

func TestTagGet(t *testing.T) {
	c := qt.New(t)

	_, testUser, testGroup, tag, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

	rr := tagsRequest(c, handler, "GET", "/api/v1/g/"+testGroup.Slug+"/tags/"+tag.ID, testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.id"), tag.ID)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.attributes.slug"), "kitchen")
}

func TestTagGet_UnknownID(t *testing.T) {
	c := qt.New(t)

	_, testUser, testGroup, _, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

	rr := tagsRequest(c, handler, "GET", "/api/v1/g/"+testGroup.Slug+"/tags/no-such-tag", testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestTagCreate(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup, _, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

	rr := tagsRequest(c, handler, "POST", "/api/v1/g/"+testGroup.Slug+"/tags", testUser.ID,
		&jsonapi.TagRequest{Data: &jsonapi.TagRequestDataWrapper{
			Type: "tags",
			Attributes: jsonapi.TagRequestData{
				Kind:  models.TagKindCommodity,
				Slug:  "garage",
				Label: "Garage",
				Color: models.TagColorGreen,
			},
		}})
	c.Assert(rr.Code, qt.Equals, http.StatusCreated)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.attributes.slug"), "garage")

	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	stored := must.Must(registrySet.TagRegistry.GetBySlug(ctx, models.TagKindCommodity, "garage"))
	c.Assert(stored.Label, qt.Equals, "Garage")
}

func TestTagCreate_Rejected(t *testing.T) {
	tests := []struct {
		name       string
		attributes jsonapi.TagRequestData
	}{
		{
			name:       "kind omitted",
			attributes: jsonapi.TagRequestData{Slug: "garage", Label: "Garage", Color: models.TagColorGreen},
		},
		{
			name: "kind unknown",
			attributes: jsonapi.TagRequestData{
				Kind: models.TagKind("everything"), Slug: "garage", Label: "Garage", Color: models.TagColorGreen,
			},
		},
		{
			name: "slug not kebab-cased",
			attributes: jsonapi.TagRequestData{
				Kind: models.TagKindCommodity, Slug: "Not A Slug", Label: "Garage", Color: models.TagColorGreen,
			},
		},
		{
			name: "color off the curated set",
			attributes: jsonapi.TagRequestData{
				Kind: models.TagKindCommodity, Slug: "garage", Label: "Garage", Color: models.TagColor("chartreuse"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			params, testUser, testGroup, _, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

			rr := tagsRequest(c, handler, "POST", "/api/v1/g/"+testGroup.Slug+"/tags", testUser.ID,
				&jsonapi.TagRequest{Data: &jsonapi.TagRequestDataWrapper{Type: "tags", Attributes: tt.attributes}})
			c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)

			// A rejected create leaves the seeded tag as the only row.
			ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
			registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
			_, total := must.Must2(registrySet.TagRegistry.ListPaginated(ctx, 0, 50,
				registry.TagListOptions{Kind: models.TagKindCommodity}))
			c.Assert(total, qt.Equals, 1)
		})
	}
}

func TestTagUpdate(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup, tag, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

	rr := tagsRequest(c, handler, "PATCH", "/api/v1/g/"+testGroup.Slug+"/tags/"+tag.ID, testUser.ID,
		&jsonapi.TagUpdateRequest{Data: &jsonapi.TagUpdateRequestDataWrapper{
			ID:         tag.ID,
			Type:       "tags",
			Attributes: jsonapi.TagUpdateRequestData{Label: "Kitchenware", Color: models.TagColorRed},
		}})
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.attributes.label"), "Kitchenware")

	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	stored := must.Must(registrySet.TagRegistry.Get(ctx, tag.ID))
	c.Assert(stored.Label, qt.Equals, "Kitchenware")
	c.Assert(stored.Color, qt.Equals, models.TagColorRed)
	// An omitted attribute means "leave unchanged", not "clear".
	c.Assert(stored.Slug, qt.Equals, "kitchen")
}

// Slugs are unique per (group, kind), and renaming onto a taken one is a
// conflict rather than a silent merge of two tags' references.
func TestTagUpdate_SlugCollisionIsAConflict(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup, tag, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	must.Must(registrySet.TagRegistry.Create(ctx, models.Tag{
		Kind: models.TagKindCommodity, Slug: "garage", Label: "Garage", Color: models.TagColorMuted,
	}))

	rr := tagsRequest(c, handler, "PATCH", "/api/v1/g/"+testGroup.Slug+"/tags/"+tag.ID, testUser.ID,
		&jsonapi.TagUpdateRequest{Data: &jsonapi.TagUpdateRequestDataWrapper{
			ID:         tag.ID,
			Type:       "tags",
			Attributes: jsonapi.TagUpdateRequestData{Slug: "garage"},
		}})
	c.Assert(rr.Code, qt.Equals, http.StatusConflict)

	stored := must.Must(registrySet.TagRegistry.Get(ctx, tag.ID))
	c.Assert(stored.Slug, qt.Equals, "kitchen")
}

func TestTagDelete(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup, tag, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

	rr := tagsRequest(c, handler, "DELETE", "/api/v1/g/"+testGroup.Slug+"/tags/"+tag.ID, testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	_, err := registrySet.TagRegistry.Get(ctx, tag.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// Deleting a tag that commodities still carry would strip it from them,
// so it takes an explicit force. Without one the response has to say how
// much would be affected.
func TestTagDelete_InUseNeedsForce(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup, tag, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	commodities := must.Must(registrySet.CommodityRegistry.List(ctx))
	c.Assert(len(commodities) > 0, qt.IsTrue)

	tagged := *commodities[0]
	tagged.Tags = []string{tag.Slug}
	must.Must(registrySet.CommodityRegistry.Update(ctx, tagged))

	rr := tagsRequest(c, handler, "DELETE", "/api/v1/g/"+testGroup.Slug+"/tags/"+tag.ID, testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict)
	c.Assert(rr.Body.String(), qt.Contains, "commodities=1")

	stillThere := must.Must(registrySet.TagRegistry.Get(ctx, tag.ID))
	c.Assert(stillThere.ID, qt.Equals, tag.ID)

	rr = tagsRequest(c, handler, "DELETE",
		"/api/v1/g/"+testGroup.Slug+"/tags/"+tag.ID+"?force=true", testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	_, err := registrySet.TagRegistry.Get(ctx, tag.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// force strips the reference rather than leaving a dangling slug on
	// the commodity.
	updated := must.Must(registrySet.CommodityRegistry.Get(ctx, tagged.ID))
	c.Assert(updated.Tags, qt.Not(qt.Contains), tag.Slug)
}

func TestTagStats(t *testing.T) {
	c := qt.New(t)

	_, testUser, testGroup, _, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

	rr := tagsRequest(c, handler, "GET", "/api/v1/g/"+testGroup.Slug+"/tags/stats", testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var body struct {
		Data registry.TagStats `json:"data"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	c.Assert(body.Data.TagsTotal, qt.Equals, 1)
}

// /tags/autocomplete is mounted before /{tagID} so the literal path does
// not get routed as a tag id.
func TestTagAutocomplete(t *testing.T) {
	c := qt.New(t)

	_, testUser, testGroup, _, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

	rr := tagsRequest(c, handler, "GET",
		"/api/v1/g/"+testGroup.Slug+"/tags/autocomplete?kind=commodity&q=kit", testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 1)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].slug"), "kitchen")

	rr = tagsRequest(c, handler, "GET",
		"/api/v1/g/"+testGroup.Slug+"/tags/autocomplete?kind=commodity&q=nothing-matches", testUser.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 0)
}

// Every route under /tags needs an authenticated caller. A missing token
// must not fall through to an unscoped registry.
func TestTags_RequireAuth(t *testing.T) {
	tests := []struct {
		method string
		path   string
	}{
		{method: "GET", path: "/tags?kind=commodity"},
		{method: "POST", path: "/tags"},
		{method: "GET", path: "/tags/stats"},
		{method: "GET", path: "/tags/autocomplete?kind=commodity&q=k"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			c := qt.New(t)

			_, _, testGroup, _, handler := tagFixture(c, models.TagKindCommodity, "kitchen", "Kitchen")

			req, err := http.NewRequest(tt.method, "/api/v1/g/"+testGroup.Slug+tt.path, bytes.NewReader(nil))
			c.Assert(err, qt.IsNil)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
		})
	}
}
