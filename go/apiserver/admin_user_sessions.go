package apiserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// AdminRevokeSessionsRequest is the request body for
// POST /admin/users/{userID}/sessions/revoke. Symmetric with the block and
// unblock bodies — `reason` is required so the audit row carries why.
type AdminRevokeSessionsRequest struct {
	// Reason is the free-form justification for ending the sessions
	// (max 500 chars).
	Reason string `json:"reason" validate:"required,max=500"`
}

// revokeSessions ends every live session for a user without disabling the
// account (#2479, #967 H6).
//
// Until now this was only reachable as a side effect: an operator who wanted to
// sign someone out — a shared laptop left logged in, a phone lost, a suspected
// token leak — had to block the account and immediately unblock it, which writes
// two misleading audit rows and locks the user out for however long the operator
// takes over the second call.
//
// There is deliberately no force flag, unlike block. Blocking another system
// admin needs one because it takes their access away; ending their sessions does
// not — they sign back in. And no self guard: a back-office identity and a tenant
// user are namespaced separately, so an operator cannot reach their own sessions
// through this route at all.
//
// @Summary End every session for a user
// @Description Revokes the user's refresh tokens and blacklists access tokens issued before now, without changing whether the account is active. The user can sign in again immediately. Idempotent: a user with no live sessions is a 200.
// @Tags admin
// @Accept json
// @Produce json-api
// @Param userID path string true "User ID"
// @Param request body AdminRevokeSessionsRequest true "Reason for ending the sessions"
// @Success 200 {object} map[string]string "Sessions revoked"
// @Failure 400 {object} jsonapi.Errors "Missing user id or malformed body"
// @Failure 401 {object} jsonapi.Errors "Back-office authentication required"
// @Failure 404 {object} jsonapi.Errors "No such user"
// @Failure 422 {object} jsonapi.Errors "Reason missing or too long"
// @Failure 500 {object} jsonapi.Errors "Revocation failed"
// @Router /admin/users/{userID}/sessions/revoke [post].
func (api *adminUsersAPI) revokeSessions(w http.ResponseWriter, r *http.Request) {
	actor := appctx.AdminActorFromContext(r.Context())
	if actor == nil {
		// Defence-in-depth: RequireBackofficeAuth should have caught this.
		_ = unauthorizedError(w, r, ErrMissingUserContext)
		return
	}

	userID := chi.URLParam(r, "userID")
	if strings.TrimSpace(userID) == "" {
		_ = badRequest(w, r, errors.New("missing user id"))
		return
	}

	req, ok := api.decodeRevokeSessionsRequest(w, r)
	if !ok {
		return
	}

	target, err := api.factorySet.UserRegistry.Get(r.Context(), userID)
	if err != nil {
		api.logRevokeSessionsOutcome(r, actor.ID, userID, "", req, false, err.Error())
		if errors.Is(err, registry.ErrNotFound) {
			_ = renderEntityError(w, r, err)
			return
		}
		slog.Error("admin revoke sessions: failed to load user", "user_id", userID, "error", err)
		_ = internalServerError(w, r, err)
		return
	}

	// The same teardown block runs, without the is_active flip.
	if errMsg := api.revokeLiveSessions(r, target.ID); errMsg != "" {
		api.logRevokeSessionsOutcome(r, actor.ID, target.ID, target.TenantID, req, false, errMsg)
		// A partial teardown is a failure worth surfacing: the operator asked for
		// the sessions to be gone and some of them may not be.
		_ = internalServerError(w, r, errors.New(errMsg))
		return
	}

	api.logRevokeSessionsOutcome(r, actor.ID, target.ID, target.TenantID, req, true, "")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"sessions_revoked"}`)); err != nil {
		slog.Warn("admin revoke sessions: failed to write the response", "error", err)
	}
}

// decodeRevokeSessionsRequest mirrors decodeUnblockRequest: the body carries only
// a reason, and the same caps apply so the three admin endpoints answer
// identically to the same mistakes.
func (api *adminUsersAPI) decodeRevokeSessionsRequest(w http.ResponseWriter, r *http.Request) (AdminRevokeSessionsRequest, bool) {
	var req AdminRevokeSessionsRequest
	if r.Body == nil {
		_ = badRequest(w, r, errors.New("missing request body"))
		return req, false
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		_ = badRequest(w, r, err)
		return req, false
	}
	if !decoderAtEOF(dec) {
		_ = badRequest(w, r, errors.New("invalid JSON body — trailing tokens"))
		return req, false
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		_ = codedUnprocessableEntityError(w, r, errors.New("reason is required"), AdminBlockReasonRequiredCode)
		return req, false
	}
	if utf8.RuneCountInString(req.Reason) > adminBlockReasonMaxLen {
		_ = codedUnprocessableEntityError(w, r, errors.New("reason is too long"), AdminBlockReasonTooLongCode)
		return req, false
	}
	return req, true
}

// logRevokeSessionsOutcome writes the admin.user_sessions_revoke audit row.
// Best-effort, and nil-safe for the case where AuditService was not wired.
func (api *adminUsersAPI) logRevokeSessionsOutcome(
	r *http.Request,
	actorID string,
	subjectID, subjectTenantID string,
	req AdminRevokeSessionsRequest,
	success bool,
	errMsg string,
) {
	if api.auditService == nil {
		return
	}
	ev := services.AdminEvent{
		Action:      AuditActionAdminUserSessionsRevoke,
		ActorID:     nullableString(actorID),
		TenantID:    nullableString(subjectTenantID),
		SubjectType: stringPtr("user"),
		SubjectID:   nullableString(subjectID),
		Success:     success,
		Request:     r,
		Reason:      req.Reason,
	}
	if errMsg != "" {
		ev.ErrMsg = new(errMsg)
	}
	api.auditService.LogAdmin(r.Context(), ev)
}
