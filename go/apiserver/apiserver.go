package apiserver

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"
	swagger "github.com/swaggo/http-swagger/v2"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob" // register azureblob driver
	// _ "gocloud.dev/blob/fileblob"  // register fileblob driver
	_ "gocloud.dev/blob/gcsblob" // register gcsblob driver
	_ "gocloud.dev/blob/memblob" // register memblob driver
	_ "gocloud.dev/blob/s3blob"  // register s3blob driver

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/csrf"
	"github.com/denisvmedia/inventario/debug"
	_ "github.com/denisvmedia/inventario/docs" // register swagger docs
	_ "github.com/denisvmedia/inventario/internal/fileblob"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// RestoreStatusQuerier reports the aggregate status of restore operations
// without requiring a running worker goroutine. It lets the HTTP API enforce
// the "one active restore at a time" invariant in deployments where the
// RestoreWorker runs in a separate process.
type RestoreStatusQuerier interface {
	HasRunningRestores(ctx context.Context) (bool, error) // Returns true if any restore is running or pending
}

type ctxValueKey string

const registrySetCtxKey ctxValueKey = "registrySet"

var defaultAPIMiddlewares = []func(http.Handler) http.Handler{
	defaultRequestContentType("application/vnd.api+json"),
	middleware.AllowContentType("application/json", "application/vnd.api+json"),
}

// createUserAwareMiddlewares creates middleware stack with user authentication and RLS context.
// For non-group-scoped routes. Group-scoped routes need GroupSlugResolverMiddleware
// inserted BEFORE RegistrySetMiddleware (see createGroupAwareMiddlewares).
func createUserAwareMiddlewares(jwtSecret []byte, factorySet *registry.FactorySet, blacklist services.TokenBlacklister, csrfService csrf.Service) []func(http.Handler) http.Handler {
	return append(defaultAPIMiddlewares,
		JWTMiddleware(jwtSecret, factorySet.UserRegistry, blacklist),
		RLSContextMiddleware(factorySet),
		RegistrySetMiddleware(factorySet),
		CSRFMiddleware(csrfService),
	)
}

// createGroupAwareMiddlewares creates middleware stack for group-scoped data routes.
// GroupSlugResolverMiddleware runs BEFORE RegistrySetMiddleware so the registry set
// is built with group context already set.
func createGroupAwareMiddlewares(jwtSecret []byte, factorySet *registry.FactorySet, blacklist services.TokenBlacklister, csrfService csrf.Service, groupService *services.GroupService) []func(http.Handler) http.Handler {
	return append(defaultAPIMiddlewares,
		JWTMiddleware(jwtSecret, factorySet.UserRegistry, blacklist),
		RLSContextMiddleware(factorySet),
		GroupSlugResolverMiddleware(groupService),
		RegistrySetMiddleware(factorySet),
		CSRFMiddleware(csrfService),
	)
}

// createUserAwareMiddlewaresForUploads creates middleware stack for uploads (without content type restrictions).
func createUserAwareMiddlewaresForUploads(jwtSecret []byte, userRegistry registry.UserRegistry, factorySet *registry.FactorySet, blacklist services.TokenBlacklister, csrfService csrf.Service) []func(http.Handler) http.Handler {
	// Only add user authentication and RLS context, no content type restrictions for uploads
	return []func(http.Handler) http.Handler{
		JWTMiddleware(jwtSecret, userRegistry, blacklist),
		RLSContextMiddleware(factorySet),
		RegistrySetMiddleware(factorySet),
		CSRFMiddleware(csrfService),
	}
}

// createGroupAwareMiddlewaresForUploads is like createUserAwareMiddlewaresForUploads
// but inserts GroupSlugResolverMiddleware before RegistrySetMiddleware.
func createGroupAwareMiddlewaresForUploads(
	jwtSecret []byte,
	userRegistry registry.UserRegistry,
	factorySet *registry.FactorySet,
	blacklist services.TokenBlacklister,
	csrfService csrf.Service,
	groupService *services.GroupService,
) []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		JWTMiddleware(jwtSecret, userRegistry, blacklist),
		RLSContextMiddleware(factorySet),
		GroupSlugResolverMiddleware(groupService),
		RegistrySetMiddleware(factorySet),
		CSRFMiddleware(csrfService),
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

// paginate is a stub middleware for pagination.
// Actual pagination is handled directly in each handler using parsePagination and setPaginationHeaders.
func paginate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// setPaginationHeaders sets standard pagination response headers.
func setPaginationHeaders(w http.ResponseWriter, page, perPage, total int) {
	w.Header().Set("X-Page", strconv.Itoa(page))
	w.Header().Set("X-Per-Page", strconv.Itoa(perPage))
	w.Header().Set("X-Total", strconv.Itoa(total))
	w.Header().Set("X-Total-Pages", strconv.Itoa(jsonapi.ComputeTotalPages(total, perPage)))
}

// parsePagination parses page and per_page query strings and returns safe defaults.
// Default: page=1, per_page=50, max per_page=100.
func parsePagination(pageStr, perPageStr string) (page, perPage int) {
	page = 1
	perPage = 50
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
		perPage = pp
	}
	return page, perPage
}

