package run

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestConfigSetDefaults_PreservesExplicitZeroEmailQueueMaxRetries(t *testing.T) {
	c := qt.New(t)

	cfg := Config{
		EmailQueueMaxRetries: 0,
	}

	cfg.setDefaults()

	c.Assert(cfg.EmailQueueMaxRetries, qt.Equals, 0)
}

func TestConfigSetDefaults_DefaultsNegativeEmailQueueMaxRetries(t *testing.T) {
	c := qt.New(t)

	cfg := Config{
		EmailQueueMaxRetries: -1,
	}

	cfg.setDefaults()

	c.Assert(cfg.EmailQueueMaxRetries, qt.Equals, 5)
}

func TestValidatePublicURLForTransactionalEmails_Valid(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name      string
		publicURL string
	}{
		{name: "https scheme", publicURL: "https://inventario.example.com"},
		{name: "http scheme", publicURL: "http://inventario.example.com"},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			err := validatePublicURLForTransactionalEmails(tc.publicURL)
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestValidatePublicURLForTransactionalEmails_Invalid(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name            string
		publicURL       string
		wantErrContains string
	}{
		{name: "missing", publicURL: "", wantErrContains: "public URL is required"},
		{name: "missing scheme", publicURL: "inventario.example.com", wantErrContains: "scheme and host are required"},
		{name: "unsupported scheme", publicURL: "ftp://inventario.example.com", wantErrContains: "unsupported scheme"},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			err := validatePublicURLForTransactionalEmails(tc.publicURL)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.wantErrContains)
		})
	}
}

func TestValidateEmailPublicURLConfig_Valid(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name      string
		provider  string
		publicURL string
	}{
		{name: "stub provider does not require public url", provider: "stub", publicURL: ""},
		{name: "supported provider with valid public url", provider: "smtp", publicURL: "https://inventario.example.com"},
		{name: "empty provider defaults to stub", provider: "", publicURL: ""},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			err := validateEmailPublicURLConfig(tc.provider, tc.publicURL)
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestValidateEmailPublicURLConfig_Invalid(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name            string
		provider        string
		publicURL       string
		wantErrContains string
	}{
		{
			name:            "supported provider with invalid public url",
			provider:        "smtp",
			publicURL:       "",
			wantErrContains: "invalid --public-url for email provider",
		},
		{
			name:            "unknown provider returns provider error",
			provider:        "unknown-provider",
			publicURL:       "",
			wantErrContains: "unsupported email provider",
		},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			err := validateEmailPublicURLConfig(tc.provider, tc.publicURL)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.wantErrContains)
		})
	}
}
