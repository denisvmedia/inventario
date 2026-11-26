package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"golang.org/x/oauth2"

	"go.5x5.cz/inventario/models"
)

// DefaultOIDCDisplayName is the button label when the operator sets none.
const DefaultOIDCDisplayName = "Single sign-on"

// defaultOIDCRetryInterval is the shortest gap between two discovery
// attempts after one failed. Sign-in clicks while the provider is down
// reuse the last error instead of each waiting on a fresh timeout.
const defaultOIDCRetryInterval = 30 * time.Second

// oidcDiscoveryTimeout bounds one fetch of the discovery document.
const oidcDiscoveryTimeout = 10 * time.Second

// oidcMaxResponseBytes caps what we read from the discovery and userinfo
// endpoints. Both documents are a few kilobytes at most.
const oidcMaxResponseBytes = 1 << 20

// ErrOIDCNotReady is returned when the discovery document has not been
// fetched yet (the provider was unreachable) and the retry interval since
// the last failed attempt has not passed.
var ErrOIDCNotReady = errx.NewSentinel("oauth/oidc: provider discovery has not succeeded yet")

// OIDCProviderConfig is the operator-supplied configuration for the
// OpenID Connect provider (#2786). RedirectURL is computed by the bootstrap
// layer from the deployment's public base URL.
type OIDCProviderConfig struct {
	// IssuerURL is the provider's issuer, for example
	// https://auth.example.com. The discovery document is read from
	// {IssuerURL}/.well-known/openid-configuration, and its "issuer" must
	// match this value.
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// DisplayName labels the sign-in button ("Authelia"). Empty falls back
	// to DefaultOIDCDisplayName.
	DisplayName string
	// HTTPClient is used for discovery, the token exchange and userinfo.
	// nil → http.DefaultClient. Tests inject an httptest client here.
	HTTPClient *http.Client
	// RetryInterval is the shortest gap between discovery attempts after
	// a failure. Zero → 30s.
	RetryInterval time.Duration
	// BackchannelURL is optional: the issuer URL as the server reaches it,
	// for a setup where the issuer's public address works in the browser
	// but not from the server (a container that cannot resolve the public
	// name, an in-cluster proxy). The server's own requests (discovery,
	// token, userinfo) go there: a URL that starts with the issuer gets
	// that prefix replaced. The browser still goes to the authorization
	// endpoint from the discovery document, and the document's issuer is
	// still checked against IssuerURL.
	BackchannelURL string
}

