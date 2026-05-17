package services

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// EmailService defines the application-facing transactional email contract.
//
// Architectural role:
//   - API handlers call this interface synchronously.
//   - Concrete implementations may enqueue work for asynchronous delivery.
//   - Transport/provider concerns (SMTP/API/SES/etc.) remain hidden behind this
//     boundary so business flows are provider-agnostic.
type EmailService interface {
	// SendVerificationEmail requests delivery of an account-verification email.
	SendVerificationEmail(ctx context.Context, to, name, verificationURL string) error

	// SendPasswordResetEmail requests delivery of a password-reset email.
	SendPasswordResetEmail(ctx context.Context, to, name, resetURL string) error

	// SendPasswordChangedEmail requests delivery of a password-change notification.
	SendPasswordChangedEmail(ctx context.Context, to, name string, changedAt time.Time) error

	// SendWelcomeEmail requests delivery of a post-verification welcome email.
	SendWelcomeEmail(ctx context.Context, to, name string) error

	// SendWarrantyReminderEmail requests delivery of a "warranty
	// expiring in N days" notification (#1367). thresholdDays is the
	// reminder cadence the worker matched (60 / 30 / 7); the email
	// surfaces it directly so the recipient knows how urgent the row
	// is. commodityURL is optional — when empty, the template
	// suppresses the link block.
	SendWarrantyReminderEmail(ctx context.Context, to, name, commodityName, expiryDate, commodityURL string, thresholdDays int) error

	// SendGroupInviteEmail requests delivery of a "you've been invited
	// to <group>" email (#1533). `to` is the invitee_email captured on
	// the GroupInvite row. inviterName / groupName are surfaced in the
	// body so the recipient knows who sent it and what they're joining.
	// role is the role-label string the UI shows (e.g. "Administrator")
	// — the email passes it through verbatim, no localisation lookup.
	// inviteURL is the constructed /invite/{token} URL the recipient
	// clicks. expiresAt makes the urgency explicit.
	SendGroupInviteEmail(ctx context.Context, to, inviterName, groupName, role, inviteURL string, expiresAt time.Time) error

	// SendStorageQuotaWarningEmail requests delivery of a "your group
	// is approaching its storage quota" email (#1585). thresholdPercent
	// is the StorageQuotaThreshold the worker matched (90); usagePercent
	// is the actual rounded percentage at send time (>= threshold,
	// possibly higher). usedHuman / quotaHuman are short
	// human-readable byte counts (e.g. "135 MiB"). breakdownLines is
	// the per-bucket label slice rendered into a bullet list — the
	// caller controls the bucket names and ordering. filesURL points
	// at the group's files page; settingsURL at Settings → Data &
	// storage. Either URL may be empty: the template suppresses the
	// matching link block when so.
	SendStorageQuotaWarningEmail(ctx context.Context, to, name, groupName string, thresholdPercent, usagePercent int, usedHuman, quotaHuman string, breakdownLines []string, filesURL, settingsURL string) error

	// SendLoanReminderEmail requests delivery of a "your borrowed-out
	// commodity is due back / is overdue" notification (#1509). `kind`
	// is one of "overdue" / "due_soon"; `daysDelta` carries the
	// positive magnitude (days-until-due for due_soon, days-overdue
	// for overdue) so the template can render either "Due in N days"
	// or "Overdue by N days" without doing date math. `commodityName`
	// is surfaced verbatim in the subject; `commodityURL` may be empty
	// — the template suppresses the link block in that case rather
	// than printing a relative URL.
	SendLoanReminderEmail(ctx context.Context, to, name, commodityName, borrowerName, lentAt, dueBackAt, commodityURL, kind string, daysDelta int) error
	// SendMaintenanceReminderEmail requests delivery of a "maintenance
	// due in N days" notification (#1368). thresholdDays is the
	// reminder cadence the worker matched (14 / 7 / 1, or 0 for an
	// overdue notice). title is the schedule's user-supplied title
	// ("Replace water filter"). dueDate is the next_due_at formatted
	// as YYYY-MM-DD. commodityURL is optional — when empty, the
	// template suppresses the link block.
	SendMaintenanceReminderEmail(ctx context.Context, to, name, commodityName, title, dueDate, commodityURL string, thresholdDays int) error
}

