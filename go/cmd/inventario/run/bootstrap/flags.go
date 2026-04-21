package bootstrap

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

// RegisterFlags populates cfg from the "run" config section (YAML/env) and
// registers every shared `inventario run` flag as a PersistentFlag on cmd so
// that the all/apiserver/workers subcommands inherit the full set without
// duplication. Subcommand-specific flags (for example --workers-only /
// --workers-exclude) must be registered separately on the owning subcommand.
func RegisterFlags(cmd *cobra.Command, cfg *Config, dbConfig *shared.DatabaseConfig) {
	shared.TryReadSection("run", cfg)
	cfg.SetDefaults()

	flags := cmd.PersistentFlags()
	flags.StringVar(&cfg.Addr, "addr", cfg.Addr, "Bind address for the server")
	flags.StringVar(&cfg.UploadLocation, "upload-location", cfg.UploadLocation, "Location for the uploaded files")
	shared.RegisterDatabaseFlags(cmd, dbConfig)
	flags.IntVar(&cfg.MaxConcurrentExports, "max-concurrent-exports", cfg.MaxConcurrentExports, "Maximum number of concurrent export processes")
	flags.IntVar(&cfg.MaxConcurrentImports, "max-concurrent-imports", cfg.MaxConcurrentImports, "Maximum number of concurrent import processes")
	flags.IntVar(&cfg.MaxConcurrentRestores, "max-concurrent-restores", cfg.MaxConcurrentRestores, "Maximum number of concurrent restore processes")
	flags.StringVar(&cfg.ExportPollInterval, "export-poll-interval", cfg.ExportPollInterval, "Export worker poll interval (e.g., 10s, 30s)")
	flags.StringVar(&cfg.ImportPollInterval, "import-poll-interval", cfg.ImportPollInterval, "Import worker poll interval (e.g., 10s, 30s)")
	flags.StringVar(&cfg.RestorePollInterval, "restore-poll-interval", cfg.RestorePollInterval, "Restore worker poll interval (e.g., 10s, 30s)")
	flags.StringVar(&cfg.RefreshTokenCleanupInterval, "refresh-token-cleanup-interval", cfg.RefreshTokenCleanupInterval, "Interval between refresh token cleanup runs (e.g., 1h, 30m)")
	flags.IntVar(&cfg.ThumbnailBatchSize, "thumbnail-batch-size", cfg.ThumbnailBatchSize, "Maximum thumbnail jobs processed per batch")
	flags.StringVar(&cfg.ThumbnailPollInterval, "thumbnail-poll-interval", cfg.ThumbnailPollInterval, "Thumbnail worker poll interval (e.g., 5s, 10s)")
	flags.StringVar(&cfg.ThumbnailCleanupInterval, "thumbnail-cleanup-interval", cfg.ThumbnailCleanupInterval, "Interval between thumbnail job cleanup runs (e.g., 5m)")
	flags.StringVar(&cfg.ThumbnailJobRetentionPeriod, "thumbnail-job-retention-period", cfg.ThumbnailJobRetentionPeriod, "How long completed thumbnail jobs are retained (e.g., 24h)")
	flags.StringVar(&cfg.ThumbnailJobBatchTimeout, "thumbnail-job-batch-timeout", cfg.ThumbnailJobBatchTimeout, "How long the thumbnail worker waits for an in-flight batch before polling again (e.g., 30s)")
	flags.StringVar(&cfg.DetachedThumbnailJobTimeout, "detached-thumbnail-job-timeout", cfg.DetachedThumbnailJobTimeout, "Per-job timeout for detached thumbnail generation (e.g., 2m)")
	flags.StringVar(&cfg.JWTSecret, "jwt-secret", cfg.JWTSecret, "JWT secret for authentication (minimum 32 characters, auto-generated if not provided)")
	flags.StringVar(&cfg.FileSigningKey, "file-signing-key", cfg.FileSigningKey, "File signing key for secure file URLs (minimum 32 characters, auto-generated if not provided)")
	flags.StringVar(&cfg.FileURLExpiration, "file-url-expiration", cfg.FileURLExpiration, "File URL expiration duration (e.g., 15m, 1h, 30s)")
	flags.StringVar(&cfg.TokenBlacklistRedisURL, "token-blacklist-redis-url", cfg.TokenBlacklistRedisURL, "Redis URL for token blacklist (e.g., redis://localhost:6379/0); omit to use in-memory blacklist")
	flags.StringVar(&cfg.AuthRateLimitRedisURL, "auth-rate-limit-redis-url", cfg.AuthRateLimitRedisURL, "Redis URL for auth rate limiting/lockout (e.g., redis://localhost:6379/0); omit to use in-memory limiter")
	flags.BoolVar(&cfg.AuthRateLimitDisabled, "no-auth-rate-limit", cfg.AuthRateLimitDisabled, "Disable auth rate limiting entirely (for testing only — do not use in production)")
	flags.StringVar(&cfg.GlobalRateLimitRedisURL, "global-rate-limit-redis-url", cfg.GlobalRateLimitRedisURL, "Redis URL for global API rate limiting (e.g., redis://localhost:6379/0); omit to use in-memory limiter")
	flags.IntVar(&cfg.GlobalRateLimit, "global-rate-limit", cfg.GlobalRateLimit, "Global per-IP request limit for API endpoints")
	flags.StringVar(&cfg.GlobalRateWindow, "global-rate-window", cfg.GlobalRateWindow, "Global API rate limit window duration (e.g., 1h, 30m)")
	flags.BoolVar(&cfg.GlobalRateLimitDisabled, "no-global-rate-limit", cfg.GlobalRateLimitDisabled, "Disable global API rate limiting entirely (for testing only — do not use in production)")
	flags.StringVar(&cfg.GlobalRateTrustedProxies, "global-rate-trusted-proxies", cfg.GlobalRateTrustedProxies, "Comma-separated trusted proxy CIDRs/IPs used when resolving client IP for global rate limiting")
	flags.StringVar(&cfg.CSRFRedisURL, "csrf-redis-url", cfg.CSRFRedisURL, "Redis URL for CSRF token storage (e.g., redis://localhost:6379/0); omit to use in-memory storage")
	flags.StringVar(&cfg.AllowedOrigins, "allowed-origins", cfg.AllowedOrigins, "Comma-separated list of allowed CORS origins (e.g., https://example.com)")
	flags.StringVar(&cfg.RegistrationMode, "registration-mode", cfg.RegistrationMode, "Registration mode: open (anyone can register), approval (admin must approve), or closed (registration disabled)")
	flags.StringVar(&cfg.PublicURL, "public-url", cfg.PublicURL, "Public base URL used in transactional email links (e.g., https://inventario.example.com)")

	flags.StringVar(&cfg.EmailProvider, "email-provider", cfg.EmailProvider, "Email provider: stub, smtp, sendgrid, ses, mandrill, or mailchimp")
	flags.BoolVar(&cfg.LogEmailURLs, "log-email-urls", cfg.LogEmailURLs, "Log full verification/password-reset URLs in stub email mode (includes sensitive tokens; unsafe for shared logs)")
	flags.StringVar(&cfg.EmailFrom, "email-from", cfg.EmailFrom, "From address for transactional emails")
	flags.StringVar(&cfg.EmailReplyTo, "email-reply-to", cfg.EmailReplyTo, "Reply-To address for transactional emails")
	flags.StringVar(&cfg.EmailQueueRedisURL, "email-queue-redis-url", cfg.EmailQueueRedisURL, "Redis URL for email queue (recommended for production); omit to use in-memory queue")
	flags.IntVar(&cfg.EmailQueueWorkers, "email-queue-workers", cfg.EmailQueueWorkers, "Number of email queue workers")
	flags.IntVar(&cfg.EmailQueueMaxRetries, "email-queue-max-retries", cfg.EmailQueueMaxRetries, "Maximum number of retries per failed email")

	flags.StringVar(&cfg.SMTPHost, "smtp-host", cfg.SMTPHost, "SMTP host")
	flags.IntVar(&cfg.SMTPPort, "smtp-port", cfg.SMTPPort, "SMTP port")
	flags.StringVar(&cfg.SMTPUsername, "smtp-username", cfg.SMTPUsername, "SMTP username")
	flags.StringVar(&cfg.SMTPPassword, "smtp-password", cfg.SMTPPassword, "SMTP password")
	flags.BoolVar(&cfg.SMTPUseTLS, "smtp-use-tls", cfg.SMTPUseTLS, "Use STARTTLS for SMTP")

	flags.StringVar(&cfg.SendGridAPIKey, "sendgrid-api-key", cfg.SendGridAPIKey, "SendGrid API key")
	flags.StringVar(&cfg.SendGridBaseURL, "sendgrid-base-url", cfg.SendGridBaseURL, "SendGrid API base URL")

	flags.StringVar(&cfg.AWSRegion, "aws-region", cfg.AWSRegion, "AWS region for SES (e.g., us-east-1)")

	flags.StringVar(&cfg.MandrillAPIKey, "mandrill-api-key", cfg.MandrillAPIKey, "Mandrill/Mailchimp Transactional API key")
	flags.StringVar(&cfg.MandrillBaseURL, "mandrill-base-url", cfg.MandrillBaseURL, "Mandrill API base URL")
}
