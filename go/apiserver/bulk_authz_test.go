package apiserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
)

// The bulk endpoints take ids and queries from the body, so the group the
// path names is the only thing that scopes them. These tests send the
// owner's ids, and a query, from a neighbour in the same tenant: from the
// neighbour's own group they must reach nothing, and the owner's group must
// refuse the neighbour outright.

// bulkAuthzItem creates an item in the owner's group of env.
func bulkAuthzItem(c *qt.C, env *filesAuthzEnv) *models.Commodity {
	c.Helper()
	ctx := createTestUserContextWithGroup(env.ownerGroup.CreatedBy, env.ownerGroup.TenantID, env.ownerGroup.ID)
	set := must.Must(env.params.FactorySet.CreateUserRegistrySet(ctx))
	return must.Must(set.CommodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Owner's camera",
		ShortName:             "Camera",
		Type:                  models.CommodityTypeOther,
		Status:                models.CommodityStatusInUse,
		Count:                 1,
		OriginalPriceCurrency: "USD",
		OriginalPrice:         new(decimal.NewFromInt(10)),
		PurchaseDate:          models.ToPDate("2024-01-01"),
		Tags:                  []string{"owner"},
	}))
}

func ownerItem(c *qt.C, env *filesAuthzEnv, id string) *models.Commodity {
	c.Helper()
	ctx := createTestUserContextWithGroup(env.ownerGroup.CreatedBy, env.ownerGroup.TenantID, env.ownerGroup.ID)
	set := must.Must(env.params.FactorySet.CreateUserRegistrySet(ctx))
	return must.Must(set.CommodityRegistry.Get(ctx, id))
}

func ownerFile(c *qt.C, env *filesAuthzEnv) *models.FileEntity {
	c.Helper()
	ctx := createTestUserContextWithGroup(env.ownerGroup.CreatedBy, env.ownerGroup.TenantID, env.ownerGroup.ID)
	set := must.Must(env.params.FactorySet.CreateUserRegistrySet(ctx))
	return must.Must(set.FileRegistry.Get(ctx, env.ownerFileID))
}

func postBulk(c *qt.C, env *filesAuthzEnv, slug, path, token, resource string, attributes map[string]any) *httptest.ResponseRecorder {
	c.Helper()
	body := must.Must(json.Marshal(map[string]any{
		"data": map[string]any{"type": resource, "attributes": attributes},
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/g/"+slug+path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)
	return rr
}

func TestCommodityBulkUpdate_CrossGroup(t *testing.T) {
	c := qt.New(t)
	env := newFilesAuthzEnv(c)
	item := bulkAuthzItem(c, env)
	patch := map[string]any{
		"type": "electronics",
		"tags": map[string]any{"add": []string{"taken"}, "remove": []string{"owner"}},
	}

	c.Run("the owner's ids from the neighbour's group are not found", func(c *qt.C) {
		rr := postBulk(c, env, env.otherGroupSlug, "/commodities/bulk-update", env.otherGroupTok, "commodities",
			map[string]any{"ids": []string{item.ID}, "patch": patch})
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
		c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.succeeded"), []any{})
		c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.failed[0].code"), "not_found")
	})

	c.Run("a query from the neighbour's group selects only its items", func(c *qt.C) {
		rr := postBulk(c, env, env.otherGroupSlug, "/commodities/bulk-update", env.otherGroupTok, "commodities",
			map[string]any{"query": "", "expected_count": 1, "expected_digest": "owner-digest", "patch": patch})
		c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
		c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].meta.matched"), "0")
	})

	c.Run("the owner's group refuses the neighbour", func(c *qt.C) {
		rr := postBulk(c, env, env.ownerGroup.Slug, "/commodities/bulk-update", env.otherGroupTok, "commodities",
			map[string]any{"ids": []string{item.ID}, "patch": patch})
		c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("body=%s", rr.Body.String()))
	})

	got := ownerItem(c, env, item.ID)
	c.Assert(got.Type, qt.Equals, models.CommodityTypeOther)
	c.Assert([]string(got.Tags), qt.DeepEquals, []string{"owner"})
}

func TestFileBulkUpdate_CrossGroup(t *testing.T) {
	c := qt.New(t)
	env := newFilesAuthzEnv(c)
	patch := map[string]any{"tags": map[string]any{"add": []string{"taken"}}}

	c.Run("the owner's ids from the neighbour's group are not found", func(c *qt.C) {
		rr := postBulk(c, env, env.otherGroupSlug, "/files/bulk-update", env.otherGroupTok, "files",
			map[string]any{"ids": []string{env.ownerFileID}, "patch": patch})
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
		c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.succeeded"), []any{})
		c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.failed[0].code"), "not_found")
	})

	// Files take ids only: there is no query that could widen the request.
	c.Run("a query is not a file selection", func(c *qt.C) {
		rr := postBulk(c, env, env.otherGroupSlug, "/files/bulk-update", env.otherGroupTok, "files",
			map[string]any{"query": "", "expected_count": 1, "patch": patch})
		c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	})

	c.Run("the owner's group refuses the neighbour", func(c *qt.C) {
		rr := postBulk(c, env, env.ownerGroup.Slug, "/files/bulk-update", env.otherGroupTok, "files",
			map[string]any{"ids": []string{env.ownerFileID}, "patch": patch})
		c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("body=%s", rr.Body.String()))
	})

	c.Assert([]string(ownerFile(c, env).Tags), qt.HasLen, 0)
}
