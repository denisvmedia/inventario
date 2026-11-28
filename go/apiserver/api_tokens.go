package apiserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-extras/errx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// apiTokenAuditEntityType is the audit_logs.entity_type of token events.
const apiTokenAuditEntityType = "api_token"

// createAPITokenMaxBodyBytes caps the create request. The body is a name, a
// scope, two optional ids/dates and up to three short secrets.
const createAPITokenMaxBodyBytes int64 = 8 * 1024

var (
	errAPITokenImpersonation   = errx.NewSentinel("API tokens cannot be created or revoked while impersonating a user")
	errAPITokenInvalidPassword = errx.NewSentinel("current password is incorrect")
	errAPITokenInvalidMFACode  = errx.NewSentinel("authenticator or backup code is invalid")
	// errAPITokenCredentialsChanged answers a create request that the account
	// changed under: the password, two-factor authentication, the account's
	// status or its sessions.
	errAPITokenCredentialsChanged = errx.NewSentinel("the account's credentials changed while the token was being created")
)

// apiTokenCredentialsChangedCode is the code of errAPITokenCredentialsChanged.
const apiTokenCredentialsChangedCode = "api_token.credentials_changed" // #nosec G101 -- an error code, not a credential

// APITokenView is a personal API token as the token list shows it. The token
// itself is never part of it; only CreateAPITokenResponse carries that, once.
type APITokenView struct {
	ID string `json:"id"`
	// Name is the label the user gave the token.
	Name string `json:"name"`
	// TokenPrefix is the start of the token, e.g. "inv_pat_AbCd", to tell
	// tokens apart.
	TokenPrefix string               `json:"token_prefix"`
	Scope       models.APITokenScope `json:"scope" enums:"read_only,read_write"`
	// GroupID is the one group the token may reach, or absent for every group
	// the owner belongs to.
	GroupID *string `json:"group_id,omitempty"`
	// ExpiresAt is absent for a token that does not expire.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	// LastUsedAt is accurate to five minutes; absent when never used.
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	// Expired is true once ExpiresAt has passed. An expired token is listed
	// so its owner can see why a script stopped working, and can be revoked
	// to remove it from the list.
	Expired bool `json:"expired"`
}

// APITokensListResponse is the envelope for GET /users/me/api-tokens.
type APITokensListResponse struct {
	Tokens []APITokenView `json:"tokens"`
}

// CreateAPITokenRequest is the body of POST /users/me/api-tokens.
type CreateAPITokenRequest struct {
	Name  string `json:"name"`
	Scope string `json:"scope" enums:"read_only,read_write"`
	// GroupID restricts the token to one group the caller is a member of.
	GroupID *string `json:"group_id,omitempty"`
	// ExpiresAt is an RFC 3339 time in the future, or absent for no expiry.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// CurrentPassword is required when the account has a password.
	CurrentPassword string `json:"current_password,omitempty"`
	// TOTPCode or BackupCode is required when the account has two-factor
	// authentication enabled.
	TOTPCode   string `json:"totp_code,omitempty"`
	BackupCode string `json:"backup_code,omitempty"`
}

// CreateAPITokenResponse is the answer to POST /users/me/api-tokens. Secret is
// the token itself. It is shown this once; the server keeps only its digest.
type CreateAPITokenResponse struct {
	Token  APITokenView `json:"token"`
	Secret string       `json:"secret"`
}

type apiTokensAPI struct {
	service     *services.APITokenService
	audit       services.AuditLogger
	mfaService  *services.MFAService
	mfaRegistry registry.UserMFASecretRegistry
	users       registry.UserRegistry
	blacklist   services.TokenBlacklister
}

func newAPITokenView(t *models.APIToken, now time.Time) APITokenView {
	return APITokenView{
		ID:          t.ID,
		Name:        t.Name,
		TokenPrefix: t.TokenPrefix,
		Scope:       t.Scope,
		GroupID:     t.GroupID,
		ExpiresAt:   t.ExpiresAt,
		CreatedAt:   t.CreatedAt,
		LastUsedAt:  t.LastUsedAt,
		Expired:     t.IsExpired(now),
	}
}

