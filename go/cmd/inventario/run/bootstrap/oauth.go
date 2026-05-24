package bootstrap

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/services/oauth"
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
		provider, err := oauth.NewGitHubProvider(oauth.GitHubProviderConfig{
			ClientID:     id,
			ClientSecret: secret,
			RedirectURL:  base + "/api/v1/auth/oauth/github/callback",
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
