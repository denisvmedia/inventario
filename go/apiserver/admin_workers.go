package apiserver

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// Worker soft-pause action names (#1308). Kept as constants so the audit
// trail uses the same literals as the CLI service path
// (services/admin emits the identical strings) and the swagger tags.
// Mirrors the "admin.<noun>_<verb>" pattern.
const (
	// AuditActionAdminWorkerPause is the audit-row Action emitted on a
	// successful (or attempted) soft-pause of a background worker.
	AuditActionAdminWorkerPause = "admin.worker_pause"
	// AuditActionAdminWorkerResume is the audit-row Action emitted on a
	// successful (or attempted) resume of a soft-paused worker.
	AuditActionAdminWorkerResume = "admin.worker_resume"
)

// JSON:API error codes returned by the worker pause/resume endpoints.
// Kept as constants so the swagger annotations, the FE branch table, and
// the tests reference the same literals.
const (
	// AdminWorkerUnknownTypeCode signals "the {workerType} path segment is
	// not one of the known worker types". Maps to a 404.
	AdminWorkerUnknownTypeCode = "admin.worker.unknown_type"
	// AdminWorkerReasonTooLongCode signals "the supplied reason exceeds the
	// 500-char cap". Maps to a 422.
	AdminWorkerReasonTooLongCode = "admin.worker.reason_too_long"
)

// adminWorkerReasonMaxLen caps the pause reason at 500 characters, matching
// the block/unblock reason cap so the audit breadcrumb stays bounded.
const adminWorkerReasonMaxLen = 500

// WorkerPauseRequest is the request body for
// POST /admin/workers/{workerType}/pause. `reason` is OPTIONAL (the
// worker_control.reason column is nullable); an empty body is accepted
// and records no reason.
type WorkerPauseRequest struct {
	// Reason is the optional operator-supplied note for the pause (max 500
	// chars). Persisted into worker_control.reason and the audit breadcrumb.
	Reason string `json:"reason,omitempty"`
}

