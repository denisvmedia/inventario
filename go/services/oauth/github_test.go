package oauth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"golang.org/x/oauth2"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services/oauth"
)

// githubProviderTestDouble overrides GitHub's hard-coded endpoints with
// httptest URLs. Production wires the constant githuboauth.Endpoint;
// this double keeps the production constructor honest while letting the
// test exercise the exchange against an in-process stub.
type githubProviderTestDouble struct {
	cfg        *oauth2.Config
	userURL    string
	userEmails string
	httpClient *http.Client
}

func (*githubProviderTestDouble) Name() models.OAuthProvider { return models.OAuthProviderGitHub }

func (p *githubProviderTestDouble) AuthCodeURL(state, codeChallenge string) string {
	return p.cfg.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func (p *githubProviderTestDouble) Exchange(ctx context.Context, code, codeVerifier string) (oauth.Profile, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
	tok, err := p.cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return oauth.Profile{}, err
	}

	userReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.userURL, http.NoBody)
	tok.SetAuthHeader(userReq)
	uresp, err := p.httpClient.Do(userReq)
	if err != nil {
		return oauth.Profile{}, err
	}
	defer func() { _ = uresp.Body.Close() }()
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(uresp.Body).Decode(&user); err != nil {
		return oauth.Profile{}, err
	}

	mailReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.userEmails, http.NoBody)
	tok.SetAuthHeader(mailReq)
	mresp, err := p.httpClient.Do(mailReq)
	if err != nil {
		return oauth.Profile{}, err
	}
	defer func() { _ = mresp.Body.Close() }()
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(mresp.Body).Decode(&emails); err != nil {
		return oauth.Profile{}, err
	}

	chosen := ""
	chosenVerified := false
	for _, e := range emails {
		if e.Primary && e.Verified {
			chosen = e.Email
			chosenVerified = true
			break
		}
	}
	if chosen == "" {
		for _, e := range emails {
			if e.Verified {
				chosen = e.Email
				chosenVerified = true
				break
			}
		}
	}
	if chosen == "" {
		chosen = user.Email
	}

	displayName := user.Name
	if displayName == "" {
		displayName = user.Login
	}
	return oauth.Profile{
		ProviderUserID: itoaI64(user.ID),
		Email:          chosen,
		EmailVerified:  chosenVerified,
		DisplayName:    displayName,
	}, nil
}

