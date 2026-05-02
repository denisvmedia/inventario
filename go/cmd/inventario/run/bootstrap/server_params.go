package bootstrap

import (
	"log/slog"
	"strings"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/debug"
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

	params.JWTSecret = jwtSecret
	params.FileSigningKey = fileSigningKey
	params.FileURLExpiration = fileURLExpiration
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

	// Parse allowed origins (comma-separated) with fail-closed default.
	params.CORSConfig = apiserver.DefaultCORSConfig()
	params.CORSConfig.AllowedOrigins, err = apiserver.ParseAllowedOrigins(cfg.AllowedOrigins)
	if err != nil {
		slog.Error("Failed to parse allowed CORS origins", "error", err)
		return serverSetup{}, err
	}
	if len(params.CORSConfig.AllowedOrigins) == 0 {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(dsn)), "memory://") {
			params.CORSConfig.AllowedOrigins = apiserver.DefaultDevAllowedOrigins()
			slog.Warn("No CORS origins explicitly configured; using local development defaults in memory-db mode. Set --allowed-origins for custom values.")
		} else {
			slog.Warn("No CORS origins explicitly configured; cross-origin requests are denied. Set --allowed-origins to allow specific origins.")
		}
	}

	params.PublicURL = strings.TrimSpace(cfg.PublicURL)
	if err = ValidateEmailPublicURLConfig(cfg.EmailProvider, params.PublicURL); err != nil {
		return serverSetup{}, err
	}

	emailLifecycle, err := buildEmailService(cfg)
	if err != nil {
		slog.Error("Failed to initialize email service", "error", err)
		return serverSetup{}, err
	}
	params.EmailService = emailLifecycle.Service

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