// oidcDiscovery holds the fields of the discovery document we use.
// https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata
type oidcDiscovery struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserinfoEndpoint                  string   `json:"userinfo_endpoint"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
}

// oidcUserInfoResponse mirrors the standard claims we read from the
// userinfo endpoint. email_verified is decoded separately because some
// providers send it as the string "true".
type oidcUserInfoResponse struct {
	Sub               string          `json:"sub"`
	Email             string          `json:"email"`
	EmailVerified     json.RawMessage `json:"email_verified"`
	Name              string          `json:"name"`
	PreferredUsername string          `json:"preferred_username"`
}

// OIDCProvider is the Provider for one operator-configured OpenID Connect
// provider. The endpoints come from the discovery document, which is
// fetched on first use and cached. If the provider is unreachable, the
// fetch is retried on a later sign-in, at most once per RetryInterval, so
// an identity provider that is down at boot does not take Inventario
// down with it.
//
// Like GoogleProvider, the profile comes from the userinfo endpoint,
// called with the access token from a PKCE-protected code exchange over
// TLS. The ID token is not parsed, so no JWKS handling is needed.
type OIDCProvider struct {
	issuer        string
	backchannel   string
	clientID      string
	clientSecret  string
	redirectURL   string
	displayName   string
	httpClient    *http.Client
	retryInterval time.Duration
	now           func() time.Time

	mu          sync.Mutex
	discovered  *oidcDiscovery
	lastAttempt time.Time
	lastErr     error
}

// NewOIDCProvider constructs an OIDCProvider from cfg. It does not contact
// the provider; call Prepare to fetch the discovery document. Returns an
// error if a required field is empty or the issuer (or the back-channel
// URL, when set) is not an absolute http(s) URL.
func NewOIDCProvider(cfg OIDCProviderConfig) (*OIDCProvider, error) {
	issuer := strings.TrimRight(strings.TrimSpace(cfg.IssuerURL), "/")
	if issuer == "" {
		return nil, errxtrace.ClassifyNew("oauth/oidc: IssuerURL is required")
	}
	if err := checkBaseURL("IssuerURL", issuer); err != nil {
		return nil, err
	}
	backchannel := strings.TrimRight(strings.TrimSpace(cfg.BackchannelURL), "/")
	if backchannel != "" {
		if err := checkBaseURL("BackchannelURL", backchannel); err != nil {
			return nil, err
		}
	}
	if cfg.ClientID == "" {
		return nil, errxtrace.ClassifyNew("oauth/oidc: ClientID is required")
	}
	if cfg.ClientSecret == "" {
		return nil, errxtrace.ClassifyNew("oauth/oidc: ClientSecret is required")
	}
	if cfg.RedirectURL == "" {
		return nil, errxtrace.ClassifyNew("oauth/oidc: RedirectURL is required")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	displayName := strings.TrimSpace(cfg.DisplayName)
	if displayName == "" {
		displayName = DefaultOIDCDisplayName
	}
	retry := cfg.RetryInterval
	if retry <= 0 {
		retry = defaultOIDCRetryInterval
	}
	return &OIDCProvider{
		issuer:        issuer,
		backchannel:   backchannel,
		clientID:      cfg.ClientID,
		clientSecret:  cfg.ClientSecret,
		redirectURL:   cfg.RedirectURL,
		displayName:   displayName,
		httpClient:    client,
		retryInterval: retry,
		now:           time.Now,
	}, nil
}

// checkBaseURL reports whether raw is an absolute http(s) URL with no query
// or fragment. name is the setting it came from, for the error.
func checkBaseURL(name, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return errxtrace.ClassifyNew(fmt.Sprintf("oauth/oidc: %s must be an absolute http(s) URL (got %q)", name, raw))
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return errxtrace.ClassifyNew(fmt.Sprintf("oauth/oidc: %s must not have a query or fragment (got %q)", name, raw))
	}
	return nil
}

// serverURL returns the URL the server itself should request for u. Without
// a back channel that is u. With one, a URL under the issuer (the issuer
// itself, or the issuer followed by "/") gets the issuer prefix replaced
// by the back-channel URL; any other URL is left alone.
func (p *OIDCProvider) serverURL(u string) string {
	if p.backchannel == "" {
		return u
	}
	if u == p.issuer || strings.HasPrefix(u, p.issuer+"/") {
		return p.backchannel + u[len(p.issuer):]
	}
	return u
}

// Name implements Provider.
func (*OIDCProvider) Name() models.OAuthProvider { return models.OAuthProviderOIDC }

// DisplayName implements DisplayNamer.
func (p *OIDCProvider) DisplayName() string { return p.displayName }

// Issuer returns the configured issuer URL, without a trailing slash.
func (p *OIDCProvider) Issuer() string { return p.issuer }

// BackchannelURL returns the configured back-channel URL, without a
// trailing slash, or "" when the server reaches the issuer directly.
func (p *OIDCProvider) BackchannelURL() string { return p.backchannel }

// Prepare implements Preparer. It makes sure the discovery document has
// been fetched, fetching it now if it has not and the retry interval since
// the last failure has passed. Safe for concurrent use; concurrent callers
// wait for one fetch.
func (p *OIDCProvider) Prepare(ctx context.Context) error {
	_, err := p.discovery(ctx)
	return err
}

// discovery returns the cached discovery document, fetching it first when
// needed.
func (p *OIDCProvider) discovery(ctx context.Context) (*oidcDiscovery, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.discovered != nil {
		return p.discovered, nil
	}
	if p.lastErr != nil && p.now().Sub(p.lastAttempt) < p.retryInterval {
		return nil, errxtrace.Wrap("oauth/oidc: waiting to retry discovery", p.lastErr, ErrOIDCNotReady)
	}
	p.lastAttempt = p.now()
	d, err := p.fetchDiscovery(ctx)
	if err != nil {
		p.lastErr = err
		return nil, err
	}
	p.discovered = d
	p.lastErr = nil
	slog.InfoContext(ctx, "OAuth: OpenID Connect discovery succeeded", "issuer", p.issuer, "backchannel", p.backchannel)
	return d, nil
}

// fetchDiscovery reads and checks {issuer}/.well-known/openid-configuration.
func (p *OIDCProvider) fetchDiscovery(ctx context.Context) (*oidcDiscovery, error) {
	ctx, cancel := context.WithTimeout(ctx, oidcDiscoveryTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.serverURL(p.issuer+"/.well-known/openid-configuration"), http.NoBody)
	if err != nil {
		return nil, errxtrace.Wrap("oauth/oidc: build discovery request", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, errxtrace.Wrap("oauth/oidc: discovery request", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, errxtrace.ClassifyNew(fmt.Sprintf("oauth/oidc: discovery HTTP %d", resp.StatusCode))
	}
	var d oidcDiscovery
	if err := json.NewDecoder(io.LimitReader(resp.Body, oidcMaxResponseBytes)).Decode(&d); err != nil {
		return nil, errxtrace.Wrap("oauth/oidc: decode discovery document", err)
	}
	// OpenID Connect Discovery 1.0, section 4.3: the issuer in the
	// document must be identical to the one used to fetch it. A trailing
	// slash is ignored on both sides because operators often add one.
	if strings.TrimRight(d.Issuer, "/") != p.issuer {
		return nil, errxtrace.ClassifyNew(fmt.Sprintf("oauth/oidc: discovery issuer %q does not match the configured issuer %q", d.Issuer, p.issuer))
	}
	if d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" || d.UserinfoEndpoint == "" {
		return nil, errxtrace.ClassifyNew("oauth/oidc: discovery document lacks authorization_endpoint, token_endpoint or userinfo_endpoint")
	}
	if len(d.CodeChallengeMethodsSupported) > 0 && !slices.Contains(d.CodeChallengeMethodsSupported, "S256") {
		// Sign-in still sends S256. A provider that does not support it
		// ignores the challenge or refuses the request; either way the
		// operator should know why.
		slog.WarnContext(ctx, "OAuth: OpenID Connect provider does not list PKCE S256 as supported",
			"issuer", p.issuer, "code_challenge_methods_supported", d.CodeChallengeMethodsSupported)
	}
	return &d, nil
}

// oauth2Config builds the oauth2 client configuration from the discovered
// endpoints.
func (p *OIDCProvider) oauth2Config(d *oidcDiscovery) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		RedirectURL:  p.redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint: oauth2.Endpoint{
			// The browser follows AuthURL, so it stays the public one; the
			// server posts to TokenURL, so it goes through the back channel.
			AuthURL:   d.AuthorizationEndpoint,
			TokenURL:  p.serverURL(d.TokenEndpoint),
			AuthStyle: tokenAuthStyle(d.TokenEndpointAuthMethodsSupported),
		},
	}
}

// tokenAuthStyle picks how the client authenticates at the token endpoint.
// client_secret_basic is the default in the spec, so it wins when the
// provider lists it or lists nothing; client_secret_post is used only when
// it is the one listed.
func tokenAuthStyle(methods []string) oauth2.AuthStyle {
	if len(methods) == 0 || slices.Contains(methods, "client_secret_basic") {
		return oauth2.AuthStyleInHeader
	}
	if slices.Contains(methods, "client_secret_post") {
		return oauth2.AuthStyleInParams
	}
	return oauth2.AuthStyleInHeader
}

// AuthCodeURL implements Provider. PKCE S256 is mandatory. It returns ""
// when discovery has not succeeded yet; the handler calls Prepare first.
func (p *OIDCProvider) AuthCodeURL(state, codeChallenge string) string {
	p.mu.Lock()
	d := p.discovered
	p.mu.Unlock()
	if d == nil {
		return ""
	}
	return p.oauth2Config(d).AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// Exchange implements Provider: it trades the code for an access token and
// reads the profile from the userinfo endpoint.
func (p *OIDCProvider) Exchange(ctx context.Context, code, codeVerifier string) (Profile, error) {
	d, err := p.discovery(ctx)
	if err != nil {
		return Profile{}, err
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
	tok, err := p.oauth2Config(d).Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return Profile{}, errxtrace.Wrap("oauth/oidc: token exchange", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.serverURL(d.UserinfoEndpoint), http.NoBody)
	if err != nil {
		return Profile{}, errxtrace.Wrap("oauth/oidc: build userinfo request", err)
	}
	req.Header.Set("Accept", "application/json")
	tok.SetAuthHeader(req)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return Profile{}, errxtrace.Wrap("oauth/oidc: userinfo request", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return Profile{}, errxtrace.ClassifyNew(fmt.Sprintf("oauth/oidc: userinfo HTTP %d: %s", resp.StatusCode, string(body)))
	}
	// A provider may answer with a signed JWT (application/jwt) when the
	// client is registered for it. We ask for JSON and only accept JSON.
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		return Profile{}, errxtrace.ClassifyNew(fmt.Sprintf("oauth/oidc: userinfo answered with %q; configure the client to return plain JSON from userinfo", ct))
	}
	var info oidcUserInfoResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, oidcMaxResponseBytes)).Decode(&info); err != nil {
		return Profile{}, errxtrace.Wrap("oauth/oidc: decode userinfo", err)
	}
	if info.Sub == "" {
		return Profile{}, errxtrace.ClassifyNew("oauth/oidc: userinfo missing sub")
	}
	if info.Email == "" {
		return Profile{}, errxtrace.ClassifyNew("oauth/oidc: userinfo missing email; request the email scope for this client")
	}

	displayName := info.Name
	if displayName == "" {
		displayName = info.PreferredUsername
	}
	return Profile{
		ProviderUserID: info.Sub,
		Email:          info.Email,
		EmailVerified:  claimIsTrue(info.EmailVerified),
		DisplayName:    displayName,
	}, nil
}

// claimIsTrue reads a boolean claim that a provider may send as a JSON
// boolean or as the string "true". Anything else, including a missing
// claim, is false.
func claimIsTrue(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.EqualFold(strings.TrimSpace(s), "true")
	}
	return false
}