func itoaI64(i int64) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	buf := make([]byte, 0, 20)
	for i > 0 {
		buf = append([]byte{digits[i%10]}, buf...)
		i /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

func newGitHubStub(t *testing.T, userBody, emailsBody []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "gh-access",
				"token_type":   "Bearer",
			})
		case "/user":
			if r.Header.Get("Authorization") == "" {
				http.Error(w, "no bearer", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(userBody)
		case "/user/emails":
			if r.Header.Get("Authorization") == "" {
				http.Error(w, "no bearer", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(emailsBody)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newGitHubProviderForStub(srv *httptest.Server) oauth.Provider {
	return &githubProviderTestDouble{
		cfg: &oauth2.Config{
			ClientID:     "id",
			ClientSecret: "secret",
			RedirectURL:  "https://app.example/api/v1/auth/oauth/github/callback",
			Scopes:       []string{"read:user", "user:email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  srv.URL + "/auth",
				TokenURL: srv.URL + "/token",
			},
		},
		userURL:    srv.URL + "/user",
		userEmails: srv.URL + "/user/emails",
		httpClient: srv.Client(),
	}
}

func TestNewGitHubProvider_MissingConfigFails(t *testing.T) {
	c := qt.New(t)
	_, err := oauth.NewGitHubProvider(oauth.GitHubProviderConfig{})
	c.Assert(err, qt.IsNotNil)
	_, err = oauth.NewGitHubProvider(oauth.GitHubProviderConfig{ClientID: "x"})
	c.Assert(err, qt.IsNotNil)
	_, err = oauth.NewGitHubProvider(oauth.GitHubProviderConfig{ClientID: "x", ClientSecret: "y"})
	c.Assert(err, qt.IsNotNil)
}

func TestNewGitHubProvider_HappyConstruct(t *testing.T) {
	c := qt.New(t)
	p, err := oauth.NewGitHubProvider(oauth.GitHubProviderConfig{
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example/api/v1/auth/oauth/github/callback",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(p.Name(), qt.Equals, models.OAuthProviderGitHub)
	url := p.AuthCodeURL("st", "ch")
	c.Assert(url, qt.Contains, "code_challenge=ch")
	c.Assert(url, qt.Contains, "code_challenge_method=S256")
}

// TestGitHubProvider_PicksPrimaryVerifiedEmail pins the email resolution
// pass order: primary+verified wins over any-verified, even when the
// public /user response carries a different (or null) email.
func TestGitHubProvider_PicksPrimaryVerifiedEmail(t *testing.T) {
	c := qt.New(t)
	userBody := []byte(`{"id":42, "login":"alice", "name":"Alice", "email":null}`)
	emailsBody := []byte(`[
		{"email":"alice-public@github.test","primary":false,"verified":true},
		{"email":"alice@example.com","primary":true,"verified":true}
	]`)
	srv := newGitHubStub(t, userBody, emailsBody)
	p := newGitHubProviderForStub(srv)
	profile, err := p.Exchange(context.Background(), "code", "verifier-padded-realistic")
	c.Assert(err, qt.IsNil)
	c.Assert(profile.ProviderUserID, qt.Equals, "42")
	c.Assert(profile.Email, qt.Equals, "alice@example.com")
	c.Assert(profile.EmailVerified, qt.IsTrue)
	c.Assert(profile.DisplayName, qt.Equals, "Alice")
}

// TestGitHubProvider_FallsBackToAnyVerified pins the second-pass behaviour
// for users whose primary email is unverified but who have at least one
// other verified address.
func TestGitHubProvider_FallsBackToAnyVerified(t *testing.T) {
	c := qt.New(t)
	userBody := []byte(`{"id":7, "login":"bob", "name":"", "email":null}`)
	emailsBody := []byte(`[
		{"email":"unverified@primary.test","primary":true,"verified":false},
		{"email":"secondary@example.com","primary":false,"verified":true}
	]`)
	srv := newGitHubStub(t, userBody, emailsBody)
	p := newGitHubProviderForStub(srv)
	profile, err := p.Exchange(context.Background(), "code", "verifier-padded-realistic")
	c.Assert(err, qt.IsNil)
	c.Assert(profile.Email, qt.Equals, "secondary@example.com")
	c.Assert(profile.EmailVerified, qt.IsTrue)
	// DisplayName falls back to login when name is empty.
	c.Assert(profile.DisplayName, qt.Equals, "bob")
}

// TestGitHubProvider_UnverifiedOnly returns the /user email but flags it
// unverified so the callback knows not to auto-link.
func TestGitHubProvider_UnverifiedOnly(t *testing.T) {
	c := qt.New(t)
	userBody := []byte(`{"id":11, "login":"carol", "name":"Carol", "email":"carol@example.com"}`)
	emailsBody := []byte(`[
		{"email":"carol@example.com","primary":true,"verified":false}
	]`)
	srv := newGitHubStub(t, userBody, emailsBody)
	p := newGitHubProviderForStub(srv)
	profile, err := p.Exchange(context.Background(), "code", "verifier-padded-realistic")
	c.Assert(err, qt.IsNil)
	c.Assert(profile.Email, qt.Equals, "carol@example.com")
	c.Assert(profile.EmailVerified, qt.IsFalse)
}

// routeFunc answers each request by URL, so one client can stand in for
// GitHub's token endpoint, /user and /user/emails at once.
type routeFunc func(*http.Request) (*http.Response, error)

func (f routeFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// githubRoutes answers the three calls Exchange makes. An empty body for a
// route means "fail this one with a 500", which is how the error paths below
// are driven without a second fixture.
type githubRoutes struct {
	token  string
	user   string
	emails string
}

func githubProvider(c *qt.C, routes githubRoutes) *oauth.GitHubProvider {
	c.Helper()
	client := &http.Client{Transport: routeFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(r.URL.Host, "github.com") && strings.Contains(r.URL.Path, "access_token"):
			if routes.token == "" {
				return jsonResponse(http.StatusInternalServerError, `{"error":"server_error"}`), nil
			}
			return jsonResponse(http.StatusOK, routes.token), nil
		case r.URL.Path == "/user/emails":
			if routes.emails == "" {
				return jsonResponse(http.StatusForbidden, `{"message":"Bad credentials"}`), nil
			}
			return jsonResponse(http.StatusOK, routes.emails), nil
		case r.URL.Path == "/user":
			if routes.user == "" {
				return jsonResponse(http.StatusUnauthorized, `{"message":"Bad credentials"}`), nil
			}
			return jsonResponse(http.StatusOK, routes.user), nil
		}
		return nil, fmt.Errorf("unexpected request to %s", r.URL.String())
	})}

	p, err := oauth.NewGitHubProvider(oauth.GitHubProviderConfig{
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "https://app.test/callback",
		HTTPClient:   client,
	})
	c.Assert(err, qt.IsNil)
	return p
}

const githubToken = `{"access_token":"gho_test","token_type":"bearer"}`

func TestGitHubExchange_PrefersThePrimaryVerifiedEmail(t *testing.T) {
	c := qt.New(t)
	p := githubProvider(c, githubRoutes{
		token: githubToken,
		user:  `{"id":42,"login":"octocat","name":"The Octocat","email":"public@example.com"}`,
		emails: `[{"email":"other@example.com","primary":false,"verified":true},
		           {"email":"primary@example.com","primary":true,"verified":true}]`,
	})

	profile, err := p.Exchange(context.Background(), "code", "verifier")
	c.Assert(err, qt.IsNil)
	c.Assert(profile.ProviderUserID, qt.Equals, "42")
	// The /user payload's public email is NOT trusted for the verified flag —
	// /user/emails is the authoritative source, so the primary row wins.
	c.Assert(profile.Email, qt.Equals, "primary@example.com")
	c.Assert(profile.EmailVerified, qt.IsTrue)
	c.Assert(profile.DisplayName, qt.Equals, "The Octocat")
}

// A user whose primary address is unverified but who has another verified one
// should still get in, on the verified address.
func TestGitHubExchange_FallsBackToAnyVerifiedEmail(t *testing.T) {
	c := qt.New(t)
	p := githubProvider(c, githubRoutes{
		token: githubToken,
		user:  `{"id":7,"login":"octocat","name":""}`,
		emails: `[{"email":"unverified@example.com","primary":true,"verified":false},
		           {"email":"verified@example.com","primary":false,"verified":true}]`,
	})

	profile, err := p.Exchange(context.Background(), "code", "verifier")
	c.Assert(err, qt.IsNil)
	c.Assert(profile.Email, qt.Equals, "verified@example.com")
	c.Assert(profile.EmailVerified, qt.IsTrue)
	// No display name on the profile: the login is the readable fallback.
	c.Assert(profile.DisplayName, qt.Equals, "octocat")
}

// With nothing verified anywhere, the /user email comes back with
// EmailVerified=false. That flag is what stops the callback auto-linking an
// address the user has not proven they own.
func TestGitHubExchange_UnverifiedEmailIsMarkedUnverified(t *testing.T) {
	c := qt.New(t)
	p := githubProvider(c, githubRoutes{
		token:  githubToken,
		user:   `{"id":9,"login":"octocat","email":"public@example.com"}`,
		emails: `[{"email":"public@example.com","primary":true,"verified":false}]`,
	})

	profile, err := p.Exchange(context.Background(), "code", "verifier")
	c.Assert(err, qt.IsNil)
	c.Assert(profile.Email, qt.Equals, "public@example.com")
	c.Assert(profile.EmailVerified, qt.IsFalse)
}

func TestGitHubExchange_NoUsableEmail(t *testing.T) {
	c := qt.New(t)
	p := githubProvider(c, githubRoutes{
		token:  githubToken,
		user:   `{"id":9,"login":"octocat"}`,
		emails: `[]`,
	})

	_, err := p.Exchange(context.Background(), "code", "verifier")
	c.Assert(err, qt.IsNotNil)
}

// An id of zero means GitHub answered with something we cannot key an account
// on; accepting it would collide every such user onto provider_user_id "0".
func TestGitHubExchange_RejectsAMissingUserID(t *testing.T) {
	c := qt.New(t)
	p := githubProvider(c, githubRoutes{
		token:  githubToken,
		user:   `{"login":"octocat","email":"public@example.com"}`,
		emails: `[{"email":"public@example.com","primary":true,"verified":true}]`,
	})

	_, err := p.Exchange(context.Background(), "code", "verifier")
	c.Assert(err, qt.IsNotNil)
}

func TestGitHubExchange_SurfacesUpstreamFailures(t *testing.T) {
	c := qt.New(t)

	// Token endpoint refuses.
	_, err := githubProvider(c, githubRoutes{user: `{"id":1}`, emails: `[]`}).
		Exchange(context.Background(), "code", "verifier")
	c.Assert(err, qt.IsNotNil)

	// /user refuses — a 401 here must not be reported as a successful login.
	_, err = githubProvider(c, githubRoutes{token: githubToken, emails: `[]`}).
		Exchange(context.Background(), "code", "verifier")
	c.Assert(err, qt.IsNotNil)

	// /user/emails refuses (missing user:email scope is the usual cause).
	_, err = githubProvider(c, githubRoutes{token: githubToken, user: `{"id":1,"login":"o"}`}).
		Exchange(context.Background(), "code", "verifier")
	c.Assert(err, qt.IsNotNil)
}

func TestGitHubExchange_RejectsMalformedJSON(t *testing.T) {
	c := qt.New(t)
	p := githubProvider(c, githubRoutes{
		token: githubToken,
		user:  `{"id": not-json`,
	})

	_, err := p.Exchange(context.Background(), "code", "verifier")
	c.Assert(err, qt.IsNotNil)
}

// TestGitHubProvider_HonorsEndpointOverrides pins the test-only hook #1929
// added so the e2e stub can stand in for GitHub. It asserts the destinations,
// not just that the exchange succeeded: an override that reached the config
// struct but not the request would still produce a valid-looking profile here
// while talking to api.github.com in the e2e run.
func TestGitHubProvider_HonorsEndpointOverrides(t *testing.T) {
	c := qt.New(t)

	stub := "http://127.0.0.1:9/gh"
	var seen []string
	client := &http.Client{Transport: routeFunc(func(r *http.Request) (*http.Response, error) {
		seen = append(seen, r.URL.String())
		switch r.URL.Path {
		case "/gh/token":
			return jsonResponse(http.StatusOK, githubToken), nil
		case "/gh/user":
			return jsonResponse(http.StatusOK, `{"id":7,"login":"stubcat","name":"Stub Cat"}`), nil
		case "/gh/user/emails":
			return jsonResponse(http.StatusOK, `[{"email":"stub@example.com","primary":true,"verified":true}]`), nil
		}
		return nil, fmt.Errorf("request escaped the stub: %s", r.URL.String())
	})}

	p, err := oauth.NewGitHubProvider(oauth.GitHubProviderConfig{
		ClientID:      "id",
		ClientSecret:  "secret",
		RedirectURL:   "https://app.test/callback",
		AuthURL:       stub + "/authorize",
		TokenURL:      stub + "/token",
		UserURL:       stub + "/user",
		UserEmailsURL: stub + "/user/emails",
		HTTPClient:    client,
	})
	c.Assert(err, qt.IsNil)

	c.Assert(p.AuthCodeURL("state", "challenge"), qt.Contains, stub+"/authorize")

	profile, err := p.Exchange(context.Background(), "code", "verifier")
	c.Assert(err, qt.IsNil)
	c.Check(profile.ProviderUserID, qt.Equals, "7")
	c.Check(profile.Email, qt.Equals, "stub@example.com")
	c.Check(profile.EmailVerified, qt.IsTrue)
	c.Check(profile.DisplayName, qt.Equals, "Stub Cat")

	for _, u := range seen {
		c.Check(u, qt.Contains, "127.0.0.1:9",
			qt.Commentf("every call has to land on the stub, not on GitHub"))
	}
	c.Check(seen, qt.HasLen, 3)
}

// Without overrides the provider still addresses GitHub. This is the half that
// keeps the override from becoming the default by accident.
func TestGitHubProvider_DefaultsToGitHubEndpoints(t *testing.T) {
	c := qt.New(t)

	var seen []string
	client := &http.Client{Transport: routeFunc(func(r *http.Request) (*http.Response, error) {
		seen = append(seen, r.URL.String())
		switch {
		case strings.Contains(r.URL.Path, "access_token"):
			return jsonResponse(http.StatusOK, githubToken), nil
		case r.URL.Path == "/user/emails":
			return jsonResponse(http.StatusOK, `[{"email":"real@example.com","primary":true,"verified":true}]`), nil
		case r.URL.Path == "/user":
			return jsonResponse(http.StatusOK, `{"id":8,"login":"realcat"}`), nil
		}
		return nil, fmt.Errorf("unexpected request to %s", r.URL.String())
	})}

	p, err := oauth.NewGitHubProvider(oauth.GitHubProviderConfig{
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "https://app.test/callback",
		HTTPClient:   client,
	})
	c.Assert(err, qt.IsNil)

	c.Assert(p.AuthCodeURL("state", "challenge"), qt.Contains, "github.com/login/oauth/authorize")

	_, err = p.Exchange(context.Background(), "code", "verifier")
	c.Assert(err, qt.IsNil)
	c.Check(seen, qt.HasLen, 3)
	for _, u := range seen {
		c.Check(u, qt.Contains, "github.com")
	}
}
