package bootstrap

import (
	"fmt"
	"log/slog"
	"strings"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/services/oauth"
)

// oauthSetup is the realized OAuth configuration produced by buildOAuth.
// All three fields land on AuthParams; the registry MAY be empty when no
// provider is configured but the state signer is built unconditionally so
// the state cookie write path is always well-defined.
type oauthSetup struct {
	Registry    *oauth.Registry
	StateSigner *oauth.StateSigner
}

// wireOAuth builds the OAuth provider registry + state signer and
// installs them on params. Extracted as a free helper so the call site
// in buildServerParams stays a single statement (keeps the parent
// function under the gocyclo budget).
func wireOAuth(cfg *Config, params *apiserver.Params) error {
	setup, err := buildOAuth(cfg)
	if err != nil {
		return err
	}
	params.OAuthRegistry = setup.Registry
	params.OAuthStateSigner = setup.StateSigner
	return nil
}

// buildOAuth assembles the OAuth provider registry + state signer from
// the operator-supplied config. A provider is registered only when its
// (client_id, client_secret) pair is set AND OAuthRedirectBaseURL is
// non-empty. The state signer is always built (with a random fallback
// key when one is not configured).
func buildOAuth(cfg *Config) (oauthSetup, error) {
	stateKey, err := getOAuthStateKey(cfg.OAuthStateKey)
	if err != nil {
		return oauthSetup{}, fmt.Errorf("oauth bootstrap: state key: %w", err)
	}
	signer, err := oauth.NewStateSigner(stateKey)
	if err != nil {
		return oauthSetup{}, fmt.Errorf("oauth bootstrap: state signer: %w", err)
	}

	registry := oauth.NewRegistry()

	base := strings.TrimRight(strings.TrimSpace(cfg.OAuthRedirectBaseURL), "/")
	if base == "" {
		// No public base URL → no providers can be wired. The state
		// signer still lives so the /auth/oauth/providers endpoint
		// surfaces an empty list cleanly.
		slog.Info("OAuth: no redirect base URL configured; providers disabled")
		return oauthSetup{Registry: registry, StateSigner: signer}, nil
	}

	if id, secret := strings.TrimSpace(cfg.OAuthGoogleClientID), strings.TrimSpace(cfg.OAuthGoogleClientSecret); id != "" && secret != "" {
		overrides, err := resolveGoogleOverrides(cfg)
		if err != nil {
			return oauthSetup{}, err
		}
		provider, err := oauth.NewGoogleProvider(oauth.GoogleProviderConfig{
			ClientID:     id,
			ClientSecret: secret,
			RedirectURL:  base + "/api/v1/auth/oauth/google/callback",
			AuthURL:      overrides.AuthURL,
			TokenURL:     overrides.TokenURL,
			UserInfoURL:  overrides.UserInfoURL,
		})
		if err != nil {
			return oauthSetup{}, fmt.Errorf("oauth bootstrap: google: %w", err)
		}
		if err := registry.Register(provider); err != nil {
			return oauthSetup{}, fmt.Errorf("oauth bootstrap: register google: %w", err)
		}
		slog.Info("OAuth: Google provider enabled")
	}

	if id, secret := strings.TrimSpace(cfg.OAuthGitHubClientID), strings.TrimSpace(cfg.OAuthGitHubClientSecret); id != "" && secret != "" {
		overrides, err := resolveGitHubOverrides(cfg)
		if err != nil {
			return oauthSetup{}, err
		}
		provider, err := oauth.NewGitHubProvider(oauth.GitHubProviderConfig{
			ClientID:      id,
			ClientSecret:  secret,
			RedirectURL:   base + "/api/v1/auth/oauth/github/callback",
			AuthURL:       overrides.AuthURL,
			TokenURL:      overrides.TokenURL,
			UserURL:       overrides.UserURL,
			UserEmailsURL: overrides.UserEmailsURL,
		})
		if err != nil {
			return oauthSetup{}, fmt.Errorf("oauth bootstrap: github: %w", err)
		}
		if err := registry.Register(provider); err != nil {
			return oauthSetup{}, fmt.Errorf("oauth bootstrap: register github: %w", err)
		}
		slog.Info("OAuth: GitHub provider enabled")
	}

	return oauthSetup{Registry: registry, StateSigner: signer}, nil
}

