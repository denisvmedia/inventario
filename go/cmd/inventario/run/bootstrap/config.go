// Package bootstrap contains the shared runtime wiring consumed by the `run`
// parent command and its `all`, `apiserver` and `workers` subcommands. It is
// intentionally not a cobra command itself; the three subcommand packages own
// their own cobra.Commands and delegate the non-CLI work to Build and the
// Start* helpers defined here.
package bootstrap

import (
	"github.com/denisvmedia/inventario/internal/defaults"
)

// Config holds every flag read by `inventario run` and its subcommands. The
// struct is shared across subcommands because the overwhelming majority of
// flags (addr, database, rate limiting, email, thumbnails, …) are common to
// both `run apiserver` and `run workers`. Subcommand-specific flags (for
// example --workers-only / --workers-exclude) live on the same struct but are
// only bound to CLI flags on the subcommand that consumes them.
type Config struct {
	Addr                          string `yaml:"addr" env:"ADDR" env-default:":3333"`
	UploadLocation                string `yaml:"upload_location" env:"UPLOAD_LOCATION" env-default:""`
	MaxConcurrentExports          int    `yaml:"max_concurrent_exports" env:"MAX_CONCURRENT_EXPORTS" env-default:"0"`
	MaxConcurrentImports          int    `yaml:"max_concurrent_imports" env:"MAX_CONCURRENT_IMPORTS" env-default:"0"`
	MaxConcurrentRestores         int    `yaml:"max_concurrent_restores" env:"MAX_CONCURRENT_RESTORES" env-default:"0"`
	ExportPollInterval            string `yaml:"export_poll_interval" env:"EXPORT_POLL_INTERVAL" env-default:""`
	ImportPollInterval            string `yaml:"import_poll_interval" env:"IMPORT_POLL_INTERVAL" env-default:""`
	RestorePollInterval           string `yaml:"restore_poll_interval" env:"RESTORE_POLL_INTERVAL" env-default:""`
	RefreshTokenCleanupInterval   string `yaml:"refresh_token_cleanup_interval" env:"REFRESH_TOKEN_CLEANUP_INTERVAL" env-default:""`
	JWTSecret                     string `yaml:"jwt_secret" env:"JWT_SECRET" env-default:""`
	FileSigningKey                string `yaml:"file_signing_key" env:"FILE_SIGNING_KEY" env-default:""`
	FileURLExpiration             string `yaml:"file_url_expiration" env:"FILE_URL_EXPIRATION" env-default:"15m"`
	ThumbnailMaxConcurrentPerUser int    `yaml:"thumbnail_max_concurrent_per_user" env:"THUMBNAIL_MAX_CONCURRENT_PER_USER" env-default:"0"`
	ThumbnailRateLimitPerMinute   int    `yaml:"thumbnail_rate_limit_per_minute" env:"THUMBNAIL_RATE_LIMIT_PER_MINUTE" env-default:"0"`
	ThumbnailSlotDuration         string `yaml:"thumbnail_slot_duration" env:"THUMBNAIL_SLOT_DURATION" env-default:"30m"`
	ThumbnailBatchSize            int    `yaml:"thumbnail_batch_size" env:"THUMBNAIL_BATCH_SIZE" env-default:"0"`
	ThumbnailPollInterval         string `yaml:"thumbnail_poll_interval" env:"THUMBNAIL_POLL_INTERVAL" env-default:""`
	ThumbnailCleanupInterval      string `yaml:"thumbnail_cleanup_interval" env:"THUMBNAIL_CLEANUP_INTERVAL" env-default:""`
	ThumbnailJobRetentionPeriod   string `yaml:"thumbnail_job_retention_period" env:"THUMBNAIL_JOB_RETENTION_PERIOD" env-default:""`
	ThumbnailJobBatchTimeout      string `yaml:"thumbnail_job_batch_timeout" env:"THUMBNAIL_JOB_BATCH_TIMEOUT" env-default:""`
	DetachedThumbnailJobTimeout   string `yaml:"detached_thumbnail_job_timeout" env:"DETACHED_THUMBNAIL_JOB_TIMEOUT" env-default:""`
	TokenBlacklistRedisURL        string `yaml:"token_blacklist_redis_url" env:"TOKEN_BLACKLIST_REDIS_URL" env-default:""`
	AuthRateLimitRedisURL         string `yaml:"auth_rate_limit_redis_url" env:"AUTH_RATE_LIMIT_REDIS_URL" env-default:""`
	AuthRateLimitDisabled         bool   `yaml:"auth_rate_limit_disabled" env:"AUTH_RATE_LIMIT_DISABLED" env-default:"false"`
	GlobalRateLimitRedisURL       string `yaml:"global_rate_limit_redis_url" env:"GLOBAL_RATE_LIMIT_REDIS_URL" env-default:""`
	GlobalRateLimit               int    `yaml:"global_rate_limit" env:"GLOBAL_RATE_LIMIT" env-default:"1000"`
	GlobalRateWindow              string `yaml:"global_rate_window" env:"GLOBAL_RATE_WINDOW" env-default:"1h"`
	GlobalRateLimitDisabled       bool   `yaml:"global_rate_limit_disabled" env:"GLOBAL_RATE_LIMIT_DISABLED" env-default:"false"`
	GlobalRateTrustedProxies      string `yaml:"global_rate_trusted_proxies" env:"GLOBAL_RATE_TRUSTED_PROXIES" env-default:""`
	CSRFRedisURL                  string `yaml:"csrf_redis_url" env:"CSRF_REDIS_URL" env-default:""`
	AllowedOrigins                string `yaml:"allowed_origins" env:"ALLOWED_ORIGINS" env-default:""`
	RegistrationMode              string `yaml:"registration_mode" env:"REGISTRATION_MODE" env-default:"open"`
	PublicURL                     string `yaml:"public_url" env:"PUBLIC_URL" env-default:""`

	LogEmailURLs bool `yaml:"log_email_urls" env:"LOG_EMAIL_URLS" env-default:"false"`

	// WorkersOnly / WorkersExclude restrict which background workers run in
	// `inventario run workers`. See the run/workers package for the accepted
	// syntax and mutual-exclusion rules. Both fields default to empty, meaning
	// "every worker", which preserves the legacy behavior.
	WorkersOnly    string `yaml:"workers_only" env:"WORKERS_ONLY" env-default:""`
	WorkersExclude string `yaml:"workers_exclude" env:"WORKERS_EXCLUDE" env-default:""`

	// ProbeAddr is the bind address of the workers' probe listener that serves
	// /healthz, /readyz and /metrics. It is only consumed by `inventario run
	// workers`; `run apiserver` and `run all` expose those endpoints on Addr.
	ProbeAddr string `yaml:"probe_addr" env:"PROBE_ADDR" env-default:":3334"`

	EmailProvider        string `yaml:"email_provider" env:"EMAIL_PROVIDER" env-default:"stub"`
	EmailFrom            string `yaml:"email_from" env:"EMAIL_FROM" env-default:""`
	EmailReplyTo         string `yaml:"email_reply_to" env:"EMAIL_REPLY_TO" env-default:""`
	EmailQueueRedisURL   string `yaml:"email_queue_redis_url" env:"EMAIL_QUEUE_REDIS_URL" env-default:""`
	EmailQueueWorkers    int    `yaml:"email_queue_workers" env:"EMAIL_QUEUE_WORKERS" env-default:"5"`
	EmailQueueMaxRetries int    `yaml:"email_queue_max_retries" env:"EMAIL_QUEUE_MAX_RETRIES" env-default:"5"`
	SMTPHost             string `yaml:"smtp_host" env:"SMTP_HOST" env-default:""`
	SMTPPort             int    `yaml:"smtp_port" env:"SMTP_PORT" env-default:"587"`
	SMTPUsername         string `yaml:"smtp_username" env:"SMTP_USERNAME" env-default:""`
	SMTPPassword         string `yaml:"smtp_password" env:"SMTP_PASSWORD" env-default:""`
	SMTPUseTLS           bool   `yaml:"smtp_use_tls" env:"SMTP_USE_TLS" env-default:"true"`
	SendGridAPIKey       string `yaml:"sendgrid_api_key" env:"SENDGRID_API_KEY" env-default:""`
	SendGridBaseURL      string `yaml:"sendgrid_base_url" env:"SENDGRID_BASE_URL" env-default:"https://api.sendgrid.com"`
	AWSRegion            string `yaml:"aws_region" env:"AWS_REGION" env-default:""`
	MandrillAPIKey       string `yaml:"mandrill_api_key" env:"MANDRILL_API_KEY" env-default:""`
	MandrillBaseURL      string `yaml:"mandrill_base_url" env:"MANDRILL_BASE_URL" env-default:"https://mandrillapp.com"`
}

