package apiserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// usersMeDefaultLoginHistoryLimit is the cap applied when the FE asks for
// "the full history" (no ?limit= or ?limit=0). 100 mirrors the design-doc
// in #1379 and is comfortably above the row count a normal user produces
// in 90 days even with daily logins from multiple devices.
const usersMeDefaultLoginHistoryLimit = 100

// usersMeMaxLoginHistoryLimit caps the upper bound of the limit query.
// The retention sweep keeps 90 days of rows; a single page above this
// would be a misuse — paginate via repeated calls if you really need it.
const usersMeMaxLoginHistoryLimit = 500

// UsersMeParams wires the dependencies of the /users/me route group.
type UsersMeParams struct {
	RefreshTokenRegistry registry.RefreshTokenRegistry
	LoginEventRegistry   registry.LoginEventRegistry
}

// usersMeAPI is the handler set behind /users/me/{sessions,login-history}.
type usersMeAPI struct {
	refreshTokenRegistry registry.RefreshTokenRegistry
	loginEventRegistry   registry.LoginEventRegistry
}

// SessionView is the FE-facing shape returned by GET /users/me/sessions.
// It deliberately omits TokenHash and any other column the FE has no use
// for — the FE renders the partial IP + UA as-is and parses the UA in
// the browser (issue #1378 option 2).
type SessionView struct {
	ID         string     `json:"id"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	IPAddress  string     `json:"ip_address,omitempty"`
	UserAgent  string     `json:"user_agent,omitempty"`
	IsCurrent  bool       `json:"is_current"`
}

// SessionsListResponse is the envelope for GET /users/me/sessions.
type SessionsListResponse struct {
	Sessions []SessionView `json:"sessions"`
}

// LoginEventView is the FE-facing shape returned by GET /users/me/login-history.
type LoginEventView struct {
	ID        string              `json:"id"`
	CreatedAt time.Time           `json:"created_at"`
	Email     string              `json:"email"`
	Outcome   models.LoginOutcome `json:"outcome"`
	Method    models.LoginMethod  `json:"method"`
	IPAddress string              `json:"ip_address,omitempty"`
	UserAgent string              `json:"user_agent,omitempty"`
}

// LoginHistoryResponse is the envelope for GET /users/me/login-history.
type LoginHistoryResponse struct {
	Events       []LoginEventView `json:"events"`
	FailedLast7d int              `json:"failed_last_7d"`
}

// UsersMe registers the /api/v1/users/me sub-routes (issue #1644). The
// JWT + CSRF middleware is applied by the caller via `r.With(userMiddlewares...)`
// — every handler here trusts appctx.UserFromContext to be non-nil.
func UsersMe(params UsersMeParams) func(r chi.Router) {
	api := &usersMeAPI{
		refreshTokenRegistry: params.RefreshTokenRegistry,
		loginEventRegistry:   params.LoginEventRegistry,
	}
	return func(r chi.Router) {
		r.Get("/sessions", api.listSessions)
		r.Delete("/sessions/{id}", api.revokeSession)
		r.Delete("/sessions", api.revokeAllOtherSessions)
		r.Get("/login-history", api.listLoginHistory)
	}
}

// listSessions returns the user's active (non-revoked, non-expired)
// refresh tokens with an is_current flag derived from the refresh
// cookie's token hash.
// @Summary List active sessions
// @Description Returns the authenticated user's active refresh-token sessions, with a flag identifying the session bound to the current refresh cookie.
// @Tags users-me
// @Produce json
// @Success 200 {object} SessionsListResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Router /users/me/sessions [get]
func (api *usersMeAPI) listSessions(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if api.refreshTokenRegistry == nil {
		// Sessions API is meaningless without a refresh-token store
		// (e.g. an old test harness booting without it). Return an empty
		// list rather than 500 so the FE can still render the page.
		writeJSON(w, http.StatusOK, SessionsListResponse{Sessions: []SessionView{}})
		return
	}

	tokens, err := api.refreshTokenRegistry.ListActiveByUserID(r.Context(), user.ID)
	if err != nil {
		slog.Error("Failed to list sessions", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to list sessions", http.StatusInternalServerError)
		return
	}

	// "Current" detection order:
	//  1. The "rti" claim on the validated access token — the cleanest
	//     signal, written at issuance, sent on every request via the
	//     Authorization header. Works for this route even though the
	//     refresh cookie isn't (cookie Path=/api/v1/auth).
	//  2. The refresh cookie hash — kept as a fallback for tokens minted
	//     before "rti" landed, or for callers that scope the cookie wider.
	currentID := ""
	if claims := appctx.JWTClaimsFromContext(r.Context()); claims != nil {
		if rti, ok := claims["rti"].(string); ok {
			currentID = rti
		}
	}
	currentHash := currentRefreshTokenHash(r)
	sessions := make([]SessionView, 0, len(tokens))
	for _, t := range tokens {
		isCurrent := false
		switch {
		case currentID != "" && t.ID == currentID:
			isCurrent = true
		case currentID == "" && currentHash != "" && t.TokenHash == currentHash:
			isCurrent = true
		}
		sessions = append(sessions, SessionView{
			ID:         t.ID,
			CreatedAt:  t.CreatedAt,
			LastUsedAt: t.LastUsedAt,
			ExpiresAt:  t.ExpiresAt,
			IPAddress:  t.IPAddress,
			UserAgent:  t.UserAgent,
			IsCurrent:  isCurrent,
		})
	}
	writeJSON(w, http.StatusOK, SessionsListResponse{Sessions: sessions})
}

// revokeSession revokes a single refresh token by id, gated on user
// ownership — a guessed id from a different account returns 404.
// @Summary Revoke one session
// @Description Mark a single refresh token as revoked. Returns 404 if the id does not belong to the authenticated user.
// @Tags users-me
// @Produce json
// @Param id path string true "Session ID"
// @Success 204 {string} string "No Content"
// @Failure 404 {string} string "Not Found"
// @Failure 401 {string} string "Unauthorized"
// @Router /users/me/sessions/{id} [delete]
func (api *usersMeAPI) revokeSession(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if api.refreshTokenRegistry == nil {
		http.Error(w, "Sessions not supported", http.StatusNotImplemented)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing session id", http.StatusBadRequest)
		return
	}
	if err := api.refreshTokenRegistry.RevokeByID(r.Context(), user.ID, id); err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}
		slog.Error("Failed to revoke session", "user_id", user.ID, "session_id", id, "error", err)
		http.Error(w, "Failed to revoke session", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// revokeAllOtherSessions revokes every refresh token for the user
// except the one identified as "current".
//
// The keep-id is resolved in this order (first match wins):
//  1. `?keep_id=<id>` query parameter — the FE supplies the ID it
//     rendered with `is_current: true`. Recommended because it's the
//     explicit signal the caller already has; the ID is validated
//     against the user's active sessions, and an ID that doesn't
//     belong to the user is silently ignored (falls through to the
//     fallbacks below).
//  2. The access token's "rti" claim (the refresh-token row id this
//     access token was minted from). Lets direct API callers omit
//     ?keep_id= entirely — the BE pulls the same id off the validated
//     JWT that the FE would have read from listSessions.
//  3. The refresh cookie's hash, retained for tokens minted before
//     "rti" landed or for callers that scope their cookie wider.
//
// When none of the three produces a match we REFUSE the wipe with 400
// rather than revoking everything (#2126). The endpoint's contract is
// "revoke all OTHER sessions" — keep the current one — and an inability
// to identify the current session means we can't honour that contract.
// The only caller that lands here is an impersonation session (no rti,
// no tenant refresh cookie); revoking all would leave the impersonated
// user with no surviving session.
// @Summary Revoke all other sessions
// @Description Revoke every refresh token for the authenticated user except the one identified as current.
// @Description Resolution order for the "current" session: `?keep_id=<id>` (recommended; the id the FE
// @Description rendered with `is_current: true` on GET /users/me/sessions) → access token `rti` claim
// @Description → refresh cookie hash. When none matches, the request is rejected with 400 (the current
// @Description session can't be identified, so revoking everything is refused).
// @Tags users-me
// @Produce json
// @Param keep_id query string false "Session id to keep alive (the is_current row from GET /users/me/sessions). Optional — when omitted the BE falls back to the access token's rti claim, then the refresh cookie."
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Router /users/me/sessions [delete]
func (api *usersMeAPI) revokeAllOtherSessions(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if api.refreshTokenRegistry == nil {
		http.Error(w, "Sessions not supported", http.StatusNotImplemented)
		return
	}

	keepID := ""
	if requested := r.URL.Query().Get("keep_id"); requested != "" {
		// Validate the requested keep_id belongs to this user — listing
		// active sessions is the same query used by GET /sessions, so
		// we trust the same authorisation boundary. An ID that doesn't
		// match a row is silently ignored: the caller may have passed
		// a stale id from a list response that's now revoked, and the
		// fallbacks below still have a chance to recover.
		if rts, err := api.refreshTokenRegistry.ListActiveByUserID(r.Context(), user.ID); err == nil {
			for _, rt := range rts {
				if rt.ID == requested {
					keepID = rt.ID
					break
				}
			}
		}
	}
	if keepID == "" {
		keepID = api.resolveKeepIDFromRTIClaim(r, user.ID)
	}
	if keepID == "" {
		if hash := currentRefreshTokenHash(r); hash != "" {
			if rt, err := api.refreshTokenRegistry.GetByTokenHash(r.Context(), hash); err == nil && rt.UserID == user.ID {
				keepID = rt.ID
			}
		}
	}

	// #2126: refuse the full wipe when the current session can't be
	// identified. This endpoint's contract is "revoke all OTHER sessions"
	// (keep the current one); if none of ?keep_id=, the access token's rti
	// claim, or the refresh cookie resolves a row to keep, revoking
	// everything is never the intent. The only shape that reaches here with
	// keepID == "" is an impersonation session (no rti, no tenant refresh
	// cookie) — a normal logged-in user always resolves a keepID via rti or
	// cookie. Calling RevokeAllExceptID with "" would nuke every refresh
	// token for the impersonated user, leaving no survivor.
	if keepID == "" {
		http.Error(w, "cannot determine the current session to keep; refusing to revoke all sessions", http.StatusBadRequest)
		return
	}

	if err := api.refreshTokenRegistry.RevokeAllExceptID(r.Context(), user.ID, keepID); err != nil {
		slog.Error("Failed to revoke other sessions", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to revoke sessions", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listLoginHistory returns the user's most recent login attempts. The
// envelope also carries failed_last_7d so the FE can render the
// "We noticed N failed sign-in attempts" banner without a second round-trip.
// @Summary List login history
// @Description Returns the authenticated user's most recent login attempts (default 100, max 500). Also returns failed_last_7d for the optional banner.
// @Tags users-me
// @Produce json
// @Param limit query int false "Cap on number of events returned (default 100, max 500)"
// @Success 200 {object} LoginHistoryResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Router /users/me/login-history [get]
func (api *usersMeAPI) listLoginHistory(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if api.loginEventRegistry == nil {
		writeJSON(w, http.StatusOK, LoginHistoryResponse{Events: []LoginEventView{}})
		return
	}

	limit := usersMeDefaultLoginHistoryLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > usersMeMaxLoginHistoryLimit {
		limit = usersMeMaxLoginHistoryLimit
	}

	events, err := api.loginEventRegistry.ListByUser(r.Context(), user.TenantID, user.ID, limit)
	if err != nil {
		slog.Error("Failed to list login history", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to list login history", http.StatusInternalServerError)
		return
	}

	out := make([]LoginEventView, 0, len(events))
	for _, e := range events {
		out = append(out, LoginEventView{
			ID:        e.ID,
			CreatedAt: e.CreatedAt,
			Email:     e.Email,
			Outcome:   e.Outcome,
			Method:    e.Method,
			IPAddress: e.IPAddress,
			UserAgent: e.UserAgent,
		})
	}

	failed, err := api.loginEventRegistry.CountFailedSince(r.Context(), user.TenantID, user.ID, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		// Log and fall through with 0 — the banner is a nice-to-have,
		// not a gate, and we don't want a count failure to kill the
		// whole page.
		slog.Warn("Failed to count failed logins in last 7d", "user_id", user.ID, "error", err)
		failed = 0
	}

	writeJSON(w, http.StatusOK, LoginHistoryResponse{Events: out, FailedLast7d: failed})
}

// resolveKeepIDFromRTIClaim returns the refresh-token row id pinned by
// the access token's "rti" claim, validated against the user's active
// sessions. Returns "" when no claim is present, the claim is empty,
// the lookup fails, or the claimed id no longer belongs to an active
// row (e.g. the row was revoked since the access token was minted).
func (api *usersMeAPI) resolveKeepIDFromRTIClaim(r *http.Request, userID string) string {
	claims := appctx.JWTClaimsFromContext(r.Context())
	if claims == nil {
		return ""
	}
	rti, ok := claims["rti"].(string)
	if !ok || rti == "" {
		return ""
	}
	rts, err := api.refreshTokenRegistry.ListActiveByUserID(r.Context(), userID)
	if err != nil {
		return ""
	}
	for _, rt := range rts {
		if rt.ID == rti {
			return rt.ID
		}
	}
	return ""
}

// currentRefreshTokenHash extracts the SHA-256 hash of the request's
// refresh cookie (if present), used to flag "this session is me" in
// the sessions list and to keep the current session alive when the
// user clicks "Sign out all other sessions".
func currentRefreshTokenHash(r *http.Request) string {
	c, err := r.Cookie(refreshTokenCookieName)
	if err != nil || c.Value == "" {
		return ""
	}
	return models.HashRefreshToken(c.Value)
}
