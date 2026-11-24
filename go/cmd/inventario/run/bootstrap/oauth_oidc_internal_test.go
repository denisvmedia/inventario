package bootstrap

// White-box: registerOIDC is unexported, and which settings turn the
// OpenID Connect provider on is the rule under test. The exported surface
// (Run) would need a whole server to observe it.

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services/oauth"
)

// The issuer points at a closed port, so the background discovery fails
// fast; the tests only look at what registerOIDC registered.
const oidcTestIssuer = "http://127.0.0.1:1"

func TestRegisterOIDC(t *testing.T) {
	const base = "https://app.example"

	t.Run("nothing set leaves it off", func(t *testing.T) {
		c := qt.New(t)
		reg := oauth.NewRegistry()
		c.Assert(registerOIDC(&Config{}, base, reg), qt.IsNil)
		c.Check(reg.Has(models.OAuthProviderOIDC), qt.IsFalse)
	})

	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"issuer alone", Config{OAuthOIDCIssuerURL: oidcTestIssuer}},
		{"client id and secret without issuer", Config{OAuthOIDCClientID: "inventario", OAuthOIDCClientSecret: "secret"}},
		{"issuer and client id without secret", Config{OAuthOIDCIssuerURL: oidcTestIssuer, OAuthOIDCClientID: "inventario"}},
	} {
		t.Run(tc.name+" leaves it off", func(t *testing.T) {
			c := qt.New(t)
			reg := oauth.NewRegistry()
			c.Assert(registerOIDC(&tc.cfg, base, reg), qt.IsNil)
			c.Check(reg.Has(models.OAuthProviderOIDC), qt.IsFalse)
		})
	}

	t.Run("all three set register it with the display name", func(t *testing.T) {
		c := qt.New(t)
		reg := oauth.NewRegistry()
		err := registerOIDC(&Config{
			OAuthOIDCIssuerURL:      oidcTestIssuer + "/",
			OAuthOIDCClientID:       "inventario",
			OAuthOIDCClientSecret:   "secret",
			OAuthOIDCDisplayName:    "Authelia",
			OAuthOIDCBackchannelURL: "http://127.0.0.1:2",
		}, base, reg)
		c.Assert(err, qt.IsNil)
		p, ok := reg.Get(models.OAuthProviderOIDC)
		c.Assert(ok, qt.IsTrue)
		oidc, ok := p.(*oauth.OIDCProvider)
		c.Assert(ok, qt.IsTrue)
		c.Check(oidc.DisplayName(), qt.Equals, "Authelia")
		c.Check(oidc.Issuer(), qt.Equals, oidcTestIssuer)
		c.Check(oidc.BackchannelURL(), qt.Equals, "http://127.0.0.1:2")
	})

	t.Run("an issuer that is not a URL fails the boot", func(t *testing.T) {
		c := qt.New(t)
		err := registerOIDC(&Config{
			OAuthOIDCIssuerURL:    "auth.example.com",
			OAuthOIDCClientID:     "inventario",
			OAuthOIDCClientSecret: "secret",
		}, base, oauth.NewRegistry())
		c.Assert(err, qt.ErrorMatches, `.*IssuerURL must be an absolute http\(s\) URL.*`)
	})
}
