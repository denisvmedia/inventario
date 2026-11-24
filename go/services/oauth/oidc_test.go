package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services/oauth"
)

// oidcStub is a minimal OpenID Connect provider: a discovery document, a
// token endpoint and a userinfo endpoint on one httptest.Server. Tests
// change its fields before the provider talks to it.
type oidcStub struct {
	srv *httptest.Server

	// issuerOverride replaces the "issuer" in the discovery document;
	// empty means the server's own URL.
	issuerOverride string
	// publicBase, when set, is the base of the issuer and of every endpoint
	// in the discovery document, as a provider behind a public name that
	// the server reaches through a back channel would publish them.
	publicBase string
	// authMethods is token_endpoint_auth_methods_supported.
	authMethods []string
	// omitUserinfo leaves userinfo_endpoint out of the discovery document.
	omitUserinfo bool
	// discoveryStatus, when non-zero, is returned instead of the document.
	discoveryStatus atomic.Int32
	discoveryHits   atomic.Int32

	// userinfo is the JSON object the userinfo endpoint returns.
	userinfo map[string]any
	// userinfoContentType overrides application/json.
	userinfoContentType string

	// lastTokenForm and lastTokenAuth record the token request.
	lastTokenForm url.Values
	lastTokenAuth string
}

func newOIDCStub(t *testing.T) *oidcStub {
	t.Helper()
	s := &oidcStub{
		userinfo: map[string]any{
			"sub":            "oidc-sub-1",
			"email":          "alice@example.com",
			"email_verified": true,
			"name":           "Alice",
		},
	}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *oidcStub) serve(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		s.discoveryHits.Add(1)
		if code := s.discoveryStatus.Load(); code != 0 {
			http.Error(w, "unavailable", int(code))
			return
		}
		base := s.srv.URL
		if s.publicBase != "" {
			base = s.publicBase
		}
		issuer := base
		if s.issuerOverride != "" {
			issuer = s.issuerOverride
		}
		doc := map[string]any{
			"issuer":                           issuer,
			"authorization_endpoint":           base + "/authorize",
			"token_endpoint":                   base + "/token",
			"code_challenge_methods_supported": []string{"S256"},
		}
		if !s.omitUserinfo {
			doc["userinfo_endpoint"] = base + "/userinfo"
		}
		if s.authMethods != nil {
			doc["token_endpoint_auth_methods_supported"] = s.authMethods
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	case "/token":
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		s.lastTokenForm = r.PostForm
		s.lastTokenAuth = r.Header.Get("Authorization")
		if r.PostForm.Get("code_verifier") == "" {
			http.Error(w, "missing code_verifier", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "oidc-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	case "/userinfo":
		if r.Header.Get("Authorization") != "Bearer oidc-access-token" {
			http.Error(w, "missing bearer", http.StatusUnauthorized)
			return
		}
		ct := s.userinfoContentType
		if ct == "" {
			ct = "application/json"
		}
		w.Header().Set("Content-Type", ct)
		_ = json.NewEncoder(w).Encode(s.userinfo)
	default:
		http.NotFound(w, r)
	}
}

func (s *oidcStub) provider(t *testing.T, mutate ...func(*oauth.OIDCProviderConfig)) *oauth.OIDCProvider {
	t.Helper()
	cfg := oauth.OIDCProviderConfig{
		IssuerURL:    s.srv.URL,
		ClientID:     "inventario",
		ClientSecret: "client-secret",
		RedirectURL:  "https://app.example/api/v1/auth/oauth/oidc/callback",
		DisplayName:  "Authelia",
		HTTPClient:   s.srv.Client(),
	}
	for _, m := range mutate {
		m(&cfg)
	}
	p, err := oauth.NewOIDCProvider(cfg)
	if err != nil {
		t.Fatalf("new oidc provider: %v", err)
	}
	return p
}

func TestNewOIDCProvider_RejectsBadConfig(t *testing.T) {
	valid := oauth.OIDCProviderConfig{
		IssuerURL:    "https://auth.example.com",
		ClientID:     "inventario",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example/api/v1/auth/oauth/oidc/callback",
	}
	cases := []struct {
		name   string
		mutate func(*oauth.OIDCProviderConfig)
		want   string
	}{
		{"missing issuer", func(c *oauth.OIDCProviderConfig) { c.IssuerURL = "" }, "IssuerURL is required"},
		{"relative issuer", func(c *oauth.OIDCProviderConfig) { c.IssuerURL = "auth.example.com" }, "absolute http(s) URL"},
		{"other scheme", func(c *oauth.OIDCProviderConfig) { c.IssuerURL = "ftp://auth.example.com" }, "absolute http(s) URL"},
		{"issuer with query", func(c *oauth.OIDCProviderConfig) { c.IssuerURL = "https://auth.example.com?x=1" }, "query or fragment"},
		{"missing client id", func(c *oauth.OIDCProviderConfig) { c.ClientID = "" }, "ClientID is required"},
		{"missing secret", func(c *oauth.OIDCProviderConfig) { c.ClientSecret = "" }, "ClientSecret is required"},
		{"missing redirect", func(c *oauth.OIDCProviderConfig) { c.RedirectURL = "" }, "RedirectURL is required"},
		{"relative back channel", func(c *oauth.OIDCProviderConfig) { c.BackchannelURL = "authelia:8080" }, "BackchannelURL must be an absolute http(s) URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			cfg := valid
			tc.mutate(&cfg)
			_, err := oauth.NewOIDCProvider(cfg)
			c.Assert(err, qt.ErrorMatches, ".*"+regexp.QuoteMeta(tc.want)+".*")
		})
	}
}

func TestNewOIDCProvider_Defaults(t *testing.T) {
	c := qt.New(t)
	p, err := oauth.NewOIDCProvider(oauth.OIDCProviderConfig{
		IssuerURL:    "https://auth.example.com/",
		ClientID:     "inventario",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example/api/v1/auth/oauth/oidc/callback",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(p.Name(), qt.Equals, models.OAuthProviderOIDC)
	c.Assert(p.DisplayName(), qt.Equals, oauth.DefaultOIDCDisplayName)
	c.Assert(p.Issuer(), qt.Equals, "https://auth.example.com")
}

func TestOIDCProvider_SignInFlow(t *testing.T) {
	c := qt.New(t)
	stub := newOIDCStub(t)
	p := stub.provider(t)

	// Before discovery there is no authorization URL to build.
	c.Assert(p.AuthCodeURL("state", "challenge"), qt.Equals, "")

	c.Assert(p.Prepare(context.Background()), qt.IsNil)
	c.Assert(p.DisplayName(), qt.Equals, "Authelia")

	authURL, err := url.Parse(p.AuthCodeURL("signed-state", "pkce-challenge"))
	c.Assert(err, qt.IsNil)
	c.Assert(authURL.Scheme+"://"+authURL.Host+authURL.Path, qt.Equals, stub.srv.URL+"/authorize")
	q := authURL.Query()
	c.Assert(q.Get("response_type"), qt.Equals, "code")
	c.Assert(q.Get("client_id"), qt.Equals, "inventario")
	c.Assert(q.Get("redirect_uri"), qt.Equals, "https://app.example/api/v1/auth/oauth/oidc/callback")
	c.Assert(q.Get("scope"), qt.Equals, "openid email profile")
	c.Assert(q.Get("state"), qt.Equals, "signed-state")
	c.Assert(q.Get("code_challenge"), qt.Equals, "pkce-challenge")
	c.Assert(q.Get("code_challenge_method"), qt.Equals, "S256")

	profile, err := p.Exchange(context.Background(), "auth-code", "pkce-verifier")
	c.Assert(err, qt.IsNil)
	c.Assert(profile, qt.DeepEquals, oauth.Profile{
		ProviderUserID: "oidc-sub-1",
		Email:          "alice@example.com",
		EmailVerified:  true,
		DisplayName:    "Alice",
	})

	// The token request carried the code and the PKCE verifier, and the
	// client authenticated with HTTP Basic (client_secret_basic, the
	// default when the provider lists no methods).
	c.Assert(stub.lastTokenForm.Get("code"), qt.Equals, "auth-code")
	c.Assert(stub.lastTokenForm.Get("code_verifier"), qt.Equals, "pkce-verifier")
	c.Assert(stub.lastTokenForm.Get("client_secret"), qt.Equals, "")
	c.Assert(strings.HasPrefix(stub.lastTokenAuth, "Basic "), qt.IsTrue)

	// Discovery ran once; the exchange reused the cached document.
	c.Assert(stub.discoveryHits.Load(), qt.Equals, int32(1))
}

func TestOIDCProvider_ClientSecretPostWhenOnlyPostIsListed(t *testing.T) {
	c := qt.New(t)
	stub := newOIDCStub(t)
	stub.authMethods = []string{"client_secret_post"}
	p := stub.provider(t)

	_, err := p.Exchange(context.Background(), "auth-code", "pkce-verifier")
	c.Assert(err, qt.IsNil)
	c.Assert(stub.lastTokenForm.Get("client_secret"), qt.Equals, "client-secret")
	c.Assert(stub.lastTokenAuth, qt.Equals, "")
}

func TestOIDCProvider_TrailingSlashOnIssuerIsIgnored(t *testing.T) {
	c := qt.New(t)
	stub := newOIDCStub(t)
	stub.issuerOverride = stub.srv.URL + "/"
	p := stub.provider(t, func(cfg *oauth.OIDCProviderConfig) { cfg.IssuerURL = stub.srv.URL + "/" })

	c.Assert(p.Prepare(context.Background()), qt.IsNil)
}

func TestOIDCProvider_DiscoveryRejected(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*oidcStub)
		want  string
	}{
		{"issuer mismatch", func(s *oidcStub) { s.issuerOverride = "https://evil.example" }, "does not match the configured issuer"},
		{"no userinfo endpoint", func(s *oidcStub) { s.omitUserinfo = true }, "lacks authorization_endpoint, token_endpoint or userinfo_endpoint"},
		{"http error", func(s *oidcStub) { s.discoveryStatus.Store(http.StatusServiceUnavailable) }, "discovery HTTP 503"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			stub := newOIDCStub(t)
			tc.setup(stub)
			p := stub.provider(t)

			err := p.Prepare(context.Background())
			c.Assert(err, qt.ErrorMatches, ".*"+regexp.QuoteMeta(tc.want)+".*")
			c.Assert(p.AuthCodeURL("state", "challenge"), qt.Equals, "")
			_, err = p.Exchange(context.Background(), "code", "verifier")
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestOIDCProvider_DiscoveryRetry(t *testing.T) {
	t.Run("waits for the retry interval after a failure", func(t *testing.T) {
		c := qt.New(t)
		stub := newOIDCStub(t)
		stub.discoveryStatus.Store(http.StatusBadGateway)
		p := stub.provider(t, func(cfg *oauth.OIDCProviderConfig) { cfg.RetryInterval = time.Hour })

		c.Assert(p.Prepare(context.Background()), qt.IsNotNil)
		// The provider is back, but the interval has not passed: the last
		// error is returned without a new request.
		stub.discoveryStatus.Store(0)
		err := p.Prepare(context.Background())
		c.Assert(err, qt.ErrorIs, oauth.ErrOIDCNotReady)
		c.Assert(stub.discoveryHits.Load(), qt.Equals, int32(1))
	})

	t.Run("retries once the interval has passed", func(t *testing.T) {
		c := qt.New(t)
		stub := newOIDCStub(t)
		stub.discoveryStatus.Store(http.StatusBadGateway)
		p := stub.provider(t, func(cfg *oauth.OIDCProviderConfig) { cfg.RetryInterval = time.Millisecond })

		c.Assert(p.Prepare(context.Background()), qt.IsNotNil)
		stub.discoveryStatus.Store(0)
		time.Sleep(5 * time.Millisecond)
		c.Assert(p.Prepare(context.Background()), qt.IsNil)
		c.Assert(stub.discoveryHits.Load(), qt.Equals, int32(2))
		c.Assert(p.AuthCodeURL("state", "challenge"), qt.Not(qt.Equals), "")
	})
}

func TestOIDCProvider_EmailVerifiedClaim(t *testing.T) {
	cases := []struct {
		name  string
		claim any
		want  bool
	}{
		{"boolean true", true, true},
		{"boolean false", false, false},
		{"string true", "true", true},
		{"string TRUE", "TRUE", true},
		{"string false", "false", false},
		{"missing", nil, false},
		{"number", 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			stub := newOIDCStub(t)
			if tc.claim == nil {
				delete(stub.userinfo, "email_verified")
			} else {
				stub.userinfo["email_verified"] = tc.claim
			}
			p := stub.provider(t)
			profile, err := p.Exchange(context.Background(), "code", "verifier")
			c.Assert(err, qt.IsNil)
			c.Assert(profile.EmailVerified, qt.Equals, tc.want)
		})
	}
}

func TestOIDCProvider_DisplayNameFallsBackToPreferredUsername(t *testing.T) {
	c := qt.New(t)
	stub := newOIDCStub(t)
	delete(stub.userinfo, "name")
	stub.userinfo["preferred_username"] = "alice"
	p := stub.provider(t)

	profile, err := p.Exchange(context.Background(), "code", "verifier")
	c.Assert(err, qt.IsNil)
	c.Assert(profile.DisplayName, qt.Equals, "alice")
}

func TestOIDCProvider_UserinfoRejected(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*oidcStub)
		want  string
	}{
		{"missing sub", func(s *oidcStub) { delete(s.userinfo, "sub") }, "missing sub"},
		{"missing email", func(s *oidcStub) { delete(s.userinfo, "email") }, "missing email"},
		{"signed JWT response", func(s *oidcStub) { s.userinfoContentType = "application/jwt" }, "plain JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			stub := newOIDCStub(t)
			tc.setup(stub)
			p := stub.provider(t)
			_, err := p.Exchange(context.Background(), "code", "verifier")
			c.Assert(err, qt.ErrorMatches, ".*"+regexp.QuoteMeta(tc.want)+".*")
		})
	}
}

