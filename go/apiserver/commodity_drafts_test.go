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
)

func draftCreateBody(key, name string) string {
	return `{"data":{"type":"commodity_drafts","attributes":{"client_key":"` + key + `","values":{"name":"` + name + `","purchase_date":""}}}}`
}

func draftSaveBody(revision, name string) string {
	return `{"data":{"type":"commodity_drafts","attributes":{"revision":` + revision + `,"values":{"name":"` + name + `"}}}}`
}

func draftPromoteBody(revision, name, areaID string) string {
	return `{"data":{"type":"commodities","attributes":{"name":"` + name + `","short_name":"` + name + `","type":"equipment","status":"in_use","count":1,"original_price_currency":"USD","area_id":"` + areaID + `"}},"meta":{"revision":` + revision + `}}`
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
		{name: "promote without revision", method: http.MethodPost, path: "/drafts/DRAFT/promote", body: `{"data":{"type":"commodities","attributes":{"name":"x","short_name":"x","type":"equipment","status":"in_use","count":1}}}`},
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
