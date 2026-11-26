package logctx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/logctx"
)

// newLogger returns a logger that writes JSON records through logctx into buf.
func newLogger(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	return slog.New(logctx.NewHandler(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level})))
}

func decode(c *qt.C, buf *bytes.Buffer) map[string]any {
	c.Helper()
	var rec map[string]any
	c.Assert(json.Unmarshal(buf.Bytes(), &rec), qt.IsNil, qt.Commentf("record %q", buf.String()))
	return rec
}

func TestHandler_AddsTheRequestIDFromTheContext(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	ctx := appctx.WithRequestID(context.Background(), "req-42")

	newLogger(&buf, slog.LevelInfo).InfoContext(ctx, "item saved", "commodity_id", "c-1")

	rec := decode(c, &buf)
	c.Assert(rec[logctx.RequestIDKey], qt.Equals, "req-42")
	c.Assert(rec["commodity_id"], qt.Equals, "c-1")
}

func TestHandler_LeavesARecordWithoutARequestIDAlone(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer

	newLogger(&buf, slog.LevelInfo).InfoContext(context.Background(), "worker tick")

	rec := decode(c, &buf)
	_, ok := rec[logctx.RequestIDKey]
	c.Assert(ok, qt.IsFalse, qt.Commentf("record %v", rec))
}

// A logger derived with With keeps its attributes and still picks the id up
// per call, since the id belongs to the call's context, not to the logger.
func TestHandler_KeepsTheRequestIDOnADerivedLogger(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	logger := newLogger(&buf, slog.LevelInfo).With("worker_id", "w-1")

	logger.WarnContext(appctx.WithRequestID(context.Background(), "req-7"), "retrying")

	rec := decode(c, &buf)
	c.Assert(rec[logctx.RequestIDKey], qt.Equals, "req-7")
	c.Assert(rec["worker_id"], qt.Equals, "w-1")
}

func TestHandler_DefersTheLevelToTheWrappedHandler(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	logger := newLogger(&buf, slog.LevelWarn)
	ctx := appctx.WithRequestID(context.Background(), "req-1")

	c.Assert(logger.Enabled(ctx, slog.LevelInfo), qt.IsFalse)
	c.Assert(logger.Enabled(ctx, slog.LevelWarn), qt.IsTrue)
	logger.InfoContext(ctx, "below the threshold")
	c.Assert(buf.Len(), qt.Equals, 0)
}

// slog never hands a handler a nil context, but a direct caller can.
func TestHandler_AcceptsANilContext(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	h := logctx.NewHandler(slog.NewJSONHandler(&buf, nil))
	var ctx context.Context

	err := h.Handle(ctx, slog.NewRecord(time.Now(), slog.LevelInfo, "direct", 0))

	c.Assert(err, qt.IsNil)
	c.Assert(decode(c, &buf)["msg"], qt.Equals, "direct")
}
