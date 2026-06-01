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
	flags.StringVar(&cfg.EmailVerificationCleanupInterval, "email-verification-cleanup-interval", cfg.EmailVerificationCleanupInterval, "Interval between email verification token cleanup runs (e.g., 1h, 30m)")
	flags.StringVar(&cfg.MagicLinkTokenCleanupInterval, "magic-link-token-cleanup-interval", cfg.MagicLinkTokenCleanupInterval, "Interval between magic-link sign-in token cleanup runs (e.g., 1h, 30m)")
	flags.StringVar(&cfg.GroupPurgeInterval, "group-purge-interval", cfg.GroupPurgeInterval, "Interval between group purge sweeps (hard-deletes pending_deletion groups and expired unused invites; e.g., 5m, 15m)")
	flags.StringVar(&cfg.WarrantyReminderInterval, "warranty-reminder-interval", cfg.WarrantyReminderInterval, "Interval between warranty reminder sweeps (60/30/7-day expiry emails; e.g., 1h)")
	flags.StringVar(&cfg.StorageQuotaReminderInterval, "storage-quota-reminder-interval", cfg.StorageQuotaReminderInterval, "Interval between storage quota warning sweeps (90% threshold emails; e.g., 1h)")
	flags.StringVar(&cfg.LoanReminderInterval, "loan-reminder-interval", cfg.LoanReminderInterval, "Interval between loan reminder sweeps (overdue + due-soon emails; e.g., 1h)")
	flags.IntVar(&cfg.LoanReminderDueSoonDays, "loan-reminder-due-soon-days", cfg.LoanReminderDueSoonDays, "Forward-looking window in days for the due-soon loan reminder (default 7)")
	flags.StringVar(&cfg.MaintenanceReminderInterval, "maintenance-reminder-interval", cfg.MaintenanceReminderInterval, "Interval between maintenance reminder sweeps (14/7/1-day + overdue maintenance emails; e.g., 1h)")
	flags.StringVar(&cfg.CurrencyMigrationInterval, "currency-migration-interval", cfg.CurrencyMigrationInterval, "Currency migration worker active-poll interval (when pending rows exist; idle cadence is fixed at 1m). Values like 5s, 10s.")
	flags.StringVar(&cfg.BusinessMetricsInterval, "business-metrics-interval", cfg.BusinessMetricsInterval, "Interval between installation-wide business-metrics collection sweeps (#843; e.g., 60s)")
	flags.IntVar(&cfg.ThumbnailBatchSize, "thumbnail-batch-size", cfg.ThumbnailBatchSize, "Maximum thumbnail jobs processed per batch")
	flags.StringVar(&cfg.ThumbnailPollInterval, "thumbnail-poll-interval", cfg.ThumbnailPollInterval, "Thumbnail worker poll interval (e.g., 5s, 10s)")
	flags.StringVar(&cfg.ThumbnailCleanupInterval, "thumbnail-cleanup-interval", cfg.ThumbnailCleanupInterval, "Interval between thumbnail job cleanup runs (e.g., 5m)")
	flags.StringVar(&cfg.ThumbnailJobRetentionPeriod, "thumbnail-job-retention-period", cfg.ThumbnailJobRetentionPeriod, "How long completed thumbnail jobs are retained (e.g., 24h)")
	flags.StringVar(&cfg.ThumbnailJobBatchTimeout, "thumbnail-job-batch-timeout", cfg.ThumbnailJobBatchTimeout, "How long the thumbnail worker waits for an in-flight batch before polling again (e.g., 30s)")
	flags.StringVar(&cfg.DetachedThumbnailJobTimeout, "detached-thumbnail-job-timeout", cfg.DetachedThumbnailJobTimeout, "Per-job timeout for detached thumbnail generation (e.g., 2m)")
	flags.StringVar(&cfg.JWTSecret, "jwt-secret", cfg.JWTSecret, "JWT secret for authentication (minimum 32 characters, auto-generated if not provided)")
	flags.StringVar(&cfg.FileSigningKey, "file-signing-key", cfg.FileSigningKey, "File signing key for secure file URLs (minimum 32 characters, auto-generated if not provided)")
	flags.StringVar(&cfg.BackupSigningKey, "backup-signing-key", cfg.BackupSigningKey, "Ed25519 seed for signing .inb backup archives (64 hex chars or 32 raw bytes, auto-generated if not provided)")
	flags.StringVar(&cfg.FileURLExpiration, "file-url-expiration", cfg.FileURLExpiration, "File URL expiration duration (e.g., 15m, 1h, 30s)")
	flags.StringVar(&cfg.ImpersonationTTL, "impersonation-ttl", cfg.ImpersonationTTL, "Admin impersonation session lifetime (e.g., 30m, 15m); values above 30m are clamped down")
	flags.StringVar(&cfg.TokenBlacklistRedisURL, "token-blacklist-redis-url", cfg.TokenBlacklistRedisURL, "Redis URL for token blacklist (e.g., redis://localhost:6379/0); omit to use in-memory blacklist")
	flags.StringVar(&cfg.AuthRateLimitRedisURL, "auth-rate-limit-redis-url", cfg.AuthRateLimitRedisURL, "Redis URL for auth rate limiting/lockout (e.g., redis://localhost:6379/0); omit to use in-memory limiter")
	flags.BoolVar(&cfg.AuthRateLimitDisabled, "no-auth-rate-limit", cfg.AuthRateLimitDisabled, "Disable auth rate limiting entirely (for testing only — do not use in production)")
	flags.StringVar(&cfg.GlobalRateLimitRedisURL, "global-rate-limit-redis-url", cfg.GlobalRateLimitRedisURL, "Redis URL for global API rate limiting (e.g., redis://localhost:6379/0); omit to use in-memory limiter")
	flags.IntVar(&cfg.GlobalRateLimit, "global-rate-limit", cfg.GlobalRateLimit, "Global per-IP request limit for API endpoints")
	flags.StringVar(&cfg.GlobalRateWindow, "global-rate-window", cfg.GlobalRateWindow, "Global API rate limit window duration (e.g., 1h, 30m)")
	flags.BoolVar(&cfg.GlobalRateLimitDisabled, "no-global-rate-limit", cfg.GlobalRateLimitDisabled, "Disable global API rate limiting entirely (for testing only — do not use in production)")
	flags.BoolVar(
		&cfg.TestTenantHeaderEnabled,
		"test-tenant-header-enabled",
		cfg.TestTenantHeaderEnabled,
		"Honor the X-Inventario-Test-Tenant request header for tenant resolution "+
			"(#1851 e2e cross-tenant flows; for testing only — do not use in production)",
	)
	flags.StringVar(&cfg.GlobalRateTrustedProxies, "global-rate-trusted-proxies", cfg.GlobalRateTrustedProxies, "Comma-separated trusted proxy CIDRs/IPs used when resolving client IP for global rate limiting")
	flags.StringVar(&cfg.CSRFRedisURL, "csrf-redis-url", cfg.CSRFRedisURL, "Redis URL for CSRF token storage (e.g., redis://localhost:6379/0); omit to use in-memory storage")
	flags.StringVar(&cfg.AllowedOrigins, "allowed-origins", cfg.AllowedOrigins, "Comma-separated list of allowed CORS origins (e.g., https://example.com)")
	flags.StringVar(&cfg.PublicURL, "public-url", cfg.PublicURL, "Public base URL used in transactional email links (e.g., https://inventario.example.com)")
	flags.BoolVar(&cfg.MagicLinkLoginEnabled, "magic-link-login-enabled", cfg.MagicLinkLoginEnabled, "Enable passwordless magic-link sign-in (auto-inert when the email provider is stub)")

	flags.StringVar(&cfg.EmailProvider, "email-provider", cfg.EmailProvider, "Email provider: stub, smtp, sendgrid, ses, mandrill, or mailchimp")
	flags.BoolVar(&cfg.LogEmailURLs, "log-email-urls", cfg.LogEmailURLs, "Log full verification/password-reset URLs in stub email mode (includes sensitive tokens; unsafe for shared logs)")
	flags.StringVar(&cfg.EmailFrom, "email-from", cfg.EmailFrom, "From address for transactional emails")
	flags.StringVar(&cfg.EmailReplyTo, "email-reply-to", cfg.EmailReplyTo, "Reply-To address for transactional emails")
	flags.StringVar(&cfg.SupportEmail, "support-email", cfg.SupportEmail, "Destination address for in-app feedback submissions (issue #1387). Empty leaves the /api/v1/feedback endpoint mounted but it returns 503.")
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

	// AI vision photo-scan tunables (issue #1720).
	flags.StringVar(&cfg.AIVisionProvider, "ai-vision-provider", cfg.AIVisionProvider, "AI vision provider: none, mock, anthropic, openai")
	flags.StringVar(&cfg.AIVisionAnthropicAPIKey, "ai-vision-anthropic-api-key", cfg.AIVisionAnthropicAPIKey, "Anthropic API key for the AI vision provider")
	flags.StringVar(&cfg.AIVisionAnthropicModel, "ai-vision-anthropic-model", cfg.AIVisionAnthropicModel, "Anthropic model id (defaults to claude-sonnet-4-6)")
	flags.StringVar(&cfg.AIVisionAnthropicBaseURL, "ai-vision-anthropic-base-url", cfg.AIVisionAnthropicBaseURL, "Anthropic API base URL override (default https://api.anthropic.com)")
	flags.StringVar(&cfg.AIVisionOpenAIAPIKey, "ai-vision-openai-api-key", cfg.AIVisionOpenAIAPIKey, "OpenAI API key for the AI vision provider")
	flags.StringVar(&cfg.AIVisionOpenAIModel, "ai-vision-openai-model", cfg.AIVisionOpenAIModel, "OpenAI model id (defaults to gpt-4o)")
	flags.StringVar(&cfg.AIVisionOpenAIBaseURL, "ai-vision-openai-base-url", cfg.AIVisionOpenAIBaseURL, "OpenAI API base URL override (default https://api.openai.com)")
	flags.StringVar(&cfg.AIVisionTimeout, "ai-vision-timeout", cfg.AIVisionTimeout, "AI vision provider per-call timeout (e.g. 20s)")
	flags.IntVar(&cfg.AIVisionMaxPhotos, "ai-vision-max-photos", cfg.AIVisionMaxPhotos, "Maximum number of photos accepted per scan request")
	flags.IntVar(&cfg.AIVisionMaxPhotoBytes, "ai-vision-max-photo-bytes", cfg.AIVisionMaxPhotoBytes, "Maximum bytes accepted per photo (defaults to 10 MiB)")
	flags.IntVar(&cfg.AIVisionRateLimitPerHour, "ai-vision-rate-limit-per-hour", cfg.AIVisionRateLimitPerHour, "Per-user hourly scan rate limit (0 disables the limit)")
	flags.BoolVar(&cfg.PublicAIVisionScanEnabled, "public-ai-vision-scan-enabled", cfg.PublicAIVisionScanEnabled, "Enable the unauthenticated public photo-scan endpoint for the landing-page CTA (#1988). Default false; spends vendor tokens.")

	// OAuth third-party sign-in (issue #1394). Each provider requires
	// BOTH a client id AND a client secret to be enabled; the redirect
	// base URL is required for any provider to work. OAuth state key
	// signs the per-request state tokens — leave empty for dev, supply
	// a stable value for multi-replica deployments.
	flags.StringVar(&cfg.OAuthGoogleClientID, "oauth-google-client-id", cfg.OAuthGoogleClientID, "OAuth client id for Google sign-in (#1394); empty disables Google")
	flags.StringVar(&cfg.OAuthGoogleClientSecret, "oauth-google-client-secret", cfg.OAuthGoogleClientSecret, "OAuth client secret for Google sign-in (#1394)")
	flags.StringVar(&cfg.OAuthGitHubClientID, "oauth-github-client-id", cfg.OAuthGitHubClientID, "OAuth client id for GitHub sign-in (#1394); empty disables GitHub")
	flags.StringVar(&cfg.OAuthGitHubClientSecret, "oauth-github-client-secret", cfg.OAuthGitHubClientSecret, "OAuth client secret for GitHub sign-in (#1394)")
	flags.StringVar(&cfg.OAuthRedirectBaseURL, "oauth-redirect-base-url", cfg.OAuthRedirectBaseURL, "Public base URL used to build provider redirect URIs (e.g., https://app.inventario.example); required for any OAuth provider")
	flags.StringVar(&cfg.OAuthStateKey, "oauth-state-key", cfg.OAuthStateKey, "OAuth state signing key (minimum 32 characters, auto-generated if not provided)")
}