func TestOIDCProvider_Backchannel(t *testing.T) {
	// The provider publishes https://auth.public.example, which the server
	// cannot reach; the stub stands in for the in-cluster proxy. The issuer
	// check and the browser's authorization URL use the public name, the
	// server's requests go to the back channel.
	c := qt.New(t)
	stub := newOIDCStub(t)
	stub.publicBase = "https://auth.public.example"
	p := stub.provider(t, func(cfg *oauth.OIDCProviderConfig) {
		cfg.IssuerURL = "https://auth.public.example"
		cfg.BackchannelURL = stub.srv.URL + "/"
	})
	c.Assert(p.BackchannelURL(), qt.Equals, stub.srv.URL)

	c.Assert(p.Prepare(context.Background()), qt.IsNil)
	c.Assert(stub.discoveryHits.Load(), qt.Equals, int32(1))

	authURL, err := url.Parse(p.AuthCodeURL("state", "challenge"))
	c.Assert(err, qt.IsNil)
	c.Assert(authURL.Host, qt.Equals, "auth.public.example")
	c.Assert(authURL.Path, qt.Equals, "/authorize")

	profile, err := p.Exchange(context.Background(), "auth-code", "pkce-verifier")
	c.Assert(err, qt.IsNil)
	c.Assert(profile.ProviderUserID, qt.Equals, "oidc-sub-1")
	c.Assert(stub.lastTokenForm.Get("code"), qt.Equals, "auth-code")
}

func TestOIDCProvider_BackchannelCarriesDiscovery(t *testing.T) {
	// With a back channel set, discovery never goes to the issuer's own
	// address: the issuer here is the stub, the back channel a closed
	// port, so discovery fails and the stub sees no request.
	c := qt.New(t)
	stub := newOIDCStub(t)
	p := stub.provider(t, func(cfg *oauth.OIDCProviderConfig) {
		cfg.BackchannelURL = "http://127.0.0.1:1"
	})
	c.Assert(p.Prepare(context.Background()), qt.IsNotNil)
	c.Assert(stub.discoveryHits.Load(), qt.Equals, int32(0))
}