// EmailProvider identifies which transport backend should be instantiated by
// AsyncEmailService.
type EmailProvider string

const (
	EmailProviderStub      EmailProvider = "stub"
	EmailProviderSMTP      EmailProvider = "smtp"
	EmailProviderSendGrid  EmailProvider = "sendgrid"
	EmailProviderSES       EmailProvider = "ses"
	EmailProviderMandrill  EmailProvider = "mandrill"
	EmailProviderMailchimp EmailProvider = "mailchimp"
)

const (
	defaultEmailQueueWorkers      = 5
	defaultEmailQueuePopTimeout   = 2 * time.Second
	defaultEmailRetryPollInterval = 1 * time.Second
	defaultEmailRetryBaseDelay    = 1 * time.Second
	defaultEmailRetryMaxDelay     = 1 * time.Minute
	defaultEmailSendTimeout       = 30 * time.Second
	defaultSMTPPort               = 587
	defaultSendGridBaseURL        = "https://api.sendgrid.com"
	defaultMandrillBaseURL        = "https://mandrillapp.com"
)

// EmailConfig is the control-plane configuration for AsyncEmailService.
//
// It combines:
//   - provider transport settings (SMTP, SendGrid, SES, Mandrill),
//   - sender identity (From/ReplyTo),
//   - queue/retry worker policy.
//
// normalize applies defaults so startup remains deterministic even when optional
// values are omitted.
type EmailConfig struct {
	// Provider selects which sender implementation to use.
	Provider EmailProvider

	// From is the sender address used by the provider.
	From string
	// ReplyTo is optional and controls reply address when supported.
	ReplyTo string

	// QueueRedisURL switches queue backend to Redis when non-empty.
	QueueRedisURL string
	// QueueWorkers controls concurrent worker goroutines.
	QueueWorkers int
	// QueueMaxRetry limits per-message retry attempts.
	QueueMaxRetry int

	// QueuePopTimeout bounds worker dequeue blocking time.
	QueuePopTimeout time.Duration
	// RetryPollInterval controls how often delayed retries are promoted.
	RetryPollInterval time.Duration
	// RetryBaseDelay is the initial backoff delay.
	RetryBaseDelay time.Duration
	// RetryMaxDelay caps exponential retry backoff.
	RetryMaxDelay time.Duration
	// SendTimeout bounds a single provider send attempt.
	SendTimeout time.Duration

	// SMTP-specific settings.
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPUseTLS   bool

	// SendGrid-specific settings.
	SendGridAPIKey  string
	SendGridBaseURL string

	// SES-specific settings.
	AWSRegion string

	// Mandrill/Mailchimp Transactional settings.
	MandrillAPIKey  string
	MandrillBaseURL string
}

// normalize canonicalizes provider values and applies service defaults.
//
// This method intentionally prefers safe operational defaults (bounded retries,
// bounded send timeout, non-zero worker count) to avoid silent misconfiguration.
func (c *EmailConfig) normalize() {
	if c.Provider == "" {
		c.Provider = EmailProviderStub
	}
	c.Provider = EmailProvider(strings.ToLower(string(c.Provider)))

	if c.QueueWorkers <= 0 {
		c.QueueWorkers = defaultEmailQueueWorkers
	}
	if c.QueueMaxRetry < 0 {
		c.QueueMaxRetry = 0
	}
	if c.QueuePopTimeout <= 0 {
		c.QueuePopTimeout = defaultEmailQueuePopTimeout
	}
	if c.RetryPollInterval <= 0 {
		c.RetryPollInterval = defaultEmailRetryPollInterval
	}
	if c.RetryBaseDelay <= 0 {
		c.RetryBaseDelay = defaultEmailRetryBaseDelay
	}
	if c.RetryMaxDelay <= 0 {
		c.RetryMaxDelay = defaultEmailRetryMaxDelay
	}
	if c.SendTimeout <= 0 {
		c.SendTimeout = defaultEmailSendTimeout
	}

	if c.SMTPPort == 0 {
		c.SMTPPort = defaultSMTPPort
	}

	if c.SendGridBaseURL == "" {
		c.SendGridBaseURL = defaultSendGridBaseURL
	}
	if c.MandrillBaseURL == "" {
		c.MandrillBaseURL = defaultMandrillBaseURL
	}
}

