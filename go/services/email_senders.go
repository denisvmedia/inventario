package services

import (
	"fmt"

	"github.com/denisvmedia/inventario/email/providers/mandrill"
	"github.com/denisvmedia/inventario/email/providers/sendgrid"
	"github.com/denisvmedia/inventario/email/providers/ses"
	"github.com/denisvmedia/inventario/email/providers/smtp"
	"github.com/denisvmedia/inventario/email/providers/smtp2go"
	"github.com/denisvmedia/inventario/email/providers/stub"
	"github.com/denisvmedia/inventario/email/sender"
)

type emailSender = sender.Sender

func newEmailSenderFromConfig(cfg EmailConfig) (emailSender, error) {
	switch cfg.Provider {
	case EmailProviderStub:
		return stub.New(), nil
	case EmailProviderSMTP:
		return smtp.New(smtp.Config{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			UseTLS:   cfg.SMTPUseTLS,
		})
	case EmailProviderSendGrid:
		return sendgrid.New(sendgrid.Config{
			APIKey:  cfg.SendGridAPIKey,
			BaseURL: cfg.SendGridBaseURL,
		})
	case EmailProviderSES:
		return ses.New(ses.Config{
			Region: cfg.AWSRegion,
		})
	case EmailProviderMandrill, EmailProviderMailchimp:
		return mandrill.New(mandrill.Config{
			APIKey:  cfg.MandrillAPIKey,
			BaseURL: cfg.MandrillBaseURL,
		})
	case EmailProviderSMTP2GO:
		return smtp2go.New(smtp2go.Config{
			APIKey:  cfg.SMTP2GOAPIKey,
			BaseURL: cfg.SMTP2GOBaseURL,
		})
	default:
		return nil, fmt.Errorf("unsupported email provider: %q", cfg.Provider)
	}
}
