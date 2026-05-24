package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"

	"github.com/denisvmedia/inventario/models"
)

// githubUserURL returns the authenticated user's profile (id, login, name).
// githubUserEmailsURL returns the list of attached email addresses with
// per-row verified + primary flags. We need both — the /user response does
// NOT include the verified flag on the email field, and a GitHub user with
// a hidden primary email returns "email":null on /user but still has the
// real address on /user/emails.
const (
	githubUserURL       = "https://api.github.com/user"
	githubUserEmailsURL = "https://api.github.com/user/emails"
)

// githubUserResponse is the subset of fields we read from /user. The
// numeric `id` is the stable subject we persist as ProviderUserID — GitHub
// documents this as never reassigned even after a username change.
type githubUserResponse struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// githubEmailEntry mirrors one row of GET /user/emails. `primary` marks
// the user's preferred address; `verified` reflects whether they've
// confirmed it. We require both before accepting an email — an unverified
// primary would be a free account-takeover vector (anyone could claim
// your-email@example.com at GitHub and inherit any inventario account
// keyed on it).
type githubEmailEntry struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// GitHubProviderConfig is the operator-supplied configuration for GitHub.
// HTTPClient is overridable for tests.
type GitHubProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// HTTPClient overrides the *http.Client used to fetch /user and
	// /user/emails. nil → http.DefaultClient.
	HTTPClient *http.Client
}

// GitHubProvider implements Provider against GitHub's OAuth 2.0 endpoints.
// Scopes are fixed at `read:user user:email` so the app can read the
// public profile and the verified primary email.
type GitHubProvider struct {
	cfg        *oauth2.Config
	httpClient *http.Client
}

// NewGitHubProvider constructs a GitHubProvider from cfg. Returns an
// error if ClientID, ClientSecret, or RedirectURL is empty.
func NewGitHubProvider(cfg GitHubProviderConfig) (*GitHubProvider, error) {
	if cfg.ClientID == "" {
		return nil, errxtrace.ClassifyNew("oauth/github: ClientID is required")
	}
	if cfg.ClientSecret == "" {
		return nil, errxtrace.ClassifyNew("oauth/github: ClientSecret is required")
	}
	if cfg.RedirectURL == "" {
		return nil, errxtrace.ClassifyNew("oauth/github: RedirectURL is required")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &GitHubProvider{
		cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     githuboauth.Endpoint,
		},
		httpClient: client,
	}, nil
}

// Name implements Provider.
func (*GitHubProvider) Name() models.OAuthProvider { return models.OAuthProviderGitHub }

// AuthCodeURL implements Provider. GitHub honours PKCE since August 2022
// per their docs — we always include code_challenge_method=S256.
func (p *GitHubProvider) AuthCodeURL(state, codeChallenge string) string {
	return p.cfg.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// Exchange implements Provider.
func (p *GitHubProvider) Exchange(ctx context.Context, code, codeVerifier string) (Profile, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)

	tok, err := p.cfg.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return Profile{}, errxtrace.Wrap("oauth/github: token exchange", err)
	}

	user, err := p.fetchUser(ctx, tok)
	if err != nil {
		return Profile{}, err
	}
	if user.ID == 0 {
		return Profile{}, errxtrace.ClassifyNew("oauth/github: /user response missing id")
	}

	email, verified, err := p.resolvePrimaryEmail(ctx, tok, user.Email)
	if err != nil {
		return Profile{}, err
	}

	displayName := user.Name
	if displayName == "" {
		displayName = user.Login
	}

	return Profile{
		ProviderUserID: strconv.FormatInt(user.ID, 10),
		Email:          email,
		EmailVerified:  verified,
		DisplayName:    displayName,
	}, nil
}

// fetchUser GETs /user and decodes the subset we need. Helper extracted so
// the body close / status check / json decode logic isn't repeated inline.
func (p *GitHubProvider) fetchUser(ctx context.Context, tok *oauth2.Token) (githubUserResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserURL, http.NoBody)
	if err != nil {
		return githubUserResponse{}, errxtrace.Wrap("oauth/github: build /user request", err)
	}
	tok.SetAuthHeader(req)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return githubUserResponse{}, errxtrace.Wrap("oauth/github: /user request", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return githubUserResponse{}, errxtrace.ClassifyNew(fmt.Sprintf("oauth/github: /user HTTP %d: %s", resp.StatusCode, string(body)))
	}
	var u githubUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return githubUserResponse{}, errxtrace.Wrap("oauth/github: decode /user", err)
	}
	return u, nil
}

// resolvePrimaryEmail picks the user's primary, verified email address.
//
// GitHub's /user response only includes the email when the user has set
// it as public on their profile. To support users who keep it private,
// we hit /user/emails (gated by the `user:email` scope) and find the
// primary+verified row. If /user did surface a public email AND that
// same address is verified on /user/emails, we still go through the
// emails list so the verified flag is grounded in /user/emails (the
// authoritative source).
func (p *GitHubProvider) resolvePrimaryEmail(ctx context.Context, tok *oauth2.Token, userEmail string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserEmailsURL, http.NoBody)
	if err != nil {
		return "", false, errxtrace.Wrap("oauth/github: build /user/emails request", err)
	}
	tok.SetAuthHeader(req)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", false, errxtrace.Wrap("oauth/github: /user/emails request", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", false, errxtrace.ClassifyNew(fmt.Sprintf("oauth/github: /user/emails HTTP %d: %s", resp.StatusCode, string(body)))
	}
	var emails []githubEmailEntry
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", false, errxtrace.Wrap("oauth/github: decode /user/emails", err)
	}

	// 1st pass — exact primary+verified match.
	for _, e := range emails {
		if e.Primary && e.Verified && e.Email != "" {
			return e.Email, true, nil
		}
	}
	// 2nd pass — any verified email, in the order GitHub returned them.
	// Saves the user from a hard failure when their primary is unverified
	// but they have at least one other verified address.
	for _, e := range emails {
		if e.Verified && e.Email != "" {
			return e.Email, true, nil
		}
	}
	// 3rd pass — fall back to the /user payload's email if /user/emails
	// returned only unverified rows. EmailVerified=false signals to the
	// callback that auto-link MUST NOT happen.
	if userEmail != "" {
		return userEmail, false, nil
	}
	return "", false, errxtrace.ClassifyNew("oauth/github: no usable email address found")
}
