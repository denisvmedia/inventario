package apiserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	oauthsvc "go.5x5.cz/inventario/services/oauth"
)

// stubOIDCProvider is the OpenID Connect counterpart of stubProvider: it
// also implements oauthsvc.DisplayNamer and oauthsvc.Preparer, the two
// optional interfaces the real OIDCProvider adds (#2786).
type stubOIDCProvider struct {
	stubProvider
	displayName  string
	prepareErr   error
	prepareCalls int
}

func (s *stubOIDCProvider) DisplayName() string { return s.displayName }

func (s *stubOIDCProvider) Prepare(context.Context) error {
	s.prepareCalls++
	return s.prepareErr
}

// addOIDC registers an OIDC stub on the fixture's provider registry. The
// handlers hold the registry by pointer, so a provider added after the
// router was built is visible to them.
func (f *oauthFixture) addOIDC(profile oauthsvc.Profile) *stubOIDCProvider {
	f.t.Helper()
	p := &stubOIDCProvider{
		stubProvider: stubProvider{
			name:     models.OAuthProviderOIDC,
			profile:  profile,
			authBase: "https://auth.example.test/api/oidc/authorization",
		},
		displayName: "Authelia",
	}
	if err := f.providers.Register(p); err != nil {
		f.t.Fatalf("register oidc stub: %v", err)
	}
	return p
}

func (f *oauthFixture) get(path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func TestOAuthProviders_OIDCUsesConfiguredDisplayName(t *testing.T) {
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})
	f.addOIDC(oauthsvc.Profile{})

	rec := f.get("/auth/oauth/providers")
	c.Assert(rec.Code, qt.Equals, http.StatusOK)

	var body struct {
		Providers []struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
		} `json:"providers"`
	}
	c.Assert(json.Unmarshal(rec.Body.Bytes(), &body), qt.IsNil)
	c.Assert(body.Providers, qt.HasLen, 2)
	c.Assert(body.Providers[0].Name, qt.Equals, "google")
	c.Assert(body.Providers[0].DisplayName, qt.Equals, "Google")
	c.Assert(body.Providers[1].Name, qt.Equals, "oidc")
	c.Assert(body.Providers[1].DisplayName, qt.Equals, "Authelia")
}

func TestOAuthStart_OIDCReady_RedirectsToProvider(t *testing.T) {
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})
	p := f.addOIDC(oauthsvc.Profile{})

	rec := f.get("/auth/oauth/oidc/start")
	c.Assert(rec.Code, qt.Equals, http.StatusFound)
	c.Assert(p.prepareCalls, qt.Equals, 1)
	loc, err := url.Parse(rec.Header().Get("Location"))
	c.Assert(err, qt.IsNil)
	c.Assert(loc.Host, qt.Equals, "auth.example.test")
	c.Assert(loc.Query().Get("code_challenge_method"), qt.Equals, "S256")

	hasState := false
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "oauth_state" && cookie.Value != "" {
			hasState = true
		}
	}
	c.Assert(hasState, qt.IsTrue)
}

func TestOAuthStart_OIDCNotReady_RedirectsWithProviderError(t *testing.T) {
	// The identity provider is down (discovery failed): the user lands on
	// the login page with the provider banner naming the button, and no
	// state cookie is written.
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})
	p := f.addOIDC(oauthsvc.Profile{})
	p.prepareErr = errors.New("discovery HTTP 502")

	rec := f.get("/auth/oauth/oidc/start")
	c.Assert(rec.Code, qt.Equals, http.StatusFound)
	loc, err := url.Parse(rec.Header().Get("Location"))
	c.Assert(err, qt.IsNil)
	c.Assert(loc.Path, qt.Equals, "/login")
	c.Assert(loc.Query().Get("oauth_error"), qt.Equals, "provider")
	c.Assert(loc.Query().Get("provider"), qt.Equals, "Authelia")
	for _, cookie := range rec.Result().Cookies() {
		c.Assert(cookie.Name, qt.Not(qt.Equals), "oauth_state")
	}
}

func TestOAuthLinkStart_OIDCNotReady_Returns502(t *testing.T) {
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})
	p := f.addOIDC(oauthsvc.Profile{})
	p.prepareErr = errors.New("discovery HTTP 502")
	user := f.seedUser("alice@example.com", true)

	rec := f.callLinkStart(models.OAuthProviderOIDC, "", f.refreshCookie(user))
	c.Assert(rec.Code, qt.Equals, http.StatusBadGateway)
	c.Assert(rec.Body.String(), qt.Contains, "not reachable")
}

func TestOAuthCallback_OIDC_LinksAndRecordsOIDCMethod(t *testing.T) {
	// A verified email from the OIDC provider auto-links to the existing
	// user, exactly as it does for Google. The identity row and the login
	// event carry the oidc provider and the oauth_oidc method.
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})
	f.addOIDC(oauthsvc.Profile{
		ProviderUserID: "authelia-sub-1",
		Email:          "Alice@Example.com",
		EmailVerified:  true,
		DisplayName:    "Alice",
	})
	user := f.seedUser("alice@example.com", true)

	state := f.signState(models.OAuthProviderOIDC, "")
	rec := f.callCallback(models.OAuthProviderOIDC, state, "code-oidc", state)
	c.Assert(rec.Code, qt.Equals, http.StatusFound)
	c.Assert(rec.Header().Get("Location"), qt.Equals, "/")

	rows := f.listIdentities(user)
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Provider, qt.Equals, models.OAuthProviderOIDC)
	c.Assert(rows[0].ProviderUserID, qt.Equals, "authelia-sub-1")

	events := f.loginEvents()
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Outcome, qt.Equals, models.LoginOutcomeOK)
	c.Assert(events[0].Method, qt.Equals, models.LoginMethodOAuthOIDC)
}
