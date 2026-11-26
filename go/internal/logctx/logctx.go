// Package logctx adds request-scoped fields from the context to log records.
//
// The backend logs through the default slog logger, and nothing hands a
// handler or a service a logger of its own. What travels from the HTTP edge to
// the code that logs is the context, so that is where the request's
// correlation id is read: a record logged with slog.InfoContext(ctx, ...) (or
// any other *Context call) carries request_id whenever ctx descends from a
// request. A call without a context cannot carry it, which is why sloglint's
// context rule is on.
//
// cmd/inventario wraps the process-wide handler with NewHandler once at
// startup. See devdocs/logging.md for the conventions this serves.
package logctx

import (
	"context"
	"log/slog"

	"go.5x5.cz/inventario/appctx"
)

// RequestIDKey is the attribute key the request's correlation id is logged
// under.
const RequestIDKey = "request_id"

type handler struct {
	inner slog.Handler
}

// NewHandler returns a handler that adds request_id to each record whose
// context carries one (see appctx.WithRequestID) and passes the record on to
// inner. Records logged without such a context reach inner unchanged.
func NewHandler(inner slog.Handler) slog.Handler {
	return handler{inner: inner}
}

func (h handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h handler) Handle(ctx context.Context, r slog.Record) error {
	// slog substitutes context.Background for a nil context, but a caller
	// can reach a handler directly, and Value on a nil context panics.
	if ctx != nil {
		if id := appctx.RequestIDFromContext(ctx); id != "" {
			r.AddAttrs(slog.String(RequestIDKey, id))
		}
	}
	return h.inner.Handle(ctx, r)
}

func (h handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return handler{inner: h.inner.WithAttrs(attrs)}
}

// WithGroup nests request_id inside the group like any other attribute of a
// record from that logger.
func (h handler) WithGroup(name string) slog.Handler {
	return handler{inner: h.inner.WithGroup(name)}
}
