package apiserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	oauthsvc "github.com/denisvmedia/inventario/services/oauth"
)

// oauthStateCookieName is the cookie that pins a particular signed state
// token to a particular browser. The handler writes it on /start and
// reads + clears it on /callback; if the value the browser presents
// doesn't match the value in the cookie, the call is rejected (CSRF +
// session-fixation defence-in-depth on top of the HMAC signature).
const oauthStateCookieName = "oauth_state" // #nosec G101 -- cookie name, not a credential

// oauthStateCookiePath scopes the OAuth state cookie to the OAuth route
// family. A narrower path means the browser doesn't gratuitously send
// the state cookie on unrelated /auth/* requests during normal use.
const oauthStateCookiePath = "/api/v1/auth/oauth"

// oauthStateCookieMaxAge mirrors the default state TTL so a stale cookie
// can't outlive the signed payload it goes with. Five minutes is plenty
// for the normal "click sign-in → consent → land back" round-trip; a
// user who walked away should just start over.
const oauthStateCookieMaxAge = 5 * 60

// allowedRedirectPrefixes are the FE app paths the OAuth callback is
// allowed to 302 the user to after a successful sign-in. Anything not in
// this list collapses to "/" — defence-in-depth against open-redirect on
// top of SanitizeRedirect.
var allowedRedirectPrefixes = []string{
	"/", "/dashboard", "/locations", "/areas", "/commodities", "/groups",
	"/settings", "/profile", "/login", "/notifications",
}

// providerListEntry is the public shape of GET /auth/oauth/providers.
type providerListEntry struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// providerListResponse wraps the providers list so the FE codegen
// produces a typed shape rather than a bare array.
type providerListResponse struct {
	Providers []providerListEntry `json:"providers"`
}

// linkedIdentityEntry is one row in GET /auth/oauth/identities.
type linkedIdentityEntry struct {
	Provider string    `json:"provider"`
	Email    string    `json:"email"`
	LinkedAt time.Time `json:"linked_at"`
}

// linkedIdentitiesResponse wraps the identities list.
type linkedIdentitiesResponse struct {
	Identities []linkedIdentityEntry `json:"identities"`
}

// OAuthAPI carries the handler state for the OAuth sub-router. Reuses
// the AuthAPI helpers (issueAccessToken, persistRefreshToken, …) so the
// callback's token-issue path stays identical to the password login path.
type OAuthAPI struct {
	auth          *AuthAPI
	registry      *oauthsvc.Registry
	state         *oauthsvc.StateSigner
	identityStore registry.OAuthIdentityRegistry
	userRegistry  registry.UserRegistry
}

// OAuthParams aggregates everything the OAuth sub-router needs to mount.
// All fields are required unless documented otherwise.
type OAuthParams struct {
	// Auth is the existing AuthAPI; the OAuth handlers reuse its
	// token-issuance helpers so an OAuth login completes via the
	// identical refresh-token + access-token path the password login uses.
	Auth *AuthAPI
	// Registry holds the enabled OAuth providers. A nil registry is
	// permitted — the FE then receives an empty providers list and the
	// start/callback paths 404 on any provider name.
	Registry *oauthsvc.Registry
	// State signs the per-request state tokens. Required even when the
	// Registry is empty so the route mount is well-formed.
	State *oauthsvc.StateSigner
	// IdentityStore is the OAuthIdentityRegistry (service-mode) used by
	// the callback to look up existing links and persist new ones.
	IdentityStore registry.OAuthIdentityRegistry
	// UserRegistry is needed independently of Auth because the OAuth
	// callback runs partially outside the access-token context (the
	// caller has no session yet).
	UserRegistry registry.UserRegistry
}

// OAuth returns a chi route builder mounting the OAuth endpoints under
// the parent /auth router. The public endpoints (`/providers`, `/{p}/start`,
// `/{p}/callback`) run without RequireAuth — by definition the caller has
// no session yet. `/identities` and the unlink DELETE are wrapped with
// requireAuth (the FE reaches them via fetch, so a Bearer header is always
// present). `/{p}/link/start` is deliberately NOT wrapped — see below.
//
// Why link-start cannot use the header-only RequireAuth (SEC-L2): the
// link-start endpoint is a GET reached by a top-level browser navigation
// (window.location.assign from Settings → Connected Accounts) that 302s to
// the provider. A top-level navigation cannot carry an Authorization header,
// so a header-only middleware would reject every real click with 401. The
// handler instead resolves the caller from the SameSite=Strict refresh-token
// cookie (api.auth.userFromBrowserSession). That SameSite=Strict scoping is
// what preserves CSRF safety: a cross-site page cannot cause the cookie to
// ride along on a forged navigation, so an attacker cannot start "link MY
// provider account to the victim's session" from another origin. Layered on
// top: (a) the cookie-bound signed-state check on the callback rejects any
// state not minted in the same browser, and (b) the operator-side allow-list
// of redirect prefixes prevents exfiltrating the resulting session.
func OAuth(params OAuthParams, requireAuth func(http.Handler) http.Handler) func(r chi.Router) {
	api := &OAuthAPI{
		auth:          params.Auth,
		registry:      params.Registry,
		state:         params.State,
		identityStore: params.IdentityStore,
		userRegistry:  params.UserRegistry,
	}
	return func(r chi.Router) {
		r.Get("/providers", api.handleListProviders)
		r.Get("/{provider}/start", api.handleStart)
		r.Get("/{provider}/callback", api.handleCallback)
		// link/start authenticates via the refresh cookie inside the handler
		// (top-level navigation has no Authorization header); see handleLinkStart.
		r.Get("/{provider}/link/start", api.handleLinkStart)

		r.With(requireAuth).Get("/identities", api.handleListIdentities)
		r.With(requireAuth).Delete("/{provider}", api.handleUnlink)
	}
}

