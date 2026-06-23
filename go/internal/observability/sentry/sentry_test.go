package sentry_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/observability/sentry"
)

func TestLoadConfig_ReadsBareEnv(t *testing.T) {
	c := qt.New(t)

	t.Setenv("SENTRY_DSN", "https://pub@example.com/1")
	t.Setenv("SENTRY_ENVIRONMENT", "staging")
	t.Setenv("SENTRY_TRACES_SAMPLE_RATE", "0.5")

	cfg, err := sentry.LoadConfig()
	c.Assert(err, qt.IsNil)
	c.Assert(cfg.DSN, qt.Equals, "https://pub@example.com/1")
	c.Assert(cfg.Environment, qt.Equals, "staging")
	c.Assert(cfg.TracesSampleRate, qt.Equals, 0.5)
}

func TestLoadConfig_DefaultsSampleRateWhenUnset(t *testing.T) {
	c := qt.New(t)

	// t.Setenv records the original value and restores it on cleanup; the
	// immediate Unsetenv makes the var absent during the test so the
	// env-default ("0.2") fires deterministically regardless of the ambient env.
	t.Setenv("SENTRY_TRACES_SAMPLE_RATE", "")
	c.Assert(os.Unsetenv("SENTRY_TRACES_SAMPLE_RATE"), qt.IsNil)

	cfg, err := sentry.LoadConfig()
	c.Assert(err, qt.IsNil)
	c.Assert(cfg.TracesSampleRate, qt.Equals, 0.2)
}

func TestInit_NoDSNReturnsNoopFlush(t *testing.T) {
	c := qt.New(t)

	flush, err := sentry.Init(sentry.Config{})
	c.Assert(err, qt.IsNil)
	c.Assert(flush, qt.IsNotNil)
	// The no-op flush reports success without blocking.
	c.Assert(flush(time.Second), qt.IsTrue)
}

func TestCaptureError_NilAndDisabledAreNoop(t *testing.T) {
	c := qt.New(t)

	// Neither a nil error nor a real error (with Sentry disabled) may panic.
	defer func() { c.Assert(recover(), qt.IsNil) }()
	sentry.CaptureError(context.Background(), nil, nil)
	sentry.CaptureError(context.Background(), errors.New("boom"), map[string]string{"component": "test"})
}

func TestMiddleware_DisabledIsPassThrough(t *testing.T) {
	c := qt.New(t)

	var called bool
	h := sentry.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	c.Assert(called, qt.IsTrue)
	c.Assert(rec.Code, qt.Equals, http.StatusTeapot)
}
