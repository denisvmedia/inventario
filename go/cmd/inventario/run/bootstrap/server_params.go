package bootstrap

import (
	"errors"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/internal/aivision"
	_ "github.com/denisvmedia/inventario/internal/aivision/anthropic" // register the anthropic provider via init()
	_ "github.com/denisvmedia/inventario/internal/aivision/mock"      // register the mock provider via init()
	_ "github.com/denisvmedia/inventario/internal/aivision/openai"    // register the openai provider via init()
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// serverSetup aggregates everything produced by buildServerParams: the
// apiserver.Params (consumed by apiserver.APIServer), the email lifecycle, and
// a closer for any Redis readiness clients allocated while building the params.
type serverSetup struct {
	params                    apiserver.Params
	emailLifecycle            EmailServiceLifecycle
	closeReadinessRedisPinger func()
}

// applyCORSConfig parses the configured allowed-origins list and falls
// back to dev defaults when running against the memory backend with no
// origins configured. Extracted from buildServerParams to keep the
// parent function's cyclomatic complexity inside the linter budget.
func applyCORSConfig(cfg *Config, dsn string, params *apiserver.Params) error {
	params.CORSConfig = apiserver.DefaultCORSConfig()
	origins, err := apiserver.ParseAllowedOrigins(cfg.AllowedOrigins)
	if err != nil {
		slog.Error("Failed to parse allowed CORS origins", "error", err)
		return err
	}
	params.CORSConfig.AllowedOrigins = origins
	if len(origins) > 0 {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(dsn)), "memory://") {
		params.CORSConfig.AllowedOrigins = apiserver.DefaultDevAllowedOrigins()
		slog.Warn("No CORS origins explicitly configured; using local development defaults in memory-db mode. Set --allowed-origins for custom values.")
		return nil
	}
	slog.Warn("No CORS origins explicitly configured; cross-origin requests are denied. Set --allowed-origins to allow specific origins.")
	return nil
}

