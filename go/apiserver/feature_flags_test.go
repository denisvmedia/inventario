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

func TestFeatureFlags(t *testing.T) {
	cases := []struct {
		name                string
		featureFlag         bool
		withAuth            bool
		wantCurrencyEnabled bool
	}{
		{
			name:                "currency_migration enabled returns the flag set to true",
			featureFlag:         true,
			withAuth:            true,
			wantCurrencyEnabled: true,
		},
		{
			name:                "currency_migration disabled returns the flag set to false",
			featureFlag:         false,
			withAuth:            true,
			wantCurrencyEnabled: false,
		},
		{
			// The endpoint is public — anonymous callers must get the
			// flags too, since the FE needs them on the login surface
			// to hide CTAs for gated-off features.
			name:                "no auth required",
			featureFlag:         true,
			withAuth:            false,
			wantCurrencyEnabled: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			params, _, _ := newParams()
			params.FeatureCurrencyMigration = tc.featureFlag
			handler := apiserver.APIServer(params, &mockRestoreWorker{})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
			// The withAuth flag is a placeholder for future flags that
			// might be tenant-scoped; today the surface is unconditionally
			// public, so no header is set in either branch.
			_ = tc.withAuth
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			c.Assert(rr.Code, qt.Equals, http.StatusOK)
			c.Assert(rr.Header().Get("Content-Type"), qt.Equals, "application/json")

			var flags apiserver.FeatureFlags
			c.Assert(json.Unmarshal(rr.Body.Bytes(), &flags), qt.IsNil)
			c.Assert(flags.CurrencyMigration, qt.Equals, tc.wantCurrencyEnabled)
		})
	}
}
