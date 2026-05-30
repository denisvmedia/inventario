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
	GroupPurgeInterval            string `yaml:"group_purge_interval" env:"GROUP_PURGE_INTERVAL" env-default:""`
	WarrantyReminderInterval      string `yaml:"warranty_reminder_interval" env:"WARRANTY_REMINDER_INTERVAL" env-default:""`
	StorageQuotaReminderInterval  string `yaml:"storage_quota_reminder_interval" env:"STORAGE_QUOTA_REMINDER_INTERVAL" env-default:""`
	LoanReminderInterval          string `yaml:"loan_reminder_interval" env:"LOAN_REMINDER_INTERVAL" env-default:""`
	LoanReminderDueSoonDays       int    `yaml:"loan_reminder_due_soon_days" env:"LOAN_REMINDER_DUE_SOON_DAYS" env-default:"0"`
	MaintenanceReminderInterval   string `yaml:"maintenance_reminder_interval" env:"MAINTENANCE_REMINDER_INTERVAL" env-default:""`
	CurrencyMigrationInterval     string `yaml:"currency_migration_interval" env:"CURRENCY_MIGRATION_INTERVAL" env-default:""`
	BusinessMetricsInterval       string `yaml:"business_metrics_interval" env:"BUSINESS_METRICS_INTERVAL" env-default:""`
	WorkerControlRefreshInterval  string `yaml:"worker_control_refresh_interval" env:"WORKER_CONTROL_REFRESH_INTERVAL" env-default:""`
	JWTSecret                     string `yaml:"jwt_secret" env:"JWT_SECRET" env-default:""`
	FileSigningKey                string `yaml:"file_signing_key" env:"FILE_SIGNING_KEY" env-default:""`
	FileURLExpiration             string `yaml:"file_url_expiration" env:"FILE_URL_EXPIRATION" env-default:"15m"`
	ImpersonationTTL              string `yaml:"impersonation_ttl" env:"IMPERSONATION_TTL" env-default:"30m"`
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
	PublicURL                     string `yaml:"public_url" env:"PUBLIC_URL" env-default:""`

	LogEmailURLs bool `yaml:"log_email_urls" env:"LOG_EMAIL_URLS" env-default:"false"`

	// FeatureCurrencyMigration gates the entire currency-migration surface
	// (issue #202): the four /currency-migrations endpoints, the
	// requireGroupNotMigrating lock middleware, and the worker.
	// Default on now that the feature shipped end-to-end under #1604 —
	// flipping it off keeps the schema + registries inert (the kill-switch
	// path) without rebuilding. Helm's `features.currencyMigration` value
	// owns the operator-facing toggle. The flag is removed entirely once
	// the rollout settles (see §8 in #202).
	FeatureCurrencyMigration bool `yaml:"feature_currency_migration" env:"FEATURE_CURRENCY_MIGRATION" env-default:"true"`

	// CurrencyMigrationHMACKey signs the stateless preview tokens issued
	// by the preview endpoint. Verification re-derives the signature from
	// this same key on commit, so the value must be identical on every
	// replica. Empty string at startup → a random 32-byte key is generated
	// (fine in single-replica deployments; tokens issued by one process
	// don't survive a restart). Provide a stable value (≥ 32 bytes
	// recommended) for multi-replica or restart-stable deployments.
	CurrencyMigrationHMACKey string `yaml:"currency_migration_hmac_key" env:"CURRENCY_MIGRATION_HMAC_KEY" env-default:""`

	// AIVision* settings drive the photo-scan endpoint (#1720). Under the
	// `run` section loader these map to INVENTARIO_RUN_AI_VISION_* env vars
	// (e.g. INVENTARIO_RUN_AI_VISION_PROVIDER). The provider discriminator
	// selects which implementation handles the scan call ("none", "mock",
	// "anthropic", "openai"); the per-provider API key / model fields
	// override the in-tree defaults.
	AIVisionProvider         string `yaml:"ai_vision_provider" env:"AI_VISION_PROVIDER" env-default:"none"`
	AIVisionAnthropicAPIKey  string `yaml:"ai_vision_anthropic_api_key" env:"AI_VISION_ANTHROPIC_API_KEY" env-default:""`
	AIVisionAnthropicModel   string `yaml:"ai_vision_anthropic_model" env:"AI_VISION_ANTHROPIC_MODEL" env-default:"claude-sonnet-4-6"`
	AIVisionAnthropicBaseURL string `yaml:"ai_vision_anthropic_base_url" env:"AI_VISION_ANTHROPIC_BASE_URL" env-default:""`
	AIVisionOpenAIAPIKey     string `yaml:"ai_vision_openai_api_key" env:"AI_VISION_OPENAI_API_KEY" env-default:""`
	AIVisionOpenAIModel      string `yaml:"ai_vision_openai_model" env:"AI_VISION_OPENAI_MODEL" env-default:"gpt-4o"`
	AIVisionOpenAIBaseURL    string `yaml:"ai_vision_openai_base_url" env:"AI_VISION_OPENAI_BASE_URL" env-default:""`
	AIVisionTimeout          string `yaml:"ai_vision_timeout" env:"AI_VISION_TIMEOUT" env-default:"20s"`
	AIVisionMaxPhotos        int    `yaml:"ai_vision_max_photos" env:"AI_VISION_MAX_PHOTOS" env-default:"5"`
	AIVisionMaxPhotoBytes    int    `yaml:"ai_vision_max_photo_bytes" env:"AI_VISION_MAX_PHOTO_BYTES" env-default:"10485760"`
	AIVisionRateLimitPerHour int    `yaml:"ai_vision_rate_limit_per_hour" env:"AI_VISION_RATE_LIMIT_PER_HOUR" env-default:"30"`

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

	EmailProvider string `yaml:"email_provider" env:"EMAIL_PROVIDER" env-default:"stub"`
	EmailFrom     string `yaml:"email_from" env:"EMAIL_FROM" env-default:""`
	EmailReplyTo  string `yaml:"email_reply_to" env:"EMAIL_REPLY_TO" env-default:""`
	// SupportEmail is the destination address for in-app feedback
	// submissions (issue #1387). Empty leaves the POST /feedback
	// endpoint mounted but it returns 503 — the FE then falls back to
	// the static mailto link surfaced in Settings → Help.
	SupportEmail         string `yaml:"support_email" env:"SUPPORT_EMAIL" env-default:""`
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

	// OAuth* settings drive the third-party sign-in flow (#1394). Each
	// provider is enabled only when its (client_id, client_secret) pair
	// is supplied AND OAuthRedirectBaseURL is set. OAuthStateKey signs
	// the per-request state tokens — leave empty in dev (a random key is
	// generated at boot with a warning), provide a stable value
	// (≥ 32 bytes) on multi-replica or restart-stable deployments.
	OAuthGoogleClientID     string `yaml:"oauth_google_client_id" env:"OAUTH_GOOGLE_CLIENT_ID" env-default:""`
	OAuthGoogleClientSecret string `yaml:"oauth_google_client_secret" env:"OAUTH_GOOGLE_CLIENT_SECRET" env-default:""`
	OAuthGitHubClientID     string `yaml:"oauth_github_client_id" env:"OAUTH_GITHUB_CLIENT_ID" env-default:""`
	OAuthGitHubClientSecret string `yaml:"oauth_github_client_secret" env:"OAUTH_GITHUB_CLIENT_SECRET" env-default:""`
	OAuthRedirectBaseURL    string `yaml:"oauth_redirect_base_url" env:"OAUTH_REDIRECT_BASE_URL" env-default:""`
	OAuthStateKey           string `yaml:"oauth_state_key" env:"OAUTH_STATE_KEY" env-default:""`

	// OAuthGoogle{Auth,Token,UserInfo}URLOverride are TEST-ONLY hooks
	// that redirect Google's three OAuth endpoints at a local stub
	// server. NEVER set these in a production deployment — they bypass
	// the real google.Endpoint + userinfo URLs. Used exclusively by the
	// #1394 e2e flow to exercise the BE's find-or-create-or-link logic
	// without making outbound network calls to Google. Empty in any
	// real config; populated only when the e2e stub server is up.
	OAuthGoogleAuthURLOverride     string `yaml:"-" env:"OAUTH_GOOGLE_AUTH_URL_OVERRIDE" env-default:""`
	OAuthGoogleTokenURLOverride    string `yaml:"-" env:"OAUTH_GOOGLE_TOKEN_URL_OVERRIDE" env-default:""`
	OAuthGoogleUserInfoURLOverride string `yaml:"-" env:"OAUTH_GOOGLE_USERINFO_URL_OVERRIDE" env-default:""`

	// TestTenantHeaderEnabled is a TEST-ONLY hook that lets a request
	// override the Host-derived tenant via the X-Inventario-Test-Tenant
	// header (#1851). It exists exclusively so the Playwright e2e suite
	// can exercise cross-tenant flows (notably the OAuth callback's
	// LoginOutcomeTenantMismatch redirect from #1394) without provisioning
	// per-tenant DNS or a multi-host fixture. NEVER set this in a
	// production deployment — a request-supplied tenant header would let
	// a caller pivot the entire pre-auth surface (registration, OAuth
	// callback, public-tenant-context handlers) onto an arbitrary tenant.
	// The bootstrap layer logs a loud warning when this is enabled.
	TestTenantHeaderEnabled bool `yaml:"-" env:"TEST_TENANT_HEADER_ENABLED" env-default:"false"`
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
	if c.ImpersonationTTL == "" {
		// #1750: 30 min is the spec default and also the hard ceiling
		// the apiserver clamps to. An empty value here just means the
		// env-default did not apply (e.g. config loaded from YAML).
		c.ImpersonationTTL = "30m"
	}
	if c.AIVisionTimeout == "" {
		// #1720: matches AI_VISION_TIMEOUT env-default. Empty values
		// come from YAML configs that omit the key entirely.
		c.AIVisionTimeout = "20s"
	}
	if c.AIVisionProvider == "" {
		c.AIVisionProvider = "none"
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
	if c.GroupPurgeInterval == "" {
		c.GroupPurgeInterval = defaults.GetGroupPurgeInterval()
	}
	if c.WarrantyReminderInterval == "" {
		c.WarrantyReminderInterval = defaults.GetWarrantyReminderInterval()
	}
	if c.StorageQuotaReminderInterval == "" {
		c.StorageQuotaReminderInterval = defaults.GetStorageQuotaReminderInterval()
	}
	if c.LoanReminderInterval == "" {
		c.LoanReminderInterval = defaults.GetLoanReminderInterval()
	}
	if c.LoanReminderDueSoonDays <= 0 {
		// Treat negative values as invalid too — operators that flip a
		// sign by mistake otherwise silently invert the due-soon window.
		c.LoanReminderDueSoonDays = defaults.GetLoanReminderDueSoonDays()
	}
	if c.MaintenanceReminderInterval == "" {
		c.MaintenanceReminderInterval = defaults.GetMaintenanceReminderInterval()
	}
	if c.CurrencyMigrationInterval == "" {
		c.CurrencyMigrationInterval = defaults.GetCurrencyMigrationInterval()
	}
	if c.BusinessMetricsInterval == "" {
		c.BusinessMetricsInterval = defaults.GetBusinessMetricsInterval()
	}
	if c.WorkerControlRefreshInterval == "" {
		c.WorkerControlRefreshInterval = defaults.GetWorkerControlRefreshInterval()
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
