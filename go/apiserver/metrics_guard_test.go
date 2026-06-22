package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
)

// The /metrics endpoint exposes installation-wide business gauges (#2102).
// When params.MetricsToken is set it must require a bearer token; when unset
// it stays open (legacy behaviour that keeps local dev frictionless).

func TestMetricsGuard_OpenWhenTokenUnset(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	// MetricsToken left empty → open.
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
}

func TestMetricsGuard_RejectsMissingTokenWhenConfigured(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	params.MetricsToken = "s3cr3t-metrics-token-at-least-32-bytes!"
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

func TestMetricsGuard_RejectsWrongTokenWhenConfigured(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	params.MetricsToken = "s3cr3t-metrics-token-at-least-32-bytes!"
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

func TestMetricsGuard_AcceptsCorrectTokenWhenConfigured(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	const token = "s3cr3t-metrics-token-at-least-32-bytes!"
	params.MetricsToken = token
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
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
