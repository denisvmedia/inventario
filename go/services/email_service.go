package services

import (
	"context"
	"log/slog"
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
type StubEmailService struct{}

// NewStubEmailService returns a new StubEmailService instance.
func NewStubEmailService() *StubEmailService {
	return &StubEmailService{}
}

// SendVerificationEmail logs the verification URL instead of sending an email.
func (s *StubEmailService) SendVerificationEmail(_ context.Context, to, name, verificationURL string) error {
	slog.Info("STUB email: verification link",
		"to", to,
		"name", name,
		"url", verificationURL,
	)
	return nil
}

// SendPasswordResetEmail logs the reset URL instead of sending an email.
func (s *StubEmailService) SendPasswordResetEmail(_ context.Context, to, name, resetURL string) error {
	slog.Info("STUB email: password-reset link",
		"to", to,
		"name", name,
		"url", resetURL,
	)
	return nil
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
