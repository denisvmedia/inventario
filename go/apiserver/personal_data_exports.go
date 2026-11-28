package apiserver

import (
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-extras/errx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// personalDataExportAuditEntityType is the audit_logs.entity_type of personal
// data export events.
const personalDataExportAuditEntityType = "personal_data_export"

// Audit actions of personal data exports (#2177).
const (
	personalDataExportRequestAction  = "personal_data_export_request"
	personalDataExportDownloadAction = "personal_data_export_download"
)

var (
	errPersonalDataExportImpersonation = errx.NewSentinel("a personal data export cannot be requested or downloaded while impersonating a user")
	errPersonalDataExportNotFound      = errx.NewSentinel("personal data export not found")
	errPersonalDataExportNoSession     = errx.NewSentinel("a personal data export downloads only in the browser session that asks for it")
)

// PersonalDataExportView is a personal data export as Account settings lists
// it.
type PersonalDataExportView struct {
	ID     string                          `json:"id"`
	Status models.PersonalDataExportStatus `json:"status" enums:"pending,in_progress,completed,failed"`
	// CreatedAt is when the user requested it.
	CreatedAt time.Time `json:"created_at"`
	// CompletedAt is when the archive was finished; absent until then.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// ExpiresAt is when the archive and this entry are deleted.
	ExpiresAt time.Time `json:"expires_at"`
	// SizeBytes is the archive's size, 0 until it is finished.
	SizeBytes  int64 `json:"size_bytes"`
	GroupCount int   `json:"group_count"`
	ItemCount  int   `json:"item_count"`
	FileCount  int   `json:"file_count"`
}

// PersonalDataExportsListResponse is the response of GET
// /users/me/data-exports.
type PersonalDataExportsListResponse struct {
	Exports []PersonalDataExportView `json:"exports"`
	// NextRequestAt is when the user may request a new export; absent when
	// they may now.
	NextRequestAt *time.Time `json:"next_request_at,omitempty"`
	// RetentionDays is how long a finished archive is kept.
	RetentionDays int `json:"retention_days"`
}

// PersonalDataExportDownloadURLResponse carries a signed download URL.
type PersonalDataExportDownloadURLResponse struct {
	// URL is relative to the server root and needs no bearer token. It is
	// bound to the session that asked for it.
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func newPersonalDataExportView(e *models.PersonalDataExport) PersonalDataExportView {
	return PersonalDataExportView{
		ID:          e.ID,
		Status:      e.Status,
		CreatedAt:   e.CreatedAt.UTC(),
		CompletedAt: e.CompletedAt,
		ExpiresAt:   e.ExpiresAt.UTC(),
		SizeBytes:   e.SizeBytes,
		GroupCount:  e.GroupCount,
		ItemCount:   e.ItemCount,
		FileCount:   e.FileCount,
	}
}

// personalDataExportsAPI serves /users/me/data-exports.
type personalDataExportsAPI struct {
	service *export.PersonalDataExportService
	signer  *services.FileSigningService
	audit   services.AuditLogger
}

func (api *personalDataExportsAPI) routes(r chi.Router) {
	r.Get("/data-exports", api.listPersonalDataExports)
	r.Post("/data-exports", api.requestPersonalDataExport)
	r.Post("/data-exports/{id}/download-url", api.personalDataExportDownloadURL)
}

func (api *personalDataExportsAPI) logEvent(r *http.Request, action string, user *models.User, exportID string, success bool, errMsg *string) {
	logPersonalDataExportEvent(r, api.audit, action, user, exportID, success, errMsg)
}

func logPersonalDataExportEvent(r *http.Request, audit services.AuditLogger, action string, user *models.User, exportID string, success bool, errMsg *string) {
	if audit == nil {
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
	if exportID != "" {
		ev.EntityType = new(personalDataExportAuditEntityType)
		ev.EntityID = new(exportID)
	}
	audit.LogAuth(r.Context(), ev)
}

// listPersonalDataExports returns the caller's personal data exports.
// @Summary List personal data exports
// @Description Returns the authenticated user's personal data exports (GDPR Art. 20), newest first, until each expires.
// @Description `next_request_at` is when a new export may be requested; it is absent when one may be requested now.
// @Tags users-me
// @Produce json
// @Success 200 {object} PersonalDataExportsListResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Router /users/me/data-exports [get]
func (api *personalDataExportsAPI) listPersonalDataExports(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	exports, err := api.service.List(r.Context(), user.ID)
	if err != nil {
		_ = internalServerError(w, r, err)
		return
	}
	resp := PersonalDataExportsListResponse{
		Exports:       make([]PersonalDataExportView, 0, len(exports)),
		RetentionDays: int(models.PersonalDataExportRetention / (24 * time.Hour)),
	}
	for _, e := range exports {
		resp.Exports = append(resp.Exports, newPersonalDataExportView(e))
	}
	if next := api.service.NextRequestAt(exports); !next.IsZero() {
		resp.NextRequestAt = new(next)
	}
	writeJSON(w, http.StatusOK, resp)
}

// requestPersonalDataExport records a request for the caller's data.
// @Summary Request a personal data export
// @Description Starts building a copy of the authenticated user's data: profile, settings, group memberships, and what they created in each group,
// @Description with the files they uploaded. The archive is a tar.gz with JSON documents; it is not a group backup and cannot be restored.
// @Description The export runs in the background and an email says when it is ready. One request is allowed per day: an earlier one answers 429
// @Description data_export.rate_limited with `retry_at` in the error meta and a Retry-After header. An impersonation session answers 403
// @Description data_export.impersonation_forbidden.
// @Tags users-me
// @Produce json
// @Success 201 {object} PersonalDataExportView "Requested"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden"
// @Failure 429 {object} jsonapi.Errors "Too Many Requests"
// @Router /users/me/data-exports [post]
func (api *personalDataExportsAPI) requestPersonalDataExport(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if isImpersonatedRequest(r.Context()) {
		api.logEvent(r, personalDataExportRequestAction, user, "", false, new("impersonation session"))
		_ = codedForbiddenError(w, r, errPersonalDataExportImpersonation, "data_export.impersonation_forbidden")
		return
	}

	created, err := api.service.Request(r.Context(), user)
	var limited *export.PersonalDataExportRateLimitedError
	switch {
	case errors.As(err, &limited):
		api.logEvent(r, personalDataExportRequestAction, user, "", false, new("rate limited"))
		retryAfter := max(int64(math.Ceil(time.Until(limited.RetryAt).Seconds())), 1)
		w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
		_ = codedTooManyRequestsError(w, r, limited, "data_export.rate_limited", map[string]any{
			"retry_at":            limited.RetryAt.UTC().Format(time.RFC3339),
			"retry_after_seconds": retryAfter,
		})
		return
	case err != nil:
		api.logEvent(r, personalDataExportRequestAction, user, "", false, new(err.Error()))
		_ = internalServerError(w, r, err)
		return
	}
	api.logEvent(r, personalDataExportRequestAction, user, created.ID, true, nil)
	writeJSON(w, http.StatusCreated, newPersonalDataExportView(created))
}

// personalDataExportDownloadURL mints a signed URL to download an export.
// @Summary Get a download URL for a personal data export
// @Description Returns a short-lived signed URL the browser downloads the archive from. Only the export's owner gets one, and only
// @Description for a finished export that has not expired: another user's export answers 404 data_export.not_found, an unfinished one
// @Description 409 data_export.not_ready. An impersonation session answers 403 data_export.impersonation_forbidden. The URL works
// @Description only with the refresh-token cookie of the session that asked for it, so a request without that cookie answers 403
// @Description data_export.session_required.
// @Tags users-me
// @Produce json
// @Param id path string true "Personal data export ID"
// @Success 200 {object} PersonalDataExportDownloadURLResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden"
// @Failure 404 {object} jsonapi.Errors "Not Found"
// @Failure 409 {object} jsonapi.Errors "Conflict"
// @Router /users/me/data-exports/{id}/download-url [post]
func (api *personalDataExportsAPI) personalDataExportDownloadURL(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if isImpersonatedRequest(r.Context()) {
		_ = codedForbiddenError(w, r, errPersonalDataExportImpersonation, "data_export.impersonation_forbidden")
		return
	}
	e, err := api.service.Get(r.Context(), user.ID, chi.URLParam(r, "id"))
	switch {
	case errors.Is(err, registry.ErrNotFound):
		_ = codedNotFoundError(w, r, errPersonalDataExportNotFound, "data_export.not_found")
		return
	case err != nil:
		_ = internalServerError(w, r, err)
		return
	}
	if !e.IsDownloadable(time.Now()) {
		_ = codedConflictError(w, r, export.ErrPersonalDataExportNotReady, "data_export.not_ready", nil)
		return
	}
	// The URL is bound to the browser session; without its cookie the URL
	// would work for anyone holding it until it expires.
	binding := services.ExtractSessionBinding(r)
	if binding == "" {
		_ = codedForbiddenError(w, r, errPersonalDataExportNoSession, "data_export.session_required")
		return
	}
	url, expiresAt, err := api.signer.GeneratePersonalDataExportURL(e.ID, user.ID, binding)
	if err != nil {
		_ = internalServerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, PersonalDataExportDownloadURLResponse{URL: url, ExpiresAt: expiresAt.UTC()})
}

// signedPersonalDataExportsAPI serves the archives from signed URLs.
type signedPersonalDataExportsAPI struct {
	service *export.PersonalDataExportService
	signer  *services.FileSigningService
	users   registry.UserRegistry
	tenants registry.TenantRegistry
	audit   services.AuditLogger
}

// SignedPersonalDataExports mounts the download of personal data exports from
// signed URLs (#2177). The signature is the credential, as for file
// downloads, so the route sits outside the bearer-token chain.
func SignedPersonalDataExports(params Params, svc *export.PersonalDataExportService, audit services.AuditLogger) func(r chi.Router) {
	api := &signedPersonalDataExportsAPI{
		service: svc,
		signer:  services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration),
		users:   params.FactorySet.UserRegistry,
		tenants: params.FactorySet.TenantRegistry,
		audit:   audit,
	}
	return func(r chi.Router) {
		r.Get("/{id}", api.downloadPersonalDataExport)
	}
}

// downloadPersonalDataExport streams an archive from a signed URL.
// @Summary Download a personal data export
// @Description Streams the tar.gz archive a signed URL from POST /users/me/data-exports/{id}/download-url points at. The URL is bound
// @Description to the session that asked for it and expires; an impersonation session is refused. Every download is recorded in the audit log.
// @Tags users-me
// @Produce application/gzip
// @Param id path string true "Personal data export ID"
// @Param uid query string true "The user the URL was issued to"
// @Param exp query int true "Expiry, Unix seconds"
// @Param sig query string true "Signature"
// @Success 200 {file} file "The archive"
// @Failure 401 {string} string "Invalid or expired URL"
// @Failure 403 {string} string "Impersonation session, or the user's tenant is not active"
// @Failure 404 {string} string "No such export, or it expired"
// @Failure 409 {string} string "The export is not finished"
// @Router /data-exports/{id} [get]
func (api *signedPersonalDataExportsAPI) downloadPersonalDataExport(w http.ResponseWriter, r *http.Request) {
	claims, err := api.signer.ValidatePersonalDataExportURL(r.URL.Path, r.URL.Query(), services.ExtractSessionBinding(r))
	if err != nil {
		slog.WarnContext(r.Context(), "Invalid signed personal data export URL", "error", err.Error(), "ip", clientIPTruncated(r))
		http.Error(w, "Invalid or expired download URL", http.StatusUnauthorized)
		return
	}
	if cookie, err := r.Cookie(appctx.RefreshTokenCookieName); err == nil && strings.HasPrefix(cookie.Value, impersonationRefreshCookieMarker) {
		http.Error(w, errPersonalDataExportImpersonation.Error(), http.StatusForbidden)
		return
	}
	user, err := api.users.Get(r.Context(), claims.UserID)
	if err != nil || !user.IsActive {
		http.Error(w, "Invalid or expired download URL", http.StatusUnauthorized)
		return
	}
	// The signature outlives a suspension of the user's tenant, and the URL
	// can be opened on another tenant's host, so the tenant is checked here
	// (#1037).
	if refuseInactiveUserTenant(w, r, api.tenants, user) {
		return
	}

	e, reader, err := api.service.Open(r.Context(), user.ID, claims.ExportID)
	switch {
	case errors.Is(err, registry.ErrNotFound):
		logPersonalDataExportEvent(r, api.audit, personalDataExportDownloadAction, user, claims.ExportID, false, new("not found"))
		http.Error(w, "Personal data export not found", http.StatusNotFound)
		return
	case errors.Is(err, export.ErrPersonalDataExportNotReady):
		logPersonalDataExportEvent(r, api.audit, personalDataExportDownloadAction, user, claims.ExportID, false, new("not ready"))
		http.Error(w, err.Error(), http.StatusConflict)
		return
	case err != nil:
		logPersonalDataExportEvent(r, api.audit, personalDataExportDownloadAction, user, claims.ExportID, false, new(err.Error()))
		http.Error(w, "Failed to open the archive", http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	logPersonalDataExportEvent(r, api.audit, personalDataExportDownloadAction, user, e.ID, true, nil)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+export.ArchiveName(e)+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if e.SizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(e.SizeBytes, 10))
	}
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, reader); err != nil {
		slog.WarnContext(r.Context(), "Personal data export download interrupted", "export_id", e.ID, "error", err)
	}
}
