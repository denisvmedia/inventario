package services

// White-box: TestNewEmailSenderFromConfig exercises the unexported
// newEmailSenderFromConfig dispatch switch directly. There is no exported entry
// point that maps an EmailProvider to its concrete sender without also building
// the full async service and its queue, so the provider-routing table cannot be
// asserted through the public API alone.

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestNewEmailSenderFromConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  EmailConfig
	}{
		{name: "stub", cfg: EmailConfig{Provider: EmailProviderStub}},
		{name: "smtp", cfg: EmailConfig{Provider: EmailProviderSMTP, SMTPHost: "smtp.example.com"}},
		{name: "sendgrid", cfg: EmailConfig{Provider: EmailProviderSendGrid, SendGridAPIKey: "sg-test"}},
		{name: "ses", cfg: EmailConfig{Provider: EmailProviderSES, AWSRegion: "us-east-1"}},
		{name: "mandrill", cfg: EmailConfig{Provider: EmailProviderMandrill, MandrillAPIKey: "md-test"}},
		// mailchimp intentionally reuses the Mandrill transport (shared switch
		// arm in newEmailSenderFromConfig), so it reads the same MandrillAPIKey.
		{name: "mailchimp", cfg: EmailConfig{Provider: EmailProviderMailchimp, MandrillAPIKey: "md-test"}},
		{name: "smtp2go", cfg: EmailConfig{Provider: EmailProviderSMTP2GO, SMTP2GOAPIKey: "api-test"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			sndr, err := newEmailSenderFromConfig(tc.cfg)
			c.Assert(err, qt.IsNil)
			c.Assert(sndr, qt.IsNotNil)
		})
	}
}

func TestNewEmailSenderFromConfig_UnknownProvider(t *testing.T) {
	c := qt.New(t)
	sndr, err := newEmailSenderFromConfig(EmailConfig{Provider: "bogus"})
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "unsupported email provider")
	c.Assert(sndr, qt.IsNil)
}