// handleListProviders surfaces the providers enabled in this deployment.
// @Summary List OAuth providers enabled in this deployment
// @Description Returns the providers operators have configured (Google, GitHub). Empty list when OAuth is not configured.
// @Tags oauth
// @Produce json
// @Success 200 {object} providerListResponse "OK"
// @Router /auth/oauth/providers [get]
func (api *OAuthAPI) handleListProviders(w http.ResponseWriter, _ *http.Request) {
	resp := providerListResponse{Providers: []providerListEntry{}}
	if api.registry != nil {
		for _, name := range api.registry.Enabled() {
			resp.Providers = append(resp.Providers, providerListEntry{
				Name:        string(name),
				DisplayName: displayNameFor(name),
			})
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// displayNameFor returns a human-friendly label for the provider. Kept
// out of the model layer so the wire string ("google") never has to
// change when marketing wants the button to say "Google Workspace".
func displayNameFor(p models.OAuthProvider) string {
	switch p {
	case models.OAuthProviderGoogle:
		return "Google"
	case models.OAuthProviderGitHub:
		return "GitHub"
	}
	return string(p)
}

// handleStart kicks off the authorization-code flow: signs a state +
// PKCE pair, writes the state cookie, and 302s the browser to the
// provider.
// @Summary Start OAuth sign-in flow
// @Description Begins the authorization-code flow against the named provider. The handler signs short-lived state + PKCE pair and 302s the browser to the provider's consent screen.
// @Tags oauth
// @Param provider path string true "Provider name (google|github)"
// @Param redirect query string false "Relative FE path to land on after sign-in"
// @Success 302 "Redirect to provider"
// @Failure 404 {string} string "Unknown provider"
// @Failure 500 {string} string "Internal error generating state token"
// @Router /auth/oauth/{provider}/start [get]
func (api *OAuthAPI) handleStart(w http.ResponseWriter, r *http.Request) {
	api.startAuthorizationCode(w, r, "")
}

// handleLinkStart is the link-an-additional-provider variant. The signed
// state carries LinkUserID so the callback attaches the resulting
// identity to the caller's user rather than running find-or-create.
//
// Unlike the other authenticated OAuth endpoints, this handler is reached by
// a top-level browser navigation (window.location.assign from Settings →
// Connected Accounts) and therefore receives no Authorization header. It
// resolves the caller from the SameSite=Strict refresh cookie instead (with a
// Bearer-header fast path for API clients / tests) via userFromBrowserSession.
// A missing or expired session 302s to /login rather than rendering a bare
// 401 in the browser tab — in practice the FE route guard has already bounced
// a fully-logged-out user, so this is the rare access-token-expired race.
// Impersonation sessions are refused inside userFromBrowserSession (#1750).
// @Summary Start link-an-additional-OAuth-provider flow
// @Description Authenticated variant of /start: the resulting callback links the new identity to the caller's user rather than creating a fresh account.
// @Description The caller is identified from the refresh-token cookie (or an Authorization Bearer header); an absent or expired session 302s to /login.
// @Tags oauth
// @Param provider path string true "Provider name (google|github)"
// @Param redirect query string false "Relative FE path to land on after link"
// @Success 302 "Redirect to provider (or to /login when no live session is present)"
// @Failure 404 {string} string "Unknown provider"
// @Router /auth/oauth/{provider}/link/start [get]
func (api *OAuthAPI) handleLinkStart(w http.ResponseWriter, r *http.Request) {
	user, err := api.auth.userFromBrowserSession(r)
	if err != nil || user == nil {
		slog.Info("OAuth link-start: no resolvable session; redirecting to login", "error", err)
		http.Redirect(w, r, "/login?reason=session_expired", http.StatusFound)
		return
	}
	api.startAuthorizationCode(w, r, user.ID)
}

// startAuthorizationCode shares the start logic between the sign-in
// flow (linkUserID="") and the link-to-existing-user flow.
func (api *OAuthAPI) startAuthorizationCode(w http.ResponseWriter, r *http.Request, linkUserID string) {
	providerName := models.OAuthProvider(chi.URLParam(r, "provider"))
	provider, ok := api.lookupProvider(providerName)
	if !ok {
		http.Error(w, "Unknown OAuth provider", http.StatusNotFound)
		return
	}

	pkce, err := oauthsvc.NewPKCE()
	if err != nil {
		slog.Error("OAuth start: failed to generate PKCE pair", "provider", providerName, "error", err)
		http.Error(w, "Failed to start OAuth flow", http.StatusInternalServerError)
		return
	}
	nonce, err := oauthsvc.NewNonce()
	if err != nil {
		slog.Error("OAuth start: failed to generate nonce", "provider", providerName, "error", err)
		http.Error(w, "Failed to start OAuth flow", http.StatusInternalServerError)
		return
	}

	redirectAfter := sanitizeAllowedRedirect(r.URL.Query().Get("redirect"))

	state := oauthsvc.State{
		Provider:      string(providerName),
		Nonce:         nonce,
		Verifier:      pkce.Verifier,
		RedirectAfter: redirectAfter,
		LinkUserID:    linkUserID,
	}
	signed, err := api.state.Sign(state)
	if err != nil {
		slog.Error("OAuth start: failed to sign state", "provider", providerName, "error", err)
		http.Error(w, "Failed to start OAuth flow", http.StatusInternalServerError)
		return
	}

	writeOAuthStateCookie(w, r, signed)
	authURL := provider.AuthCodeURL(signed, pkce.Challenge)
	// #nosec G710 -- authURL is built by oauth2.Config.AuthCodeURL against
	// the provider's hard-coded endpoint (google.Endpoint or
	// githuboauth.Endpoint); the only user-controlled inputs are the
	// signed state token and the PKCE S256 challenge, both server-issued.
	// There is no untrusted host component the caller can supply.
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleCallback is the provider redirect target. Validates state,
// exchanges the code, runs the find-or-create-or-link branch, mints
// tokens, and 302s the user back into the app.
// @Summary OAuth callback (provider redirect target)
// @Description Validates state + cookie, exchanges the code, and either signs the caller in (existing identity),
// @Description auto-links a verified-email match, prompts a password sign-in (unverified email), or creates a new
// @Description account. Redirects to the FE path the state carried.
// @Tags oauth
// @Param provider path string true "Provider name (google|github)"
// @Param code query string true "Authorization code returned by the provider"
// @Param state query string true "Signed state token issued by /start"
// @Success 302 "Redirect to the FE redirect path the state carried"
// @Failure 400 {string} string "Bad Request - invalid state or code"
// @Failure 404 {string} string "Unknown provider"
// @Failure 500 {string} string "Internal error during exchange or user provisioning"
// @Router /auth/oauth/{provider}/callback [get]
func (api *OAuthAPI) handleCallback(w http.ResponseWriter, r *http.Request) {
	providerName := models.OAuthProvider(chi.URLParam(r, "provider"))
	provider, ok := api.lookupProvider(providerName)
	if !ok {
		http.Error(w, "Unknown OAuth provider", http.StatusNotFound)
		return
	}

	code := strings.TrimSpace(r.URL.Query().Get("code"))
	rawState := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || rawState == "" {
		http.Error(w, "Missing OAuth code or state", http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie(oauthStateCookieName)
	if err != nil || cookie.Value != rawState {
		// The cookie is gone or doesn't match what the provider sent
		// back. Could be a replay across browser sessions, a cross-tab
		// race, or someone forging a state. Drop the cookie just in
		// case it's stale and 400 out.
		clearOAuthStateCookie(w, r)
		http.Error(w, "OAuth state cookie mismatch", http.StatusBadRequest)
		return
	}
	clearOAuthStateCookie(w, r)

	st, err := api.state.Verify(rawState)
	if err != nil {
		http.Error(w, "Invalid or expired OAuth state", http.StatusBadRequest)
		return
	}
	if st.Provider != string(providerName) {
		// State was minted for a different provider — refuse to proceed
		// rather than silently route the credentials to the wrong place.
		http.Error(w, "OAuth state provider mismatch", http.StatusBadRequest)
		return
	}

	tenantID := TenantIDFromContext(r.Context())
	if tenantID == "" {
		slog.Error("OAuth callback ran without tenant context")
		http.Error(w, "Tenant context not established", http.StatusInternalServerError)
		return
	}

	profile, err := provider.Exchange(r.Context(), code, st.Verifier)
	if err != nil {
		slog.Warn("OAuth callback: token exchange failed", "provider", providerName, "error", err)
		http.Error(w, "OAuth exchange failed", http.StatusBadRequest)
		return
	}

	method := loginMethodFor(providerName)

	// Link flow: an authenticated user wanted to attach a new provider.
	if st.LinkUserID != "" {
		api.completeLinkFlow(w, r, providerName, profile, st, tenantID, method)
		return
	}

	// Sign-in flow.
	api.completeSignInFlow(w, r, providerName, profile, st, tenantID, method)
}

// completeSignInFlow runs the find-or-create-or-link branch documented on
// the issue:
//
//  1. lookup by (provider, provider_user_id) — existing identity → log in.
//  2. otherwise lookup users.email = profile.Email:
//     - profile.EmailVerified=true → auto-link, log in.
//     - profile.EmailVerified=false → 302 to /login?oauth_link_required=1.
//  3. otherwise → register a new user.
func (api *OAuthAPI) completeSignInFlow(
	w http.ResponseWriter, r *http.Request,
	providerName models.OAuthProvider, profile oauthsvc.Profile,
	st oauthsvc.State, tenantID string, method models.LoginMethod,
) {
	// Normalize the provider email up-front (REV-1). users.email is stored
	// lowercase, GetByEmail does exact equality, and Google/GitHub return
	// mixed-case (e.g. "User@Example.com"). Without this normalization an
	// existing local "user@example.com" silently falls through to the
	// new-user-provisioning branch, duplicating the row.
	profile.Email = strings.ToLower(strings.TrimSpace(profile.Email))

	existingIdentity, err := api.identityStore.GetByProviderSubject(r.Context(), providerName, profile.ProviderUserID)
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		slog.Error("OAuth callback: identity lookup failed", "provider", providerName, "error", err)
		http.Error(w, "OAuth sign-in failed", http.StatusInternalServerError)
		return
	}
	if existingIdentity != nil {
		user, err := api.userRegistry.Get(r.Context(), existingIdentity.UserID)
		if err != nil || user == nil || !user.IsActive {
			slog.Warn("OAuth callback: linked user missing or inactive", "user_id", existingIdentity.UserID, "error", err)
			http.Error(w, "User account is not available", http.StatusForbidden)
			return
		}
		// SEC: reject cross-tenant callback completions. The
		// (provider, provider_user_id) index is GLOBALLY unique — that's
		// intentional, so the same Google account can't log in to two
		// Inventario accounts simultaneously — but it means a user owned
		// by tenant A can be resolved when the callback is running on
		// tenant B's host. Refuse to sign in or link across tenants. The
		// outcome surfaces a tenant_mismatch row in login_events so this
		// shows up clearly in the audit history.
		if user.TenantID != tenantID {
			slog.Error("OAuth callback: cross-tenant sign-in attempted (existing identity)",
				"user_id", user.ID, "user_tenant", user.TenantID, "callback_tenant", tenantID, "provider", providerName)
			api.auth.recordLoginEventWithMethod(r.Context(), tenantID, profile.Email, &user.ID, models.LoginOutcomeTenantMismatch, method, r)
			http.Redirect(w, r, oauthErrorURL("tenant_mismatch", st.RedirectAfter), http.StatusFound)
			return
		}
		api.completeOAuthLogin(w, r, user, providerName, tenantID, method, st.RedirectAfter)
		return
	}

	// No identity row. Try to match by email.
	user, err := api.userRegistry.GetByEmail(r.Context(), tenantID, profile.Email)
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		slog.Error("OAuth callback: user-by-email lookup failed", "error", err)
		http.Error(w, "OAuth sign-in failed", http.StatusInternalServerError)
		return
	}

	if user != nil {
		// SEC: defense-in-depth tenant check on email-match. GetByEmail is
		// tenant-scoped via its tenantID parameter, but a future change to
		// the registry contract MUST NOT silently widen the lookup — make
		// the invariant explicit at the call site so a cross-tenant
		// auto-link can never happen even if the lower layer regresses.
		if user.TenantID != tenantID {
			slog.Error("OAuth callback: cross-tenant sign-in attempted (email match)",
				"user_id", user.ID, "user_tenant", user.TenantID, "callback_tenant", tenantID, "provider", providerName)
			api.auth.recordLoginEventWithMethod(r.Context(), tenantID, profile.Email, &user.ID, models.LoginOutcomeTenantMismatch, method, r)
			http.Redirect(w, r, oauthErrorURL("tenant_mismatch", st.RedirectAfter), http.StatusFound)
			return
		}
		// SEC-2: refuse if the local user is deactivated — same outcome
		// the password path emits at auth.go's IsActive check. Doing this
		// before the EmailVerified branch keeps a disabled account from
		// learning whether its email had been used at a given provider.
		if !user.IsActive {
			slog.Warn("OAuth callback: email-match auto-link refused for deactivated user",
				"user_id", user.ID, "provider", providerName)
			api.auth.recordLoginEventWithMethod(r.Context(), tenantID, profile.Email, &user.ID, models.LoginOutcomeAccountDisabled, method, r)
			http.Redirect(w, r, oauthErrorURL("account_disabled", st.RedirectAfter), http.StatusFound)
			return
		}
		if !profile.EmailVerified {
			// Unverified email at the provider AND a local user with
			// that email exists → never auto-link. Tell the FE to
			// prompt the user to sign in with their password and
			// link from settings.
			api.auth.recordLoginEventWithMethod(r.Context(), tenantID, profile.Email, &user.ID, models.LoginOutcomeEmailNotVerified, method, r)
			http.Redirect(w, r, oauthLinkRequiredURL(st.RedirectAfter, profile.Email, providerName), http.StatusFound)
			return
		}
		// Verified — auto-link.
		if _, err := api.createIdentity(r.Context(), user, providerName, profile); err != nil {
			slog.Error("OAuth callback: failed to auto-link identity", "user_id", user.ID, "error", err)
			http.Error(w, "OAuth link failed", http.StatusInternalServerError)
			return
		}
		api.completeOAuthLogin(w, r, user, providerName, tenantID, method, st.RedirectAfter)
		return
	}

	// SEC: never provision a brand-new user from an unverified provider
	// email. Issue #1394 spec explicitly forbids auto-linking on
	// unverified emails; creating a user is one step beyond that risk —
	// it stamps the unverified email into our own users.email column and
	// flips IsActive=true (bypassing the in-app verification round-trip).
	// An attacker who claims someone else's email at GitHub (with email
	// hidden / unverified) could otherwise seed an Inventario account
	// keyed on that email and later collide with the legitimate owner.
	if !profile.EmailVerified {
		slog.Warn("OAuth callback: refusing to provision new user from unverified provider email",
			"provider", providerName, "email", profile.Email)
		api.auth.recordLoginEventWithMethod(r.Context(), tenantID, profile.Email, nil, models.LoginOutcomeEmailNotVerified, method, r)
		http.Redirect(w, r, oauthErrorURL("email_unverified", st.RedirectAfter), http.StatusFound)
		return
	}

	// Brand-new user. Provision and link in one logical operation.
	newUser, err := api.provisionUserFromProfile(r.Context(), tenantID, profile)
	if err != nil {
		slog.Error("OAuth callback: user provisioning failed", "email", profile.Email, "error", err)
		// REV-6: if the provider sent us a profile that fails our own
		// User validation (no email, invalid characters, …), don't echo
		// the validator output to the wire — surface a generic banner.
		if errors.Is(err, errProvisionUserInvalidProfile) {
			http.Redirect(w, r, oauthErrorURL("invalid_profile", st.RedirectAfter), http.StatusFound)
			return
		}
		http.Error(w, "OAuth account creation failed", http.StatusInternalServerError)
		return
	}
	if _, err := api.createIdentity(r.Context(), newUser, providerName, profile); err != nil {
		slog.Error("OAuth callback: failed to persist OAuth identity for new user",
			"user_id", newUser.ID, "error", err)
		http.Error(w, "OAuth account creation failed", http.StatusInternalServerError)
		return
	}
	api.completeOAuthLogin(w, r, newUser, providerName, tenantID, method, st.RedirectAfter)
}

// completeLinkFlow attaches the provider identity to the user who initiated
// the link request. Refuses if either:
//   - the (provider, provider_user_id) row is already attached to a
//     different user (cross-account fraud guard), or
//   - the same provider is already attached to this user (no-op).
func (api *OAuthAPI) completeLinkFlow(
	w http.ResponseWriter, r *http.Request,
	providerName models.OAuthProvider, profile oauthsvc.Profile,
	st oauthsvc.State, tenantID string, method models.LoginMethod,
) {
	// Normalize the provider email — same reason as completeSignInFlow
	// (REV-1). The link path writes the email into user_oauth_identities;
	// we don't want a row's email column to drift away from lowercase.
	profile.Email = strings.ToLower(strings.TrimSpace(profile.Email))

	user, err := api.userRegistry.Get(r.Context(), st.LinkUserID)
	if err != nil || user == nil || !user.IsActive {
		slog.Warn("OAuth link: user from state missing or inactive", "user_id", st.LinkUserID, "error", err)
		http.Error(w, "User account is not available", http.StatusForbidden)
		return
	}
	// SEC: reject cross-tenant link completions. The link flow takes the
	// user ID from the signed state token, which was minted in handleLinkStart
	// against the user's session on tenant A. If the callback URL is then
	// completed against tenant B's host (e.g. the user manually rewrote the
	// callback host, or a stolen state replayed elsewhere), refuse to attach
	// the provider identity. Without this, the link path would otherwise
	// write a row with tenant_id=B but user_id pointing into tenant A —
	// corrupting tenant boundaries.
	if user.TenantID != tenantID {
		slog.Error("OAuth link: cross-tenant link attempted",
			"user_id", user.ID, "user_tenant", user.TenantID, "callback_tenant", tenantID, "provider", providerName)
		api.auth.recordLoginEventWithMethod(r.Context(), tenantID, user.Email, &user.ID, models.LoginOutcomeTenantMismatch, method, r)
		http.Redirect(w, r, oauthErrorURL("tenant_mismatch", st.RedirectAfter), http.StatusFound)
		return
	}

	// Refuse if the same provider account is already linked to a
	// different user — this would otherwise let a malicious user
	// commandeer someone else's OAuth account.
	existingIdentity, err := api.identityStore.GetByProviderSubject(r.Context(), providerName, profile.ProviderUserID)
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		slog.Error("OAuth link: identity lookup failed", "provider", providerName, "error", err)
		http.Error(w, "OAuth link failed", http.StatusInternalServerError)
		return
	}
	if existingIdentity != nil && existingIdentity.UserID != user.ID {
		slog.Warn("OAuth link: provider account already attached to another user",
			"user_id", user.ID, "existing_user_id", existingIdentity.UserID,
			"provider", providerName)
		http.Error(w, "This account is already linked to a different user", http.StatusConflict)
		return
	}
	if existingIdentity != nil && existingIdentity.UserID == user.ID {
		// Idempotent re-link — emit identity_linked so the audit reader
		// doesn't count this as a fresh credential check (REV-8) and
		// write a security-audit row (SEC-3) just like the create path.
		api.auth.recordLoginEventWithMethod(r.Context(), tenantID, user.Email, &user.ID, models.LoginOutcomeIdentityLinked, method, r)
		api.auth.logAuth(r.Context(), "oauth_link_added", &user.ID, &user.TenantID, true, r, nil)
		// #nosec G710 -- the redirect target is a server-built relative path:
		// st.RedirectAfter was passed through sanitizeAllowedRedirect (allow-list)
		// in handleLinkStart before being signed into the state, and oauthLinkedURL
		// only appends the server-controlled oauth_linked=<provider> query param.
		// No untrusted host/scheme component is present.
		http.Redirect(w, r, oauthLinkedURL(st.RedirectAfter, providerName), http.StatusFound)
		return
	}

	if _, err := api.createIdentity(r.Context(), user, providerName, profile); err != nil {
		slog.Error("OAuth link: persistence failed", "user_id", user.ID, "error", err)
		http.Error(w, "OAuth link failed", http.StatusInternalServerError)
		return
	}
	// REV-8: distinct outcome so successful link events do not double-
	// count as sign-ins in the audit history page.
	api.auth.recordLoginEventWithMethod(r.Context(), tenantID, user.Email, &user.ID, models.LoginOutcomeIdentityLinked, method, r)
	// SEC-3: mirror the audit pattern used elsewhere in auth.go for
	// security-sensitive events so security ops can search a single
	// stream for "OAuth provider attached to account".
	api.auth.logAuth(r.Context(), "oauth_link_added", &user.ID, &user.TenantID, true, r, nil)
	slog.Info("OAuth identity linked", "user_id", user.ID, "provider", providerName)
	_ = method // method is informational here; the link path does not mint tokens.
	// #nosec G710 -- see the idempotent-relink branch above: the target is a
	// server-built relative path (allow-list-sanitized RedirectAfter + a
	// server-controlled oauth_linked query param), not attacker-controlled.
	http.Redirect(w, r, oauthLinkedURL(st.RedirectAfter, providerName), http.StatusFound)
}

// completeOAuthLogin runs the same token-issue + cookie-write sequence as
// AuthAPI.login, then 302s the user back to the FE app path the state
// carried. MFA-enrolled users are NOT auto-signed-in: SEC-1 gates the
// token-issue path on api.auth.userMFAEnabled and 302s to a separate
// FE banner when the gate fires.
func (api *OAuthAPI) completeOAuthLogin(
	w http.ResponseWriter, r *http.Request,
	user *models.User, providerName models.OAuthProvider,
	tenantID string, method models.LoginMethod, redirectAfter string,
) {
	// SEC-1: refuse to mint tokens when the user has TOTP enrolled.
	// The password login path runs maybeIssueMFAChallenge between the
	// password check and persistRefreshToken; the OAuth flow has to
	// honour the same gate or a user with TOTP enabled could bypass
	// the second factor by signing in via Google/GitHub. The OAuth
	// callback is a browser 302 (not a JSON POST), so we don't mint an
	// mfa_token here — the FE consumes mfa_token only on the JSON
	// /auth/login step-1 path, and re-using it would require additional
	// FE plumbing the v1 cut isn't ready for. Conservative behaviour:
	// reject with a redirect to /login with a banner asking the user to
	// sign in with email + password to complete MFA, after which they
	// can use the OAuth provider freely (since the link is already
	// recorded; no second pass is needed).
	mfaEnabled, err := api.auth.userMFAEnabled(r.Context(), user)
	if err != nil {
		slog.Error("OAuth callback: MFA enrollment lookup failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to verify MFA state", http.StatusInternalServerError)
		return
	}
	if mfaEnabled {
		slog.Info("OAuth callback: MFA enrolled — refusing OAuth-only sign-in",
			"user_id", user.ID, "provider", providerName)
		api.auth.recordLoginEventWithMethod(r.Context(), tenantID, user.Email, &user.ID, models.LoginOutcomeMFARequired, method, r)
		api.auth.logAuth(r.Context(), "oauth_login_mfa_required", &user.ID, &user.TenantID, true, r, nil)
		http.Redirect(w, r, oauthMFARequiredURL(redirectAfter, user.Email, providerName), http.StatusFound)
		return
	}

	rti, rawRefreshToken, err := api.auth.persistRefreshToken(r.Context(), r, user)
	if err != nil {
		slog.Error("OAuth callback: failed to persist refresh token", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}
	accessTokenString, _, err := api.auth.issueAccessToken(r.Context(), user, rti)
	if err != nil {
		slog.Error("OAuth callback: failed to issue access token", "user_id", user.ID, "error", err)
		api.auth.rollbackRefreshToken(r.Context(), user.ID, rti)
		http.Error(w, "Failed to generate session", http.StatusInternalServerError)
		return
	}
	api.auth.setRefreshTokenCookie(w, r, rawRefreshToken)

	// Best-effort last_login bookkeeping.
	now := time.Now()
	user.LastLoginAt = &now
	if _, err := api.userRegistry.Update(r.Context(), *user); err != nil {
		slog.Error("OAuth callback: failed to update last_login", "user_id", user.ID, "error", err)
	}

	// CSRF token is minted into the cookie store; FE picks it up on the
	// /auth/me round-trip immediately after the redirect lands. Burn it
	// in so the next request that needs CSRF protection finds a token.
	_ = api.auth.generateCSRFTokenForUser(r.Context(), user.ID)

	api.recordOAuthLoginEvent(r, tenantID, user, method)
	_ = accessTokenString // returned only via the cookie path on browser redirects

	http.Redirect(w, r, redirectOrDefault(redirectAfter, "/"), http.StatusFound)
}

// recordOAuthLoginEvent writes a single OAuth-flavored login_event using
// the same registry the password login uses, but with method=oauth_<p>.
func (api *OAuthAPI) recordOAuthLoginEvent(r *http.Request, tenantID string, user *models.User, method models.LoginMethod) {
	if api.auth == nil || api.auth.loginEventRegistry == nil {
		return
	}
	userID := user.ID
	event := models.LoginEvent{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		UserID:              &userID,
		Email:               user.Email,
		Outcome:             models.LoginOutcomeOK,
		Method:              method,
		IPAddress:           clientIPTruncated(r),
		UserAgent:           r.UserAgent(),
	}
	if _, err := api.auth.loginEventRegistry.Create(r.Context(), event); err != nil {
		slog.Warn("OAuth callback: failed to record login event", "user_id", user.ID, "error", err)
	}
}

// errProvisionUserInvalidProfile is the sentinel returned by
// provisionUserFromProfile when the provider sent us a profile that fails
// our User model validation (REV-6). Callers translate it into a banner
// rather than echoing the validator output to the wire, which could leak
// our validation rules into a "fix your email" round-trip the user has
// no control over.
var errProvisionUserInvalidProfile = errors.New("oauth: provider profile failed user validation")

// provisionUserFromProfile creates a brand-new user for an OAuth sign-up.
// password_hash is left empty — the user has no password yet. They can
// set one later via /auth/change-password (which has a branch for the
// password_hash="" case).
//
// REV-6: the user struct runs ValidateWithContext BEFORE the create call so
// a malformed provider profile (e.g. a Google email that somehow contained
// invalid characters) gets caught here and surfaced as
// errProvisionUserInvalidProfile, not as a generic 500 from the registry.
func (api *OAuthAPI) provisionUserFromProfile(ctx context.Context, tenantID string, profile oauthsvc.Profile) (*models.User, error) {
	displayName := strings.TrimSpace(profile.DisplayName)
	if displayName == "" {
		displayName = strings.SplitN(profile.Email, "@", 2)[0]
	}
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: uuid.New().String()},
			TenantID: tenantID,
		},
		Email: strings.ToLower(strings.TrimSpace(profile.Email)),
		Name:  displayName,
		// IsActive=true because the provider already verified the email;
		// we skip the in-app verification round-trip on OAuth sign-up.
		IsActive: true,
	}

	if err := user.ValidateWithContext(ctx); err != nil {
		slog.Warn("OAuth callback: provider profile failed user validation",
			"email", user.Email, "error", err)
		return nil, errProvisionUserInvalidProfile
	}

	created, err := api.userRegistry.Create(ctx, user)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// createIdentity persists the row mapping (user, provider, provider_user_id).
// Centralized so all three call sites (sign-in auto-link, sign-up,
// authenticated link) use exactly the same shape.
func (api *OAuthAPI) createIdentity(ctx context.Context, user *models.User, providerName models.OAuthProvider, profile oauthsvc.Profile) (*models.OAuthIdentity, error) {
	identity := models.OAuthIdentity{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		UserID:              user.ID,
		Provider:            providerName,
		ProviderUserID:      profile.ProviderUserID,
		Email:               profile.Email,
	}
	return api.identityStore.Create(ctx, identity)
}

// handleListIdentities returns the OAuth identities currently linked to
// the authenticated caller.
// @Summary List the caller's linked OAuth identities
// @Description Returns each OAuth provider the authenticated user has linked along with the email recorded at link time.
// @Tags oauth
// @Produce json
// @Success 200 {object} linkedIdentitiesResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/oauth/identities [get]
func (api *OAuthAPI) handleListIdentities(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if api.identityStore == nil {
		// OAuth not configured — surface an empty list rather than 500.
		writeJSON(w, http.StatusOK, linkedIdentitiesResponse{Identities: []linkedIdentityEntry{}})
		return
	}
	rows, err := api.identityStore.ListByUser(r.Context(), user.TenantID, user.ID)
	if err != nil {
		slog.Error("OAuth identities: list failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to list identities", http.StatusInternalServerError)
		return
	}
	resp := linkedIdentitiesResponse{Identities: []linkedIdentityEntry{}}
	for _, row := range rows {
		resp.Identities = append(resp.Identities, linkedIdentityEntry{
			Provider: string(row.Provider),
			Email:    row.Email,
			LinkedAt: row.LinkedAt,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleUnlink removes an OAuth identity from the caller's account.
// Refuses if the user has no password AND this is their only remaining
// OAuth identity — otherwise they'd lock themselves out.
// @Summary Unlink an OAuth provider from the caller's account
// @Description Removes the identity row mapping the caller to the named provider. Refused with 409 if it is the caller's only remaining sign-in method.
// @Tags oauth
// @Param provider path string true "Provider name (google|github)"
// @Success 204 "No Content"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Unknown provider"
// @Failure 409 {string} string "Conflict - cannot remove the last remaining sign-in method"
// @Router /auth/oauth/{provider} [delete]
func (api *OAuthAPI) handleUnlink(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	providerName := models.OAuthProvider(chi.URLParam(r, "provider"))
	if !providerName.IsValid() {
		http.Error(w, "Unknown OAuth provider", http.StatusNotFound)
		return
	}
	if api.identityStore == nil {
		// Nothing to unlink — surface 204 to keep the unlink endpoint
		// idempotent in a deployment that doesn't wire OAuth. Audit
		// the (no-op) attempt so security ops see the request shape
		// in the same stream as the real removals (SEC-3).
		api.auth.logAuth(r.Context(), "oauth_link_removed", &user.ID, &user.TenantID, true, r, nil)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	hasPassword := user.PasswordHash != ""
	rows, err := api.identityStore.ListByUser(r.Context(), user.TenantID, user.ID)
	if err != nil {
		slog.Error("OAuth unlink: list identities failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to unlink identity", http.StatusInternalServerError)
		return
	}
	hasThisProvider := false
	for _, row := range rows {
		if row.Provider == providerName {
			hasThisProvider = true
			break
		}
	}
	if !hasThisProvider {
		// Idempotent — nothing to do. Still audit so the read-trail is
		// consistent with the "real removal" case (SEC-3): an attacker
		// who somehow reached this endpoint while no link existed must
		// leave the same fingerprint as a legitimate caller.
		api.auth.logAuth(r.Context(), "oauth_link_removed", &user.ID, &user.TenantID, true, r, nil)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !hasPassword && len(rows) <= 1 {
		http.Error(w, "Cannot remove the last sign-in method", http.StatusConflict)
		return
	}

	if err := api.identityStore.DeleteByUserAndProvider(r.Context(), user.TenantID, user.ID, providerName); err != nil {
		slog.Error("OAuth unlink: delete failed", "user_id", user.ID, "provider", providerName, "error", err)
		http.Error(w, "Failed to unlink identity", http.StatusInternalServerError)
		return
	}
	// SEC-3: record the security-sensitive event in the same audit
	// stream as oauth_link_added so a single query reads both sides of
	// every OAuth identity's lifecycle.
	api.auth.logAuth(r.Context(), "oauth_link_removed", &user.ID, &user.TenantID, true, r, nil)
	slog.Info("OAuth identity unlinked", "user_id", user.ID, "provider", providerName)
	w.WriteHeader(http.StatusNoContent)
}

// lookupProvider resolves the provider name from the URL and returns the
// Provider plus a found flag. Centralized so the start, callback, and
// link-start handlers branch identically on unknown providers.
//
// Returns (nil, false) when OAuth is not configured in this deployment
// (no registry, no state signer, or no identity store). Handlers turn
// that into a 404 so probing /api/v1/auth/oauth/google/start in a
// deployment that hasn't wired Google doesn't leak details.
func (api *OAuthAPI) lookupProvider(name models.OAuthProvider) (oauthsvc.Provider, bool) {
	if !name.IsValid() {
		return nil, false
	}
	if api.registry == nil || api.state == nil || api.identityStore == nil {
		return nil, false
	}
	return api.registry.Get(name)
}

// writeOAuthStateCookie writes the per-request signed-state cookie.
func writeOAuthStateCookie(w http.ResponseWriter, r *http.Request, value string) {
	secureCookie := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	// #nosec G124 -- HttpOnly + SameSiteLax is required for cross-site OAuth redirect-back.
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    value,
		Path:     oauthStateCookiePath,
		MaxAge:   oauthStateCookieMaxAge,
		HttpOnly: true,
		Secure:   secureCookie,
		// Lax is required because the provider 302s us back from a
		// different site. Strict would drop the cookie on the way in
		// and the cookie/state match below would always fail.
		SameSite: http.SameSiteLaxMode,
	})
}

// clearOAuthStateCookie clears the state cookie so a stale value can't be
// replayed across browser sessions.
func clearOAuthStateCookie(w http.ResponseWriter, r *http.Request) {
	secureCookie := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	// #nosec G124 -- HttpOnly + SameSiteLax mirror writeOAuthStateCookie.
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     oauthStateCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

// loginMethodFor maps a provider name onto the LoginEvent.Method enum.
// Centralized so the callback never branches on a string literal.
//
// REV-7: the default branch is LoginMethodOAuthOther rather than
// LoginMethodPassword. This sentinel is unreachable in correct code (the
// lookupProvider helper rejects unknown providers before we get here), but
// returning "password" would silently mislabel any future hole as a
// password sign-in in the audit history. "oauth_other" surfaces the bug
// instead of hiding it.
func loginMethodFor(p models.OAuthProvider) models.LoginMethod {
	switch p {
	case models.OAuthProviderGoogle:
		return models.LoginMethodOAuthGoogle
	case models.OAuthProviderGitHub:
		return models.LoginMethodOAuthGitHub
	}
	return models.LoginMethodOAuthOther
}

// sanitizeAllowedRedirect runs the raw FE redirect through both the
// SanitizeRedirect filter and the allow-list prefix check.
func sanitizeAllowedRedirect(raw string) string {
	cleaned := oauthsvc.SanitizeRedirect(raw)
	if cleaned == "" {
		return ""
	}
	for _, prefix := range allowedRedirectPrefixes {
		if cleaned == prefix || strings.HasPrefix(cleaned, prefix+"/") || strings.HasPrefix(cleaned, prefix+"?") {
			return cleaned
		}
	}
	return ""
}

// redirectOrDefault picks the cleaned redirect target when present,
// otherwise falls back to defaultPath.
func redirectOrDefault(cleaned, defaultPath string) string {
	if cleaned != "" {
		return cleaned
	}
	return defaultPath
}

// oauthLinkedURL builds the post-link success redirect, appending the
// `oauth_linked=<provider>` marker the FE Connected Accounts card reads to
// fire a one-time "Linked your <provider> account" toast (#1395). Without the
// marker the card's success useEffect never runs, so the user gets no
// confirmation that the link took. redirectAfter has already been sanitized
// against allowedRedirectPrefixes by handleLinkStart; it is "/settings" for
// the Settings → Connected Accounts entry point.
func oauthLinkedURL(redirectAfter string, provider models.OAuthProvider) string {
	target := redirectOrDefault(redirectAfter, "/settings")
	u, err := url.Parse(target)
	if err != nil {
		// redirectOrDefault only ever returns server-built relative paths, so
		// a parse error is not expected; fall back to a clean default.
		return "/settings?oauth_linked=" + url.QueryEscape(string(provider))
	}
	q := u.Query()
	q.Set("oauth_linked", string(provider))
	u.RawQuery = q.Encode()
	return u.String()
}

// oauthLinkRequiredURL builds the FE-facing redirect that prompts the
// user to sign in with their password and link the provider from
// settings afterwards. The email is included so the FE can prefill the
// sign-in form (note that we did NOT auto-link, so revealing the email
// is no worse than the existing user-enumeration surface — the user
// already entered that email at the provider).
//
// REV-2: built via net/url.Values so emails with '+', '&', or '#'
// (perfectly legal in RFC 5321) survive the round-trip to the FE
// unchanged. The previous JSON-encode-then-strip-quotes approach
// returned the raw email and broke FE query parsing.
//
// REV-3: also threads the provider name so the FE's banner can name
// the provider whose flow the user just came back from.
func oauthLinkRequiredURL(_ /*redirectAfter*/ string, email string, provider models.OAuthProvider) string {
	v := url.Values{}
	v.Set("oauth_link_required", "1")
	if email != "" {
		v.Set("email", email)
	}
	if provider != "" {
		v.Set("provider", string(provider))
	}
	return "/login?" + v.Encode()
}

// oauthErrorURL builds the FE-facing redirect for the non-fatal error
// branches: SEC-2 ("account_disabled") and REV-6 ("invalid_profile"). The
// FE's LoginPage reads ?oauth_error=<code> and renders the destructive
// Alert variant. We deliberately ignore the original redirectAfter target
// for error redirects — the user lands on the login form, not the page
// they were trying to reach. (Arg kept for callsite symmetry / future use.)
func oauthErrorURL(code string, _ /*redirectAfter*/ string) string {
	v := url.Values{}
	v.Set("oauth_error", code)
	return "/login?" + v.Encode()
}

// oauthMFARequiredURL builds the FE-facing redirect when SEC-1 fires:
// the user has TOTP enrolled and we refuse to mint OAuth-only tokens.
// The FE LoginPage doesn't have a dedicated banner for this yet — the
// user lands on /login and signs in with email + password (which then
// runs the existing maybeIssueMFAChallenge gate). The query parameters
// are written so a future FE update can render "please complete sign-in
// with your second factor — Google linking is preserved" without a BE
// change.
func oauthMFARequiredURL(_ /*redirectAfter*/, email string, provider models.OAuthProvider) string {
	v := url.Values{}
	v.Set("mfa_required", "1")
	if provider != "" {
		v.Set("oauth_provider", string(provider))
	}
	if email != "" {
		v.Set("email", maskEmail(email))
	}
	return "/login?" + v.Encode()
}

// maskEmail returns a redacted form of email suitable for surfacing in a
// URL the user can copy or share. Empty input → empty output. Inputs
// without "@" are returned untouched (treated as already-masked / opaque).
//
// The mask preserves the first character of the local-part and the entire
// domain, replacing the rest of the local-part with three dots. "a@b.com"
// is preserved verbatim (single-char locals would otherwise reveal as
// "a...@b.com" which gives away nothing).
func maskEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at < 0 {
		return email
	}
	local := email[:at]
	domain := email[at:]
	if len(local) <= 1 {
		return email
	}
	return string(local[0]) + "..." + domain
}
