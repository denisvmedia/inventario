package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services"
)

const (
	secretToken  = "s3cr3t-token-value"
	verifyURL    = "https://app.test/verify-email?token=" + secretToken
	resetURL     = "https://app.test/reset-password?token=" + secretToken + "&utm=email"
	magicLinkURL = "https://app.test/magic-link?token=" + secretToken
)

// The default must not put a live token in the logs. A verification or reset
// link is a bearer credential: anyone who can read the log can take the
// account, and logs travel further than the mailbox does.
func TestStubEmailService_RedactsTokensByDefault(t *testing.T) {
	c := qt.New(t)
	svc := services.NewStubEmailService()
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		send func() error
	}{
		{"verification", func() error { return svc.SendVerificationEmail(ctx, "u@test", "U", verifyURL) }},
		{"password reset", func() error { return svc.SendPasswordResetEmail(ctx, "u@test", "U", resetURL) }},
		{"magic link", func() error { return svc.SendMagicLinkEmail(ctx, "u@test", "U", magicLinkURL) }},
	} {
		c.Run(tc.name, func(c *qt.C) {
			var err error
			logged := captureLogs(c, func() { err = tc.send() })
			c.Assert(err, qt.IsNil)

			c.Assert(logged, qt.Not(qt.Contains), secretToken)
			// The rest of the URL survives, so an operator can still tell
			// which link was sent and to where.
			c.Assert(logged, qt.Contains, "app.test")
			c.Assert(logged, qt.Contains, "u@test")
		})
	}
}

// The escape hatch exists for local debugging and has to actually work, or
// someone will reach for a worse one.
func TestStubEmailService_LogsFullURLsWhenAsked(t *testing.T) {
	c := qt.New(t)
	svc := services.NewStubEmailService(services.WithLogEmailURLs(true))

	var err error
	logged := captureLogs(c, func() {
		err = svc.SendPasswordResetEmail(context.Background(), "u@test", "U", resetURL)
	})
	c.Assert(err, qt.IsNil)
	c.Assert(logged, qt.Contains, secretToken)
}

// A nil option is ignored rather than panicking: the constructor is called
// from wiring code that builds its option slice conditionally.
func TestStubEmailService_TolerantOfNilOptions(t *testing.T) {
	c := qt.New(t)
	svc := services.NewStubEmailService(nil, services.WithLogEmailURLs(false), nil)

	var err error
	logged := captureLogs(c, func() {
		err = svc.SendVerificationEmail(context.Background(), "u@test", "U", verifyURL)
	})
	c.Assert(err, qt.IsNil)
	c.Assert(logged, qt.Not(qt.Contains), secretToken)
}

// Every sender is a no-op delivery that must still succeed — a handler that
// treats a send error as fatal would break the whole flow in dev otherwise.
func TestStubEmailService_EverySenderSucceeds(t *testing.T) {
	c := qt.New(t)
	svc := services.NewStubEmailService()
	ctx := context.Background()
	now := time.Now()

	logged := captureLogs(c, func() {
		c.Assert(svc.SendPasswordChangedEmail(ctx, "u@test", "U", now), qt.IsNil)
		c.Assert(svc.SendWelcomeEmail(ctx, "u@test", "U"), qt.IsNil)
		c.Assert(svc.SendWarrantyReminderEmail(ctx, "u@test", "U", "Drill",
			"2027-01-01", "https://app.test/c/1", 30), qt.IsNil)
		c.Assert(svc.SendLoanReminderEmail(ctx, "u@test", "U", "Drill", "Bob",
			"2026-01-01", "2026-02-01", "https://app.test/c/1", "due", 3), qt.IsNil)
		c.Assert(svc.SendStorageQuotaWarningEmail(ctx, "u@test", "U", "Household", 90, 92,
			"9.2 GB", "10 GB", []string{"images 8 GB"},
			"https://app.test/files", "https://app.test/settings"), qt.IsNil)
		c.Assert(svc.SendMaintenanceReminderEmail(ctx, "u@test", "U", "Boiler", "Service",
			"2026-03-01", "https://app.test/c/2", 7), qt.IsNil)
		c.Assert(svc.SendFeedbackEmail(ctx, "support@test", "", "", "", "bug", "it broke",
			"reply@test", []string{"ua: test"}), qt.IsNil)
		c.Assert(svc.SendGroupInviteEmail(ctx, "u@test", "Inviter", "Household", "admin",
			"https://app.test/invite/abc", now), qt.IsNil)
	})

	// Each one left a trace: a stub that silently drops the call would make a
	// broken flow look healthy in development.
	for _, want := range []string{
		"password", "welcome", "warranty", "loan", "quota", "maintenance", "feedback", "invite",
	} {
		c.Assert(logged, qt.Contains, want)
	}
}
