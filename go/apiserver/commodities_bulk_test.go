package apiserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// bulkItem creates a live item in the env's group.
func (e *trashAPIEnv) bulkItem(name string, tags ...string) *models.Commodity {
	e.c.Helper()
	return must.Must(getRegistrySetFromParams(e.params, e.user).CommodityRegistry.Create(context.Background(), models.Commodity{
		Name:                  name,
		ShortName:             strings.ReplaceAll(name, " ", ""),
		Type:                  models.CommodityTypeOther,
		Status:                models.CommodityStatusInUse,
		Count:                 1,
		OriginalPriceCurrency: "USD",
		OriginalPrice:         new(decimal.NewFromInt(10)),
		PurchaseDate:          models.ToPDate("2024-01-01"),
		Tags:                  tags,
	}))
}

func (e *trashAPIEnv) getItem(id string) *models.Commodity {
	e.c.Helper()
	return must.Must(getRegistrySetFromParams(e.params, e.user).CommodityRegistry.Get(context.Background(), id))
}

// listDigest returns the meta.selection_digest the items list answers for
// query, the value a client sends back as expected_digest.
func (e *trashAPIEnv) listDigest(query string) string {
	e.c.Helper()
	rr := e.do(http.MethodGet, "/commodities?"+query, "")
	e.c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var body struct {
		Meta struct {
			SelectionDigest string `json:"selection_digest"`
		} `json:"meta"`
	}
	e.c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	return body.Meta.SelectionDigest
}

// bulkBody builds a bulk request body from its attributes.
func bulkBody(attributes map[string]any) string {
	return string(must.Must(json.Marshal(map[string]any{
		"data": map[string]any{"type": "commodities", "attributes": attributes},
	})))
}

func TestCommodityBulkUpdate_TagsByIDs(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	drill := env.bulkItem("Bulk Drill", "tools")
	saw := env.bulkItem("Bulk Saw", "saws")
	insured := env.bulkItem("Bulk Insured", "insured")

	rr := env.do(http.MethodPost, "/commodities/bulk-update", bulkBody(map[string]any{
		"ids":   []string{drill.ID, saw.ID, insured.ID, "missing"},
		"patch": map[string]any{"tags": map[string]any{"add": []string{"Insured"}, "remove": []string{"tools"}}},
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	body := rr.Body.String()
	c.Check(body, checkers.JSONPathEquals("$.data.type"), "commodities.bulk_result")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.succeeded"), []any{drill.ID, saw.ID})
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.skipped[0].id"), insured.ID)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.skipped[0].reason"), "unchanged")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.failed[0].id"), "missing")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.failed[0].code"), "not_found")

	// Tags the patch does not name stay on each item.
	c.Assert([]string(env.getItem(drill.ID).Tags), qt.DeepEquals, []string{"insured"})
	c.Assert([]string(env.getItem(saw.ID).Tags), qt.DeepEquals, []string{"saws", "insured"})

	// The change is in the item's history, as a single edit puts it there.
	rr = env.do(http.MethodGet, "/commodities/"+drill.ID+"/events", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data[0].kind"), "updated")
}