// SetDefaults applies repository-wide defaults for fields left at their zero
// value. It is invoked by RegisterFlags after the config has been populated
// from YAML/env so the flag registrations see the final defaults.
func (c *Config) SetDefaults() {
	if c.Addr == "" {
		c.Addr = defaults.GetServerAddr()
	}
	if c.UploadLocation == "" {
		c.UploadLocation = defaults.GetUploadLocation()
	}
	if c.JWTSecret == "" {
		c.JWTSecret = defaults.GetJWTSecret()
	}
	c.setWorkerDefaults()
	c.setThumbnailDefaults()
	if !c.GlobalRateLimitDisabled && c.GlobalRateLimit <= 0 {
		c.GlobalRateLimit = 1000
	}
	if c.GlobalRateWindow == "" {
		c.GlobalRateWindow = "1h"
	}
	if c.EmailQueueWorkers <= 0 {
		c.EmailQueueWorkers = 5
	}
	if c.EmailQueueMaxRetries < 0 {
		c.EmailQueueMaxRetries = 5
	}
	if c.SMTPPort == 0 {
		c.SMTPPort = 587
	}
	if c.ProbeAddr == "" {
		c.ProbeAddr = ":3334"
	}
}

// setWorkerDefaults applies defaults to background worker tunables (concurrency
// limits and poll intervals for export, import, restore, and refresh-token workers).
func (c *Config) setWorkerDefaults() {
	if c.MaxConcurrentExports == 0 {
		c.MaxConcurrentExports = defaults.GetMaxConcurrentExports()
	}
	if c.MaxConcurrentImports == 0 {
		c.MaxConcurrentImports = defaults.GetMaxConcurrentImports()
	}
	if c.MaxConcurrentRestores == 0 {
		c.MaxConcurrentRestores = defaults.GetMaxConcurrentRestores()
	}
	if c.ExportPollInterval == "" {
		c.ExportPollInterval = defaults.GetExportPollInterval()
	}
	if c.ImportPollInterval == "" {
		c.ImportPollInterval = defaults.GetImportPollInterval()
	}
	if c.RestorePollInterval == "" {
		c.RestorePollInterval = defaults.GetRestorePollInterval()
	}
	if c.RefreshTokenCleanupInterval == "" {
		c.RefreshTokenCleanupInterval = defaults.GetRefreshTokenCleanupInterval()
	}
}

