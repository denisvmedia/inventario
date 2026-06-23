package sentry

import (
	"context"

	sentrygo "github.com/getsentry/sentry-go"
)

// CaptureError reports err to Sentry with optional tags. It resolves the
// per-request hub attached by Middleware from ctx when present (so the event
// carries the request's URL/method/headers), falling back to the current hub.
// It is a no-op when err is nil or Sentry is disabled, so request-path callers
// (e.g. the apiserver 5xx funnels) can call it unconditionally on every error.
func CaptureError(ctx context.Context, err error, tags map[string]string) {
	if err == nil || !enabled.Load() {
		return
	}
	hub := sentrygo.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentrygo.CurrentHub()
	}
	hub.WithScope(func(scope *sentrygo.Scope) {
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		hub.CaptureException(err)
	})
}