func TestCommodityBulkUpdate_ByQuery(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	lamp := env.bulkItem("Bulk Lamp", "clear-out")
	chair := env.bulkItem("Bulk Chair", "clear-out")
	env.bulkItem("Bulk Keeper", "keep")

	query := "tags=clear-out&include_inactive=false&sort=name"
	disposed := map[string]any{"status": map[string]any{"value": "disposed", "date": "2026-11-25", "note": "clear-out"}}

	digest := env.listDigest(query)
	c.Assert(digest, qt.Not(qt.Equals), "")
	rr := env.do(http.MethodPost, "/commodities/bulk-update", bulkBody(map[string]any{
		"query": query, "expected_count": 2, "expected_digest": digest, "patch": disposed,
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.succeeded"), []any{chair.ID, lamp.ID})
	got := env.getItem(lamp.ID)
	c.Assert(got.Status, qt.Equals, models.CommodityStatusDisposed)
	c.Assert(string(*got.StatusDate), qt.Equals, "2026-11-25")
	c.Assert(got.StatusNote, qt.Equals, "clear-out")

	// Disposed items are inactive, so the same query selects none of them
	// now. The request is refused instead of acting on another set.
	rr = env.do(http.MethodPost, "/commodities/bulk-update", bulkBody(map[string]any{
		"query": query, "expected_count": 2, "expected_digest": digest, "patch": map[string]any{"type": "electronics"},
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), "bulk.selection_changed")
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].meta.expected"), "2")
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].meta.matched"), "0")
	c.Assert(env.getItem(lamp.ID).Type, qt.Equals, models.CommodityTypeOther)
}

func TestCommodityBulkUpdate_Refused(t *testing.T) {
	ids101 := make([]string, 101)
	for i := range ids101 {
		ids101[i] = "id"
	}
	typePatch := map[string]any{"type": "electronics"}
	tests := []struct {
		name   string
		attrs  map[string]any
		status int
		code   string
	}{
		{"a parameter the query contract does not know", map[string]any{"query": "page=2", "expected_count": 1, "expected_digest": "x", "patch": typePatch}, http.StatusBadRequest, "query.invalid_parameter"},
		{"items in Trash", map[string]any{"query": "trashed=true", "expected_count": 1, "expected_digest": "x", "patch": typePatch}, http.StatusBadRequest, "query.invalid_parameter"},
		{"ids and a query", map[string]any{"ids": []string{"a"}, "query": "", "expected_count": 1, "expected_digest": "x", "patch": typePatch}, http.StatusUnprocessableEntity, ""},
		{"a query without expected_count", map[string]any{"query": "", "expected_digest": "x", "patch": typePatch}, http.StatusUnprocessableEntity, ""},
		{"a query without expected_digest", map[string]any{"query": "", "expected_count": 1, "patch": typePatch}, http.StatusUnprocessableEntity, ""},
		{"ids with expected_digest", map[string]any{"ids": []string{"a"}, "expected_digest": "x", "patch": typePatch}, http.StatusUnprocessableEntity, ""},
		{"more than 100 ids", map[string]any{"ids": ids101, "patch": typePatch}, http.StatusUnprocessableEntity, ""},
		{"no patch", map[string]any{"ids": []string{"a"}}, http.StatusUnprocessableEntity, ""},
		{"an empty patch", map[string]any{"ids": []string{"a"}, "patch": map[string]any{}}, http.StatusUnprocessableEntity, ""},
		{"an unknown type", map[string]any{"ids": []string{"a"}, "patch": map[string]any{"type": "spaceship"}}, http.StatusUnprocessableEntity, ""},
		{"sold", map[string]any{"ids": []string{"a"}, "patch": map[string]any{"status": map[string]any{"value": "sold", "date": "2026-11-25"}}}, http.StatusUnprocessableEntity, ""},
		{"leaving in use without a date", map[string]any{"ids": []string{"a"}, "patch": map[string]any{"status": map[string]any{"value": "lost"}}}, http.StatusUnprocessableEntity, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newTrashAPIEnv(c)
			rr := env.do(http.MethodPost, "/commodities/bulk-update", bulkBody(tt.attrs))
			c.Assert(rr.Code, qt.Equals, tt.status, qt.Commentf("body=%s", rr.Body.String()))
			if tt.code != "" {
				c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), tt.code)
			}
		})
	}
}

