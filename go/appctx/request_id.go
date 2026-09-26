package appctx

import "context"

// requestIDKey carries the per-request correlation id from the HTTP edge, where
// the router generates it, down to code that has no business importing a router
// to read it — the audit service being the reason it exists (#2479, #967 H5).
//
// The log line and the audit row were previously joinable only by timestamp and
// user, which is enough to guess a correlation and not enough to prove one. The
// same id in both makes it exact.
const requestIDKey contextKey = "requestID"

// WithRequestID returns a context carrying the request's correlation id. An
// empty id is ignored, so a caller can pass a best-effort lookup result
// unconditionally.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext returns the correlation id set by WithRequestID, or ""
// when none is set — which is the normal answer off the HTTP path, for a worker
// or a CLI command.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}