// listAPITokens returns the caller's tokens that are not revoked.
// @Summary List personal API tokens
// @Description Returns the authenticated user's personal API tokens that are not revoked, newest first, expired ones included. The token values themselves are never returned.
// @Description Requires a browser session; API tokens are refused here (403 api_token.endpoint_not_allowed).
// @Tags users-me
// @Produce json
// @Success 200 {object} APITokensListResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden"
// @Router /users/me/api-tokens [get]
func (api *apiTokensAPI) listAPITokens(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	tokens, err := api.service.List(r.Context(), user.ID)
	if err != nil {
		_ = internalServerError(w, r, err)
		return
	}
	now := time.Now()
	views := make([]APITokenView, 0, len(tokens))
	for _, t := range tokens {
		views = append(views, newAPITokenView(t, now))
	}
	writeJSON(w, http.StatusOK, APITokensListResponse{Tokens: views})
}

// createAPIToken mints a personal API token.
// @Summary Create a personal API token
// @Description Creates a token for scripts and other applications and returns it once in `secret`; only its SHA-256 digest is stored.
// @Description Requires a browser session that is not an impersonation session. The caller re-authenticates: `current_password` when the account has a password, and `totp_code` or `backup_code` when two-factor authentication is enabled.
// @Description `scope` is `read_only` (GET, HEAD and OPTIONS only) or `read_write`. `group_id` restricts the token to one group the caller belongs to. `expires_at` is optional.
// @Description Validation failures answer 422 with codes api_token.name_required, api_token.name_too_long, api_token.invalid_scope,
// @Description api_token.expiry_in_past, api_token.group_not_allowed, api_token.limit_reached, api_token.invalid_password or api_token.invalid_mfa_code.
// @Description A password change or reset, disabling two-factor authentication, an account block or a session revoke that lands while the request runs answers 401 api_token.credentials_changed, and no token is created.
// @Tags users-me
// @Accept json
// @Produce json
// @Param data body CreateAPITokenRequest true "Token settings and re-authentication"
// @Success 201 {object} CreateAPITokenResponse "Created"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity"
// @Router /users/me/api-tokens [post]
func (api *apiTokensAPI) createAPIToken(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if !api.requireOwnSession(w, r) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, createAPITokenMaxBodyBytes)
	var req CreateAPITokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	in := services.CreateAPITokenInput{
		Name:      req.Name,
		Scope:     models.APITokenScope(strings.TrimSpace(req.Scope)),
		GroupID:   req.GroupID,
		ExpiresAt: req.ExpiresAt,
	}
	// Validate before the step-up: a consumed TOTP step cannot be reused, so
	// a typo in the name must not cost the user their code.
	if err := api.service.ValidateCreate(r.Context(), user, in); err != nil {
		api.renderCreateError(w, r, err)
		return
	}
	mfaID, ok := api.reauthenticate(w, r, user, req)
	if !ok {
		return
	}

	token, secret, err := api.service.Create(r.Context(), user, in)
	if err != nil {
		api.renderCreateError(w, r, err)
		return
	}
	changed, err := api.credentialsChanged(r, user, mfaID)
	if err != nil || changed {
		api.discardMintedToken(w, r, user, token.ID, err)
		return
	}

	api.logTokenEvent(r, "api_token_create", user, token.ID, true, nil)
	// The response body is the only copy of the token. Keep it out of every
	// cache between here and the browser.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, CreateAPITokenResponse{
		Token:  newAPITokenView(token, time.Now()),
		Secret: secret,
	})
}

// revokeAPIToken revokes one of the caller's tokens.
// @Summary Revoke a personal API token
// @Description Revokes the token at once; the next request made with it answers 401. Returns 404 for an id that is not one of the caller's tokens.
// @Description Requires a browser session that is not an impersonation session.
// @Tags users-me
// @Produce json
// @Param id path string true "Token ID"
// @Success 204 {string} string "No Content"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Router /users/me/api-tokens/{id} [delete]
func (api *apiTokensAPI) revokeAPIToken(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if !api.requireOwnSession(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing token id", http.StatusBadRequest)
		return
	}
	token, err := api.service.Revoke(r.Context(), user.ID, id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			http.Error(w, "API token not found", http.StatusNotFound)
			return
		}
		_ = internalServerError(w, r, err)
		return
	}
	api.logTokenEvent(r, "api_token_revoke", user, token.ID, true, nil)
	w.WriteHeader(http.StatusNoContent)
}

