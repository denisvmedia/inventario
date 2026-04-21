package bootstrap_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/run/bootstrap"
)

func TestValidatePublicURLForTransactionalEmails_Valid(t *testing.T) {
	cases := []struct {
		name      string
		publicURL string
	}{
		{name: "https scheme", publicURL: "https://inventario.example.com"},
		{name: "http scheme", publicURL: "http://inventario.example.com"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			err := bootstrap.ValidatePublicURLForTransactionalEmails(tc.publicURL)
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestValidatePublicURLForTransactionalEmails_Invalid(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			err := bootstrap.ValidatePublicURLForTransactionalEmails(tc.publicURL)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.wantErrContains)
		})
	}
}

func TestValidateEmailPublicURLConfig_Valid(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			err := bootstrap.ValidateEmailPublicURLConfig(tc.provider, tc.publicURL)
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestValidateEmailPublicURLConfig_Invalid(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			err := bootstrap.ValidateEmailPublicURLConfig(tc.provider, tc.publicURL)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.wantErrContains)
		})
	}
}
