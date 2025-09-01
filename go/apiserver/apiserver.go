package apiserver

import (
	"context"
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

const registrySetCtxKey ctxValueKey = "registrySet"

var defaultAPIMiddlewares = []func(http.Handler) http.Handler{
	defaultRequestContentType("application/vnd.api+json"),
	middleware.AllowContentType("application/json", "application/vnd.api+json"),
}

// createUserAwareMiddlewares creates middleware stack with user authentication and RLS context
func createUserAwareMiddlewares(jwtSecret []byte, factorySet *registry.FactorySet) []func(http.Handler) http.Handler {
	return append(defaultAPIMiddlewares,
		JWTMiddleware(jwtSecret, factorySet.UserRegistry),
		RLSContextMiddleware(factorySet),
		RegistrySetMiddleware(factorySet),
	)
}

// createUserAwareMiddlewaresForUploads creates middleware stack for uploads (without content type restrictions)
func createUserAwareMiddlewaresForUploads(jwtSecret []byte, userRegistry registry.UserRegistry, factorySet *registry.FactorySet) []func(http.Handler) http.Handler {
	// Only add user authentication and RLS context, no content type restrictions for uploads
	return []func(http.Handler) http.Handler{
		JWTMiddleware(jwtSecret, userRegistry),
		RLSContextMiddleware(factorySet),
		RegistrySetMiddleware(factorySet),
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
	FactorySet     *registry.FactorySet
	EntityService  *services.EntityService
	UploadLocation string
	DebugInfo      *debug.Info
	StartTime      time.Time
	JWTSecret      []byte // JWT secret for user authentication
}

func (p *Params) Validate() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&p.FactorySet, validation.Required),
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
		r.Route("/auth", Auth(params.FactorySet.UserRegistry, params.JWTSecret))
		r.Route("/currencies", Currencies())
		// Seed endpoint is public for e2e testing and development
		// Seed uses a service registry set since it's a privileged operation in dev/test
		r.With(defaultAPIMiddlewares...).Route("/seed", Seed(params.FactorySet))

		// Create user aware middlewares for protected routes
		userMiddlewares := createUserAwareMiddlewares(params.JWTSecret, params.FactorySet)
		userUploadMiddlewares := createUserAwareMiddlewaresForUploads(params.JWTSecret, params.FactorySet.UserRegistry, params.FactorySet)

		// Protected routes (authentication required)
		// Note: RegistrySetMiddleware creates user-aware registries and adds them to context
		// System requires a settings registry
		r.With(userMiddlewares...).Route("/system", System(params.DebugInfo, params.StartTime))
		locReg := params.FactorySet.LocationRegistryFactory.CreateServiceRegistry()
		r.With(userMiddlewares...).Route("/locations", Locations(locReg))
		areaReg := params.FactorySet.AreaRegistryFactory.CreateServiceRegistry()
		r.With(userMiddlewares...).Route("/areas", Areas(areaReg))
		r.With(userMiddlewares...).Route("/commodities", Commodities(params))
		userSettingsReg := params.FactorySet.SettingsRegistryFactory.CreateServiceRegistry()
		r.With(userMiddlewares...).Route("/settings", Settings(userSettingsReg))
		r.With(userMiddlewares...).Route("/exports", Exports(params, restoreWorker))
		r.With(userMiddlewares...).Route("/files", Files(params))
		r.With(userMiddlewares...).Route("/search", Search(params.EntityService))
		r.With(userMiddlewares...).Route("/commodities/values", Values())
		r.With(userMiddlewares...).Route("/debug", Debug(params))

		// Uploads need special middleware without content type restrictions
		r.With(userUploadMiddlewares...).Route("/uploads", Uploads(params))
	})

	// use Frontend as a root directory
	r.Handle("/*", FrontendHandler())

	return r
}

// RLSContextMiddleware validates user context for RLS security
// This middleware ensures that user context is properly set and validates security requirements
// The actual database RLS context is set at the transaction level in repository operations
func RLSContextMiddleware(factorySet *registry.FactorySet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context (set by JWTMiddleware)
			user := appctx.UserFromContext(r.Context())
			if user == nil {
				slog.Error("RLS Security Violation: No user context found",
					"method", r.Method,
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
					"user_agent", r.UserAgent())
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			// Validate user has required fields for RLS
			if user.ID == "" {
				slog.Error("RLS Security Violation: User ID is empty",
					"method", r.Method,
					"path", r.URL.Path,
					"user_email", user.Email,
					"remote_addr", r.RemoteAddr)
				http.Error(w, "Invalid user context", http.StatusUnauthorized)
				return
			}

			if user.TenantID == "" {
				slog.Error("RLS Security Violation: Tenant ID is empty",
					"method", r.Method,
					"path", r.URL.Path,
					"user_id", user.ID,
					"user_email", user.Email,
					"remote_addr", r.RemoteAddr)
				http.Error(w, "Invalid tenant context", http.StatusUnauthorized)
				return
			}

			// Validate user is active
			if !user.IsActive {
				slog.Error("RLS Security Violation: Inactive user attempted access",
					"method", r.Method,
					"path", r.URL.Path,
					"user_id", user.ID,
					"user_email", user.Email,
					"remote_addr", r.RemoteAddr)
				http.Error(w, "User account disabled", http.StatusForbidden)
				return
			}

			// Log successful security validation for monitoring
			slog.Debug("RLS Security: User context validated",
				"user_id", user.ID,
				"tenant_id", user.TenantID,
				"user_email", user.Email,
				"method", r.Method,
				"path", r.URL.Path)

			// Context is already set by JWTMiddleware, but ensure it's properly propagated
			// The actual database RLS context will be set when repositories create transactions
			next.ServeHTTP(w, r)
		})
	}
}

// RegistrySetMiddleware creates a user-aware registry set and adds it to the context
func RegistrySetMiddleware(factorySet *registry.FactorySet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create user-aware registry set from factory set
			registrySet, err := factorySet.CreateUserRegistrySet(r.Context())
			if err != nil {
				slog.Error("Failed to create user registry set", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// Add registry set to context for route handlers
			ctx := context.WithValue(r.Context(), registrySetCtxKey, registrySet)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RegistrySetFromContext extracts the registry set from the context
func RegistrySetFromContext(ctx context.Context) *registry.Set {
	if registrySet, ok := ctx.Value(registrySetCtxKey).(*registry.Set); ok {
		return registrySet
	}
	return nil
}
