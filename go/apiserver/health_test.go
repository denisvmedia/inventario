package apiserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
)

type healthResponse struct {
	Status string `json:"status"`
}

type readinessCheckResponse struct {
	Status  string `json:"status"`
	Latency string `json:"latency"`
	Error   string `json:"error"`
}

type readinessResponse struct {
	Status    string                            `json:"status"`
	Timestamp string                            `json:"timestamp"`
	Checks    map[string]readinessCheckResponse `json:"checks"`
}

type failingRedisPinger struct {
	err error
}

func (p *failingRedisPinger) Ping(_ context.Context) error {
	return p.err
}

func TestHealthz(t *testing.T) {
	c := qt.New(t)

	params, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var body healthResponse
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	c.Assert(err, qt.IsNil)
	c.Assert(body.Status, qt.Equals, "alive")
}

func TestReadyz_DBAndRedisSkipped(t *testing.T) {
	c := qt.New(t)

	params, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var body readinessResponse
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	c.Assert(err, qt.IsNil)
	c.Assert(body.Status, qt.Equals, "ready")
	c.Assert(body.Timestamp, qt.Not(qt.Equals), "")
	c.Assert(body.Checks["database"].Status, qt.Equals, "ok")
	c.Assert(body.Checks["redis"].Status, qt.Equals, "skipped")
}

func TestReadyz_DBFailure(t *testing.T) {
	c := qt.New(t)

	params, _ := newParams()
	params.FactorySet.PingFn = func(_ context.Context) error {
		return errors.New("database unavailable")
	}
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusServiceUnavailable)

	var body readinessResponse
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	c.Assert(err, qt.IsNil)
	c.Assert(body.Status, qt.Equals, "not_ready")
	c.Assert(body.Checks["database"].Status, qt.Equals, "error")
	c.Assert(body.Checks["database"].Error, qt.Contains, "database unavailable")
}

func TestReadyz_RedisFailureWhenConfigured(t *testing.T) {
	c := qt.New(t)

	params, _ := newParams()
	params.RedisPinger = &failingRedisPinger{err: errors.New("redis unavailable")}
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusServiceUnavailable)

	var body readinessResponse
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	c.Assert(err, qt.IsNil)
	c.Assert(body.Status, qt.Equals, "not_ready")
	c.Assert(body.Checks["database"].Status, qt.Equals, "ok")
	c.Assert(body.Checks["redis"].Status, qt.Equals, "error")
	c.Assert(body.Checks["redis"].Error, qt.Contains, "redis unavailable")
}
