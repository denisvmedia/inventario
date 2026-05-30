package metrics

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// metricsRoute is the path we never self-instrument: counting the
// scrape itself would pollute latency and rate dashboards.
const metricsRoute = "/metrics"

// HTTPMiddleware is a chi-aware RED (Rate, Errors, Duration)
// middleware. It must be installed as router middleware (r.Use) so that
// chi.RouteContext(...).RoutePattern() resolves to the matched template
// (e.g. "/x/{id}") rather than the concrete path. It should also wrap
// chi's Recoverer (be registered BEFORE it) so the deferred status read
// observes the 500 Recoverer writes for a recovered panic.
//
// Unmatched requests (404) keep the empty route pattern chi returns;
// we deliberately do NOT fall back to r.URL.Path, which would be
// unbounded and blow up label cardinality.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip the scrape endpoint entirely — including the in-flight
		// gauge — so an otherwise idle instance does not report an
		// in-flight request on every scrape. The raw path is reliable
		// here because /metrics is mounted at a fixed path; this runs
		// before any routing, so RoutePattern is not yet available.
		if r.URL.Path == metricsRoute {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()

		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			// chi fills the route pattern during ServeHTTP, so it is
			// only reliable here, after next has returned.
			route := chi.RouteContext(r.Context()).RoutePattern()
			elapsed := time.Since(start)
			class := statusClass(ww.Status())
			method := normalizeMethod(r.Method)

			httpRequestsTotal.WithLabelValues(method, route, class).Inc()
			httpRequestDuration.WithLabelValues(method, route).Observe(elapsed.Seconds())
		}()

		next.ServeHTTP(ww, r)
	})
}

// normalizeMethod maps an HTTP method to a bounded label value. r.Method
// is client-controlled — an arbitrary token (e.g. on an unmatched route)
// would otherwise create an unbounded `method` label — so anything
// outside the standard set collapses to "OTHER".
func normalizeMethod(m string) string {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect,
		http.MethodOptions, http.MethodTrace:
		return m
	default:
		return "OTHER"
	}
}

// statusClass maps an HTTP status code to its class label ("1xx" ..
// "5xx"). A zero code means the handler never wrote a header, which
// the wrapped writer reports as the default 200, so we treat 0 as
// "2xx" defensively.
func statusClass(code int) string {
	switch code / 100 {
	case 1:
		return "1xx"
	case 2:
		return "2xx"
	case 3:
		return "3xx"
	case 4:
		return "4xx"
	case 5:
		return "5xx"
	default:
		return "2xx"
	}
}
