package bootstrap_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/cmd/inventario/run/bootstrap"
	"github.com/denisvmedia/inventario/registry"
)

type stubRedisPinger struct {
	err error
}

func (p *stubRedisPinger) Ping(context.Context) error { return p.err }

func newProbeRuntimeSetup(pinger apiserver.RedisPinger) *bootstrap.RuntimeSetup {
	return &bootstrap.RuntimeSetup{
		FactorySet: &registry.FactorySet{},
		Params: apiserver.Params{
			FactorySet:  &registry.FactorySet{},
			RedisPinger: pinger,
		},
	}
}

func TestProbesHandler_HealthzReturnsAlive(t *testing.T) {
	c := qt.New(t)

	handler := bootstrap.ProbesHandler(newProbeRuntimeSetup(nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	c.Assert(rec.Code, qt.Equals, http.StatusOK)
	c.Assert(rec.Body.String(), qt.Contains, `"status":"alive"`)
}

func TestProbesHandler_ReadyzWithNoRedisSkipsRedisCheck(t *testing.T) {
	c := qt.New(t)

	handler := bootstrap.ProbesHandler(newProbeRuntimeSetup(nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	c.Assert(rec.Code, qt.Equals, http.StatusOK)
	body := rec.Body.String()
	c.Assert(body, qt.Contains, `"status":"ready"`)
	c.Assert(body, qt.Contains, `"skipped"`)
}

func TestProbesHandler_ReadyzRedisFailureReturns503(t *testing.T) {
	c := qt.New(t)

	handler := bootstrap.ProbesHandler(newProbeRuntimeSetup(&stubRedisPinger{err: errors.New("down")}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	c.Assert(rec.Code, qt.Equals, http.StatusServiceUnavailable)
	c.Assert(rec.Body.String(), qt.Contains, `"status":"not_ready"`)
}

func TestProbesHandler_MetricsEndpointServesPrometheus(t *testing.T) {
	c := qt.New(t)

	handler := bootstrap.ProbesHandler(newProbeRuntimeSetup(nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	c.Assert(rec.Code, qt.Equals, http.StatusOK)
	// The default Go collectors are registered, so at minimum the go_goroutines
	// metric must be exported by the Prometheus handler.
	c.Assert(rec.Body.String(), qt.Contains, "go_goroutines")
}

func TestStartProbes_ServesAllThreeEndpointsOverNetwork(t *testing.T) {
	c := qt.New(t)

	cfg := &bootstrap.Config{ProbeAddr: "127.0.0.1:0"}
	srv, errCh := bootstrap.StartProbes(cfg, newProbeRuntimeSetup(nil))
	c.Assert(srv, qt.IsNotNil)

	baseURL := "http://127.0.0.1:" + strconv.Itoa(srv.Port())
	client := &http.Client{Timeout: 2 * time.Second}
	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		resp, err := client.Get(baseURL + path)
		c.Assert(err, qt.IsNil, qt.Commentf("path=%s", path))
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		c.Assert(resp.StatusCode, qt.Equals, http.StatusOK, qt.Commentf("path=%s", path))
	}

	c.Assert(srv.Shutdown(), qt.IsNil)

	select {
	case _, open := <-errCh:
		c.Assert(open, qt.IsFalse, qt.Commentf("errCh should be closed after Shutdown"))
	case <-time.After(2 * time.Second):
		t.Fatal("errCh was not closed after Shutdown")
	}
}
