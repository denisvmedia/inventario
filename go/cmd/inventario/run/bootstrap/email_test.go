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
		allowStub bool
	}{
		{name: "stub provider does not require public url", provider: "stub", publicURL: "", allowStub: false},
		{name: "supported provider with valid public url", provider: "smtp", publicURL: "https://inventario.example.com", allowStub: false},
		{name: "empty provider defaults to stub", provider: "", publicURL: "", allowStub: false},
		{name: "stub with public url and allow-stub flag passes with warning", provider: "stub", publicURL: "https://inventario.example.com", allowStub: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			err := bootstrap.ValidateEmailPublicURLConfig(tc.provider, tc.publicURL, tc.allowStub)
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestValidateEmailPublicURLConfig_Invalid(t *testing.T) {
	cases := []struct {
		name            string
		provider        string
		publicURL       string
		allowStub       bool
		wantErrContains string
	}{
		{
			name:            "supported provider with invalid public url",
			provider:        "smtp",
			publicURL:       "",
			allowStub:       false,
			wantErrContains: "invalid --public-url for email provider",
		},
		{
			name:            "unknown provider returns provider error",
			provider:        "unknown-provider",
			publicURL:       "",
			allowStub:       false,
			wantErrContains: "unsupported email provider",
		},
		{
			name:            "stub with public url and no allow flag is fatal",
			provider:        "stub",
			publicURL:       "https://inventario.example.com",
			allowStub:       false,
			wantErrContains: "would be silently dropped",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			err := bootstrap.ValidateEmailPublicURLConfig(tc.provider, tc.publicURL, tc.allowStub)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.wantErrContains)
		})
	}
}
