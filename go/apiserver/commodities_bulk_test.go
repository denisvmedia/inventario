package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
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

	rr := env.do(http.MethodPost, "/commodities/bulk-update", bulkBody(map[string]any{
		"query": query, "expected_count": 2, "patch": disposed,
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
		"query": query, "expected_count": 2, "patch": map[string]any{"type": "electronics"},
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
		{"a parameter the query contract does not know", map[string]any{"query": "page=2", "expected_count": 1, "patch": typePatch}, http.StatusBadRequest, "query.invalid_parameter"},
		{"items in Trash", map[string]any{"query": "trashed=true", "expected_count": 1, "patch": typePatch}, http.StatusBadRequest, "query.invalid_parameter"},
		{"ids and a query", map[string]any{"ids": []string{"a"}, "query": "", "expected_count": 1, "patch": typePatch}, http.StatusUnprocessableEntity, ""},
		{"a query without expected_count", map[string]any{"query": "", "patch": typePatch}, http.StatusUnprocessableEntity, ""},
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
	rr = env.do(http.MethodPost, "/commodities/bulk-move", bulkBody(map[string]any{
		"query": "tags=shed", "expected_count": 2, "area_id": area.ID,
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.succeeded"), []any{b.ID})
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.skipped[0].id"), a.ID)
	c.Assert(*env.getItem(b.ID).AreaID, qt.Equals, area.ID)

	rr = env.do(http.MethodPost, "/commodities/bulk-delete", bulkBody(map[string]any{
		"query": "tags=shed", "expected_count": 3,
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))

	rr = env.do(http.MethodPost, "/commodities/bulk-delete", bulkBody(map[string]any{
		"query": "tags=shed", "expected_count": 2,
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.succeeded"), []any{a.ID, b.ID})
	_, err := regs.CommodityRegistry.GetTrashed(context.Background(), a.ID)
	c.Assert(err, qt.IsNil)
}
