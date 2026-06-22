package apiserver

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	"github.com/denisvmedia/inventario/internal/backupsign"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register the in-memory + file blob drivers
	"github.com/denisvmedia/inventario/internal/metrics"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
	"github.com/denisvmedia/inventario/services/oauth"
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

// renderDecodeOnce guards the package-global `render.Decode` swap so it
// happens at most once across every APIServer() call. The previous
// per-call assignment was racy under `go test -race`: two parallel tests
// calling APIServer() concurrently both wrote to the same package
// variable, and the race detector flagged whichever pair of tests
// happened to be running when the second write landed.
//
// sync.Once gives the lazy "first-APIServer-call wins, the rest are
// no-ops" semantics without an init() side-effect at import time —
// packages that import apiserver but never call APIServer() (e.g. some
// table-driven tests of helper functions) leave render.Decode at its
// upstream default.
var renderDecodeOnce sync.Once

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

// createGroupAwareMiddlewaresForUploads — its sibling
// `createUserAwareMiddlewaresForUploads` was removed under #1421
// alongside the legacy upload routes / security tests it shipped for.
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
	FactorySet     *registry.FactorySet
	EntityService  *services.EntityService
	UploadLocation string
	// MaxUploadBytes caps the size of a single uploaded file (issue #2101).
	// The multipart upload path streams the body straight to the blob store
	// and bypasses the 1 MiB request-body cap in security_middleware.go, so
	// without this limit one user can exhaust the shared storage. Threaded
	// into uploadsAPI and enforced in saveFile via http.MaxBytesReader; a
	// breach renders 413. <= 0 disables the cap (back-compat / opt-out).
	MaxUploadBytes             int64
	DebugInfo                  *debug.Info
	StartTime                  time.Time
	JWTSecret                  []byte                             // JWT secret for user authentication
	FileSigningKey             []byte                             // File signing key for secure file URLs
	BackupSigner               *backupsign.Signer                 // Ed25519 signer for .inb backup archives (#534)
	FileURLExpiration          time.Duration                      // File URL expiration duration
	ThumbnailConfig            services.ThumbnailGenerationConfig // Thumbnail generation configuration
	TokenBlacklister           services.TokenBlacklister          // Token blacklist service (Redis or in-memory)
	AuthRateLimiter            services.AuthRateLimiter           // Auth rate limiter (Redis or in-memory)
	GlobalRateLimiter          services.GlobalRateLimiter         // Global API rate limiter (Redis or in-memory)
	GlobalRateTrustedProxyNets []*net.IPNet                       // Trusted proxies for extracting real client IP in global limiter
	CSRFService                csrf.Service                       // CSRF token service (Redis or in-memory)
	CORSConfig                 CORSConfig                         // CORS configuration for API routes
	TenantResolver             TenantResolver                     // resolves host → tenant; nil = single-tenant (HostTenantResolver with no BaseDomain)
	// TestTenantHeaderEnabled is the TEST-ONLY gate for the #1851 cross-tenant
	// e2e fixture: exempts the namespaced X-Inventario-Test-Tenant header from
	// the rejectTenantHeader scan AND lets the tenant resolver consult that
	// header instead of the Host. Wired through bootstrap from
	// INVENTARIO_RUN_TEST_TENANT_HEADER_ENABLED; defaults false. Never enable
	// in a production deployment — the bootstrap layer emits a loud warning
	// at startup when it is set.
	TestTenantHeaderEnabled bool
	EmailService            services.EmailService // Transactional email service (queue + providers)
	PublicURL               string                // Public base URL used in transactional links
	SupportEmail            string                // Destination for /api/v1/feedback submissions (issue #1387). Empty leaves the route mounted but it returns 503.
	RedisPinger             RedisPinger           // Optional Redis dependency check for /readyz

	// MetricsToken is the optional shared-secret bearer token for GET
	// /metrics (issue #2102). When non-empty, /metrics requires the header
	// "Authorization: Bearer <token>", compared with
	// crypto/subtle.ConstantTimeCompare. When empty, /metrics is open
	// (legacy behaviour) and APIServer() logs a one-time startup warning so
	// operators know the installation-wide business gauges are exposed.
	MetricsToken string

	// EnableAPIDocs gates GET /swagger/* (Swagger UI + doc.json) (issue
	// #2102 / L-5). Default true preserves dev / e2e; production sets it
	// false so the API surface is not served publicly. When false the
	// /swagger routes are not mounted (404).
	EnableAPIDocs bool

	// FeatureCurrencyMigration gates the /currency-migrations endpoints
	// and the requireGroupNotMigrating lock middleware (issue #202 / #1551).
	// Default true now that the feature shipped under #1604 — flipping
	// to false is the operator kill-switch and turns the endpoints into
	// 404s while the lock middleware no-ops. The Helm chart exposes this
	// via `features.currencyMigration`.
	FeatureCurrencyMigration bool

	// MagicLinkLoginEnabled is the effective gate for the passwordless
	// "magic link" sign-in flow. It is the AND of the config flag and a
	// non-stub email provider, computed in bootstrap — when false the two
	// /auth/magic-link routes return 404 and the magic_link_login feature
	// flag reads false so the FE hides the entry point.
	MagicLinkLoginEnabled bool

	// ImpersonationTTL is the lifetime of an admin impersonation session
	// (#1750). Operators tune it via INVENTARIO_RUN_IMPERSONATION_TTL;
	// zero falls back to the 30-min default and any value above 30 min
	// is clamped down inside the impersonation handler. A negative value
	// is rejected by Validate().
	ImpersonationTTL time.Duration

	// ImpersonationStore records the server-side return slots for active
	// impersonation sessions (#1750). When nil, APIServer() falls back to
	// an in-memory store — fine for single-replica deployments and tests.
	// The same single instance is threaded into both AuthParams and
	// AdminParams: `start` records a slot, `end`/`logout` read it, so they
	// MUST share one store. Injectable so a future Redis-backed
	// implementation can be wired without touching the apiserver.
	ImpersonationStore services.ImpersonationStore

	// CommodityScanService runs the AI vision photo-scan flow for the
	// Add Item dialog (#1720). When nil (or wrapping a nil provider)
	// the POST /commodities/scan endpoint stays mounted but always
	// returns 503 commodity_scan.provider_disabled — that's the
	// "feature off in this deployment" contract the FE branches on.
	CommodityScanService *services.CommodityScanService

	// CommodityScanMaxBodyBytes caps the entire multipart body for
	// the photo-scan endpoint. Computed at bootstrap from the configured
	// per-photo cap + max photo count; zero disables the cap so unit
	// tests can pass small fixtures without minding the limit.
	CommodityScanMaxBodyBytes int64

	// CommodityScanMaxPhotoBytes caps a single multipart part. Mirrors
	// AIVisionMaxPhotoBytes from config. A hostile request whose total
	// body stays inside CommodityScanMaxBodyBytes but whose individual
	// photo exceeds this cap is rejected with 413 +
	// commodity_scan.photo_too_large before the entire part is read
	// into memory.
	CommodityScanMaxPhotoBytes int

	// PublicScanEnabled gates the unauthenticated public photo-scan
	// endpoint (#1988) that backs the landing-page "add your first item"
	// CTA. Default FALSE: the endpoint spends real vendor tokens with no
	// auth wall, so it must be opted into explicitly. When false the
	// POST /public/commodities/scan route is NOT mounted (404) and the
	// public_scan feature flag reads false so the FE hides the CTA. The
	// route is also only mounted when CommodityScanService is non-nil.
	PublicScanEnabled bool

	// PublicScanRateLimiter enforces the public scan's per-IP and
	// global-daily caps (#1988). It is INTENTIONALLY separate from
	// AuthRateLimiter: the anonymous endpoint spends real vendor tokens, so
	// its cost/abuse backstop must stay enforced even when login throttling
	// is turned off via --no-auth-rate-limit (a test-only switch that swaps
	// AuthRateLimiter for a no-op). When nil the server falls back to
	// AuthRateLimiter so existing callers/tests keep working.
	PublicScanRateLimiter services.AuthRateLimiter

	// SeedEndpointEnabled gates the PUBLIC, UNAUTHENTICATED POST /seed
	// endpoint (#2039). Default FALSE: Seed runs a privileged,
	// RLS-bypassing service-registry operation (debug/seeddata), so an
	// anonymous caller could pollute the production tenant. It must be
	// opted into explicitly and MUST stay off in production. When false
	// the POST /api/v1/seed route is NOT mounted (404). It is enabled only
	// where seeding is intended: dev / e2e stacks and the throwaway
	// localhost server the Helm init-data Job boots to curl /seed.
	SeedEndpointEnabled bool

	// OAuthRegistry holds the third-party sign-in providers enabled in
	// this deployment (#1394). Empty means OAuth is unconfigured — the
	// /auth/oauth/providers endpoint surfaces an empty list and start /
	// callback paths return 404. OAuthStateSigner is the HMAC signer for
	// the per-request state tokens; required whenever any OAuth route
	// is mounted (so the apiserver builds a signer with a random fallback
	// key even when no provider is configured).
	OAuthRegistry    *oauth.Registry
	OAuthStateSigner *oauth.StateSigner
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
		validation.Field(&p.BackupSigner, validation.Required),                                   // Ed25519 signer for .inb backups (#534)
		validation.Field(&p.FileURLExpiration, validation.Required, validation.Min(time.Minute)), // Require at least 1 minute expiration
		// ImpersonationTTL (#1750): zero is allowed (falls back to the
		// 30-min default), but a negative duration would mint already-expired
		// sessions — reject it. Not Required, so zero passes the check.
		validation.Field(&p.ImpersonationTTL, validation.Min(time.Duration(0))),
	)

	if err := validation.ValidateStruct(p, fields...); err != nil {
		return err
	}

	// OAuth dependency set must be all-present or all-absent (#1394).
	// The /auth/oauth/* sub-router silently 404s on missing providers,
	// which is the correct shape when OAuth is disabled — but a caller
	// passing one or two of the three pieces gets the same silent 404
	// instead of a loud bootstrap failure that tells them which piece
	// is missing. Enforce the all-or-none invariant here so the
	// misconfiguration surfaces at bootstrap.
	hasRegistry := p.OAuthRegistry != nil
	hasSigner := p.OAuthStateSigner != nil
	hasIdentities := p.FactorySet != nil && p.FactorySet.OAuthIdentityRegistry != nil
	provided := 0
	if hasRegistry {
		provided++
	}
	if hasSigner {
		provided++
	}
	if hasIdentities {
		provided++
	}
	if provided != 0 && provided != 3 {
		missing := make([]string, 0, 3)
		if !hasRegistry {
			missing = append(missing, "OAuthRegistry")
		}
		if !hasSigner {
			missing = append(missing, "OAuthStateSigner")
		}
		if !hasIdentities {
			missing = append(missing, "FactorySet.OAuthIdentityRegistry")
		}
		return fmt.Errorf("oauth: dependency set is partially configured — missing: %s (must be all-present or all-absent)", strings.Join(missing, ", "))
	}

	return nil
}

