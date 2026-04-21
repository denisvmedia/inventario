package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/denisvmedia/inventario/services"
)

// EmailServiceLifecycle bundles the initialized email service together with its
// start/stop hooks. The service itself is always wired into apiserver.Params so
// that API handlers can enqueue transactional messages; the start/stop
// goroutine lifecycle is only invoked by `run all` / `run workers` because
// otherwise a split deployment with a shared Redis queue would double-deliver.
type EmailServiceLifecycle struct {
	Service services.EmailService
	Start   func(context.Context)
	Stop    func()
}

func normalizeEmailProvider(raw string) services.EmailProvider {
	provider := services.EmailProvider(strings.ToLower(strings.TrimSpace(raw)))
	if provider == "" {
		provider = services.EmailProviderStub
	}
	return provider
}

// buildEmailService constructs the configured email provider and its lifecycle
// hooks. The stub provider uses no-op start/stop functions; async providers
// return the underlying queue worker's Start/Stop methods.
func buildEmailService(cfg *Config) (EmailServiceLifecycle, error) {
	provider := normalizeEmailProvider(cfg.EmailProvider)

	if provider == services.EmailProviderStub {
		svc := services.NewStubEmailService(services.WithLogEmailURLs(cfg.LogEmailURLs))
		return EmailServiceLifecycle{
			Service: svc,
			Start:   func(context.Context) {},
			Stop:    func() {},
		}, nil
	}

	asyncSvc, err := services.NewAsyncEmailService(services.EmailConfig{
		Provider:        provider,
		From:            cfg.EmailFrom,
		ReplyTo:         cfg.EmailReplyTo,
		QueueRedisURL:   cfg.EmailQueueRedisURL,
		QueueWorkers:    cfg.EmailQueueWorkers,
		QueueMaxRetry:   cfg.EmailQueueMaxRetries,
		SMTPHost:        cfg.SMTPHost,
		SMTPPort:        cfg.SMTPPort,
		SMTPUsername:    cfg.SMTPUsername,
		SMTPPassword:    cfg.SMTPPassword,
		SMTPUseTLS:      cfg.SMTPUseTLS,
		SendGridAPIKey:  cfg.SendGridAPIKey,
		SendGridBaseURL: cfg.SendGridBaseURL,
		AWSRegion:       cfg.AWSRegion,
		MandrillAPIKey:  cfg.MandrillAPIKey,
		MandrillBaseURL: cfg.MandrillBaseURL,
	})
	if err != nil {
		return EmailServiceLifecycle{}, err
	}

	return EmailServiceLifecycle{
		Service: asyncSvc,
		Start:   asyncSvc.Start,
		Stop:    asyncSvc.Stop,
	}, nil
}

// ValidatePublicURLForTransactionalEmails enforces that the provided public URL
// is a well-formed http(s) URL suitable for inclusion in transactional email
// bodies (verification links, password reset links, …).
func ValidatePublicURLForTransactionalEmails(publicURL string) error {
	base := strings.TrimSpace(publicURL)
	if base == "" {
		return errors.New("public URL is required")
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("scheme and host are required")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	return nil
}

// ValidateEmailPublicURLConfig returns an error when the combination of the
// configured email provider and public URL would result in broken transactional
// links. The stub provider is exempt because it never sends real email.
func ValidateEmailPublicURLConfig(provider, publicURL string) error {
	normalizedEmailProvider := normalizeEmailProvider(provider)

	switch normalizedEmailProvider {
	case services.EmailProviderStub:
		return nil
	case services.EmailProviderSMTP,
		services.EmailProviderSendGrid,
		services.EmailProviderSES,
		services.EmailProviderMandrill,
		services.EmailProviderMailchimp:
		if err := ValidatePublicURLForTransactionalEmails(publicURL); err != nil {
			return fmt.Errorf("invalid --public-url for email provider %q: %w", normalizedEmailProvider, err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported email provider: %q", normalizedEmailProvider)
	}
}
