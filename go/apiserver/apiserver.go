package apiserver

import (
	"context"
	"fmt"
	"log/slog"
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

	"github.com/denisvmedia/inventario/appctx"
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

// createUserAwareMiddlewares creates middleware stack with user authentication and RLS context
func createUserAwareMiddlewares(jwtSecret []byte, userRegistry registry.UserRegistry, registrySet *registry.Set) []func(http.Handler) http.Handler {
	return append(defaultAPIMiddlewares,
		JWTMiddleware(jwtSecret, userRegistry),
		RLSContextMiddleware(registrySet),
	)
}

// createUserAwareMiddlewaresForUploads creates middleware stack for uploads (without content type restrictions)
func createUserAwareMiddlewaresForUploads(jwtSecret []byte, userRegistry registry.UserRegistry, registrySet *registry.Set) []func(http.Handler) http.Handler {
	// Only add user authentication and RLS context, no content type restrictions for uploads
	return []func(http.Handler) http.Handler{
		JWTMiddleware(jwtSecret, userRegistry),
		RLSContextMiddleware(registrySet),
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
	JWTSecret      []byte // JWT secret for user authentication
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
		validation.Field(&p.JWTSecret, validation.Required, validation.Length(32, 0)), // Require at least 32 bytes for security
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

	// SECURITY: Add tenant ID validation middleware FIRST (before any other processing)
	r.Use(ValidateNoUserProvidedTenantID())
	r.Use(RejectSpecificTenantHeaders())

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
		// Public routes (no authentication required)
		r.Route("/auth", Auth(params.RegistrySet.UserRegistry, params.JWTSecret))
		r.Route("/currencies", Currencies())
		// Seed endpoint is public for e2e testing and development
		r.With(defaultAPIMiddlewares...).Route("/seed", Seed(params.RegistrySet))

		// Create user aware middlewares for protected routes
		userMiddlewares := createUserAwareMiddlewares(params.JWTSecret, params.RegistrySet.UserRegistry, params.RegistrySet)
		userUploadMiddlewares := createUserAwareMiddlewaresForUploads(params.JWTSecret, params.RegistrySet.UserRegistry, params.RegistrySet)

		// Protected routes (authentication required)
		r.With(userMiddlewares...).Route("/system", System(params.RegistrySet.SettingsRegistry, params.DebugInfo, params.StartTime))
		r.With(userMiddlewares...).Route("/locations", Locations(params.RegistrySet.LocationRegistry))
		r.With(userMiddlewares...).Route("/areas", Areas(params.RegistrySet.AreaRegistry))
		r.With(userMiddlewares...).Route("/commodities", Commodities(params))
		r.With(userMiddlewares...).Route("/settings", Settings(params.RegistrySet.SettingsRegistry))
		r.With(userMiddlewares...).Route("/exports", Exports(params, restoreWorker))
		r.With(userMiddlewares...).Route("/files", Files(params))
		r.With(userMiddlewares...).Route("/search", Search(params.RegistrySet))
		r.With(userMiddlewares...).Route("/commodities/values", Values(params.RegistrySet))
		r.With(userMiddlewares...).Route("/debug", Debug(params))

		// Uploads need special middleware without content type restrictions
		r.With(userUploadMiddlewares...).Route("/uploads", Uploads(params))
	})

	// use Frontend as a root directory
	r.Handle("/*", FrontendHandler())

	return r
}

// RLSContextMiddleware sets the user and tenant context for RLS policies
func RLSContextMiddleware(registrySet *registry.Set) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context (set by JWTMiddleware)
			user := appctx.UserFromContext(r.Context())
			if user == nil {
				http.Error(w, "User context required", http.StatusInternalServerError)
				return
			}

			slog.Info("RLS Middleware: Setting context for user",
				"user_id", user.ID,
				"email", user.Email,
				"commodity_registry_type", fmt.Sprintf("%T", registrySet.CommodityRegistry))
			r = r.WithContext(appctx.WithUser(r.Context(), user))

			next.ServeHTTP(w, r)
		})
	}
}
