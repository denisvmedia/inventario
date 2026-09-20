package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/apiserver"
)

// /metrics is not served by the public router (#2244) — it lives on the probe
// listener, which the chart keeps off the ingress. The guard itself is still
// used there, so its unit tests stay here next to the middleware.

func TestPublicRouter_DoesNotServeMetrics(t *testing.T) {
	c := qt.New(t)

	// Both with and without a token: the endpoint is absent either way, so a
	// deployment cannot expose it by leaving the token unset.
	for _, token := range []string{"", "s3cr3t-metrics-token-at-least-32-bytes!"} {
		params, _, _ := newParams()
		params.MetricsToken = token
		handler := apiserver.APIServer(params, &mockRestoreWorker{})

		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		c.Assert(rr.Code, qt.Equals, http.StatusNotFound,
			qt.Commentf("the default ingress forwards every path on this router"))
	}
}

func TestMetricsTokenMiddleware_NoOpWhenEmpty(t *testing.T) {
	c := qt.New(t)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	h := apiserver.MetricsTokenMiddleware("")(next)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	c.Assert(called, qt.IsTrue)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
}
