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
		provider, err := oauth.NewGoogleProvider(oauth.GoogleProviderConfig{
			ClientID:     id,
			ClientSecret: secret,
			RedirectURL:  base + "/api/v1/auth/oauth/google/callback",
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