type Params struct {
	FactorySet                 *registry.FactorySet
	EntityService              *services.EntityService
	UploadLocation             string
	DebugInfo                  *debug.Info
	StartTime                  time.Time
	JWTSecret                  []byte                             // JWT secret for user authentication
	FileSigningKey             []byte                             // File signing key for secure file URLs
	FileURLExpiration          time.Duration                      // File URL expiration duration
	ThumbnailConfig            services.ThumbnailGenerationConfig // Thumbnail generation configuration
	TokenBlacklister           services.TokenBlacklister          // Token blacklist service (Redis or in-memory)
	AuthRateLimiter            services.AuthRateLimiter           // Auth rate limiter (Redis or in-memory)
	GlobalRateLimiter          services.GlobalRateLimiter         // Global API rate limiter (Redis or in-memory)
	GlobalRateTrustedProxyNets []*net.IPNet                       // Trusted proxies for extracting real client IP in global limiter
	CSRFService                csrf.Service                       // CSRF token service (Redis or in-memory)
	CORSConfig                 CORSConfig                         // CORS configuration for API routes
	TenantResolver             TenantResolver                     // resolves host → tenant; nil = single-tenant (HostTenantResolver with no BaseDomain)
	RegistrationMode           models.RegistrationMode            // Registration mode: open, approval, or closed
	EmailService               services.EmailService              // Transactional email service (queue + providers)
	PublicURL                  string                             // Public base URL used in transactional links
	RedisPinger                RedisPinger                        // Optional Redis dependency check for /readyz
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
		validation.Field(&p.JWTSecret, validation.Required, validation.Length(32, 0)),            // Require at least 32 bytes for security
		validation.Field(&p.FileSigningKey, validation.Required, validation.Length(32, 0)),       // Require at least 32 bytes for security
		validation.Field(&p.FileURLExpiration, validation.Required, validation.Min(time.Minute)), // Require at least 1 minute expiration
	)

	return validation.ValidateStruct(p, fields...)
}

