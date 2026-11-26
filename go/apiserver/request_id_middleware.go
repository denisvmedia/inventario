package apiserver

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"go.5x5.cz/inventario/appctx"
)

// RequestIDHeader carries a request's correlation id in both directions: a
// client or a proxy may send one, and every response returns the id the server
// used.
const RequestIDHeader = "X-Request-ID"

// requestIDMaxLen bounds a client-supplied id.
//
// The id lands on every log line the request writes and in the request_id
// column of every audit row it produces, so without a bound the client decides
// how many bytes that is. 128 is far past any real correlation id: a UUID is 36
// characters, and chi's own format is around 40.
const requestIDMaxLen = 128

// RequestIDMiddleware gives every request a correlation id, puts it on the
// request context and returns it in the X-Request-ID response header.
//
// An incoming X-Request-ID is kept when it is at most requestIDMaxLen bytes of
// letters, digits and "-_.:/+=". That covers UUIDs, hex and base64 ids, and the
// "host/prefix-000001" form chi generates, so an id from a proxy or a sibling
// service survives and correlates across them. Anything else, including a
// missing header, gets a fresh UUID. A bad id is replaced rather than truncated
// or escaped: a truncated id correlates with nothing, and the excluded
// characters (whitespace, quotes, separators, control bytes, non-ASCII) are
// the ones that can break a log line, a header or a page that renders an
// audit row.
//
// The id goes on the context twice. appctx carries it to the log handler
// (internal/logctx), which adds it to every record logged with this context
// as request_id, and to the audit service, which stores it on the row. chi's
// key carries it to chi's access log, which reads only that key.
//
// Register it before anything that can log or reject a request, so that the
// lines those write and the responses they send carry the id too.
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			supplied := r.Header.Get(RequestIDHeader)
			id := supplied
			if !validRequestID(id) {
				id = uuid.NewString()
			}
			w.Header().Set(RequestIDHeader, id)

			ctx := appctx.WithRequestID(r.Context(), id)
			ctx = context.WithValue(ctx, middleware.RequestIDKey, id)
			if supplied != "" && supplied != id {
				// The length only: the value is whatever the client sent, which
				// is the thing that was not fit to log.
				slog.DebugContext(ctx, "Replaced an unusable client-supplied request id",
					"length", len(supplied), "path", r.URL.Path)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// validRequestID reports whether a client-supplied id is short enough and made
// only of characters that are safe in a log line, a header and HTML.
func validRequestID(id string) bool {
	if id == "" || len(id) > requestIDMaxLen {
		return false
	}
	for i := range len(id) {
		if !requestIDByte(id[i]) {
			return false
		}
	}
	return true
}

func requestIDByte(b byte) bool {
	switch {
	case 'a' <= b && b <= 'z', 'A' <= b && b <= 'Z', '0' <= b && b <= '9':
		return true
	}
	switch b {
	case '-', '_', '.', ':', '/', '+', '=':
		return true
	}
	return false
}