// MetricsTokenMiddleware enforces optional bearer-token authentication on
// GET /metrics (issue #2102). When token is non-empty it requires the header
// "Authorization: Bearer <token>" and compares it with
// crypto/subtle.ConstantTimeCompare to keep the check constant-time. When the
// token is empty the middleware is a no-op, preserving the legacy "open
// /metrics" behaviour that keeps local development frictionless — the one-time
// "open metrics" warning is emitted by APIServer() at startup, not here.
func MetricsTokenMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Build the expected header once per construction, not per request.
		expected := []byte("Bearer " + token)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			got := []byte(r.Header.Get("Authorization"))
			// ConstantTimeCompare returns 0 on a length mismatch, so the
			// explicit length guard is only to keep the comparison's timing
			// independent of where the first differing byte falls.
			if len(got) != len(expected) || subtle.ConstantTimeCompare(got, expected) != 1 {
				slog.Warn("Unauthorized /metrics access attempt",
					"remote_addr", r.RemoteAddr,
					"user_agent", r.UserAgent(),
					"path", r.URL.Path,
				)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func APIServer(params Params, restoreStatus RestoreStatusQuerier) http.Handler {
	renderDecodeOnce.Do(func() {
		render.Decode = JSONAPIAwareDecoder
	})

	r := chi.NewRouter()
	// CORS middleware — strict and explicit origin-based policy.
	r.Use(NewCORSMiddleware(params.CORSConfig).Handler)

	// SECURITY: Add tenant ID validation middleware FIRST (before any other processing).
	// params.TestTenantHeaderEnabled is the #1851 e2e gate that exempts the
	// single namespaced X-Inventario-Test-Tenant header from the scan; it stays
	// false in any non-test deployment.
	r.Use(ValidateNoUserProvidedTenantID(params.TestTenantHeaderEnabled))
	r.Use(RejectSpecificTenantHeaders())

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	// RED metrics (#843): registered BEFORE Recoverer so it WRAPS it — the
	// deferred status read then observes the 500 that Recoverer writes for a
	// recovered panic. Installed inside Recoverer it would run its defer as
	// the panic unwinds, before the 500 is written, and miscount panics as
	// 2xx. It still sits inside chi routing, so RoutePattern resolves; it
	// self-skips "/metrics".
	r.Use(metrics.HTTPMiddleware)
	r.Use(middleware.Recoverer)

	// r.Get("/", func(w http.ResponseWriter, _r *http.Request) {
	//	w.Write([]byte("Welcome to Inventario!"))
	// })
	//
	// RESTy routes for "swagger" resource. Gated behind EnableAPIDocs
	// (issue #2102 / L-5): default on for dev/e2e, production sets it false
	// so the API documentation surface is not served publicly. When off the
	// routes are simply not mounted and 404 like any unknown path.
	if params.EnableAPIDocs {
		r.Mount("/swagger", swagger.Handler(
			swagger.URL("/swagger/doc.json"),
		))
	}
	r.Group(Health(params.FactorySet, params.RedisPinger))
	// GET /metrics (issue #2102): gated behind an optional bearer token. When
	// params.MetricsToken is empty the middleware is a no-op (legacy open
	// behaviour) and the one-time warning below fires so operators notice the
	// installation-wide gauges are exposed; when set, /metrics requires
	// "Authorization: Bearer <token>".
	if params.MetricsToken == "" {
		slog.Warn("GET /metrics is unauthenticated (no metrics token configured). " +
			"This is acceptable for local development but exposes installation-wide business gauges " +
			"(inventario_tenants/users/commodities/file_storage_bytes). " +
			"Set INVENTARIO_RUN_METRICS_TOKEN (or --metrics-token) to a strong random value to require bearer-token auth.")
	}
	r.With(MetricsTokenMiddleware(params.MetricsToken)).Method(http.MethodGet, "/metrics", promhttp.Handler())

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
	// The public scan's cost/abuse caps are deliberately independent of the
	// auth login limiter so --no-auth-rate-limit (a test-only switch) cannot
	// silently strip them. Fall back to the auth limiter only when a dedicated
	// one wasn't injected (older callers / tests that don't exercise the cap).
	publicScanRateLimiter := params.PublicScanRateLimiter
	if publicScanRateLimiter == nil {
		publicScanRateLimiter = rateLimiter
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
	// Enable EnsureDefaultGroup auto-promotion (#1592). Without this, the
	// service can't update users.default_group_id after CreateGroup /
	// AcceptInvite / RemoveMember.
	groupService.SetUserRegistry(params.FactorySet.UserRegistry)

	// The impersonation return-slot store (#1750) MUST be a single shared
	// instance: Admin()'s impersonation endpoints record/restore slots and
	// /auth/logout consults the SAME store to revoke the operator's genuine
	// refresh token when an impersonation session is ended via logout. Two
	// separate stores would leave logout unable to see the slot a `start`
	// recorded. Injectable via Params.ImpersonationStore so a shared
	// (Redis) implementation can be wired for multi-replica deployments;
	// in-memory is the default for single-replica deployments and tests.
	impersonationStore := params.ImpersonationStore
	if impersonationStore == nil {
		impersonationStore = services.NewInMemoryImpersonationStore()
	}

	r.Route("/api/v1", func(r chi.Router) {
		// Resolve tenant from request host and place it in context for all handlers,
		// including public ones (login, registration, password reset).
		r.Use(PublicTenantMiddleware(tenantResolver, params.FactorySet.TenantRegistry))

		// Auth routes have dedicated per-endpoint rate limiters (login, registration,
		// password-reset); applying the global per-IP limit here would lock users out
		// of the login page when the global budget is exhausted — the exact failure
		// mode described in issue #1208. Keep auth outside the global limiter.
		// MFA service derives its encryption subkey from the same root
		// signing secret the JWT helper uses (HKDF with a distinct
		// label, see secrets package). Returning the error here is
		// fatal — the bootstrap path already validates JWTSecret is
		// at least 32 bytes.
		mfaSvc, err := services.NewMFAService(params.JWTSecret)
		if err != nil {
			panic("init MFA service: " + err.Error())
		}
		// Self-service account deletion (#2147) reuses the per-group purge
		// machinery: the AccountDeletionService classifies the user's groups,
		// purges the private ones via GroupPurgeService, then drops the user's
		// auth rows + the users row.
		accountDeletionFileSvc := services.NewFileService(params.FactorySet, params.UploadLocation)
		accountDeletionSvc := services.NewAccountDeletionService(
			params.FactorySet,
			services.NewGroupPurgeService(params.FactorySet, accountDeletionFileSvc),
		)
		r.Route("/auth", Auth(AuthParams{
			UserRegistry:             params.FactorySet.UserRegistry,
			RefreshTokenRegistry:     params.FactorySet.RefreshTokenRegistry,
			GroupMembershipRegistry:  params.FactorySet.GroupMembershipRegistry,
			LoginEventRegistry:       params.FactorySet.LoginEventRegistry,
			MFARegistry:              params.FactorySet.UserMFASecretRegistry,
			SystemAdminGrantRegistry: params.FactorySet.SystemAdminGrantRegistry,
			BlacklistService:         blacklist,
			RateLimiter:              rateLimiter,
			CSRFService:              csrfSvc,
			AuditService:             auditSvc,
			JWTSecret:                params.JWTSecret,
			EmailService:             emailSvc,
			MFAService:               mfaSvc,
			OAuthRegistry:            params.OAuthRegistry,
			OAuthStateSigner:         params.OAuthStateSigner,
			OAuthIdentityRegistry:    params.FactorySet.OAuthIdentityRegistry,
			MagicLinkRegistry:        params.FactorySet.MagicLinkTokenRegistry,
			PublicBaseURL:            params.PublicURL,
			MagicLinkLoginEnabled:    params.MagicLinkLoginEnabled,
			UserPurger:               params.FactorySet.UserPurger,
			AccountDeletionService:   accountDeletionSvc,
		}))

		// Back-office auth plane (issue #1785, Phase 2). Completely
		// isolated from /auth/* — separate audience claim, separate
		// `admin_id` identifier, separate cookie name + path, separate
		// middleware. The same JWT secret is intentional: rotating the
		// secret invalidates everything, and the `aud` claim is the
		// real boundary. Mounted at /api/v1/backoffice/auth so the
		// cookie path `/api/v1/backoffice` covers every back-office
		// endpoint a future phase will add (Phase 3 will mount the
		// admin surface under the same prefix).
		r.Route("/backoffice/auth", BackofficeAuth(BackofficeAuthParams{
			BackofficeUserRegistry:          params.FactorySet.BackofficeUserRegistry,
			BackofficeRefreshTokenRegistry:  params.FactorySet.BackofficeRefreshTokenRegistry,
			BackofficeUserMFASecretRegistry: params.FactorySet.BackofficeUserMFASecretRegistry,
			// Reuse the tenant-plane MFAService — same HKDF subkey
			// label means a TOTP secret encrypted by either plane's CLI
			// is decryptable by either plane's verifier (no key drift),
			// which keeps the operational surface uniform across the
			// two MFA tables.
			MFAService:       mfaSvc,
			BlacklistService: blacklist,
			RateLimiter:      rateLimiter,
			AuditService:     auditSvc,
			JWTSecret:        params.JWTSecret,
			// Public URL drives the Origin/Referer CSRF check on
			// /backoffice/auth/refresh (#2113, L-1). Empty leaves the
			// check off (dev / single-host setups rely on SameSite=Strict).
			PublicURL: params.PublicURL,
		}))

		// Create user aware middlewares for protected routes. Defined here
		// (before the public group below) because /invites — whose
		// POST /invites/{token}/accept leg is authenticated — is mounted
		// inside the global-rate-limit group so its public GET leg is also
		// rate-limited (issue #2113, L-3).
		userMiddlewares := createUserAwareMiddlewares(params.JWTSecret, params.FactorySet, blacklist, csrfSvc)

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
			// Feature flags are deployment-scoped (operator kill-switches,
			// #1604) so the FE reads them once at boot — including before
			// login, to hide entry points for features whose backend is
			// gated off (#1616). Hence: unauthenticated, behind the same
			// global rate limit as the other public reads.
			r.Route("/feature-flags", FeatureFlagsHandler(params))
			// Public, unauthenticated AI photo-scan (#1988) backing the
			// landing-page "add your first item" CTA. Mounted ONLY when the
			// operator has opted in (PublicScanEnabled) AND a scan service
			// is wired — otherwise the route stays absent (404) so an
			// anonymous endpoint that spends real vendor tokens is never
			// exposed by default. It sits here, OUTSIDE the JWT / RLS /
			// registry / group middleware, with its own per-IP + global-
			// daily limiter layered on top of the global per-IP budget the
			// surrounding group already applies.
			if params.PublicScanEnabled && params.CommodityScanService != nil {
				r.With(PublicScanRateLimitMiddleware(publicScanRateLimiter)).Route(
					"/public/commodities/scan",
					CommodityScanPublic(params.CommodityScanService, params.CommodityScanMaxBodyBytes, params.CommodityScanMaxPhotoBytes),
				)
			}
			// Seed endpoint (#2039). Mounted ONLY when the operator opts in
			// via SeedEndpointEnabled — otherwise the route stays absent
			// (404). Seed uses a SERVICE registry set, so it is a privileged,
			// RLS-bypassing operation with no auth wall: leaving it public by
			// default lets an anonymous caller pollute the production tenant.
			// It is enabled only where seeding is intended (dev / e2e stacks
			// and the throwaway localhost server the Helm init-data Job boots);
			// production app servers leave it off.
			if params.SeedEndpointEnabled {
				r.With(defaultAPIMiddlewares...).Route("/seed", Seed(params.FactorySet, params.UploadLocation))
			}
			// Invites are mounted WITHOUT userMiddlewares so that
			// GET /invites/{token} remains public (the invitee is typically
			// unauthenticated at first). It lives inside this global
			// rate-limit group (issue #2113, L-3) so the unauthenticated
			// token-lookup leg gets per-IP rate limiting as defence-in-depth
			// against invite-token enumeration. POST /invites/{token}/accept
			// is wrapped with the userMiddlewares chain inside the Invites
			// router itself.
			r.Route("/invites", Invites(groupService, userMiddlewares))
		})

		// Protected routes (authentication required).
		// Authenticated users are not subject to the global per-IP rate limit; a
		// valid JWT already proves legitimacy and the SPA issues several API calls
		// per page navigation, making the global budget easy to exhaust legitimately.
		// Note: RegistrySetMiddleware creates user-aware registries and adds them to context.
		// System requires a settings registry.
		// Non-group-scoped routes (system, debug, users, groups management)
		r.With(userMiddlewares...).Route("/system", System(params.DebugInfo, params.StartTime))
		// GET /debug moved to /admin/debug behind the back-office plane
		// (issue #2113, L-4): it leaks operational config (storage/db driver,
		// OS) and must not be readable by an ordinary tenant user. Wired into
		// AdminParams.DebugInfo below.
		// Backup-signing public key (#534): server-global, any authenticated
		// user may read the public half so the FE / CLI can verify .inb archives.
		r.With(userMiddlewares...).Route("/backup", BackupSigning(params))
		// Platform-administrative subtree (#1745 foundation; #1744 umbrella).
		// Every admin route — including the cross-plane impersonation
		// start/end/current trio added by #1785 Phase 5 — is gated by the
		// back-office auth plane: RequireBackofficeAuth validates a
		// `aud=backoffice` JWT and rejects any tenant token, and
		// /current widens the gate to also admit an active impersonation
		// token so the FE banner can read its own state. Mounting at the
		// same level as /system keeps the surface tenant-agnostic.
		r.Route("/admin", Admin(AdminParams{
			FactorySet:             params.FactorySet,
			BackofficeUserRegistry: params.FactorySet.BackofficeUserRegistry,
			Blacklist:              blacklist,
			AuditService:           auditSvc,
			GroupService:           groupService,
			JWTSecret:              params.JWTSecret,
			RateLimiter:            rateLimiter,
			CSRFService:            csrfSvc,
			ImpersonationTTL:       params.ImpersonationTTL,
			// ImpersonationStore is the SAME instance /auth/logout
			// receives, so logout can revoke the operator's genuine
			// refresh token when an impersonation session is ended via
			// logout rather than POST /admin/impersonation/end (#1750).
			ImpersonationStore: impersonationStore,
			// #2113 L-4: GET /admin/debug now lives on the back-office plane.
			DebugInfo: params.DebugInfo,
		}))
		// The former /api/v1/users admin CRUD was removed together with the
		// tenant-level `users.role` column. Per-group user management lives
		// under /groups/{id}/members; a tenant-wide admin surface will be
		// re-introduced only when group-based admin authorization is designed.
		r.With(userMiddlewares...).Route("/groups", Groups(params, groupService, auditSvc))
		// Per-user account surfaces (#1644): active sessions + login history.
		// These don't fit under /auth (which is unauth-tolerant on purpose)
		// nor under /g/{slug}/* (they're tenant-scoped, not group-scoped).
		r.With(userMiddlewares...).Route("/users/me", UsersMe(UsersMeParams{
			RefreshTokenRegistry: params.FactorySet.RefreshTokenRegistry,
			LoginEventRegistry:   params.FactorySet.LoginEventRegistry,
		}))
		// In-app feedback / contact support (#1387). Auth-required and
		// per-user rate-limited (5/hour). The handler returns a typed 503
		// (feedback.not_configured) when SupportEmail is empty, so it stays
		// mounted in the "feedback not configured" deployment shape too —
		// the FE can rely on a stable URL and surface a "feedback isn't
		// configured" toast (not the global /maintenance bounce) when the
		// typed 503 comes back.
		r.With(userMiddlewares...).Route("/feedback", Feedback(FeedbackParams{
			EmailService: emailSvc,
			SupportEmail: params.SupportEmail,
		}, rateLimiter))
		// (/invites is mounted above, inside the global rate-limit group —
		// issue #2113, L-3.)

		// Group-scoped data routes: /api/v1/g/{groupSlug}/...
		// GroupSlugResolverMiddleware runs BEFORE RegistrySetMiddleware so the
		// registry set is built with group context.
		groupScopedMiddlewares := createGroupAwareMiddlewares(params.JWTSecret, params.FactorySet, blacklist, csrfSvc, groupService)
		// Per-resource role gates (#1533): reads stay viewer+ (membership
		// is checked by GroupSlugResolverMiddleware). Writes split by
		// resource — structural resources (locations, areas, exports)
		// require admin+; content resources (commodities, files, tags,
		// loans, services) require user+. Each gate is method-conditional
		// — GET/HEAD/OPTIONS bypass the role check.
		structuralWriteGate := requireGroupRoleForWrite(groupService, models.GroupRoleAdmin)
		contentWriteGate := requireGroupRoleForWrite(groupService, models.GroupRoleUser)
		r.With(groupScopedMiddlewares...).Route("/g/{groupSlug}", func(r chi.Router) {
			r.With(structuralWriteGate).Route("/locations", Locations(params))
			r.With(structuralWriteGate).Route("/areas", Areas(params))
			// Commodity write paths are guarded by requireGroupNotMigrating
			// (issue #202 §3.2) so an in-flight currency migration locks
			// concurrent edits with HTTP 423. The middleware is a no-op
			// when the feature flag is off.
			r.With(
				requireGroupNotMigrating(GroupMigrationLockOptions{FeatureEnabled: params.FeatureCurrencyMigration}),
				contentWriteGate,
			).Route("/commodities", Commodities(params))
			r.With(contentWriteGate).Route("/files", Files(params))
			r.With(contentWriteGate).Route("/tags", Tags(params))
			r.With(contentWriteGate).Route("/loans", GroupLoans(params))
			r.With(contentWriteGate).Route("/services", GroupServices(params))
			r.With(contentWriteGate).Route("/maintenance", GroupMaintenance(params))
			r.With(structuralWriteGate).Route("/exports", Exports(params, restoreStatus))
			r.Route("/settings", Settings())
			r.Route("/commodities/values", Values())
			r.Route("/upload-slots", UploadSlots(params.FactorySet))
			r.Route("/search", Search(params.EntityService))
			// Currency-migration endpoints are always mounted so swagger
			// stays consistent regardless of flag state. Each handler
			// returns 404 when params.FeatureCurrencyMigration is false,
			// keeping the surface inert in production (#202 §8) until
			// the operator flips the flag on.
			r.Route("/currency-migrations", CurrencyMigrations(params, groupService, auditSvc))
			r.Route("/storage-usage", StorageUsage())
			r.Route("/plan", GroupPlan())
			r.Route("/notifications", GroupNotifications(params.FactorySet))
		})

		// Uploads need special middleware without content type restrictions (group-scoped).
		// GroupSlugResolverMiddleware runs BEFORE RegistrySetMiddleware so the
		// registry set is built with group context.
		groupUploadMiddlewares := createGroupAwareMiddlewaresForUploads(params.JWTSecret, params.FactorySet.UserRegistry, params.FactorySet, blacklist, csrfSvc, groupService)
		r.With(groupUploadMiddlewares...).Route("/g/{groupSlug}/uploads", Uploads(params))

		// AI vision photo-scan endpoint (#1720). Bypasses the default
		// JSON:API content-type guards because it accepts
		// multipart/form-data; otherwise the middleware chain matches
		// the group-scoped content tree (JWT → RLS → group → registry
		// → CSRF) plus the role gate that POST /commodities itself
		// applies, since this is effectively a "prepare an Add Item"
		// affordance.
		contentWriteGateScan := requireGroupRoleForWrite(groupService, models.GroupRoleUser)
		r.With(append(groupUploadMiddlewares, contentWriteGateScan)...).Route(
			"/g/{groupSlug}/commodities/scan",
			CommodityScan(params.CommodityScanService, params.CommodityScanMaxBodyBytes, params.CommodityScanMaxPhotoBytes),
		)

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