// WorkerControlView is the JSON:API attributes block returned by the
// worker endpoints. Mirrors the operator-facing fields of
// models.WorkerControl plus the worker_type (also carried as the
// resource id).
type WorkerControlView struct {
	WorkerType string     `json:"worker_type"`
	Paused     bool       `json:"paused"`
	PausedBy   *string    `json:"paused_by,omitempty"`
	PausedAt   *time.Time `json:"paused_at,omitempty"`
	Reason     *string    `json:"reason,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

// WorkerControlResource is the JSON:API resource block. `type` is
// "worker_control" and `id` is the worker-type string (the natural key).
type WorkerControlResource struct {
	Type       string            `json:"type"`
	ID         string            `json:"id"`
	Attributes WorkerControlView `json:"attributes"`
}

// WorkerControlEnvelope is the single-resource JSON:API envelope returned
// by pause / resume.
type WorkerControlEnvelope struct {
	Data WorkerControlResource `json:"data"`
}

// WorkerControlListEnvelope is the list JSON:API envelope returned by
// GET /admin/workers — one resource per models.AllWorkerTypes().
type WorkerControlListEnvelope struct {
	Data []WorkerControlResource `json:"data"`
}

// adminWorkersAPI backs the /admin/workers/* routes (#1308). Holds the
// FactorySet directly (not the per-request user-aware Set) for the same
// reason the other admin APIs do — the worker_control table is a
// platform-operator control with no tenant scope. AuditService is shared
// with the rest of the apiserver so the worker audit trail lands in the
// same audit_logs table.
type adminWorkersAPI struct {
	factorySet   *registry.FactorySet
	auditService services.AuditLogger
}

// listWorkers returns every canonical worker type with its current pause
// state. Worker types with no control row render as not-paused
// (paused=false) — "no row" is the running default.
//
// @Summary List background workers and their pause state (admin)
// @Description Returns one resource per canonical worker type with its soft-pause state (#1308). A worker with no control row renders as paused=false. Resource `type` is "worker_control"; `id` is the worker-type string.
// @Tags admin
// @Produce json-api
// @Success 200 {object} WorkerControlListEnvelope "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Failure 403 {object} jsonapi.Errors "Account disabled"
// @Router /admin/workers [get]
func (api *adminWorkersAPI) listWorkers(w http.ResponseWriter, r *http.Request) {
	rows, err := api.factorySet.WorkerControlRegistry.List(r.Context())
	if err != nil {
		slog.Error("admin listWorkers: failed to list worker controls", "error", err)
		_ = internalServerError(w, r, err)
		return
	}

	// Index the rows by worker type so the canonical-order merge below is
	// O(1) per type. Absent => the worker is running.
	byType := make(map[models.WorkerType]*models.WorkerControl, len(rows))
	for _, row := range rows {
		if row != nil {
			byType[row.WorkerType] = row
		}
	}

	resources := make([]WorkerControlResource, 0, len(models.AllWorkerTypes()))
	for _, wt := range models.AllWorkerTypes() {
		resources = append(resources, workerControlResource(wt, byType[wt]))
	}

	api.writeEnvelope(w, r, http.StatusOK, WorkerControlListEnvelope{Data: resources})
}

// pauseWorker soft-pauses the worker named in the {workerType} path
// segment. The reason body field is optional. Idempotent — re-pausing an
// already-paused worker updates the reason but preserves the original
// paused_at.
//
// @Summary Soft-pause a background worker (admin)
// @Description Soft-pauses the worker named by {workerType} (#1308): its run loop keeps ticking but skips claiming new work until resumed; in-flight jobs finish. Idempotent.
// @Description The optional `reason` body field is recorded on the control row and the audit breadcrumb. An empty body is accepted.
// @Description Unknown worker types return 404 with `admin.worker.unknown_type`; a reason over 500 characters returns 422 with `admin.worker.reason_too_long`.
// @Tags admin
// @Accept json
// @Produce json-api
// @Param workerType path string true "Worker type (e.g. export, import, thumbnail)"
// @Param data body WorkerPauseRequest false "Optional pause request (reason)"
// @Success 200 {object} WorkerControlEnvelope "OK"
// @Failure 400 {object} jsonapi.Errors "Bad Request - invalid body"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Failure 403 {object} jsonapi.Errors "Account disabled"
// @Failure 404 {object} jsonapi.Errors "Not Found - unknown worker type"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - reason too long"
// @Router /admin/workers/{workerType}/pause [post]
func (api *adminWorkersAPI) pauseWorker(w http.ResponseWriter, r *http.Request) {
	actor := appctx.AdminActorFromContext(r.Context())
	if actor == nil {
		// Defence-in-depth: RequireBackofficeAuth should have populated this.
		_ = unauthorizedError(w, r, ErrMissingUserContext)
		return
	}

	wt, ok := api.resolveWorkerType(w, r)
	if !ok {
		return
	}

	req, ok := api.decodePauseRequest(w, r)
	if !ok {
		return
	}

	// Record the operator's id as paused_by so the control row attributes
	// the pause to a back-office identity (the CLI path records "cli"
	// instead). Email would be more human-readable, but the id is the
	// stable key and matches the actor recorded on the audit row.
	control, err := api.factorySet.WorkerControlRegistry.Pause(r.Context(), string(wt), actor.ID, req.Reason)
	if err != nil {
		slog.Error("admin pauseWorker: failed to pause worker", "worker_type", string(wt), "error", err)
		api.logWorkerOutcome(r, AuditActionAdminWorkerPause, actor.ID, string(wt), req.Reason, false, err.Error())
		_ = internalServerError(w, r, err)
		return
	}

	api.writeEnvelope(w, r, http.StatusOK, WorkerControlEnvelope{Data: workerControlResource(wt, control)})
	// Audit AFTER render so a writer failure lands as Success=false.
	api.logWorkerOutcome(r, AuditActionAdminWorkerPause, actor.ID, string(wt), req.Reason, true, "")
}

// resumeWorker clears the soft-pause on the worker named in the
// {workerType} path segment. No body is required. Idempotent — resuming
// a worker that is not paused is a no-op returning the not-paused state.
//
// @Summary Resume a soft-paused background worker (admin)
// @Description Clears the soft-pause on the worker named by {workerType} (#1308) so it resumes claiming work on its next tick. Idempotent. Unknown worker types return 404 with `admin.worker.unknown_type`.
// @Tags admin
// @Produce json-api
// @Param workerType path string true "Worker type (e.g. export, import, thumbnail)"
// @Success 200 {object} WorkerControlEnvelope "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Failure 403 {object} jsonapi.Errors "Account disabled"
// @Failure 404 {object} jsonapi.Errors "Not Found - unknown worker type"
// @Router /admin/workers/{workerType}/resume [post]
func (api *adminWorkersAPI) resumeWorker(w http.ResponseWriter, r *http.Request) {
	actor := appctx.AdminActorFromContext(r.Context())
	if actor == nil {
		_ = unauthorizedError(w, r, ErrMissingUserContext)
		return
	}

	wt, ok := api.resolveWorkerType(w, r)
	if !ok {
		return
	}

	control, err := api.factorySet.WorkerControlRegistry.Resume(r.Context(), string(wt))
	if err != nil {
		slog.Error("admin resumeWorker: failed to resume worker", "worker_type", string(wt), "error", err)
		api.logWorkerOutcome(r, AuditActionAdminWorkerResume, actor.ID, string(wt), "", false, err.Error())
		_ = internalServerError(w, r, err)
		return
	}

	api.writeEnvelope(w, r, http.StatusOK, WorkerControlEnvelope{Data: workerControlResource(wt, control)})
	api.logWorkerOutcome(r, AuditActionAdminWorkerResume, actor.ID, string(wt), "", true, "")
}

// resolveWorkerType reads + validates the {workerType} path segment.
// Writes a coded 404 and returns ok=false when the type is unknown so
// the caller can early-return.
func (api *adminWorkersAPI) resolveWorkerType(w http.ResponseWriter, r *http.Request) (models.WorkerType, bool) {
	raw := strings.TrimSpace(chi.URLParam(r, "workerType"))
	wt, ok := models.ParseWorkerType(raw)
	if !ok {
		_ = codedNotFoundError(w, r, errors.New("unknown worker type"), AdminWorkerUnknownTypeCode)
		return "", false
	}
	return wt, true
}

// decodePauseRequest parses + validates the POST /pause body. An empty
// body is allowed (no reason). Writes the right error response and
// returns ok=false on failure so the caller can early-return.
func (api *adminWorkersAPI) decodePauseRequest(w http.ResponseWriter, r *http.Request) (WorkerPauseRequest, bool) {
	var req WorkerPauseRequest
	if r.Body == nil {
		// No body at all is fine — reason is optional.
		return req, true
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		// An empty body decodes to io.EOF — treat it as "no reason"
		// rather than a 400, since the reason is optional.
		if errors.Is(err, io.EOF) {
			return WorkerPauseRequest{}, true
		}
		_ = badRequest(w, r, err)
		return req, false
	}
	if !decoderAtEOF(dec) {
		_ = badRequest(w, r, errors.New("invalid JSON body — trailing tokens"))
		return req, false
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if utf8.RuneCountInString(req.Reason) > adminWorkerReasonMaxLen {
		_ = codedUnprocessableEntityError(w, r, errors.New("reason is too long"), AdminWorkerReasonTooLongCode)
		return req, false
	}
	return req, true
}

// writeEnvelope encodes v as a JSON:API response on w. Centralised so the
// content-type + encode-error handling is identical across list / single
// responses.
func (api *adminWorkersAPI) writeEnvelope(w http.ResponseWriter, _ *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/vnd.api+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers (and status) already flushed — cannot recover, but log
		// so the operator-side trail is honest.
		slog.Error("admin workers: failed to encode response", "error", err)
	}
}

// logWorkerOutcome writes the admin.worker_pause / admin.worker_resume
// audit row. Best-effort: nil-safe when AuditService was not wired in.
func (api *adminWorkersAPI) logWorkerOutcome(
	r *http.Request,
	action, actorID, workerType, reason string,
	success bool,
	errMsg string,
) {
	if api.auditService == nil {
		return
	}
	ev := services.AdminEvent{
		Action:      action,
		ActorID:     nullableString(actorID),
		SubjectType: stringPtr("worker"),
		SubjectID:   nullableString(workerType),
		Success:     success,
		Request:     r,
		Reason:      reason,
	}
	if errMsg != "" {
		ev.ErrMsg = new(errMsg)
	}
	api.auditService.LogAdmin(r.Context(), ev)
}

// workerControlResource builds the JSON:API resource for worker type wt.
// A nil control row (no row in the table) renders as the not-paused
// default — "no row" means the worker is running.
func workerControlResource(wt models.WorkerType, control *models.WorkerControl) WorkerControlResource {
	view := WorkerControlView{
		WorkerType: string(wt),
		Paused:     false,
	}
	if control != nil {
		view.Paused = control.Paused
		view.PausedBy = control.PausedBy
		view.PausedAt = control.PausedAt
		view.Reason = control.Reason
		updatedAt := control.UpdatedAt
		view.UpdatedAt = &updatedAt
	}
	return WorkerControlResource{
		Type:       "worker_control",
		ID:         string(wt),
		Attributes: view,
	}
}
