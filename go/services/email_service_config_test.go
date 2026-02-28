package services

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestEmailConfigNormalize_DoesNotDefaultFrom(t *testing.T) {
	c := qt.New(t)

	cfg := EmailConfig{}
	cfg.normalize()

	c.Assert(cfg.From, qt.Equals, "")
}

func TestEmailConfigNormalize_PreservesExplicitZeroQueueRetry(t *testing.T) {
	c := qt.New(t)

	cfg := EmailConfig{QueueMaxRetry: 0}
	cfg.normalize()

	c.Assert(cfg.QueueMaxRetry, qt.Equals, 0)
}

func TestNewAsyncEmailService_RequiresFromForNonStubProviders(t *testing.T) {
	c := qt.New(t)

	_, err := NewAsyncEmailService(EmailConfig{
		Provider:       EmailProviderSendGrid,
		SendGridAPIKey: "test-key",
		From:           "",
	})
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "email FROM address is required")
}