// requireOwnSession refuses token management to anything but the account
// owner's own browser session. JWTMiddleware already refuses API tokens on
// this subtree; the token check repeats it so the rule does not depend on how
// the route is mounted. An impersonating operator may look at the list but
// may not mint a credential that outlives the impersonation, nor revoke the
// user's integrations.
func (api *apiTokensAPI) requireOwnSession(w http.ResponseWriter, r *http.Request) bool {
	if appctx.APITokenFromContext(r.Context()) != nil {
		_ = codedForbiddenError(w, r, errAPITokenEndpointNotAllowed, apiTokenEndpointNotAllowedCode)
		return false
	}
	if isImpersonatedRequest(r.Context()) {
		_ = codedForbiddenError(w, r, errAPITokenImpersonation, "api_token.impersonation_forbidden")
		return false
	}
	return true
}

// reauthenticate asks for the factors the account has before a token is
// minted. A session access token alone could be a stolen one, and a token
// outlives the session and skips the second factor on every later request, so
// minting one needs the password (when the account has one) and a fresh TOTP
// or backup code (when two-factor authentication is on). An account with
// neither — OAuth sign-in only, no MFA — has nothing more to prove, the same
// rule account deletion applies. It returns the id of the two-factor
// enrollment the code was checked against, empty when two-factor
// authentication is off.
func (api *apiTokensAPI) reauthenticate(w http.ResponseWriter, r *http.Request, user *models.User, req CreateAPITokenRequest) (mfaID string, ok bool) {
	if user.PasswordHash != "" && !services.VerifyPassword(user, req.CurrentPassword) {
		api.logTokenEvent(r, "api_token_create", user, "", false, new("incorrect password"))
		_ = codedUnprocessableEntityError(w, r, errAPITokenInvalidPassword, "api_token.invalid_password")
		return "", false
	}

	if api.mfaRegistry == nil || api.mfaService == nil {
		return "", true
	}
	row, err := api.mfaRegistry.GetByUser(r.Context(), user.TenantID, user.ID)
	if errors.Is(err, registry.ErrNotFound) || (err == nil && !row.IsEnabled()) {
		return "", true
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "API token create: MFA lookup failed", "user_id", user.ID, "error", err)
		_ = renderInternalServerError(w, r, err)
		return "", false
	}
	if !consumeMFACode(r.Context(), api.mfaService, api.mfaRegistry, user, row, req.TOTPCode, req.BackupCode) {
		api.logTokenEvent(r, "api_token_create", user, "", false, new("invalid mfa code"))
		_ = codedUnprocessableEntityError(w, r, errAPITokenInvalidMFACode, "api_token.invalid_mfa_code")
		return "", false
	}
	return row.ID, true
}

// credentialsChanged reports whether the account changed under a create
// request after reauthenticate checked it: a new password, two-factor
// authentication turned off or enrolled again, the account blocked or
// deleted, or its sessions revoked. Each of those revokes every token, but a
// revocation that runs between this request's checks and its insert finds
// nothing to revoke, so the request looks again once its row exists. The
// events revoke the tokens once more after they store their change, so
// whichever of the two runs second sees the other.
func (api *apiTokensAPI) credentialsChanged(r *http.Request, user *models.User, mfaID string) (bool, error) {
	ctx := r.Context()
	if api.users != nil {
		current, err := api.users.Get(ctx, user.ID)
		if errors.Is(err, registry.ErrNotFound) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if current.PasswordHash != user.PasswordHash || !current.IsActive {
			return true, nil
		}
	}
	if mfaID != "" && api.mfaRegistry != nil {
		row, err := api.mfaRegistry.GetByUser(ctx, user.TenantID, user.ID)
		if errors.Is(err, registry.ErrNotFound) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if !row.IsEnabled() || row.ID != mfaID {
			return true, nil
		}
	}
	if api.blacklist != nil {
		claims := appctx.JWTClaimsFromContext(ctx)
		since, revoked, err := api.blacklist.UserBlacklistedSince(ctx, user.ID)
		switch {
		case err != nil:
			// JWTMiddleware lets a request through when this lookup fails;
			// do the same. The password and MFA checks above do not depend
			// on it.
			slog.ErrorContext(r.Context(), "API token create: session revocation lookup failed", "user_id", user.ID, "error", err)
		case revoked && claims != nil && checkUserBlacklistIat(claims, since) != nil:
			return true, nil
		}
	}
	return false, nil
}

