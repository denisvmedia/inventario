package apiserver

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"
	"github.com/rs/cors"
	swagger "github.com/swaggo/http-swagger"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob" // register azureblob driver
	// _ "gocloud.dev/blob/fileblob"  // register fileblob driver
	_ "gocloud.dev/blob/gcsblob" // register gcsblob driver
	_ "gocloud.dev/blob/memblob" // register memblob driver
	_ "gocloud.dev/blob/s3blob"  // register s3blob driver

	"github.com/denisvmedia/inventario/debug"
	_ "github.com/denisvmedia/inventario/docs" // register swagger docs
	_ "github.com/denisvmedia/inventario/internal/fileblob"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// RestoreWorkerInterface defines the interface for the restore worker
type RestoreWorkerInterface interface {
	HasRunningRestores(ctx context.Context) (bool, error) // Returns true if any restore is running or pending
}

type ctxValueKey string

var defaultAPIMiddlewares = []func(http.Handler) http.Handler{
	defaultRequestContentType("application/vnd.api+json"),
	middleware.AllowContentType("application/json", "application/vnd.api+json"),
}

// createTenantUserAwareMiddlewares creates middleware stack with tenant and user context
func createTenantUserAwareMiddlewares(registrySet *registry.Set) []func(http.Handler) http.Handler {
	// Create simple resolvers with fallback to default IDs for backward compatibility
	tenantResolver := NewSimpleTenantResolver("test-tenant-id")
	userResolver := NewSimpleUserResolver("test-user-id")

	return append(defaultAPIMiddlewares,
		TenantAwareMiddleware(tenantResolver),
		UserAwareMiddleware(userResolver),
	)
}

// createTenantUserAwareMiddlewaresForUploads creates middleware stack for uploads (without content type restrictions)
func createTenantUserAwareMiddlewaresForUploads(registrySet *registry.Set) []func(http.Handler) http.Handler {
	// Create simple resolvers with fallback to default IDs for backward compatibility
	tenantResolver := NewSimpleTenantResolver("test-tenant-id")
	userResolver := NewSimpleUserResolver("test-user-id")

	// Only add tenant/user context, no content type restrictions for uploads
	return []func(http.Handler) http.Handler{
		TenantAwareMiddleware(tenantResolver),
		UserAwareMiddleware(userResolver),
	}
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
	RegistrySet    *registry.Set
	EntityService  *services.EntityService
	UploadLocation string
	DebugInfo      *debug.Info
	StartTime      time.Time
}

func (p *Params) Validate() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&p.RegistrySet, validation.Required),
		validation.Field(&p.EntityService, validation.Required),
		validation.Field(&p.UploadLocation, validation.Required, validation.By(func(_value any) error {
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

func APIServer(params Params, restoreWorker RestoreWorkerInterface) http.Handler {
	render.Decode = JSONAPIAwareDecoder

	r := chi.NewRouter()

	// c := cors.New(cors.Options{
	//	AllowedOrigins: []string{"http://foo.com", "http://foo.com:8080"},
	//	AllowCredentials: true,
	//	// Enable Debugging for testing, consider disabling in production
	//	Debug: true,
	// })

	// CORS middleware
	r.Use(cors.AllowAll().Handler)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// r.Get("/", func(w http.ResponseWriter, _r *http.Request) {
	//	w.Write([]byte("Welcome to Inventario!"))
	// })
	//
	// RESTy routes for "swagger" resource
	r.Mount("/swagger", swagger.Handler(
		swagger.URL("/swagger/doc.json"),
	))

	r.Route("/api/v1", func(r chi.Router) {
		// Create tenant and user aware middlewares
		tenantUserMiddlewares := createTenantUserAwareMiddlewares(params.RegistrySet)
		tenantUserUploadMiddlewares := createTenantUserAwareMiddlewaresForUploads(params.RegistrySet)

		// Apply tenant/user context to main API routes
		r.With(tenantUserMiddlewares...).Route("/locations", Locations(params.RegistrySet.LocationRegistry))
		r.With(tenantUserMiddlewares...).Route("/areas", Areas(params.RegistrySet.AreaRegistry))
		r.With(tenantUserMiddlewares...).Route("/commodities", Commodities(params))
		r.With(defaultAPIMiddlewares...).Route("/settings", Settings(params.RegistrySet.SettingsRegistry))
		r.With(defaultAPIMiddlewares...).Route("/system", System(params.RegistrySet.SettingsRegistry, params.DebugInfo, params.StartTime))
		r.With(tenantUserMiddlewares...).Route("/exports", Exports(params, restoreWorker))
		r.With(tenantUserMiddlewares...).Route("/files", Files(params))
		r.With(tenantUserMiddlewares...).Route("/search", Search(params.RegistrySet))

		// Routes that don't need tenant/user context
		r.Route("/currencies", Currencies())
		// Uploads need special middleware without content type restrictions
		r.With(tenantUserUploadMiddlewares...).Route("/uploads", Uploads(params))
		r.Route("/seed", Seed(params.RegistrySet))
		r.Route("/commodities/values", Values(params.RegistrySet))
		r.Route("/debug", Debug(params))
	})

	// use Frontend as a root directory
	r.Handle("/*", FrontendHandler())

	return r
}
