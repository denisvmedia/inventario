package apiserver

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"go.5x5.cz/inventario/appctx"
)

// RequestIDContextMiddleware copies the correlation id chi's RequestID
// middleware generated into the application context.
//
// It exists so that everything below the HTTP layer — the audit service, in
// particular — can read the id through appctx instead of importing a router to
// call middleware.GetReqID. Must be registered AFTER middleware.RequestID;
// before it, there is no id to copy and every row would carry an empty one.
func RequestIDContextMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := middleware.GetReqID(r.Context())
			if id == "" {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(appctx.WithRequestID(r.Context(), id)))
		})
	}
}