// discardMintedToken revokes a token whose create request failed after the
// insert and answers the request. The secret has not left the server, so
// nobody can use the row; revoking it keeps it off the list and out of the
// cap.
func (api *apiTokensAPI) discardMintedToken(w http.ResponseWriter, r *http.Request, user *models.User, tokenID string, checkErr error) {
	if _, err := api.service.Revoke(r.Context(), user.ID, tokenID); err != nil {
		slog.ErrorContext(r.Context(), "API token create: failed to revoke a discarded token", "user_id", user.ID, "token_id", tokenID, "error", err)
	}
	if checkErr != nil {
		slog.ErrorContext(r.Context(), "API token create: credential re-check failed", "user_id", user.ID, "error", checkErr)
		_ = renderInternalServerError(w, r, checkErr)
		return
	}
	api.logTokenEvent(r, "api_token_create", user, tokenID, false, new("account credentials changed during creation"))
	_ = codedUnauthorizedError(w, r, errAPITokenCredentialsChanged, apiTokenCredentialsChangedCode)
}

func (api *apiTokensAPI) renderCreateError(w http.ResponseWriter, r *http.Request, err error) {
	codes := []struct {
		err  error
		code string
	}{
		{services.ErrAPITokenNameRequired, "api_token.name_required"},
		{services.ErrAPITokenNameTooLong, "api_token.name_too_long"},
		{services.ErrAPITokenInvalidScope, "api_token.invalid_scope"},
		{services.ErrAPITokenExpiryInPast, "api_token.expiry_in_past"},
		{services.ErrAPITokenGroupNotAllowed, "api_token.group_not_allowed"},
		{services.ErrAPITokenLimitReached, "api_token.limit_reached"},
	}
	for _, c := range codes {
		if errors.Is(err, c.err) {
			_ = codedUnprocessableEntityError(w, r, c.err, c.code)
			return
		}
	}
	_ = internalServerError(w, r, err)
}

func (api *apiTokensAPI) logTokenEvent(r *http.Request, action string, user *models.User, tokenID string, success bool, errMsg *string) {
	if api.audit == nil {
		return
	}
	ev := services.AuthEvent{
		Action:   action,
		UserID:   new(user.ID),
		TenantID: new(user.TenantID),
		Success:  success,
		Request:  r,
		ErrMsg:   errMsg,
	}
	if tokenID != "" {
		ev.EntityType = new(apiTokenAuditEntityType)
		ev.EntityID = new(tokenID)
	}
	api.audit.LogAuth(r.Context(), ev)
}

// revokeAllAPITokens revokes every token of the user and records it. The
// account-security events that end every session call it — password change
// and reset, disabling MFA, an admin block — so a credential minted before the
// event does not survive it. The self-service events call it before they
// change anything and fail when it fails: tokens revoked without the change
// is the safe outcome, a change reported as done with the tokens still alive
// is not. After the change is stored they call sweepAPITokens.
func revokeAllAPITokens(r *http.Request, svc *services.APITokenService, audit services.AuditLogger, userID, tenantID, reason string) error {
	if svc == nil {
		return nil
	}
	n, err := svc.RevokeAllForUser(r.Context(), userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to revoke API tokens", "user_id", userID, "reason", reason, "error", err)
		return err
	}
	// The row shares its request_id with the row of the event that caused
	// it (password_change, mfa_disable, …), which is what says why.
	if n > 0 && audit != nil {
		audit.LogAuth(r.Context(), services.AuthEvent{
			Action:   "api_token_revoke_all",
			UserID:   new(userID),
			TenantID: new(tenantID),
			Success:  true,
			Request:  r,
		})
	}
	return nil
}

// sweepAPITokens revokes the user's tokens a second time, after an event has
// stored its change. A token minted between revokeAllAPITokens and the change
// was checked against the old credentials; createAPIToken re-reads the
// account after its insert and this reads the tokens after the change, so one
// of the two catches it. The change already took effect, so a failure is
// logged (revokeAllAPITokens does) and not reported as a failed change.
func sweepAPITokens(r *http.Request, svc *services.APITokenService, audit services.AuditLogger, userID, tenantID, reason string) {
	_ = revokeAllAPITokens(r, svc, audit, userID, tenantID, reason)
}