// StubEmailService is a no-I/O EmailService used when external delivery is
// intentionally disabled.
//
// It preserves application behavior (handlers still "send" emails) while
// replacing delivery with structured logs, making it suitable for development
// and tests.
//
// SECURITY: By default, StubEmailService redacts sensitive tokens from URLs
// before logging them. Full URL logging must be explicitly enabled.
type StubEmailService struct {
	logEmailURLs bool
}

type StubEmailServiceOption func(*StubEmailService)

// NewStubEmailService returns a new StubEmailService instance.
func NewStubEmailService(opts ...StubEmailServiceOption) *StubEmailService {
	svc := &StubEmailService{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(svc)
	}
	return svc
}

// WithLogEmailURLs enables or disables logging of full transactional URLs.
//
// WARNING: Full URLs include sensitive tokens (verification/password reset) and
// should not be enabled in shared environments.
func WithLogEmailURLs(enabled bool) StubEmailServiceOption {
	return func(s *StubEmailService) {
		s.logEmailURLs = enabled
	}
}

// SendVerificationEmail logs a safe version of the verification URL instead of sending an email.
func (s *StubEmailService) SendVerificationEmail(_ context.Context, to, name, verificationURL string) error {
	attrs := []any{
		"to", to,
		"name", name,
	}
	if s.logEmailURLs {
		attrs = append(attrs, "url", verificationURL)
	} else {
		attrs = append(attrs, "url_redacted", redactTokenFromURLForLogs(verificationURL))
	}
	//nolint:sloglint // structured fields are constructed dynamically.
	slog.Info("STUB email: verification link", attrs...)
	return nil
}

// SendPasswordResetEmail logs a safe version of the reset URL instead of sending an email.
func (s *StubEmailService) SendPasswordResetEmail(_ context.Context, to, name, resetURL string) error {
	attrs := []any{
		"to", to,
		"name", name,
	}
	if s.logEmailURLs {
		attrs = append(attrs, "url", resetURL)
	} else {
		attrs = append(attrs, "url_redacted", redactTokenFromURLForLogs(resetURL))
	}
	//nolint:sloglint // structured fields are constructed dynamically.
	slog.Info("STUB email: password-reset link", attrs...)
	return nil
}

func redactTokenFromURLForLogs(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "<redacted>"
	}
	parsed.Fragment = ""

	q := parsed.Query()
	q.Del("token")
	parsed.RawQuery = q.Encode()

	return parsed.String()
}

// SendPasswordChangedEmail logs the password-changed notification event.
func (s *StubEmailService) SendPasswordChangedEmail(_ context.Context, to, name string, changedAt time.Time) error {
	slog.Info("STUB email: password changed notification",
		"to", to,
		"name", name,
		"changed_at", changedAt.UTC().Format(time.RFC3339),
	)
	return nil
}

// SendWelcomeEmail logs the welcome email event.
func (s *StubEmailService) SendWelcomeEmail(_ context.Context, to, name string) error {
	slog.Info("STUB email: welcome message",
		"to", to,
		"name", name,
	)
	return nil
}

// SendWarrantyReminderEmail logs the warranty reminder event without
// dispatching anything externally — useful in tests and the
// "stub" provider profile.
func (s *StubEmailService) SendWarrantyReminderEmail(_ context.Context, to, name, commodityName, expiryDate, commodityURL string, thresholdDays int) error {
	slog.Info("STUB email: warranty reminder",
		"to", to,
		"name", name,
		"commodity_name", commodityName,
		"expiry_date", expiryDate,
		"commodity_url", commodityURL,
		"threshold_days", thresholdDays,
	)
	return nil
}

