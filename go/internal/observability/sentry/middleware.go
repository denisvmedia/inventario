package sentry

import (
	"net/http"

	sentryhttp "github.com/getsentry/sentry-go/http"
)

// Middleware returns a chi-compatible middleware that attaches a per-request
// Sentry hub to the request context and reports panics.
//
// ORDERING: it MUST be registered AFTER chi's middleware.Recoverer (so it sits
// INSIDE it). Repanic is true, so on a handler panic this middleware recovers,
// reports the event to Sentry, then re-panics; the re-panic propagates out to
// Recoverer, which writes the 500. Registered the other way round (outside
// Recoverer), Recoverer would swallow the panic first and Sentry would never
// see it. Contrast metrics.HTTPMiddleware, which sits OUTSIDE Recoverer because
// it only needs to observe the final 500, not catch the panic.
//
// When Sentry is disabled the middleware is a straight pass-through (no
// per-request hub is cloned), so it is safe and cheap to install unconditionally.
func Middleware() func(http.Handler) http.Handler {
	handler := sentryhttp.New(sentryhttp.Options{Repanic: true})
	return func(next http.Handler) http.Handler {
		wrapped := handler.Handle(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled.Load() {
				next.ServeHTTP(w, r)
				return
			}
			wrapped.ServeHTTP(w, r)
		})
	}
}
