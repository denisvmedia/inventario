package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
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
// no session yet. The link/unlink endpoints are wrapped with requireAuth
// at registration time.
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

		r.With(requireAuth).Get("/identities", api.handleListIdentities)
		r.With(requireAuth).Get("/{provider}/link/start", api.handleLinkStart)
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
// @Summary Start link-an-additional-OAuth-provider flow
// @Description Authenticated variant of /start: the resulting callback links the new identity to the caller's user rather than creating a fresh account.
// @Tags oauth
// @Param provider path string true "Provider name (google|github)"
// @Param redirect query string false "Relative FE path to land on after link"
// @Success 302 "Redirect to provider"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Unknown provider"
// @Router /auth/oauth/{provider}/link/start [get]
func (api *OAuthAPI) handleLinkStart(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
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
		api.completeOAuthLogin(w, r, user, tenantID, method, st.RedirectAfter)
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
		if !profile.EmailVerified {
			// Unverified email at the provider AND a local user with
			// that email exists → never auto-link. Tell the FE to
			// prompt the user to sign in with their password and
			// link from settings.
			api.auth.recordLoginEvent(r.Context(), tenantID, profile.Email, &user.ID, models.LoginOutcomeEmailNotVerified, r)
			http.Redirect(w, r, oauthLinkRequiredURL(st.RedirectAfter, profile.Email), http.StatusFound)
			return
		}
		// Verified — auto-link.
		if _, err := api.createIdentity(r.Context(), user, providerName, profile); err != nil {
			slog.Error("OAuth callback: failed to auto-link identity", "user_id", user.ID, "error", err)
			http.Error(w, "OAuth link failed", http.StatusInternalServerError)
			return
		}
		api.completeOAuthLogin(w, r, user, tenantID, method, st.RedirectAfter)
		return
	}

	// Brand-new user. Provision and link in one logical operation.
	newUser, err := api.provisionUserFromProfile(r.Context(), tenantID, profile)
	if err != nil {
		slog.Error("OAuth callback: user provisioning failed", "email", profile.Email, "error", err)
		http.Error(w, "OAuth account creation failed", http.StatusInternalServerError)
		return
	}
	if _, err := api.createIdentity(r.Context(), newUser, providerName, profile); err != nil {
		slog.Error("OAuth callback: failed to persist OAuth identity for new user",
			"user_id", newUser.ID, "error", err)
		http.Error(w, "OAuth account creation failed", http.StatusInternalServerError)
		return
	}
	api.completeOAuthLogin(w, r, newUser, tenantID, method, st.RedirectAfter)
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
	user, err := api.userRegistry.Get(r.Context(), st.LinkUserID)
	if err != nil || user == nil || !user.IsActive {
		slog.Warn("OAuth link: user from state missing or inactive", "user_id", st.LinkUserID, "error", err)
		http.Error(w, "User account is not available", http.StatusForbidden)
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
		// Idempotent re-link — just 302 back.
		api.auth.recordLoginEvent(r.Context(), tenantID, user.Email, &user.ID, models.LoginOutcomeOK, r)
		http.Redirect(w, r, redirectOrDefault(st.RedirectAfter, "/settings"), http.StatusFound)
		return
	}

	if _, err := api.createIdentity(r.Context(), user, providerName, profile); err != nil {
		slog.Error("OAuth link: persistence failed", "user_id", user.ID, "error", err)
		http.Error(w, "OAuth link failed", http.StatusInternalServerError)
		return
	}
	api.auth.recordLoginEvent(r.Context(), tenantID, user.Email, &user.ID, models.LoginOutcomeOK, r)
	_ = method // method is informational here; the link path does not mint tokens.
	http.Redirect(w, r, redirectOrDefault(st.RedirectAfter, "/settings"), http.StatusFound)
}

// completeOAuthLogin runs the same token-issue + cookie-write sequence as
// AuthAPI.login, then 302s the user back to the FE app path the state
// carried.
func (api *OAuthAPI) completeOAuthLogin(
	w http.ResponseWriter, r *http.Request,
	user *models.User, tenantID string,
	method models.LoginMethod, redirectAfter string,
) {
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

// provisionUserFromProfile creates a brand-new user for an OAuth sign-up.
// password_hash is left empty — the user has no password yet. They can
// set one later via /auth/change-password (which has a branch for the
// password_hash="" case).
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
		// idempotent in a deployment that doesn't wire OAuth.
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
		// Idempotent — nothing to do.
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
func loginMethodFor(p models.OAuthProvider) models.LoginMethod {
	switch p {
	case models.OAuthProviderGoogle:
		return models.LoginMethodOAuthGoogle
	case models.OAuthProviderGitHub:
		return models.LoginMethodOAuthGitHub
	}
	return models.LoginMethodPassword
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

// oauthLinkRequiredURL builds the FE-facing redirect that prompts the
// user to sign in with their password and link the provider from
// settings afterwards. The email is included so the FE can prefill
// the sign-in form — note that we did NOT auto-link, so revealing the
// email is no worse than the existing user-enumeration surface (the
// user already entered that email at the provider).
func oauthLinkRequiredURL(_ /*redirectAfter*/, email string) string {
	// Build manually rather than via url.URL to keep the path stable
	// regardless of which FE route handles the prompt.
	if email == "" {
		return "/login?oauth_link_required=1"
	}
	// Use a JSON-encode round-trip to escape the email. We don't want a
	// rogue ? or # in a malicious email to break parsing on the FE.
	encoded, err := json.Marshal(email)
	if err != nil {
		return "/login?oauth_link_required=1"
	}
	// Strip the surrounding quotes from the JSON encoding.
	emailToken := string(encoded[1 : len(encoded)-1])
	return "/login?oauth_link_required=1&email=" + emailToken
}
