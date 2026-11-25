package apiserver_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// patchDraftMark sends PATCH /commodities/{id}/draft with body.
func patchDraftMark(params apiserver.Params, user *models.User, group *models.LocationGroup, id, body string) *httptest.ResponseRecorder {
	req := must.Must(http.NewRequest("PATCH", "/api/v1/g/"+group.Slug+"/commodities/"+id+"/draft", bytes.NewReader([]byte(body))))
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	apiserver.APIServer(params, &mockRestoreWorker{}).ServeHTTP(rr, req)
	return rr
}

func draftMarkBody(draft string) string {
	return `{"data":{"type":"commodity_draft_mark","attributes":{"draft":` + draft + `}}}`
}

func TestCommodityDraftMark_SetsAndClearsTheMarkAlone(t *testing.T) {
	c := qt.New(t)
	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	ctx := context.Background()
	item := must.Must(registrySet.CommodityRegistry.List(ctx))[0]

	// An edit that lands after a client read the item: a write built from
	// that read would undo it, the mark alone does not.
	edited := *item
	edited.Comments = "Keep the box"
	_ = must.Must(registrySet.CommodityRegistry.Update(ctx, edited))

	rr := patchDraftMark(params, testUser, testGroup, item.ID, draftMarkBody("true"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("%s", rr.Body.String()))
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.draft"), true)
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.comments"), "Keep the box")

	stored := must.Must(registrySet.CommodityRegistry.Get(ctx, item.ID))
	c.Check(stored.Draft, qt.IsTrue)
	c.Check(stored.Comments, qt.Equals, "Keep the box")

	rr = patchDraftMark(params, testUser, testGroup, item.ID, draftMarkBody("false"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("%s", rr.Body.String()))
	stored = must.Must(registrySet.CommodityRegistry.Get(ctx, item.ID))
	c.Check(stored.Draft, qt.IsFalse)
	c.Check(stored.Comments, qt.Equals, "Keep the box")
}

func TestCommodityDraftMark_RefusesAnIncompleteItem(t *testing.T) {
	c := qt.New(t)
	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	ctx := context.Background()
	item := must.Must(registrySet.CommodityRegistry.List(ctx))[0]

	// A draft may leave the currency out; a reviewed item may not.
	draft := *item
	draft.Draft = true
	draft.OriginalPrice = nil
	draft.ConvertedOriginalPrice = nil
	draft.OriginalPriceCurrency = ""
	_ = must.Must(registrySet.CommodityRegistry.Update(ctx, draft))

	rr := patchDraftMark(params, testUser, testGroup, item.ID, draftMarkBody("false"))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("%s", rr.Body.String()))
	// Nested as the refusal of a full update nests it.
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].error.errorCodes.data.attributes.original_price_currency.code"), "validation_required")

	stored := must.Must(registrySet.CommodityRegistry.Get(ctx, item.ID))
	c.Check(stored.Draft, qt.IsTrue)
}

func TestCommodityDraftMark_RejectsABadRequest(t *testing.T) {
	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	item := must.Must(registrySet.CommodityRegistry.List(context.Background()))[0]

	tests := []struct {
		name string
		body string
	}{
		{name: "no mark", body: `{"data":{"type":"commodity_draft_mark","attributes":{}}}`},
		{name: "wrong type", body: `{"data":{"type":"commodities","attributes":{"draft":false}}}`},
		{name: "no data", body: `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			rr := patchDraftMark(params, testUser, testGroup, item.ID, tt.body)
			c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("%s", rr.Body.String()))
		})
	}
}

func TestCommodityDraftMark_ItemInTrashIsNotFound(t *testing.T) {
	c := qt.New(t)
	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	ctx := context.Background()
	item := must.Must(registrySet.CommodityRegistry.List(ctx))[0]

	_, err := registrySet.CommodityRegistry.Trash(ctx, item.ID, testUser.ID, time.Now())
	c.Assert(err, qt.IsNil)

	rr := patchDraftMark(params, testUser, testGroup, item.ID, draftMarkBody("false"))
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound, qt.Commentf("%s", rr.Body.String()))

	_, err = registrySet.CommodityRegistry.SetDraft(ctx, item.ID, false, nil)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
