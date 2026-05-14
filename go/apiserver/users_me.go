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

	currentHash := currentRefreshTokenHash(r)
	sessions := make([]SessionView, 0, len(tokens))
	for _, t := range tokens {
		sessions = append(sessions, SessionView{
			ID:         t.ID,
			CreatedAt:  t.CreatedAt,
			LastUsedAt: t.LastUsedAt,
			ExpiresAt:  t.ExpiresAt,
			IPAddress:  t.IPAddress,
			UserAgent:  t.UserAgent,
			IsCurrent:  currentHash != "" && t.TokenHash == currentHash,
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
// except the one whose hash matches the current refresh cookie. When
// no cookie is present we still revoke everything — the caller has
// already proven they hold a valid access token, so wiping all sessions
// is the correct outcome for that legitimate (if rare) shape.
// @Summary Revoke all other sessions
// @Description Revoke every refresh token for the authenticated user except the one bound to the current refresh cookie.
// @Tags users-me
// @Produce json
// @Success 204 {string} string "No Content"
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
	if hash := currentRefreshTokenHash(r); hash != "" {
		if rt, err := api.refreshTokenRegistry.GetByTokenHash(r.Context(), hash); err == nil && rt.UserID == user.ID {
			keepID = rt.ID
		}
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
