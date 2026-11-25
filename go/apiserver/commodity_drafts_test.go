package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

func draftCreateBody(key, name string) string {
	return `{"data":{"type":"commodity_drafts","attributes":{"client_key":"` + key + `","values":{"name":"` + name + `","purchase_date":""}}}}`
}

func draftSaveBody(revision, name string) string {
	return `{"data":{"type":"commodity_drafts","attributes":{"revision":` + revision + `,"values":{"name":"` + name + `"}}}}`
}

func draftPromoteBody(revision, name, areaID string) string {
	return draftPromoteBodyWithKey(revision, name, areaID, "promotion-1")
}

func draftPromoteBodyWithKey(revision, name, areaID, key string) string {
	return `{"data":{"type":"commodities","attributes":{"name":"` + name + `","short_name":"` + name + `","type":"equipment","status":"in_use","count":1,"original_price_currency":"USD","area_id":"` + areaID + `"}},"meta":{"revision":` + revision + `,"promotion_key":"` + key + `"}}`
}

func decodeDraft(c *qt.C, body []byte) jsonapi.CommodityDraftResponse {
	c.Helper()
	var resp jsonapi.CommodityDraftResponse
	c.Assert(json.Unmarshal(body, &resp), qt.IsNil)
	return resp
}

func TestCommodityDraftsAPI_Lifecycle(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	regs := getRegistrySetFromParams(env.params, env.user)
	ctx := context.Background()
	areaID := must.Must(regs.AreaRegistry.List(ctx))[0].ID
	itemsBefore := must.Must(regs.CommodityRegistry.Count(ctx))

	// Create, then repeat the create as a client would after a timeout.
	rr := env.do(http.MethodPost, "/drafts", draftCreateBody("key-1", "Drill"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	created := decodeDraft(c, rr.Body.Bytes())
	draftID := created.Data.ID
	c.Assert(created.Data.Attributes.Title, qt.Equals, "Drill")
	c.Assert(created.Data.Attributes.Revision, qt.Equals, 1)
	c.Assert(created.Data.Attributes.Status, qt.Equals, models.CommodityDraftOpen)

	rr = env.do(http.MethodPost, "/drafts", draftCreateBody("key-1", "Drill again"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Assert(decodeDraft(c, rr.Body.Bytes()).Data.ID, qt.Equals, draftID)

	rr = env.do(http.MethodPost, "/drafts", draftCreateBody("key-2", "Saw"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))

	rr = env.do(http.MethodGet, "/drafts", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.drafts"), float64(2))

	// Drafts are not items.
	c.Assert(must.Must(regs.CommodityRegistry.Count(ctx)), qt.Equals, itemsBefore)

	// Save, then a stale save from another tab.
	rr = env.do(http.MethodPut, "/drafts/"+draftID, draftSaveBody("1", "Cordless drill"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Assert(decodeDraft(c, rr.Body.Bytes()).Data.Attributes.Revision, qt.Equals, 2)

	rr = env.do(http.MethodPut, "/drafts/"+draftID, draftSaveBody("1", "Stale"))
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), apiserver.DraftRevisionConflictCode)
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].meta.revision"), float64(2))

	// Attach a file to the draft.
	file := must.Must(regs.FileRegistry.Create(ctx, models.FileEntity{
		Title:    "photo",
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		File:     &models.File{Path: "photo", OriginalPath: "photo.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	}))
	rr = env.do(http.MethodPost, "/files/"+file.ID+"/links",
		`{"data":{"type":"file-links","attributes":{"entity_type":"draft","entity_id":"`+draftID+`"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, `"entity_name":"Cordless drill"`)

	rr = env.do(http.MethodGet, "/drafts/"+draftID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Assert(decodeDraft(c, rr.Body.Bytes()).Data.Attributes.FileCount, qt.Equals, 1)

	rr = env.do(http.MethodGet, "/files?linked_entity_type=draft&linked_entity_id="+draftID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, file.ID)

	// An invalid item is refused and promotes nothing.
	rr = env.do(http.MethodPost, "/drafts/"+draftID+"/promote", draftPromoteBody("2", "", areaID))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))

	// Promote, then retry.
	rr = env.do(http.MethodPost, "/drafts/"+draftID+"/promote", draftPromoteBody("2", "Drill", areaID))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	var item jsonapi.CommodityResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &item), qt.IsNil)
	itemID := item.Data.ID

	rr = env.do(http.MethodPost, "/drafts/"+draftID+"/promote", draftPromoteBody("2", "Drill", areaID))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &item), qt.IsNil)
	c.Assert(item.Data.ID, qt.Equals, itemID)
	c.Assert(must.Must(regs.CommodityRegistry.Count(ctx)), qt.Equals, itemsBefore+1)

	// Another device that promotes with its own key learns that the draft
	// is already an item; it does not get this item as if it made it.
	rr = env.do(http.MethodPost, "/drafts/"+draftID+"/promote", draftPromoteBodyWithKey("2", "Drill", areaID, "promotion-2"))
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), apiserver.DraftPromotedCode)
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].meta.commodity_id"), itemID)
	c.Assert(must.Must(regs.CommodityRegistry.Count(ctx)), qt.Equals, itemsBefore+1)

	// A file attached to the promoted draft is refused the same way.
	lateFile := must.Must(regs.FileRegistry.Create(ctx, models.FileEntity{
		Title:    "late",
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		File:     &models.File{Path: "late", OriginalPath: "late.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	}))
	rr = env.do(http.MethodPost, "/files/"+lateFile.ID+"/links",
		`{"data":{"type":"file-links","attributes":{"entity_type":"draft","entity_id":"`+draftID+`"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), apiserver.DraftPromotedCode)
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].meta.commodity_id"), itemID)

	// The file is the item's now.
	rr = env.do(http.MethodGet, "/files?linked_entity_type=commodity&linked_entity_id="+itemID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, file.ID)

	// A promoted draft leaves the list and cannot be saved or discarded.
	rr = env.do(http.MethodGet, "/drafts", "")
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.drafts"), float64(1))
	rr = env.do(http.MethodPut, "/drafts/"+draftID, draftSaveBody("3", "Late"))
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), apiserver.DraftPromotedCode)
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].meta.commodity_id"), itemID)
	rr = env.do(http.MethodDelete, "/drafts/"+draftID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
}

func TestCommodityDraftsAPI_Discard(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)

	rr := env.do(http.MethodPost, "/drafts", draftCreateBody("key-1", "Drill"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	draftID := decodeDraft(c, rr.Body.Bytes()).Data.ID

	rr = env.do(http.MethodDelete, "/drafts/"+draftID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodGet, "/drafts/"+draftID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	rr = env.do(http.MethodDelete, "/drafts/"+draftID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestCommodityDraftsAPI_DraftBeingDiscarded(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	regs := getRegistrySetFromParams(env.params, env.user)
	ctx := context.Background()
	areaID := must.Must(regs.AreaRegistry.List(ctx))[0].ID

	rr := env.do(http.MethodPost, "/drafts", draftCreateBody("key-1", "Drill"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	draftID := decodeDraft(c, rr.Body.Bytes()).Data.ID
	// A discard that stopped after its first step.
	_, err := regs.CommodityDraftRegistry.MarkDiscarding(ctx, draftID)
	c.Assert(err, qt.IsNil)

	rr = env.do(http.MethodGet, "/drafts/"+draftID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodGet, "/drafts", "")
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.drafts"), float64(0))
	for _, req := range []struct{ method, path, body string }{
		{http.MethodPut, "/drafts/" + draftID, draftSaveBody("1", "Late")},
		{http.MethodPost, "/drafts/" + draftID + "/promote", draftPromoteBody("1", "Drill", areaID)},
		{http.MethodPost, "/drafts", draftCreateBody("key-1", "Drill")},
	} {
		rr = env.do(req.method, req.path, req.body)
		c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("%s %s body=%s", req.method, req.path, rr.Body.String()))
		c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), apiserver.DraftDiscardedCode)
	}

	// Discarding again finishes the work.
	rr = env.do(http.MethodDelete, "/drafts/"+draftID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))
	_, err = regs.CommodityDraftRegistry.Get(ctx, draftID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestCommodityDraftsAPI_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "missing client key", method: http.MethodPost, path: "/drafts", body: `{"data":{"type":"commodity_drafts","attributes":{"values":{}}}}`},
		{name: "wrong type", method: http.MethodPost, path: "/drafts", body: `{"data":{"type":"commodities","attributes":{"client_key":"k"}}}`},
		{name: "client key with spaces", method: http.MethodPost, path: "/drafts", body: `{"data":{"type":"commodity_drafts","attributes":{"client_key":"a b"}}}`},
		{name: "save without revision", method: http.MethodPut, path: "/drafts/DRAFT", body: `{"data":{"type":"commodity_drafts","attributes":{"values":{}}}}`},
		{name: "promote without revision", method: http.MethodPost, path: "/drafts/DRAFT/promote", body: `{"data":{"type":"commodities","attributes":{"name":"x","short_name":"x","type":"equipment","status":"in_use","count":1}},"meta":{"promotion_key":"k"}}`},
		{name: "promote without a promotion key", method: http.MethodPost, path: "/drafts/DRAFT/promote", body: `{"data":{"type":"commodities","attributes":{"name":"x","short_name":"x","type":"equipment","status":"in_use","count":1}},"meta":{"revision":1}}`},
		{name: "promotion key with spaces", method: http.MethodPost, path: "/drafts/DRAFT/promote", body: `{"data":{"type":"commodities","attributes":{"name":"x","short_name":"x","type":"equipment","status":"in_use","count":1}},"meta":{"revision":1,"promotion_key":"a b"}}`},
		{name: "create with a form field of the wrong type", method: http.MethodPost, path: "/drafts", body: `{"data":{"type":"commodity_drafts","attributes":{"client_key":"k","values":{"short_name":null}}}}`},
		{name: "save with a form field of the wrong type", method: http.MethodPut, path: "/drafts/DRAFT", body: `{"data":{"type":"commodity_drafts","attributes":{"revision":1,"values":{"tags":"one"}}}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			env := newTrashAPIEnv(c)
			rr := env.do(http.MethodPost, "/drafts", draftCreateBody("existing", "Drill"))
			c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
			draftID := decodeDraft(c, rr.Body.Bytes()).Data.ID
			path := tc.path
			if path != "/drafts" {
				path = "/drafts/" + draftID + path[len("/drafts/DRAFT"):]
			}
			rr = env.do(tc.method, path, tc.body)
			c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
		})
	}
}

func TestCommodityDraftsAPI_NamesTheFieldOfTheWrongType(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	rr := env.do(http.MethodPost, "/drafts",
		`{"data":{"type":"commodity_drafts","attributes":{"client_key":"k","values":{"name":"Drill","count":2,"short_name":7,"from_intake":{"any":"shape"}}}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].error.error.data.attributes.values.short_name"), "must be a string")
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].error.errorCodes.data.attributes.values.short_name.code"), "validation_draft_value_text")

	// A number field takes a JSON number, and unknown fields are kept.
	rr = env.do(http.MethodPost, "/drafts",
		`{"data":{"type":"commodity_drafts","attributes":{"client_key":"k","values":{"name":"Drill","count":2,"from_intake":{"any":"shape"}}}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
}

func TestCommodityDraftsAPI_ViewerCanReadOnly(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	rr := env.do(http.MethodPost, "/drafts", draftCreateBody("key-1", "Drill"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	draftID := decodeDraft(c, rr.Body.Bytes()).Data.ID

	env.setRole(models.GroupRoleViewer)
	rr = env.do(http.MethodGet, "/drafts", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	rr = env.do(http.MethodGet, "/drafts/"+draftID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	rr = env.do(http.MethodPost, "/drafts", draftCreateBody("key-2", "Saw"))
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	rr = env.do(http.MethodPut, "/drafts/"+draftID, draftSaveBody("1", "x"))
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	rr = env.do(http.MethodDelete, "/drafts/"+draftID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
}

func TestCommodityDraftsAPI_PromoteChecksTheSeller(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	regs := getRegistrySetFromParams(env.params, env.user)
	ctx := context.Background()
	shop := must.Must(regs.SellerRegistry.Create(ctx, models.Seller{Name: "Hardware store"}))
	gone := must.Must(regs.SellerRegistry.Create(ctx, models.Seller{Name: "Closed shop"}))

	promoteBody := func(revision, name, sellerID string) string {
		return `{"data":{"type":"commodities","attributes":{"name":"` + name + `","short_name":"` + name +
			`","type":"equipment","status":"in_use","count":1,"original_price_currency":"USD",` +
			`"brand":"Bosch","model":"GSR 18V","seller_id":"` + sellerID + `"}},"meta":{"revision":` + revision + `,"promotion_key":"key-` + name + `"}}`
	}
	createDraft := func(key, sellerID string) string {
		rr := env.do(http.MethodPost, "/drafts",
			`{"data":{"type":"commodity_drafts","attributes":{"client_key":"`+key+`","values":{"name":"Drill","seller_id":"`+sellerID+`"}}}}`)
		c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
		return decodeDraft(c, rr.Body.Bytes()).Data.ID
	}

	// The item gets the seller, brand and model the finished form names.
	kept := createDraft("seller-kept", shop.ID)
	rr := env.do(http.MethodPost, "/drafts/"+kept+"/promote", promoteBody("1", "Drill", shop.ID))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	var item jsonapi.CommodityResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &item), qt.IsNil)
	stored := must.Must(regs.CommodityRegistry.Get(ctx, item.Data.ID))
	c.Assert(stored.SellerID, qt.IsNotNil)
	c.Check(*stored.SellerID, qt.Equals, shop.ID)
	c.Check(stored.Brand, qt.Equals, "Bosch")
	c.Check(stored.Model, qt.Equals, "GSR 18V")

	// A seller deleted while the draft was open is refused on the field and
	// promotes nothing; the draft stays open for the user to pick another.
	stale := createDraft("seller-gone", gone.ID)
	c.Assert(regs.SellerRegistry.Delete(ctx, gone.ID), qt.IsNil)
	itemsBefore := must.Must(regs.CommodityRegistry.Count(ctx))
	rr = env.do(http.MethodPost, "/drafts/"+stale+"/promote", promoteBody("1", "Saw", gone.ID))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].error.error.data.attributes.seller_id"), "no seller with this ID in the group")
	c.Assert(must.Must(regs.CommodityRegistry.Count(ctx)), qt.Equals, itemsBefore)
	c.Assert(must.Must(regs.CommodityDraftRegistry.Get(ctx, stale)).IsOpen(), qt.IsTrue)

	rr = env.do(http.MethodPost, "/drafts/"+stale+"/promote", promoteBody("1", "Saw", ""))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))

	// The retry of a promotion is not checked: it creates nothing.
	c.Assert(regs.SellerRegistry.Delete(ctx, shop.ID), qt.IsNil)
	rr = env.do(http.MethodPost, "/drafts/"+kept+"/promote", promoteBody("1", "Drill", shop.ID))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
}
