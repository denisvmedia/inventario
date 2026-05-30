package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/denisvmedia/inventario/internal/metrics"
)

func TestHTTPMiddleware_RecordsRoutePatternNotConcretePath(t *testing.T) {
	c := qt.New(t)

	r := chi.NewRouter()
	r.Use(metrics.HTTPMiddleware)
	r.Get("/x/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	before := testutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues(http.MethodGet, "/x/{id}", "2xx"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x/123", nil))

	c.Assert(rec.Code, qt.Equals, http.StatusOK)

	after := testutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues(http.MethodGet, "/x/{id}", "2xx"))
	c.Assert(after-before, qt.Equals, float64(1))

	// The concrete path must NOT have produced its own series.
	concrete := testutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues(http.MethodGet, "/x/123", "2xx"))
	c.Assert(concrete, qt.Equals, float64(0))
}

func TestHTTPMiddleware_InFlightReturnsToZero(t *testing.T) {
	c := qt.New(t)

	r := chi.NewRouter()
	r.Use(metrics.HTTPMiddleware)
	r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))

	c.Assert(testutil.ToFloat64(metrics.HTTPRequestsInFlight), qt.Equals, float64(0))
}

func TestHTTPMiddleware_SkipsMetricsRoute(t *testing.T) {
	c := qt.New(t)

	r := chi.NewRouter()
	r.Use(metrics.HTTPMiddleware)
	r.Get("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	before := testutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues(http.MethodGet, "/metrics", "2xx"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	c.Assert(rec.Code, qt.Equals, http.StatusOK)

	after := testutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues(http.MethodGet, "/metrics", "2xx"))
	c.Assert(after-before, qt.Equals, float64(0))
}

func TestStatusClass(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{0, "2xx"},
		{100, "1xx"},
		{200, "2xx"},
		{301, "3xx"},
		{404, "4xx"},
		{503, "5xx"},
	}
	for _, tc := range tests {
		t.Run(http.StatusText(tc.code), func(t *testing.T) {
			c := qt.New(t)
			c.Assert(metrics.StatusClass(tc.code), qt.Equals, tc.want)
		})
	}
}
