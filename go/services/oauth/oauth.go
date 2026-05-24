// Package oauth implements the third-party sign-in flow for #1394: a
// provider-agnostic Profile + Provider abstraction plus the two concrete
// providers Inventario ships with (Google and GitHub). The package is
// deliberately stateless — the OAuth callback path persists nothing of its
// own; the apiserver handlers in apiserver/oauth.go own the registry +
// token-issue side effects.
//
// The provider abstraction normalizes Google's OIDC userinfo and GitHub's
// /user + /user/emails responses into a single Profile shape so the
// find-or-create-or-link logic in the callback doesn't need to branch on
// provider name. PKCE (RFC 7636) is enforced on every flow; state cookies
// are signed via StateSigner with a short TTL so a stolen state value
// cannot be replayed after the user closes the browser tab.
package oauth

import (
	"context"
	"fmt"

	"github.com/denisvmedia/inventario/models"
)

// Profile is the normalized view of the user's account at the OAuth
// provider, as returned by Provider.Exchange. The four fields are the
// minimum the callback needs:
//
//   - ProviderUserID is the stable subject identifier ("sub" on Google,
//     numeric "id" on GitHub). NEVER use email or username as the lookup
//     key — both can be reassigned at the provider, while the
//     provider-issued subject is documented stable.
//   - Email is the primary email address. GitHub callers must take care to
//     surface ONLY a verified primary email here; the Google flow returns
//     verified=true (OIDC) so any address it returns is trustworthy.
//   - EmailVerified pins whether the provider considers the email proven.
//     The callback uses this to decide whether to auto-link a new OAuth
//     identity to an existing user whose `users.email` matches: only when
//     verified=true is it safe to do so (otherwise a malicious account
//     creator could squat someone's email at the provider and hijack the
//     account).
//   - DisplayName is the human-friendly name used to seed `users.name` on
//     first sign-up. Falls back to the local-part of Email when the
//     provider returns no display name.
type Profile struct {
	ProviderUserID string
	Email          string
	EmailVerified  bool
	DisplayName    string
}

// Provider abstracts a single OAuth 2.0 / OIDC provider. The interface is
// intentionally narrow: the handlers only ever need (a) the redirect URL
// for the start path and (b) the normalized Profile for the callback path.
// Everything provider-specific (id_token parsing, multi-email selection,
// scope set) is hidden inside the implementation.
type Provider interface {
	// Name returns the provider's stable identifier. The value is the
	// same string used in URL paths (`/api/v1/auth/oauth/{provider}/...`)
	// and `login_events.method` (`oauth_{provider}`), so it MUST be one
	// of the models.OAuthProvider* constants.
	Name() models.OAuthProvider

	// AuthCodeURL returns the URL the browser should be redirected to in
	// order to start the authorization-code flow. The implementation
	// MUST include the codeChallenge as a PKCE S256 challenge so the
	// /callback exchange can prove possession of the original code
	// verifier (RFC 7636). The state value is the opaque token the
	// caller signed with StateSigner — the implementation just forwards
	// it as-is.
	AuthCodeURL(state, codeChallenge string) string

	// Exchange swaps the authorization code for tokens and returns a
	// normalized Profile. The codeVerifier MUST be the same value that
	// produced the codeChallenge in AuthCodeURL, otherwise the provider
	// will reject the exchange (which the implementation must surface
	// as a non-nil error). Implementations are responsible for
	// validating the provider's response — id_token signature for
	// Google, email_verified flag for GitHub, etc.
	Exchange(ctx context.Context, code, codeVerifier string) (Profile, error)
}

// Registry is the set of OAuth providers enabled in this deployment.
// Bootstrap registers each provider for which the operator supplied a
// (client_id, client_secret, redirect_base_url) triple; providers without
// configuration are omitted entirely so the `/auth/oauth/providers` listing
// reflects exactly what works.
//
// The zero value is a valid empty Registry — handlers must tolerate it
// (returning an empty list from the providers endpoint, 404 from start /
// callback). This is the "OAuth not configured" deployment shape.
type Registry struct {
	providers map[models.OAuthProvider]Provider
	order     []models.OAuthProvider
}

// NewRegistry constructs an empty Registry. Use Register to add providers.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[models.OAuthProvider]Provider),
	}
}

// Register adds a provider to the registry. Registration order is
// preserved by Enabled() so the FE list reads stable across restarts
// (operators typically add Google first, GitHub second).
//
// Registering a provider whose name is invalid per OAuthProvider.IsValid()
// returns an error rather than panicking — the bootstrap layer is allowed
// to make typos and we want a loud boot failure, not a silent miswire.
// Re-registering the same provider name replaces the previous entry; this
// is a feature for tests that inject a stub Provider.
func (r *Registry) Register(p Provider) error {
	if p == nil {
		return fmt.Errorf("oauth: nil provider")
	}
	name := p.Name()
	if !name.IsValid() {
		return fmt.Errorf("oauth: invalid provider name %q", string(name))
	}
	if _, exists := r.providers[name]; !exists {
		r.order = append(r.order, name)
	}
	r.providers[name] = p
	return nil
}

// Enabled returns the names of all registered providers in registration
// order. A handler turns this into the public listing surfaced by
// `GET /auth/oauth/providers`.
func (r *Registry) Enabled() []models.OAuthProvider {
	if r == nil {
		return nil
	}
	out := make([]models.OAuthProvider, len(r.order))
	copy(out, r.order)
	return out
}

// Get returns the provider registered for name, or (nil, false) when the
// provider is not registered. Handlers branch on the boolean to return 404
// for unknown providers without leaking which providers are enabled in
// other deployments.
func (r *Registry) Get(name models.OAuthProvider) (Provider, bool) {
	if r == nil {
		return nil, false
	}
	p, ok := r.providers[name]
	return p, ok
}

// Has reports whether a provider with the given name is registered.
// Equivalent to Get + boolean cast; provided for readability at call sites
// that don't need the Provider value itself.
func (r *Registry) Has(name models.OAuthProvider) bool {
	_, ok := r.Get(name)
	return ok
}
