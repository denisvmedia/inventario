package apiserver_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services/commodityquery"
)

// TestSwaggerDeclaresCommodityQueryParams checks that docs/swagger.json
// declares every filter of the item query contract on the endpoints that
// read it, so generated clients see the same parameters the handlers
// accept. /search takes `type` as the entity type, not as a filter.
func TestSwaggerDeclaresCommodityQueryParams(t *testing.T) {
	c := qt.New(t)

	raw, err := os.ReadFile(filepath.Join("..", "docs", "swagger.json"))
	c.Assert(err, qt.IsNil)
	var doc struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				Name string `json:"name"`
				In   string `json:"in"`
			} `json:"parameters"`
		} `json:"paths"`
	}
	c.Assert(json.Unmarshal(raw, &doc), qt.IsNil)

	queryParams := func(path string) map[string]bool {
		op, ok := doc.Paths[path]["get"]
		c.Assert(ok, qt.IsTrue, qt.Commentf("GET %s is not documented", path))
		names := map[string]bool{}
		for _, p := range op.Parameters {
			if p.In == "query" {
				names[p.Name] = true
			}
		}
		return names
	}

	list := queryParams("/g/{groupSlug}/commodities")
	tagCounts := queryParams("/g/{groupSlug}/commodities/tag-counts")
	search := queryParams("/g/{groupSlug}/search")
	for _, name := range commodityquery.FilterParams() {
		c.Check(list[name], qt.IsTrue, qt.Commentf("GET /commodities does not declare %q", name))
		c.Check(tagCounts[name], qt.IsTrue, qt.Commentf("GET /commodities/tag-counts does not declare %q", name))
		if name == commodityquery.ParamType {
			continue
		}
		c.Check(search[name], qt.IsTrue, qt.Commentf("GET /search does not declare %q", name))
	}
}