func TestCommodityBulkUpdate_ViewerIsRefused(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	item := env.bulkItem("Bulk Viewer")
	env.setRole(models.GroupRoleViewer)

	rr := env.do(http.MethodPost, "/commodities/bulk-update", bulkBody(map[string]any{
		"ids": []string{item.ID}, "patch": map[string]any{"type": "electronics"},
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("body=%s", rr.Body.String()))
	c.Assert(env.getItem(item.ID).Type, qt.Equals, models.CommodityTypeOther)
}

func TestCommodityBulkMoveAndDelete_ByQuery(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	regs := getRegistrySetFromParams(env.params, env.user)
	location := must.Must(regs.LocationRegistry.Create(context.Background(), models.Location{Name: "Bulk Home"}))
	area := must.Must(regs.AreaRegistry.Create(context.Background(), models.Area{Name: "Bulk Shed", LocationID: location.ID}))
	a := env.bulkItem("Bulk Move A", "shed")
	b := env.bulkItem("Bulk Move B", "shed")

	rr := env.do(http.MethodPost, "/commodities/bulk-move", bulkBody(map[string]any{
		"ids": []string{a.ID}, "area_id": area.ID,
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	// The query selects both; the one already in the shed is skipped.
	digest := env.listDigest("tags=shed")
	rr = env.do(http.MethodPost, "/commodities/bulk-move", bulkBody(map[string]any{
		"query": "tags=shed", "expected_count": 2, "expected_digest": digest, "area_id": area.ID,
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.succeeded"), []any{b.ID})
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.skipped[0].id"), a.ID)
	c.Assert(*env.getItem(b.ID).AreaID, qt.Equals, area.ID)

	rr = env.do(http.MethodPost, "/commodities/bulk-delete", bulkBody(map[string]any{
		"query": "tags=shed", "expected_count": 3, "expected_digest": digest,
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))

	rr = env.do(http.MethodPost, "/commodities/bulk-delete", bulkBody(map[string]any{
		"query": "tags=shed", "expected_count": 2, "expected_digest": digest,
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.succeeded"), []any{a.ID, b.ID})
	_, err := regs.CommodityRegistry.GetTrashed(context.Background(), a.ID)
	c.Assert(err, qt.IsNil)
}

// The same number of items is not the same items: when one item leaves the
// query and another joins it, the digest the list gave no longer matches.
func TestCommodityBulkUpdate_RefusesASwappedItem(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	a := env.bulkItem("Swap A", "sale")
	b := env.bulkItem("Swap B", "sale")
	joiner := env.bulkItem("Swap C")

	query := "tags=sale"
	digest := env.listDigest(query)

	regs := getRegistrySetFromParams(env.params, env.user)
	left := *env.getItem(b.ID)
	left.Tags = nil
	must.Must(regs.CommodityRegistry.Update(context.Background(), left))
	joined := *env.getItem(joiner.ID)
	joined.Tags = []string{"sale"}
	must.Must(regs.CommodityRegistry.Update(context.Background(), joined))

	rr := env.do(http.MethodPost, "/commodities/bulk-update", bulkBody(map[string]any{
		"query": query, "expected_count": 2, "expected_digest": digest, "patch": map[string]any{"type": "electronics"},
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), "bulk.selection_changed")
	c.Assert(env.getItem(a.ID).Type, qt.Equals, models.CommodityTypeOther)
	c.Assert(env.getItem(joiner.ID).Type, qt.Equals, models.CommodityTypeOther)

	// The list's digest now names the new set, and the action goes through.
	rr = env.do(http.MethodPost, "/commodities/bulk-update", bulkBody(map[string]any{
		"query": query, "expected_count": 2, "expected_digest": env.listDigest(query), "patch": map[string]any{"type": "electronics"},
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.succeeded"), []any{a.ID, joiner.ID})
}

// The digest covers every page, so a client on page 2 of a short list gets
// the same one as on page 1; a query a bulk action cannot take gets none.
func TestCommodityList_SelectionDigest(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	env.bulkItem("Digest A", "digest")
	env.bulkItem("Digest B", "digest")
	env.bulkItem("Digest C", "digest")
	inTrash := env.bulkItem("Digest Trashed", "digest")
	rr := env.do(http.MethodDelete, "/commodities/"+inTrash.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))

	whole := env.listDigest("tags=digest")
	c.Assert(whole, qt.Not(qt.Equals), "")
	c.Assert(env.listDigest("tags=digest&page=2&per_page=2"), qt.Equals, whole)
	c.Assert(env.listDigest("tags=digest&sort=-name"), qt.Equals, whole)

	c.Run("no digest for a query without matches", func(c *qt.C) {
		c.Assert(env.listDigest("tags=nothing-has-this"), qt.Equals, "")
	})

	// The Trash query matches an item, but bulk actions take live items
	// only, so the list gives no digest for it.
	c.Run("no digest for items in Trash", func(c *qt.C) {
		rr := env.do(http.MethodGet, "/commodities?tags=digest&trashed=true", "")
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
		c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.commodities"), float64(1))
		c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data[0].id"), inTrash.ID)
		c.Assert(env.listDigest("tags=digest&trashed=true"), qt.Equals, "")
	})
}

// listIDsFails is a commodity registry whose id lookup fails, as when the
// database drops the connection after the page was read.
type listIDsFails struct{ registry.CommodityRegistry }

func (listIDsFails) ListIDs(context.Context, registry.CommodityListOptions, int) ([]string, error) {
	return nil, errors.New("connection lost")
}

type listIDsFailsFactory struct {
	registry.CommodityRegistryFactory
}

func (f listIDsFailsFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityRegistry, error) {
	reg, err := f.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}
	return listIDsFails{reg}, nil
}

// The digest is optional: when reading the ids for it fails, the list still
// answers with its page, without a digest.
func TestCommodityList_DigestLookupFailureKeepsTheList(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	env.bulkItem("Lookup A", "lookup")
	env.bulkItem("Lookup B", "lookup")

	fs := *env.params.FactorySet
	fs.CommodityRegistryFactory = listIDsFailsFactory{fs.CommodityRegistryFactory}
	params := env.params
	params.FactorySet = &fs
	env.handler = apiserver.APIServer(params, &mockRestoreWorker{})

	// Page 2 holds one of the two items, so the digest needs the id lookup.
	rr := env.do(http.MethodGet, "/commodities?tags=lookup&page=2&per_page=1", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.commodities"), float64(2))
	c.Assert(rr.Body.String(), qt.Not(qt.Contains), "selection_digest")
}

func TestCommodityBulkMove_RefusesAMissingArea(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	item := env.bulkItem("Bulk Nowhere")

	rr := env.do(http.MethodPost, "/commodities/bulk-move", bulkBody(map[string]any{
		"ids": []string{item.ID}, "area_id": "no-such-area",
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, "area_id")
	c.Assert(env.getItem(item.ID).AreaID, qt.IsNil)
}

// A move validates each item as a single edit does: an item that would not
// pass validation fails, and is not written, while the others move.
func TestCommodityBulkMove_ValidatesEachItem(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	regs := getRegistrySetFromParams(env.params, env.user)
	location := must.Must(regs.LocationRegistry.Create(context.Background(), models.Location{Name: "Bulk Valid Home"}))
	area := must.Must(regs.AreaRegistry.Create(context.Background(), models.Area{Name: "Bulk Valid Shed", LocationID: location.ID}))
	good := env.bulkItem("Bulk Valid")
	item := env.bulkItem("Bulk Unnamed Move")
	broken := *item
	broken.ShortName = ""
	must.Must(regs.CommodityRegistry.Update(context.Background(), broken))

	rr := env.do(http.MethodPost, "/commodities/bulk-move", bulkBody(map[string]any{
		"ids": []string{good.ID, item.ID}, "area_id": area.ID,
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	body := rr.Body.String()
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.succeeded"), []any{good.ID})
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.failed[0].id"), item.ID)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.failed[0].code"), "invalid")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.failed[0].field"), "short_name")
	c.Assert(env.getItem(item.ID).AreaID, qt.IsNil)
	c.Assert(*env.getItem(good.ID).AreaID, qt.Equals, area.ID)
}

// A validation failure names the field and the validator's code, so the
// client can say it in the user's language.
func TestCommodityBulkUpdate_InvalidItemNamesFieldAndReason(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	item := env.bulkItem("Bulk Unnamed")
	regs := getRegistrySetFromParams(env.params, env.user)
	broken := *item
	broken.ShortName = ""
	must.Must(regs.CommodityRegistry.Update(context.Background(), broken))

	rr := env.do(http.MethodPost, "/commodities/bulk-update", bulkBody(map[string]any{
		"ids": []string{item.ID}, "patch": map[string]any{"type": "electronics"},
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	body := rr.Body.String()
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.failed[0].code"), "invalid")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.failed[0].field"), "short_name")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.failed[0].reason"), "validation_is_empty")
}