// buildServerParams constructs apiserver.Params from cfg + the resolved registry
// factorySet. On any failure it releases the Redis readiness clients it
// allocated locally so the caller never observes a partial state. On success
// the close function is returned in serverSetup and lifetime ownership moves to
// the caller.
func buildServerParams(cfg *Config, factorySet *registry.FactorySet, dsn string) (_ serverSetup, err error) {
	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: cfg.UploadLocation,
		StartTime:      time.Now(),
	}
	params.EntityService = services.NewEntityService(factorySet, params.UploadLocation)
	params.DebugInfo = debug.NewInfo(dsn, params.UploadLocation)

	// Configure JWT secret from config/environment or generate a secure default.
	jwtSecret, err := getJWTSecret(cfg.JWTSecret)
	if err != nil {
		slog.Error("Failed to configure JWT secret", "error", err)
		return serverSetup{}, err
	}

	// Configure file signing key from config/environment or generate a secure default.
	fileSigningKey, err := getFileSigningKey(cfg.FileSigningKey)
	if err != nil {
		slog.Error("Failed to configure file signing key", "error", err)
		return serverSetup{}, err
	}

	// Configure the .inb backup signing key (#534) from config/environment
	// or generate a secure random Ed25519 seed.
	backupSigner, err := getBackupSigningKey(cfg.BackupSigningKey)
	if err != nil {
		slog.Error("Failed to configure backup signing key", "error", err)
		return serverSetup{}, err
	}

	// Parse file URL expiration duration.
	fileURLExpiration, err := time.ParseDuration(cfg.FileURLExpiration)
	if err != nil {
		slog.Error("Failed to parse file URL expiration duration", "error", err, "duration", cfg.FileURLExpiration)
		return serverSetup{}, err
	}

	// Parse thumbnail slot duration and create thumbnail config.
	thumbnailSlotDuration, err := time.ParseDuration(cfg.ThumbnailSlotDuration)
	if err != nil {
		slog.Error("Failed to parse thumbnail slot duration", "error", err, "duration", cfg.ThumbnailSlotDuration)
		return serverSetup{}, err
	}

	// Parse the admin impersonation-session TTL (#1750). The apiserver
	// clamps any value above the 30-min spec ceiling, so an over-long
	// duration here is not rejected — only a syntactically invalid one.
	impersonationTTL, err := time.ParseDuration(cfg.ImpersonationTTL)
	if err != nil {
		slog.Error("Failed to parse impersonation TTL duration", "error", err, "duration", cfg.ImpersonationTTL)
		return serverSetup{}, err
	}

	params.JWTSecret = jwtSecret
	params.FileSigningKey = fileSigningKey
	params.BackupSigner = backupSigner
	params.FileURLExpiration = fileURLExpiration
	params.ImpersonationTTL = impersonationTTL
	params.ThumbnailConfig = services.ThumbnailGenerationConfig{
		MaxConcurrentPerUser: cfg.ThumbnailMaxConcurrentPerUser,
		RateLimitPerMinute:   cfg.ThumbnailRateLimitPerMinute,
		SlotDuration:         thumbnailSlotDuration,
	}
	params.TokenBlacklister = services.NewTokenBlacklister(cfg.TokenBlacklistRedisURL)
	if cfg.AuthRateLimitDisabled {
		slog.Warn("Auth rate limiting is disabled via configuration — do not use this in production")
		params.AuthRateLimiter = services.NewNoOpAuthRateLimiter()
	} else {
		params.AuthRateLimiter = services.NewAuthRateLimiter(cfg.AuthRateLimitRedisURL)
	}
	if cfg.GlobalRateLimitDisabled {
		slog.Warn("Global API rate limiting is disabled via configuration — do not use this in production")
		params.GlobalRateLimiter = services.NewNoOpGlobalRateLimiter()
	} else {
		globalRateWindow, parseErr := time.ParseDuration(cfg.GlobalRateWindow)
		if parseErr != nil {
			slog.Error("Failed to parse global rate window duration", "error", parseErr, "duration", cfg.GlobalRateWindow)
			return serverSetup{}, parseErr
		}
		params.GlobalRateLimiter = services.NewGlobalRateLimiter(cfg.GlobalRateLimitRedisURL, cfg.GlobalRateLimit, globalRateWindow)
	}

	params.GlobalRateTrustedProxyNets, err = apiserver.ParseTrustedProxyCIDRs(cfg.GlobalRateTrustedProxies)
	if err != nil {
		slog.Error("Failed to parse global rate trusted proxies", "error", err)
		return serverSetup{}, err
	}

	params.CSRFService = services.NewCSRFService(cfg.CSRFRedisURL)
	// Assign through a typed local so that a nil *ReadinessRedisPinger is stored
	// as a genuinely-nil apiserver.RedisPinger (avoiding the typed-nil-in-
	// interface pitfall) and so the close closure can be omitted entirely when
	// no Redis readiness clients were allocated.
	redisPinger := NewReadinessRedisPinger(cfg)
	closeReadinessRedisPinger := func() {}
	if redisPinger != nil {
		params.RedisPinger = redisPinger
		closeReadinessRedisPinger = func() {
			if closeErr := redisPinger.Close(); closeErr != nil {
				slog.Warn("Failed to close Redis readiness client(s)", "error", closeErr)
			}
		}
	}
	// Release Redis readiness clients on any failure path below. On success the
	// closer is returned in serverSetup and the caller owns its lifetime.
	defer func() {
		if err != nil {
			closeReadinessRedisPinger()
		}
	}()

	if err = applyCORSConfig(cfg, dsn, &params); err != nil {
		return serverSetup{}, err
	}

	params.PublicURL = strings.TrimSpace(cfg.PublicURL)
	if err = ValidateEmailPublicURLConfig(cfg.EmailProvider, params.PublicURL); err != nil {
		return serverSetup{}, err
	}

	params.FeatureCurrencyMigration = cfg.FeatureCurrencyMigration
	params.MagicLinkLoginEnabled = resolveMagicLinkLoginEnabled(cfg)

	emailLifecycle, err := buildEmailService(cfg)
	if err != nil {
		slog.Error("Failed to initialize email service", "error", err)
		return serverSetup{}, err
	}
	params.EmailService = emailLifecycle.Service
	params.SupportEmail = strings.TrimSpace(cfg.SupportEmail)

	if err = wireCommodityScan(cfg, &params); err != nil {
		slog.Error("Failed to wire commodity scan service", "error", err)
		return serverSetup{}, err
	}

	if err = wireOAuth(cfg, &params); err != nil {
		slog.Error("Failed to wire OAuth providers", "error", err)
		return serverSetup{}, err
	}

	maybeWireTestTenantHeader(cfg, &params)

	if err = validation.Validate(params); err != nil {
		slog.Error("Invalid server parameters", "error", err)
		return serverSetup{}, err
	}

	return serverSetup{
		params:                    params,
		emailLifecycle:            emailLifecycle,
		closeReadinessRedisPinger: closeReadinessRedisPinger,
	}, nil
}

