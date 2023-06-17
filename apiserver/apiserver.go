package apiserver

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"
	swagger "github.com/swaggo/http-swagger"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob" // register azureblob driver
	_ "gocloud.dev/blob/fileblob"  // register fileblob driver
	_ "gocloud.dev/blob/gcsblob"   // register gcsblob driver
	_ "gocloud.dev/blob/memblob"   // register memblob driver
	_ "gocloud.dev/blob/s3blob"    // register s3blob driver

	_ "github.com/denisvmedia/inventario/docs" // register swagger docs
	"github.com/denisvmedia/inventario/registry"
)

type ctxValueKey string

var defaultAPIMiddlewares = []func(http.Handler) http.Handler{
	defaultRequestContentType("application/vnd.api+json"),
	middleware.AllowContentType("application/json", "application/vnd.api+json"),
}

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
	LocationRegistry  registry.LocationRegistry
	AreaRegistry      registry.AreaRegistry
	CommodityRegistry registry.CommodityRegistry
	ImageRegistry     registry.ImageRegistry
	ManualRegistry    registry.ManualRegistry
	InvoiceRegistry   registry.InvoiceRegistry

	UploadLocation string
}

func (p *Params) Validate() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&p.LocationRegistry, validation.Required),
		validation.Field(&p.AreaRegistry, validation.Required),
		validation.Field(&p.CommodityRegistry, validation.Required),
		validation.Field(&p.UploadLocation, validation.Required, validation.By(func(value interface{}) error {
			ctx := context.Background()
			b, err := blob.OpenBucket(ctx, p.UploadLocation)
			if err != nil {
				return err
			}
			_ = b.Close() // best effort
			return nil
		})),
	)

	return validation.ValidateStruct(p, fields...)
}

func APIServer(params Params) http.Handler {
	render.Decode = JSONAPIAwareDecoder

	r := chi.NewRouter()

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome to Inventario!"))
	})

	// RESTy routes for "swagger" resource
	r.Mount("/swagger", swagger.Handler(
		swagger.URL("/swagger/doc.json"),
	))

	r.Route("/api/v1", func(r chi.Router) {
		r.With(defaultAPIMiddlewares...).Route("/locations", Locations(params.LocationRegistry))
		r.With(defaultAPIMiddlewares...).Route("/areas", Areas(params.AreaRegistry))
		r.With(defaultAPIMiddlewares...).Route("/commodities", Commodities(params.CommodityRegistry))
		r.Route("/uploads", Uploads(params))
	})

	return r
}
