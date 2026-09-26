package apiserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"go.5x5.cz/inventario/appctx"
)

// requestIDMaxLen bounds what may be persisted as a correlation id.
//
// chi honors an incoming X-Request-Id header verbatim, and the audit service now
// writes that value to a TEXT column — so without a bound a client chooses how
// many bytes land in every audit row its request produces. 128 is far past any
// real correlation id: chi's own is around 40 characters and a UUID is 36.
const requestIDMaxLen = 128

// RequestIDContextMiddleware copies the correlation id chi's RequestID
// middleware resolved into the application context, when it is one we are
// willing to store.
//
// It exists so that everything below the HTTP layer — the audit service, in
// particular — can read the id through appctx instead of importing a router to
// call middleware.GetReqID. Must be registered AFTER middleware.RequestID;
// before it, there is no id to copy and every row would carry an empty one.
//
// An id that is too long, or that carries anything but printable ASCII, is
// dropped rather than truncated or escaped: it came from the client, the row is
// better off NULL than holding junk, and a truncated id correlates with nothing
// anyway. chi's value still reaches the request log, so nothing is lost for
// debugging.
func RequestIDContextMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := middleware.GetReqID(r.Context())
			if id == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !storableRequestID(id) {
				slog.Debug("Ignoring an unusable client-supplied request id",
					"length", len(id), "path", r.URL.Path)
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(appctx.WithRequestID(r.Context(), id)))
		})
	}
}

// storableRequestID reports whether an id is short enough and clean enough to
// persist. Printable ASCII only, which keeps control characters out of both the
// database and anything that later renders a row.
func storableRequestID(id string) bool {
	if len(id) > requestIDMaxLen {
		return false
	}
	for i := range len(id) {
		if id[i] < 0x20 || id[i] > 0x7E {
			return false
		}
	}
	return true
}
