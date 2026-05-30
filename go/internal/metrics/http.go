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
// middleware. It must be installed after chi has had a chance to fill
// the route pattern, i.e. as router middleware (r.Use), so that
// chi.RouteContext(...).RoutePattern() resolves to the matched
// template (e.g. "/x/{id}") rather than the concrete path.
//
// Unmatched requests (404) keep the empty route pattern chi returns;
// we deliberately do NOT fall back to r.URL.Path, which would be
// unbounded and blow up label cardinality.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			// chi fills the route pattern during ServeHTTP, so it is
			// only reliable here, after next has returned.
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == metricsRoute {
				// Skip self-instrumentation of the scrape endpoint.
				return
			}

			elapsed := time.Since(start)
			class := statusClass(ww.Status())

			httpRequestsTotal.WithLabelValues(r.Method, route, class).Inc()
			httpRequestDuration.WithLabelValues(r.Method, route).Observe(elapsed.Seconds())
		}()

		next.ServeHTTP(ww, r)
	})
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
