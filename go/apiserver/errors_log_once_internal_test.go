package apiserver

// White-box: renderEntityError, renderLoggedEntityError, renderScanError and
// errorLogLevel are unexported, and the 500 they handle comes from a failing
// registry or a wiring bug, which no route can be made to produce on demand.

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

func TestRenderEntityError_LogsAServerFailureOnce(t *testing.T) {
	tests := []struct {
		name   string
		render func(http.ResponseWriter, *http.Request, error) error
		err    error
		status int
		lines  int
	}{
		{
			name:   "renderEntityError logs an unrecognised error",
			render: renderEntityError,
			err:    errors.New("connection reset by peer"),
			status: http.StatusInternalServerError,
			lines:  1,
		},
		{
			name:   "renderEntityError does not log a missing row",
			render: renderEntityError,
			err:    errxtrace.Wrap("load item", registry.ErrNotFound),
			status: http.StatusNotFound,
			lines:  0,
		},
		{
			name:   "renderLoggedEntityError leaves the line to its caller",
			render: renderLoggedEntityError,
			err:    errors.New("connection reset by peer"),
			status: http.StatusInternalServerError,
			lines:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			c.Cleanup(func() { slog.SetDefault(prev) })

			rr := httptest.NewRecorder()
			_ = tt.render(rr, httptest.NewRequest(http.MethodGet, "/", nil), tt.err)
			c.Assert(rr.Code, qt.Equals, tt.status)
			c.Assert(strings.Count(buf.String(), `"msg":"internal server error"`), qt.Equals, tt.lines)
		})
	}
}

func TestErrorLogLevel(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want slog.Level
	}{
		{"a missing row", errxtrace.Wrap("load item", registry.ErrNotFound), slog.LevelWarn},
		{"a refused delete", errxtrace.Wrap("delete area", registry.ErrCannotDelete), slog.LevelWarn},
		{"an unrecognised failure", errors.New("connection reset by peer"), slog.LevelError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qt.New(t).Assert(errorLogLevel(tt.err), qt.Equals, tt.want)
		})
	}
}

// The scan service logs a missing identity itself, with which half was
// missing, so the handler renders the 500 without a second line. A
// misconfigured provider is logged by the handler alone.
func TestRenderScanError_LogsAServerFailureOnce(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		lines int
	}{
		{"a missing identity, logged by the service", errxtrace.Classify(services.ErrScanIdentityMissing), 0},
		{"a misconfigured provider", errxtrace.Classify(services.ErrScanProviderMisconfigured), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			c.Cleanup(func() { slog.SetDefault(prev) })

			rr := httptest.NewRecorder()
			renderScanError(rr, httptest.NewRequest(http.MethodPost, "/", nil), tt.err)
			c.Assert(rr.Code, qt.Equals, http.StatusInternalServerError)
			c.Assert(strings.Count(buf.String(), `"msg":"internal server error"`), qt.Equals, tt.lines)
		})
	}
}
