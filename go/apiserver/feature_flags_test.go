package apiserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
)

// Public, unauthenticated feature-flags endpoint (#1616). The FE reads
// these at boot to hide entry points for features whose backend is
// gated off. Stable JSON keys; flipping a flag at server boot must
// flip the corresponding field in the response and no other field.

func TestFeatureFlags_CurrencyMigrationEnabled(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	params.FeatureCurrencyMigration = true
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Header().Get("Content-Type"), qt.Equals, "application/json")

	var flags apiserver.FeatureFlags
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &flags), qt.IsNil)
	c.Assert(flags.CurrencyMigration, qt.IsTrue)
}

func TestFeatureFlags_CurrencyMigrationDisabled(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams() // flag defaults to false in newParams
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var flags apiserver.FeatureFlags
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &flags), qt.IsNil)
	c.Assert(flags.CurrencyMigration, qt.IsFalse)
}

// The endpoint is public — anonymous callers must get the flags too
// since the FE needs them on the login surface to hide CTAs for
// gated-off features.
func TestFeatureFlags_NoAuthRequired(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	params.FeatureCurrencyMigration = true
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	// No Authorization header on purpose.
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
}
