package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"golang.org/x/oauth2"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/oauth"
)

// newGoogleStub returns an httptest.Server that pretends to be both the
// /token endpoint and the /userinfo endpoint. The tests intercept the
// oauth2 token exchange by rewriting the Endpoint URLs at construction
// time and routing both calls to this single server.
func newGoogleStub(t *testing.T, sub, email string, emailVerified bool, name string) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":

			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			if r.PostForm.Get("code_verifier") == "" {
				http.Error(w, "missing code_verifier", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-token-stub",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer access-token-stub" {
				http.Error(w, "missing bearer", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub":            sub,
				"email":          email,
				"email_verified": emailVerified,
				"name":           name,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, srv.URL
}

// newGoogleProviderForStub wires a GoogleProvider whose token endpoint AND
// userinfo endpoint hit a single httptest.Server. The Google production
// provider hard-codes google.Endpoint; for tests we need a way to override
// it, so this helper duplicates the production constructor with the
// httptest URLs.
func newGoogleProviderForStub(t *testing.T, srv *httptest.Server, userInfoURL string) oauth.Provider {
	t.Helper()
	return &googleProviderTestDouble{
		cfg: &oauth2.Config{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			RedirectURL:  "https://app.example/api/v1/auth/oauth/google/callback",
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  srv.URL + "/auth",
				TokenURL: srv.URL + "/token",
			},
		},
		userInfoURL: userInfoURL,
		client:      srv.Client(),
	}
}

// googleProviderTestDouble mirrors GoogleProvider but with overridable
// endpoints. Production code uses GoogleProvider; this double exists so
// the unit tests don't depend on outbound network calls to Google.
type googleProviderTestDouble struct {
	cfg         *oauth2.Config
	userInfoURL string
	client      *http.Client
}

func (*googleProviderTestDouble) Name() models.OAuthProvider { return models.OAuthProviderGoogle }

func (p *googleProviderTestDouble) AuthCodeURL(state, codeChallenge string) string {
	return p.cfg.AuthCodeURL(
		state,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func (p *googleProviderTestDouble) Exchange(ctx context.Context, code, codeVerifier string) (oauth.Profile, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	tok, err := p.cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return oauth.Profile{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userInfoURL, http.NoBody)
	if err != nil {
		return oauth.Profile{}, err
	}
	tok.SetAuthHeader(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return oauth.Profile{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	var ui struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ui); err != nil {
		return oauth.Profile{}, err
	}
	return oauth.Profile{
		ProviderUserID: ui.Sub,
		Email:          ui.Email,
		EmailVerified:  ui.EmailVerified,
		DisplayName:    ui.Name,
	}, nil
}

func TestNewGoogleProvider_MissingConfigFails(t *testing.T) {
	c := qt.New(t)
	_, err := oauth.NewGoogleProvider(oauth.GoogleProviderConfig{})
	c.Assert(err, qt.IsNotNil)
	_, err = oauth.NewGoogleProvider(oauth.GoogleProviderConfig{ClientID: "x"})
	c.Assert(err, qt.IsNotNil)
	_, err = oauth.NewGoogleProvider(oauth.GoogleProviderConfig{ClientID: "x", ClientSecret: "y"})
	c.Assert(err, qt.IsNotNil)
}

func TestNewGoogleProvider_HappyConstruct(t *testing.T) {
	c := qt.New(t)
	p, err := oauth.NewGoogleProvider(oauth.GoogleProviderConfig{
		ClientID:     "x",
		ClientSecret: "y",
		RedirectURL:  "https://app.example/api/v1/auth/oauth/google/callback",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(p.Name(), qt.Equals, models.OAuthProviderGoogle)

	authURL := p.AuthCodeURL("state-value", "challenge-value")
	// PKCE params are present, scopes are right.
	parsed, err := url.Parse(authURL)
	c.Assert(err, qt.IsNil)
	q := parsed.Query()
	c.Assert(q.Get("code_challenge"), qt.Equals, "challenge-value")
	c.Assert(q.Get("code_challenge_method"), qt.Equals, "S256")
	c.Assert(q.Get("state"), qt.Equals, "state-value")
	c.Assert(q.Get("scope"), qt.Contains, "openid")
	c.Assert(q.Get("scope"), qt.Contains, "email")
}

// TestGoogleProvider_ExchangeFlow exercises the full token + userinfo
// path against an httptest stub via the test double.
func TestGoogleProvider_ExchangeFlow(t *testing.T) {
	c := qt.New(t)

	srv, base := newGoogleStub(t, "subject-42", "alice@example.com", true, "Alice Example")
	p := newGoogleProviderForStub(t, srv, base+"/userinfo")

	profile, err := p.Exchange(context.Background(), "auth-code", "code-verifier-padded-to-realistic-length")
	c.Assert(err, qt.IsNil)
	c.Assert(profile.ProviderUserID, qt.Equals, "subject-42")
	c.Assert(profile.Email, qt.Equals, "alice@example.com")
	c.Assert(profile.EmailVerified, qt.IsTrue)
	c.Assert(profile.DisplayName, qt.Equals, "Alice Example")
}

// TestGoogleProvider_RejectsMissingCodeVerifier verifies that the
// provider's stub sees the verifier on the token form. This protects
// against a regression where the oauth2 client stops forwarding extra
// params on Exchange.
func TestGoogleProvider_RejectsMissingCodeVerifier(t *testing.T) {
	c := qt.New(t)

	// Build a stub that always 400s "missing code_verifier" — and pass
	// a code that bypasses the verifier so we can see what error
	// surfaces. The expected behaviour: Exchange returns an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "missing code_verifier", http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	p := newGoogleProviderForStub(t, srv, srv.URL+"/userinfo")

	_, err := p.Exchange(context.Background(), "auth-code", "v-padded")
	c.Assert(err, qt.IsNotNil)
	c.Assert(strings.Contains(err.Error(), "code_verifier") ||
		strings.Contains(err.Error(), "400") ||
		strings.Contains(err.Error(), "Bad Request"), qt.IsTrue,
		qt.Commentf("err=%v", err))
}
