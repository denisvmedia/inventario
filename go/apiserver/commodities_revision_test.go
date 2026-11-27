package apiserver_test

import (
	"encoding/json"
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
)

// openItemForm reads an item the way its edit form does and returns the
// attributes it would send back.
func (e *trashAPIEnv) openItemForm(id string) map[string]any {
	e.c.Helper()
	rr := e.do(http.MethodGet, "/commodities/"+id, "")
	e.c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var body struct {
		Data struct {
			Attributes map[string]any `json:"attributes"`
		} `json:"data"`
	}
	e.c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	return body.Data.Attributes
}

func itemSaveBody(id string, attributes map[string]any) string {
	return string(must.Must(json.Marshal(map[string]any{
		"data": map[string]any{"id": id, "type": "commodities", "attributes": attributes},
	})))
}

// An edit form opened before a bulk action tagged the item would drop the
// tag on save. The save names the revision the form was opened at, so it is
// refused and the tag stays (#2981).
func TestCommodityUpdate_FormOpenedBeforeABulkTagIsRefused(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	item := env.bulkItem("Revision Drill", "tools")
	form := env.openItemForm(item.ID)

	rr := env.do(http.MethodPost, "/commodities/bulk-update", bulkBody(map[string]any{
		"ids":   []string{item.ID},
		"patch": map[string]any{"tags": map[string]any{"add": []string{"insured"}}},
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	form["name"] = "Cordless drill"
	rr = env.do(http.MethodPut, "/commodities/"+item.ID, itemSaveBody(item.ID, form))
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.CommodityRevisionConflictCode)

	stored := env.getItem(item.ID)
	c.Check(stored.Name, qt.Equals, "Revision Drill")
	c.Check([]string(stored.Tags), qt.DeepEquals, []string{"tools", "insured"})
}

// A form opened after the bulk action, the reload the conflict offers, saves
// and keeps the tag.
func TestCommodityUpdate_FormReloadedAfterABulkTagSaves(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	item := env.bulkItem("Reloaded Drill", "tools")

	rr := env.do(http.MethodPost, "/commodities/bulk-update", bulkBody(map[string]any{
		"ids":   []string{item.ID},
		"patch": map[string]any{"tags": map[string]any{"add": []string{"insured"}}},
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	form := env.openItemForm(item.ID)
	form["name"] = "Cordless drill"
	rr = env.do(http.MethodPut, "/commodities/"+item.ID, itemSaveBody(item.ID, form))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	stored := env.getItem(item.ID)
	c.Check(stored.Name, qt.Equals, "Cordless drill")
	c.Check([]string(stored.Tags), qt.DeepEquals, []string{"tools", "insured"})
	c.Check(stored.Revision, qt.Equals, item.Revision+2)
}

// A client that sends no revision is not checked: its save replaces the
// item as before.
func TestCommodityUpdate_WithoutARevisionIsNotChecked(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	item := env.bulkItem("Unchecked Drill", "tools")
	form := env.openItemForm(item.ID)
	delete(form, "revision")

	rr := env.do(http.MethodPost, "/commodities/bulk-update", bulkBody(map[string]any{
		"ids":   []string{item.ID},
		"patch": map[string]any{"tags": map[string]any{"add": []string{"insured"}}},
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	form["name"] = "Cordless drill"
	rr = env.do(http.MethodPut, "/commodities/"+item.ID, itemSaveBody(item.ID, form))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(env.getItem(item.ID).Name, qt.Equals, "Cordless drill")
}
