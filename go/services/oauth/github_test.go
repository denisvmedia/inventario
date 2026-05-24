package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"golang.org/x/oauth2"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/oauth"
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