// googleOverrides holds the resolved (auth, token, userinfo) endpoint URL
// overrides. Empty fields mean "use the real Google endpoint"; all three
// non-empty means the e2e stub server is wired in.
type googleOverrides struct {
	AuthURL     string
	TokenURL    string
	UserInfoURL string
}

// resolveGoogleOverrides reads the test-only Google endpoint URL overrides
// from cfg and enforces all-or-nothing. Returns the resolved overrides,
// or a descriptive error if exactly 1 or 2 of the three are set.
//
// A partial set is a security hazard: mixing the stub authorize URL with
// the real Google token endpoint would leak the authorization code +
// client secret to Google when the stub is the expected recipient. The
// e2e harness only ever flips all three together; refuse to start in any
// other shape so a misconfiguration can't silently land in production.
func resolveGoogleOverrides(cfg *Config) (googleOverrides, error) {
	ov := googleOverrides{
		AuthURL:     strings.TrimSpace(cfg.OAuthGoogleAuthURLOverride),
		TokenURL:    strings.TrimSpace(cfg.OAuthGoogleTokenURLOverride),
		UserInfoURL: strings.TrimSpace(cfg.OAuthGoogleUserInfoURLOverride),
	}
	count := 0
	if ov.AuthURL != "" {
		count++
	}
	if ov.TokenURL != "" {
		count++
	}
	if ov.UserInfoURL != "" {
		count++
	}
	if count != 0 && count != 3 {
		return googleOverrides{}, fmt.Errorf(
			"oauth bootstrap: google endpoint overrides must set auth, token, and userinfo together (got auth=%q, token=%q, userinfo=%q)",
			ov.AuthURL, ov.TokenURL, ov.UserInfoURL,
		)
	}
	if count == 3 {
		// LOUD warning: these overrides should never appear in a
		// production deployment. The e2e harness flips them on so the
		// stub server can serve Google's three endpoints.
		slog.Warn("OAuth: Google endpoint overrides active — TEST-ONLY; never set in production",
			"auth_override", ov.AuthURL,
			"token_override", ov.TokenURL,
			"userinfo_override", ov.UserInfoURL)
	}
	return ov, nil
}

// githubOverrides holds the resolved (auth, token, user, user-emails)
// endpoint URL overrides. Empty fields mean "use the real GitHub endpoint";
// all four non-empty means the e2e stub server is wired in.
type githubOverrides struct {
	AuthURL       string
	TokenURL      string
	UserURL       string
	UserEmailsURL string
}

// resolveGitHubOverrides is resolveGoogleOverrides for GitHub, with four
// endpoints instead of three: GitHub serves the profile and the verified
// email addresses from separate URLs.
//
// The all-or-nothing rule carries the same reason. A partial set would mix
// the stub's authorize URL with the real GitHub token endpoint, handing the
// authorization code and the client secret to GitHub when the stub is the
// expected recipient. The e2e harness only ever flips all four together;
// refuse to start in any other shape so a misconfiguration cannot land in
// production quietly.
func resolveGitHubOverrides(cfg *Config) (githubOverrides, error) {
	ov := githubOverrides{
		AuthURL:       strings.TrimSpace(cfg.OAuthGitHubAuthURLOverride),
		TokenURL:      strings.TrimSpace(cfg.OAuthGitHubTokenURLOverride),
		UserURL:       strings.TrimSpace(cfg.OAuthGitHubUserURLOverride),
		UserEmailsURL: strings.TrimSpace(cfg.OAuthGitHubUserEmailsURLOverride),
	}
	count := 0
	for _, v := range []string{ov.AuthURL, ov.TokenURL, ov.UserURL, ov.UserEmailsURL} {
		if v != "" {
			count++
		}
	}
	if count != 0 && count != 4 {
		return githubOverrides{}, fmt.Errorf(
			"oauth bootstrap: github endpoint overrides must set auth, token, user, and user-emails together (got auth=%q, token=%q, user=%q, user_emails=%q)",
			ov.AuthURL, ov.TokenURL, ov.UserURL, ov.UserEmailsURL,
		)
	}
	if count == 4 {
		// LOUD warning: these overrides should never appear in a
		// production deployment. The e2e harness flips them on so the
		// stub server can serve GitHub's four endpoints.
		slog.Warn("OAuth: GitHub endpoint overrides active — TEST-ONLY; never set in production",
			"auth_override", ov.AuthURL,
			"token_override", ov.TokenURL,
			"user_override", ov.UserURL,
			"user_emails_override", ov.UserEmailsURL)
	}
	return ov, nil
}
