package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestSeed_FirstCallReportsFreshSeed(t *testing.T) {
	c := qt.New(t)

	// The real apiserver mounts /seed under defaultAPIMiddlewares, but those
	// middlewares are unrelated to the idempotency contract under test, so
	// we skip them and exercise the handler directly.
	factorySet := memory.NewFactorySet()
	r := chi.NewRouter()
	r.Route("/seed", apiserver.Seed(factorySet))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/seed", strings.NewReader("")))

	c.Assert(w.Code, qt.Equals, http.StatusOK)

	var resp apiserver.SeedResponse
	c.Assert(json.Unmarshal(w.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Status, qt.Equals, "success")
	c.Assert(resp.AlreadySeeded, qt.IsFalse)
	c.Assert(resp.Message, qt.Equals, "Database seeded successfully")
}

func TestSeed_SecondCallIsIdempotentNoOp(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	factorySet := memory.NewFactorySet()
	r := chi.NewRouter()
	r.Route("/seed", apiserver.Seed(factorySet))

	// First call inserts the seed payload.
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodPost, "/seed", strings.NewReader("")))
	c.Assert(w1.Code, qt.Equals, http.StatusOK)

	// Snapshot data-table counts after the first call so we can assert the
	// second call doesn't double them — this is the regression from #1482.
	registrySet := factorySet.CreateServiceRegistrySet()
	locationsAfterFirst, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	areasAfterFirst, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	commoditiesAfterFirst, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)

	// Second call against the same in-memory DB.
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/seed", strings.NewReader("")))

	c.Assert(w2.Code, qt.Equals, http.StatusOK)

	var resp apiserver.SeedResponse
	c.Assert(json.Unmarshal(w2.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Status, qt.Equals, "success")
	c.Assert(resp.AlreadySeeded, qt.IsTrue)
	c.Assert(resp.Message, qt.Equals, "Database already seeded")

	// And the data-table counts must not have grown.
	locations, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locations, qt.HasLen, len(locationsAfterFirst))

	areas, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, len(areasAfterFirst))

	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, len(commoditiesAfterFirst))
}