// SendLoanReminderEmail logs the loan reminder event without
// dispatching anything externally — useful in tests and the "stub"
// provider profile. Mirrors the rest of the stub: the deep-link is
// only logged in clear text when LogEmailURLs is enabled, otherwise a
// redacted form lands in the log (the URL itself carries no tokens,
// but the convention is "internal paths stay off shared logs by
// default" — matches the storage-quota stub).
func (s *StubEmailService) SendLoanReminderEmail(_ context.Context, to, name, commodityName, borrowerName, lentAt, dueBackAt, commodityURL, kind string, daysDelta int) error {
	attrs := []any{
		"to", to,
		"name", name,
		"commodity_name", commodityName,
		"borrower_name", borrowerName,
		"lent_at", lentAt,
		"due_back_at", dueBackAt,
		"kind", kind,
		"days_delta", daysDelta,
	}
	if s.logEmailURLs {
		attrs = append(attrs, "commodity_url", commodityURL)
	} else {
		attrs = append(attrs, "commodity_url_redacted", redactTokenFromURLForLogs(commodityURL))
	}
	//nolint:sloglint // structured fields are constructed dynamically.
	slog.Info("STUB email: loan reminder", attrs...)
	return nil
}

// SendStorageQuotaWarningEmail logs the storage quota warning event
// without dispatching anything externally — useful in tests and the
// "stub" provider profile.
func (s *StubEmailService) SendStorageQuotaWarningEmail(_ context.Context, to, name, groupName string, thresholdPercent, usagePercent int, usedHuman, quotaHuman string, breakdownLines []string, filesURL, settingsURL string) error {
	attrs := []any{
		"to", to,
		"name", name,
		"group_name", groupName,
		"threshold_percent", thresholdPercent,
		"usage_percent", usagePercent,
		"used", usedHuman,
		"quota", quotaHuman,
		"breakdown_lines", breakdownLines,
	}
	if s.logEmailURLs {
		attrs = append(attrs, "files_url", filesURL, "settings_url", settingsURL)
	}
	//nolint:sloglint // structured fields are constructed dynamically.
	slog.Info("STUB email: storage quota warning", attrs...)
	return nil
}

// SendMaintenanceReminderEmail logs the maintenance reminder event
// without dispatching anything externally — useful in tests and the
// "stub" provider profile.
func (s *StubEmailService) SendMaintenanceReminderEmail(_ context.Context, to, name, commodityName, title, dueDate, commodityURL string, thresholdDays int) error {
	attrs := []any{
		"to", to,
		"name", name,
		"commodity_name", commodityName,
		"title", title,
		"due_date", dueDate,
		"threshold_days", thresholdDays,
	}
	if s.logEmailURLs {
		attrs = append(attrs, "commodity_url", commodityURL)
	}
	//nolint:sloglint // structured fields are constructed dynamically.
	slog.Info("STUB email: maintenance reminder", attrs...)
	return nil
}

// SendGroupInviteEmail logs the group-invite event without dispatching
// anything externally.
func (s *StubEmailService) SendGroupInviteEmail(_ context.Context, to, inviterName, groupName, role, inviteURL string, expiresAt time.Time) error {
	attrs := []any{
		"to", to,
		"inviter_name", inviterName,
		"group_name", groupName,
		"role", role,
		"expires_at", expiresAt.UTC().Format(time.RFC3339),
	}
	if s.logEmailURLs {
		attrs = append(attrs, "url", inviteURL)
	} else {
		attrs = append(attrs, "url_redacted", redactTokenFromURLForLogs(inviteURL))
	}
	//nolint:sloglint // structured fields are constructed dynamically.
	slog.Info("STUB email: group invite", attrs...)
	return nil
}