// setThumbnailDefaults applies defaults to thumbnail generation worker tunables
// (per-user limits, batch size, and the various interval/timeout knobs).
func (c *Config) setThumbnailDefaults() {
	if c.ThumbnailMaxConcurrentPerUser == 0 {
		c.ThumbnailMaxConcurrentPerUser = defaults.GetThumbnailMaxConcurrentPerUser()
	}
	if c.ThumbnailRateLimitPerMinute == 0 {
		c.ThumbnailRateLimitPerMinute = defaults.GetThumbnailRateLimitPerMinute()
	}
	if c.ThumbnailSlotDuration == "" {
		c.ThumbnailSlotDuration = defaults.GetThumbnailSlotDuration()
	}
	if c.ThumbnailBatchSize == 0 {
		c.ThumbnailBatchSize = defaults.GetThumbnailBatchSize()
	}
	if c.ThumbnailPollInterval == "" {
		c.ThumbnailPollInterval = defaults.GetThumbnailPollInterval()
	}
	if c.ThumbnailCleanupInterval == "" {
		c.ThumbnailCleanupInterval = defaults.GetThumbnailCleanupInterval()
	}
	if c.ThumbnailJobRetentionPeriod == "" {
		c.ThumbnailJobRetentionPeriod = defaults.GetThumbnailJobRetentionPeriod()
	}
	if c.ThumbnailJobBatchTimeout == "" {
		c.ThumbnailJobBatchTimeout = defaults.GetThumbnailJobBatchTimeout()
	}
	if c.DetachedThumbnailJobTimeout == "" {
		c.DetachedThumbnailJobTimeout = defaults.GetDetachedThumbnailJobTimeout()
	}
}