// resolveMagicLinkLoginEnabled computes the effective magic-link gate. The
// feature is auto-inert when the email provider is the stub: a stubbed provider
// black-holes the sign-in link, so even with the flag on the user could never
// complete the flow. Both conditions fold into one boolean; the flag-on-but-stub
// combo warns loudly. Extracted from buildServerParams to keep its cyclomatic
// complexity inside the linter budget.
func resolveMagicLinkLoginEnabled(cfg *Config) bool {
	stubEmail := normalizeEmailProvider(cfg.EmailProvider) == services.EmailProviderStub
	if cfg.MagicLinkLoginEnabled && stubEmail {
		slog.Warn("Magic-link login is enabled but the email provider is 'stub'; the feature is inert because sign-in links cannot be delivered. Configure a real email provider or set --magic-link-login-enabled=false to silence this warning.")
	}
	return cfg.MagicLinkLoginEnabled && !stubEmail
}

// wireCommodityScan constructs the apiserver.Params CommodityScanService
// + CommodityScanMaxBodyBytes from the AIVision* config. A provider of
// "none" (the default) builds the service with a nil provider so the
// route stays mounted but returns 503 — matches the "feature gated
// off" contract the FE branches on. An unknown provider name fails
// boot loudly so a typo doesn't silently downgrade to "disabled".
func wireCommodityScan(cfg *Config, params *apiserver.Params) error {
	timeout, err := time.ParseDuration(cfg.AIVisionTimeout)
	if err != nil {
		return err
	}

	provider, err := aivision.NewProvider(aivision.ProviderConfig{
		Name:             strings.TrimSpace(cfg.AIVisionProvider),
		AnthropicAPIKey:  cfg.AIVisionAnthropicAPIKey,
		AnthropicModel:   cfg.AIVisionAnthropicModel,
		AnthropicBaseURL: cfg.AIVisionAnthropicBaseURL,
		OpenAIAPIKey:     cfg.AIVisionOpenAIAPIKey,
		OpenAIModel:      cfg.AIVisionOpenAIModel,
		OpenAIBaseURL:    cfg.AIVisionOpenAIBaseURL,
	})
	switch {
	case err == nil:
		// got a provider; carry on
	case errors.Is(err, aivision.ErrProviderDisabled):
		// intentional "feature off" path — keep provider nil; the
		// service surfaces ErrScanProviderDisabled per request.
		provider = nil
	default:
		// Unknown provider name or per-provider construction failure
		// (e.g. anthropic/openai with an empty API key while a real
		// provider was selected). Loud boot failure is correct.
		return err
	}

	params.CommodityScanService = services.NewCommodityScanService(
		provider,
		params.FactorySet.CommodityScanAuditRegistry,
		services.CommodityScanConfig{
			MaxPhotos:        cfg.AIVisionMaxPhotos,
			MaxPhotoBytes:    cfg.AIVisionMaxPhotoBytes,
			RateLimitPerHour: cfg.AIVisionRateLimitPerHour,
			Timeout:          timeout,
		},
	)
	// Body cap = per-photo cap * max photos + a 1MB headroom for
	// multipart overhead/JSON form fields. Zero when either cap is
	// unset so the handler doesn't accidentally clamp to zero.
	//
	// Guard against operator misconfiguration: silly-large caps would
	// overflow int64 during the multiplication and yield a tiny or
	// negative cap, which is much worse than refusing to scale beyond
	// math.MaxInt64. When the product would overflow we clamp to
	// math.MaxInt64 and skip the +1MB headroom — at that magnitude
	// the headroom is noise anyway and saturating is the safe choice.
	if cfg.AIVisionMaxPhotoBytes > 0 && cfg.AIVisionMaxPhotos > 0 {
		perPhoto := int64(cfg.AIVisionMaxPhotoBytes)
		count := int64(cfg.AIVisionMaxPhotos)
		const headroom int64 = 1 << 20
		switch {
		case perPhoto > math.MaxInt64/count:
			// Multiplication overflow guard.
			params.CommodityScanMaxBodyBytes = math.MaxInt64
		case perPhoto*count > math.MaxInt64-headroom:
			// Multiplication fits but adding the 1MB headroom would
			// overflow. Cap at MaxInt64 without the headroom.
			params.CommodityScanMaxBodyBytes = math.MaxInt64
		default:
			params.CommodityScanMaxBodyBytes = perPhoto*count + headroom
		}
	}
	// Per-part cap mirrors the service-level validator. A single
	// hostile multipart part is rejected before io.ReadAll allocates
	// more than (cap+1) bytes.
	params.CommodityScanMaxPhotoBytes = cfg.AIVisionMaxPhotoBytes

	// Effective gate for the public, unauthenticated scan endpoint (#1988):
	// the config opt-in AND a real provider. With no provider the endpoint
	// would only ever 503, so mounting an anonymous route in that state is
	// pointless; gate it off and warn so a misconfigured opt-in is visible.
	params.PublicScanEnabled = cfg.PublicAIVisionScanEnabled && provider != nil
	if cfg.PublicAIVisionScanEnabled && provider == nil {
		slog.Warn("Public AI photo-scan is enabled but no AI vision provider is configured; the endpoint is inert. Set --ai-vision-provider to a real provider or set --public-ai-vision-scan-enabled=false to silence this warning.")
	}
	return nil
}

// maybeWireTestTenantHeader installs the test-only
// X-Inventario-Test-Tenant override (#1851) when the operator flips
// cfg.TestTenantHeaderEnabled (env: INVENTARIO_RUN_TEST_TENANT_HEADER_
// ENABLED, CLI: --test-tenant-header-enabled). It wraps whatever
// resolver buildServerParams produced in a TestHeaderTenantResolver
// AND opens the matching exemption in ValidateNoUserProvidedTenantID
// so the namespaced header isn't blanket-rejected before the resolver
// sees it. Both effects are no-ops when the flag is off, which is the
// production default — the helper exists mainly to keep
// buildServerParams under the gocyclo budget.
func maybeWireTestTenantHeader(cfg *Config, params *apiserver.Params) {
	if !cfg.TestTenantHeaderEnabled {
		return
	}
	slog.Warn("Test-only tenant override header is ENABLED — do not use this in production",
		"header", apiserver.TestTenantHeaderName)
	inner := params.TenantResolver
	if inner == nil {
		inner = &apiserver.HostTenantResolver{}
	}
	params.TenantResolver = &apiserver.TestHeaderTenantResolver{Inner: inner}
	params.TestTenantHeaderEnabled = true
}
