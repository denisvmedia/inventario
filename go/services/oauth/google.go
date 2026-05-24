package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/denisvmedia/inventario/models"
)

// googleUserInfoURL is Google's OIDC userinfo endpoint. We call this with
// the access token rather than parsing the id_token JWT because (a) the
// access token is already validated by the exchange, (b) the userinfo
// endpoint is documented to return only verified claims, and (c) Google
// rotates the id_token signing keys on a schedule that would force this
// package to maintain a JWK fetcher — a needless dependency given the
// userinfo path returns exactly the same fields.
const googleUserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"

// googleUserInfoResponse mirrors the fields we read from googleUserInfoURL.
// Google's response is documented at:
// https://developers.google.com/identity/openid-connect/openid-connect#an-id-tokens-payload
// — we ignore everything except Sub / Email / EmailVerified / Name; the
// extra fields are tolerated by json.Unmarshal.
type googleUserInfoResponse struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
}

// GoogleProviderConfig is the operator-supplied configuration for the
// Google provider. RedirectURL is computed by the bootstrap layer from
// the deployment's public base URL.
type GoogleProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// HTTPClient overrides the *http.Client used to fetch the userinfo
	// endpoint. nil → http.DefaultClient. Tests inject an httptest stub
	// here.
	HTTPClient *http.Client
	// AuthURL, TokenURL, and UserInfoURL are test-only overrides that
	// redirect the provider's three external endpoints at a local stub.
	// Production deployments MUST leave these empty so the provider uses
	// google.Endpoint + googleUserInfoURL. Wiring is gated at the
	// bootstrap layer behind the INVENTARIO_RUN_OAUTH_GOOGLE_*_OVERRIDE
	// env vars (#1394 e2e). See bootstrap/oauth.go for the gate.
	AuthURL     string
	TokenURL    string
	UserInfoURL string
}

// GoogleProvider is the Provider implementation backed by Google's OIDC
// endpoints. Scopes are fixed at `openid email profile` — the minimum
// needed to read a stable subject + a verified email + a display name.
type GoogleProvider struct {
	cfg         *oauth2.Config
	httpClient  *http.Client
	userInfoURL string
}

// NewGoogleProvider constructs a GoogleProvider from cfg. Returns an
// error if ClientID, ClientSecret, or RedirectURL is empty — those are
// not catchable at request time.
func NewGoogleProvider(cfg GoogleProviderConfig) (*GoogleProvider, error) {
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("oauth/google: ClientID is required")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("oauth/google: ClientSecret is required")
	}
	if cfg.RedirectURL == "" {
		return nil, fmt.Errorf("oauth/google: RedirectURL is required")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	// Endpoint defaults to google.Endpoint. AuthURL / TokenURL are
	// individually overridable so a deployment can swap just one (e.g.
	// pointing the token endpoint at a proxy) without giving up the
	// real auth URL — and so the e2e stub can drive both with a single
	// httptest.Server.
	endpoint := google.Endpoint
	if cfg.AuthURL != "" {
		endpoint.AuthURL = cfg.AuthURL
	}
	if cfg.TokenURL != "" {
		endpoint.TokenURL = cfg.TokenURL
	}
	userInfoURL := googleUserInfoURL
	if cfg.UserInfoURL != "" {
		userInfoURL = cfg.UserInfoURL
	}
	return &GoogleProvider{
		cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     endpoint,
		},
		httpClient:  client,
		userInfoURL: userInfoURL,
	}, nil
}

// Name implements Provider.
func (*GoogleProvider) Name() models.OAuthProvider { return models.OAuthProviderGoogle }

// AuthCodeURL implements Provider. PKCE S256 is mandatory.
func (p *GoogleProvider) AuthCodeURL(state, codeChallenge string) string {
	return p.cfg.AuthCodeURL(
		state,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// Exchange implements Provider. The codeVerifier is passed back so the
// provider can re-derive the S256 challenge and reject a substituted
// authorization code.
func (p *GoogleProvider) Exchange(ctx context.Context, code, codeVerifier string) (Profile, error) {
	// The oauth2 client uses ctx.Value(oauth2.HTTPClient) to override the
	// HTTP client used for the token exchange itself. This lets tests
	// drive the exchange against an httptest.Server while production
	// uses the default client.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)

	tok, err := p.cfg.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return Profile{}, fmt.Errorf("oauth/google: token exchange: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userInfoURL, http.NoBody)
	if err != nil {
		return Profile{}, fmt.Errorf("oauth/google: build userinfo request: %w", err)
	}
	tok.SetAuthHeader(req)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return Profile{}, fmt.Errorf("oauth/google: userinfo request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return Profile{}, fmt.Errorf("oauth/google: userinfo HTTP %d: %s", resp.StatusCode, string(body))
	}
	var info googleUserInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return Profile{}, fmt.Errorf("oauth/google: decode userinfo: %w", err)
	}
	if info.Sub == "" {
		return Profile{}, fmt.Errorf("oauth/google: userinfo missing sub")
	}
	if info.Email == "" {
		return Profile{}, fmt.Errorf("oauth/google: userinfo missing email")
	}

	displayName := info.Name
	if displayName == "" {
		displayName = info.GivenName
	}

	return Profile{
		ProviderUserID: info.Sub,
		Email:          info.Email,
		EmailVerified:  info.EmailVerified,
		DisplayName:    displayName,
	}, nil
}
