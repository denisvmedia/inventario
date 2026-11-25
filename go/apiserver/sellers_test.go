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

func TestSellers_CRUD(t *testing.T) {
	c := qt.New(t)
	params, testUser, testGroup := newParams()
	regSet := getRegistrySetFromParams(params, testUser)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	base := "/api/v1/g/" + testGroup.Slug + "/sellers"

	rr := doJSON(c, handler, testUser.ID, http.MethodPost, base,
		`{"data":{"type":"sellers","attributes":{"name":"  Hardware   Store ","website":"https://hw.example","contact":"Ann","note":"cash only"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	var created jsonapi.SellerResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &created), qt.IsNil)
	c.Check(created.Data.Type, qt.Equals, "sellers")
	c.Check(created.Data.Attributes.Name, qt.Equals, "Hardware Store")
	c.Check(created.Data.Attributes.Website, qt.Equals, "https://hw.example")
	c.Check(created.Data.Attributes.ItemCount, qt.Equals, 0)
	sellerID := created.Data.ID

	// The name key is not part of the API.
	var raw map[string]any
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &raw), qt.IsNil)
	attrs := raw["data"].(map[string]any)["attributes"].(map[string]any)
	c.Check(attrs["name_key"], qt.IsNil)

	rr = doJSON(c, handler, testUser.ID, http.MethodPost, base,
		`{"data":{"type":"sellers","attributes":{"name":"HARDWARE STORE"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, "seller.name_taken")

	other := doJSON(c, handler, testUser.ID, http.MethodPost, base,
		`{"data":{"type":"sellers","attributes":{"name":"Corner Market"}}}`)
	c.Assert(other.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", other.Body.String()))
	var otherSeller jsonapi.SellerResponse
	c.Assert(json.Unmarshal(other.Body.Bytes(), &otherSeller), qt.IsNil)

	ctx := context.Background()
	area := must.Must(regSet.AreaRegistry.List(ctx))[0]
	item := must.Must(regSet.CommodityRegistry.Create(ctx, models.Commodity{
		Name: "Drill", ShortName: "drill", AreaID: new(area.ID), Status: models.CommodityStatusInUse,
		Type: models.CommodityTypeOther, Count: 1, Draft: true, SellerID: new(sellerID),
	}))

	rr = doJSON(c, handler, testUser.ID, http.MethodGet, base, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var list jsonapi.SellersResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &list), qt.IsNil)
	c.Assert(list.Data, qt.HasLen, 2)
	c.Check(list.Meta.Sellers, qt.Equals, 2)
	c.Check(list.Data[0].Attributes.Name, qt.Equals, "Corner Market")
	c.Check(list.Data[0].Attributes.ItemCount, qt.Equals, 0)
	c.Check(list.Data[1].Attributes.Name, qt.Equals, "Hardware Store")
	c.Check(list.Data[1].Attributes.ItemCount, qt.Equals, 1)

	rr = doJSON(c, handler, testUser.ID, http.MethodGet, base+"?q=hard", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &list), qt.IsNil)
	c.Assert(list.Data, qt.HasLen, 1)
	c.Check(list.Data[0].ID, qt.Equals, sellerID)

	rr = doJSON(c, handler, testUser.ID, http.MethodGet, base+"/"+sellerID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var got jsonapi.SellerResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &got), qt.IsNil)
	c.Check(got.Data.Attributes.ItemCount, qt.Equals, 1)

	// Renaming onto another seller's name is a conflict; a partial patch
	// keeps the fields it leaves out.
	rr = doJSON(c, handler, testUser.ID, http.MethodPatch, base+"/"+sellerID,
		`{"data":{"id":"`+sellerID+`","type":"sellers","attributes":{"name":"corner market"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	rr = doJSON(c, handler, testUser.ID, http.MethodPatch, base+"/"+sellerID,
		`{"data":{"id":"`+sellerID+`","type":"sellers","attributes":{"name":"Hardware Store Ltd","note":""}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var updated jsonapi.SellerResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &updated), qt.IsNil)
	c.Check(updated.Data.Attributes.Name, qt.Equals, "Hardware Store Ltd")
	c.Check(updated.Data.Attributes.Website, qt.Equals, "https://hw.example")
	c.Check(updated.Data.Attributes.Note, qt.Equals, "")
	c.Check(updated.Data.Attributes.ItemCount, qt.Equals, 1)

	rr = doJSON(c, handler, testUser.ID, http.MethodDelete, base+"/"+sellerID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))
	rr = doJSON(c, handler, testUser.ID, http.MethodGet, base+"/"+sellerID, "")
	c.Check(rr.Code, qt.Equals, http.StatusNotFound)

	// The item stays and loses the seller.
	kept := must.Must(regSet.CommodityRegistry.Get(ctx, item.ID))
	c.Check(kept.SellerID, qt.IsNil)
}

func TestSellers_RejectsInvalidInput(t *testing.T) {
	params, testUser, testGroup := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	base := "/api/v1/g/" + testGroup.Slug + "/sellers"

	for _, tc := range []struct {
		name, body string
	}{
		{name: "no name", body: `{"data":{"type":"sellers","attributes":{"name":"   "}}}`},
		{name: "a website that is not a URL", body: `{"data":{"type":"sellers","attributes":{"name":"Shop","website":"shop dot com"}}}`},
		{name: "a website that is not http", body: `{"data":{"type":"sellers","attributes":{"name":"Shop","website":"ftp://shop.example"}}}`},
		{name: "a name over the limit", body: `{"data":{"type":"sellers","attributes":{"name":"` + longString(models.MaxSellerNameRunes+1) + `"}}}`},
		{name: "the wrong type", body: `{"data":{"type":"tags","attributes":{"name":"Shop"}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			rr := doJSON(c, handler, testUser.ID, http.MethodPost, base, tc.body)
			c.Check(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
		})
	}
}

func TestCommodities_SellerFilterAndCheck(t *testing.T) {
	c := qt.New(t)
	params, testUser, testGroup := newParams()
	regSet := getRegistrySetFromParams(params, testUser)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	base := "/api/v1/g/" + testGroup.Slug

	ctx := context.Background()
	shop := must.Must(regSet.SellerRegistry.Create(ctx, models.Seller{Name: "Shop"}))
	area := must.Must(regSet.AreaRegistry.List(ctx))[0]
	item := must.Must(regSet.CommodityRegistry.Create(ctx, models.Commodity{
		Name: "Bought at the shop", ShortName: "shop", AreaID: new(area.ID), Status: models.CommodityStatusInUse,
		Type: models.CommodityTypeOther, Count: 1, Draft: true, SellerID: new(shop.ID),
	}))

	rr := doJSON(c, handler, testUser.ID, http.MethodGet, base+"/commodities?include_inactive=true&seller_id="+shop.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var list jsonapi.CommoditiesResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &list), qt.IsNil)
	c.Assert(list.Data, qt.HasLen, 1)
	c.Check(list.Data[0].ID, qt.Equals, item.ID)
	c.Check(list.Data[0].Attributes.SellerID, qt.DeepEquals, &shop.ID)

	// An item cannot name a seller the group does not have.
	payload := must.Must(json.Marshal(jsonapi.CommodityRequest{Data: &jsonapi.CommodityData{
		ID:   item.ID,
		Type: "commodities",
		Attributes: &models.Commodity{
			Name: item.Name, ShortName: item.ShortName, AreaID: item.AreaID, Status: item.Status,
			Type: item.Type, Count: 1, Draft: true, OriginalPriceCurrency: "USD", SellerID: new("no-such-seller"),
		},
	}}))
	rr = doJSON(c, handler, testUser.ID, http.MethodPut, base+"/commodities/"+item.ID, string(payload))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].error.error.data.attributes.seller_id"), "no seller with this ID in the group")
	kept := must.Must(regSet.CommodityRegistry.Get(ctx, item.ID))
	c.Check(kept.SellerID, qt.DeepEquals, &shop.ID)
}

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

// TestSellers_ViewerCannotWrite checks the role gate on the /sellers routes:
// a viewer reads the group's sellers but cannot create, rename or delete one,
// while a user can.
func TestSellers_ViewerCannotWrite(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)

	rr := env.do(http.MethodPost, "/sellers", `{"data":{"type":"sellers","attributes":{"name":"Corner Market"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	var created jsonapi.SellerResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &created), qt.IsNil)
	sellerPath := "/sellers/" + created.Data.ID

	env.setRole(models.GroupRoleViewer)
	rr = env.do(http.MethodGet, "/sellers", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodGet, sellerPath, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	writes := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"create", http.MethodPost, "/sellers", `{"data":{"type":"sellers","attributes":{"name":"Hardware Store"}}}`},
		{"rename", http.MethodPatch, sellerPath, `{"data":{"id":"` + created.Data.ID + `","type":"sellers","attributes":{"name":"Renamed"}}}`},
		{"delete", http.MethodDelete, sellerPath, ""},
	}
	for _, w := range writes {
		c.Run(w.name, func(c *qt.C) {
			rr := env.do(w.method, w.path, w.body)
			c.Assert(rr.Code, qt.Equals, http.StatusForbidden,
				qt.Commentf("a viewer got %d from %s %s: %s", rr.Code, w.method, w.path, rr.Body.String()))
		})
	}

	// Nothing the viewer sent changed the seller list.
	rr = env.do(http.MethodGet, "/sellers", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var list jsonapi.SellersResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &list), qt.IsNil)
	c.Assert(list.Data, qt.HasLen, 1)
	c.Check(list.Data[0].Attributes.Name, qt.Equals, "Corner Market")

	env.setRole(models.GroupRoleUser)
	rr = env.do(http.MethodPost, "/sellers", `{"data":{"type":"sellers","attributes":{"name":"Hardware Store"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
}
