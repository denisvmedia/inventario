package apiserver

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	swagger "github.com/swaggo/http-swagger"

	_ "github.com/denisvmedia/inventario/docs" // register swagger docs
	"github.com/denisvmedia/inventario/registry"
)

type ctxValueKey string

func defaultRequestContentType(contentType string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if Content-Type header is not set
			if r.Header.Get("Content-Type") == "" {
				// Set default content type
				r.Header.Set("Content-Type", contentType)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// paginate is a stub, but very possible to implement middleware logic
// to handle the request params for handling a paginated request.
func paginate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// just a stub... some ideas are to look at URL query params for something like
		// the page number, or the limit, and send a query cursor down the chain
		next.ServeHTTP(w, r)
	})
}

type Params struct {
	LocationRegistry registry.LocationRegistry
}

func APIServer(params Params) http.Handler {
	render.Decode = JSONAPIAwareDecoder

	r := chi.NewRouter()

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.Logger)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})

	// RESTy routes for "swagger" resource
	r.Mount("/swagger", swagger.Handler(
		swagger.URL("/swagger/doc.json"),
	))

	r.Route("/api/v1", func(r chi.Router) {
		// Middleware to set default content type if not provided
		r.Use(
			defaultRequestContentType("application/vnd.api+json"),
			middleware.AllowContentType("application/json", "application/vnd.api+json"),
		)

		// RESTy routes for "locations" resource
		r.Route("/locations", Locations(params.LocationRegistry))
	})

	return r
}