func APIServer(params Params, restoreStatus RestoreStatusQuerier) http.Handler {
	render.Decode = JSONAPIAwareDecoder

	r := chi.NewRouter()
	// CORS middleware — strict and explicit origin-based policy.
	r.Use(NewCORSMiddleware(params.CORSConfig).Handler)

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
	r.Group(Health(params.FactorySet, params.RedisPinger))

	// Resolve blacklister: default to in-memory if not provided.
	blacklist := params.TokenBlacklister
	if blacklist == nil {
		slog.Warn("TokenBlacklister not provided; falling back to in-memory implementation. This configuration is not suitable for production use.")
		blacklist = services.NewInMemoryTokenBlacklister()
	}

	// Resolve auth rate limiter: default to in-memory if not provided.
	rateLimiter := params.AuthRateLimiter
	if rateLimiter == nil {
		slog.Warn("AuthRateLimiter not provided; falling back to in-memory implementation. This configuration is not suitable for production use.")
		rateLimiter = services.NewInMemoryAuthRateLimiter()
	}
	globalRateLimiter := params.GlobalRateLimiter
	if globalRateLimiter == nil {
		slog.Warn("GlobalRateLimiter not provided; falling back to in-memory implementation. This configuration is not suitable for production use.")
		globalRateLimiter = services.NewInMemoryGlobalRateLimiter(1000, time.Hour)
	}

	// Use CSRF service from params (nil disables CSRF validation — see CSRFMiddleware).
	// In production, run.go always provides a concrete implementation.
	csrfSvc := params.CSRFService

	// Create a shared audit service for use across multiple routes.
	auditSvc := services.NewAuditService(params.FactorySet.AuditLogRegistry)

	emailSvc := params.EmailService
	if emailSvc == nil {
		slog.Warn("EmailService not provided; falling back to stub email service")
		emailSvc = services.NewStubEmailService()
	}

	// Resolve tenant resolver: default to single-tenant mode if not provided.
	tenantResolver := params.TenantResolver
	if tenantResolver == nil {
		tenantResolver = &HostTenantResolver{}
	}

	groupService := services.NewGroupService(
		params.FactorySet.LocationGroupRegistry,
		params.FactorySet.GroupMembershipRegistry,
		params.FactorySet.GroupInviteRegistry,
	)

	r.Route("/api/v1", func(r chi.Router) {
		// Resolve tenant from request host and place it in context for all handlers,
		// including public ones (login, registration, password reset).
		r.Use(PublicTenantMiddleware(tenantResolver, params.FactorySet.TenantRegistry))

		// Auth routes have dedicated per-endpoint rate limiters (login, registration,
		// password-reset); applying the global per-IP limit here would lock users out
		// of the login page when the global budget is exhausted — the exact failure
		// mode described in issue #1208. Keep auth outside the global limiter.
		r.Route("/auth", Auth(AuthParams{
			UserRegistry:            params.FactorySet.UserRegistry,
			RefreshTokenRegistry:    params.FactorySet.RefreshTokenRegistry,
			GroupMembershipRegistry: params.FactorySet.GroupMembershipRegistry,
			BlacklistService:        blacklist,
			RateLimiter:             rateLimiter,
			CSRFService:             csrfSvc,
			AuditService:            auditSvc,
			JWTSecret:               params.JWTSecret,
			EmailService:            emailSvc,
		}))

		// Unauthenticated public routes: apply the global per-IP rate limit as a
		// defence-in-depth layer on top of their dedicated rate limiters.
		r.Group(func(r chi.Router) {
			r.Use(GlobalRateLimitMiddleware(globalRateLimiter, params.GlobalRateTrustedProxyNets))
			r.Group(Registration(RegistrationParams{
				UserRegistry:         params.FactorySet.UserRegistry,
				VerificationRegistry: params.FactorySet.EmailVerificationRegistry,
				EmailService:         emailSvc,
				AuditService:         auditSvc,
				RateLimiter:          rateLimiter,
				GroupService:         groupService,
				RegistrationMode:     params.RegistrationMode,
				PublicBaseURL:        params.PublicURL,
			}))
			r.Group(PasswordReset(PasswordResetParams{
				UserRegistry:          params.FactorySet.UserRegistry,
				PasswordResetRegistry: params.FactorySet.PasswordResetRegistry,
				RefreshTokenRegistry:  params.FactorySet.RefreshTokenRegistry,
				BlacklistService:      blacklist,
				EmailService:          emailSvc,
				AuditService:          auditSvc,
				RateLimiter:           rateLimiter,
				PublicBaseURL:         params.PublicURL,
			}))
			r.Route("/currencies", Currencies())
			// Seed endpoint is public for e2e testing and development.
			// Seed uses a service registry set since it's a privileged operation in dev/test.
			r.With(defaultAPIMiddlewares...).Route("/seed", Seed(params.FactorySet))
		})

		// Create user aware middlewares for protected routes
		userMiddlewares := createUserAwareMiddlewares(params.JWTSecret, params.FactorySet, blacklist, csrfSvc)

		// Protected routes (authentication required).
		// Authenticated users are not subject to the global per-IP rate limit; a
		// valid JWT already proves legitimacy and the SPA issues several API calls
		// per page navigation, making the global budget easy to exhaust legitimately.
		// Note: RegistrySetMiddleware creates user-aware registries and adds them to context.
		// System requires a settings registry.
		// Non-group-scoped routes (system, debug, users, groups management)
		r.With(userMiddlewares...).Route("/system", System(params.DebugInfo, params.StartTime))
		r.With(userMiddlewares...).Route("/debug", Debug(params))
		// The former /api/v1/users admin CRUD was removed together with the
		// tenant-level `users.role` column. Per-group user management lives
		// under /groups/{id}/members; a tenant-wide admin surface will be
		// re-introduced only when group-based admin authorization is designed.
		r.With(userMiddlewares...).Route("/groups", Groups(params, groupService))
		// Invites are mounted WITHOUT userMiddlewares so that GET /invites/{token}
		// remains public (the invitee is typically unauthenticated at first).
		// POST /invites/{token}/accept is wrapped with the userMiddlewares chain
		// inside the Invites router itself.
		r.Route("/invites", Invites(groupService, userMiddlewares))

		// Group-scoped data routes: /api/v1/g/{groupSlug}/...
		// GroupSlugResolverMiddleware runs BEFORE RegistrySetMiddleware so the
		// registry set is built with group context.
		groupScopedMiddlewares := createGroupAwareMiddlewares(params.JWTSecret, params.FactorySet, blacklist, csrfSvc, groupService)
		r.With(groupScopedMiddlewares...).Route("/g/{groupSlug}", func(r chi.Router) {
			r.Route("/locations", Locations(params))
			r.Route("/areas", Areas())
			r.Route("/commodities", Commodities(params))
			r.Route("/files", Files(params))
			r.Route("/exports", Exports(params, restoreStatus))
			r.Route("/settings", Settings())
			r.Route("/commodities/values", Values())
			r.Route("/upload-slots", UploadSlots(params.FactorySet))
			r.Route("/search", Search(params.EntityService))
		})

		// Uploads need special middleware without content type restrictions (group-scoped).
		// GroupSlugResolverMiddleware runs BEFORE RegistrySetMiddleware so the
		// registry set is built with group context.
		groupUploadMiddlewares := createGroupAwareMiddlewaresForUploads(params.JWTSecret, params.FactorySet.UserRegistry, params.FactorySet, blacklist, csrfSvc, groupService)
		r.With(groupUploadMiddlewares...).Route("/g/{groupSlug}/uploads", Uploads(params))

		// File downloads use signed URL validation instead of JWT authentication
		fileSigningService := services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration)
		signedURLMiddleware := SignedURLMiddleware(
			fileSigningService,
			params.FactorySet.UserRegistry,
			params.FactorySet.FileRegistryFactory,
			params.FactorySet.LocationGroupRegistry,
		)
		r.With(signedURLMiddleware, RLSContextMiddleware(params.FactorySet), RegistrySetMiddleware(params.FactorySet)).Route("/files/download", SignedFiles(params))
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
