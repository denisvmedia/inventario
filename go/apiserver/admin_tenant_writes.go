package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/go-extras/errx"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
	adminservice "go.5x5.cz/inventario/services/admin"
)

// Audit actions of the tenant writes (#1037), in the "admin.<noun>_<verb>"
// pattern of the other back-office writes.
const (
	AuditActionAdminTenantCreate     = "admin.tenant_create"
	AuditActionAdminTenantUpdate     = "admin.tenant_update"
	AuditActionAdminTenantSuspend    = "admin.tenant_suspend"
	AuditActionAdminTenantReactivate = "admin.tenant_reactivate"
	AuditActionAdminTenantDelete     = "admin.tenant_delete"
)

// JSON:API error codes of the tenant writes.
const (
	// AdminTenantReasonRequiredCode: suspend or reactivate without a reason.
	AdminTenantReasonRequiredCode = "admin.tenant.reason_required"
	// AdminTenantReasonTooLongCode: the reason is longer than 500 characters.
	AdminTenantReasonTooLongCode = "admin.tenant.reason_too_long"
	// AdminTenantConfirmationMismatchCode: the slug typed to confirm a
	// deletion is not the tenant's slug.
	AdminTenantConfirmationMismatchCode = "admin.tenant.confirmation_mismatch"
	// AdminTenantServesThisHostCode: the request reached the back office
	// through this tenant's host, so suspending or deleting the tenant
	// would lock the operator out of the back office.
	AdminTenantServesThisHostCode = "admin.tenant.serves_this_host"
	// TenantSlugTakenCode: another tenant has the slug.
	TenantSlugTakenCode = "tenant.slug_taken"
	// TenantDomainTakenCode: another tenant has the domain.
	TenantDomainTakenCode = "tenant.domain_taken"
)

// adminTenantReasonMaxLen caps the suspend and reactivate reason, in
// characters, at the same length as the user block reason.
const adminTenantReasonMaxLen = adminBlockReasonMaxLen

// ErrTenantServesThisHost is returned when an operator tries to suspend or
// delete the tenant that the back-office request itself resolved to. Every
// /api/v1 request, the back office included, is refused when its host's
// tenant is not active, so the change would take the back office down with
// the tenant. The CLI can still make it.
var ErrTenantServesThisHost = errx.NewSentinel("the back office is served through this tenant's host; suspending or deleting it would lock the back office out")

// AdminTenantCreateRequest is the body of POST /admin/tenants.
type AdminTenantCreateRequest struct {
	// Name is the tenant's display name (1 to 100 characters).
	Name string `json:"name"`
	// Slug is generated from the name when empty: lowercase letters, digits
	// and hyphens, at most 50 characters.
	Slug string `json:"slug,omitempty"`
	// Domain is an optional custom domain the tenant is reached on.
	Domain string `json:"domain,omitempty"`
	// Status defaults to active.
	Status models.TenantStatus `json:"status,omitempty" enums:"active,suspended,inactive"`
}

// AdminTenantUpdateRequest is the body of PATCH /admin/tenants/{tenantID}.
// A field that is left out does not change.
type AdminTenantUpdateRequest struct {
	Name *string `json:"name,omitempty"`
	Slug *string `json:"slug,omitempty"`
	// Domain set to an empty string removes the custom domain.
	Domain *string `json:"domain,omitempty"`
	// Status away from active ends the sessions of the tenant's users, as
	// POST /admin/tenants/{tenantID}/suspend does.
	Status *models.TenantStatus `json:"status,omitempty" enums:"active,suspended,inactive"`
}

// AdminTenantStatusRequest is the body of the suspend and reactivate
// endpoints. The reason is required and goes into the audit log.
type AdminTenantStatusRequest struct {
	// Reason is the operator's justification (1 to 500 characters).
	Reason string `json:"reason"`
}

// adminTenantCreatedResponse is the 201 a create answers with; the body is
// the tenant detail row. It holds the row rather than embedding
// jsonapi.AdminTenantResponse: render walks the fields after the parent and
// the embedded Render would set the status back to 200.
type adminTenantCreatedResponse struct {
	Data *jsonapi.AdminTenantListItem `json:"data"`
}

func (adminTenantCreatedResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusCreated)
	return nil
}

// createTenant creates a tenant.
//
// @Summary Create a tenant (admin)
// @Description Creates a tenant with the same rules as `inventario tenants create`: an empty slug is generated from the name, the status defaults to active and registration to closed.
// @Description A slug another tenant has is refused with 409 `tenant.slug_taken`, a domain another tenant has with 409 `tenant.domain_taken`, and invalid fields with 422 naming them under data.attributes.
// @Description Requires the platform_admin role.
// @Tags admin
// @Accept json
// @Produce json-api
// @Param data body AdminTenantCreateRequest true "Tenant"
// @Success 201 {object} jsonapi.AdminTenantResponse "Created"
// @Failure 400 {object} jsonapi.Errors "Bad Request - invalid body"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Failure 403 {object} jsonapi.Errors "Forbidden - the operator is not a platform admin (admin.role_required)"
// @Failure 409 {object} jsonapi.Errors "Conflict - the slug or the domain is taken"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - invalid fields"
// @Router /admin/tenants [post]
func (api *adminTenantsAPI) createTenant(w http.ResponseWriter, r *http.Request) {
	var req AdminTenantCreateRequest
	if !decodeAdminBody(w, r, &req) {
		return
	}
	var domain *string
	if d := strings.TrimSpace(req.Domain); d != "" {
		domain = new(d)
	}
	tenant, err := api.service.CreateTenant(r.Context(), adminservice.TenantCreateRequest{
		Name:   strings.TrimSpace(req.Name),
		Slug:   strings.TrimSpace(req.Slug),
		Domain: domain,
		Status: req.Status,
	})
	if err != nil {
		api.auditWrite(r, tenantWriteAudit{action: AuditActionAdminTenantCreate, err: err})
		renderTenantWriteError(w, r, err)
		return
	}
	api.auditWrite(r, tenantWriteAudit{
		action:   AuditActionAdminTenantCreate,
		tenantID: tenant.ID,
		extra:    map[string]any{"slug": tenant.Slug, "status": tenant.Status},
	})
	api.respondCreatedTenant(w, r, tenant.ID)
}

// updateTenant changes a tenant's name, slug, domain or status.
//
// @Summary Update a tenant (admin)
// @Description Changes the fields the body sets, with the same rules as `inventario tenants update`. An empty `domain` removes the custom domain.
// @Description A status away from active ends the sessions of every user in the tenant, as the suspend endpoint does. Making the tenant that serves this request's host inactive is refused with 422 `admin.tenant.serves_this_host`.
// @Description A slug another tenant has is refused with 409 `tenant.slug_taken`, a domain another tenant has with 409 `tenant.domain_taken`, and invalid fields with 422 naming them under data.attributes.
// @Description Requires the platform_admin role.
// @Tags admin
// @Accept json
// @Produce json-api
// @Param tenantID path string true "Tenant ID"
// @Param data body AdminTenantUpdateRequest true "Fields to change"
// @Success 200 {object} jsonapi.AdminTenantResponse "OK"
// @Failure 400 {object} jsonapi.Errors "Bad Request - invalid body"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Failure 403 {object} jsonapi.Errors "Forbidden - the operator is not a platform admin (admin.role_required)"
// @Failure 404 {object} jsonapi.Errors "Tenant not found"
// @Failure 409 {object} jsonapi.Errors "Conflict - the slug or the domain is taken"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - invalid fields, or the tenant serves this host"
// @Router /admin/tenants/{tenantID} [patch]
func (api *adminTenantsAPI) updateTenant(w http.ResponseWriter, r *http.Request) {
	tenant, ok := api.loadTenant(w, r, AuditActionAdminTenantUpdate)
	if !ok {
		return
	}
	var req AdminTenantUpdateRequest
	if !decodeAdminBody(w, r, &req) {
		return
	}
	update := adminservice.TenantUpdateRequest{
		Name:   trimmedPtr(req.Name),
		Slug:   trimmedPtr(req.Slug),
		Domain: trimmedPtr(req.Domain),
		Status: req.Status,
	}
	audit := tenantWriteAudit{action: AuditActionAdminTenantUpdate, tenantID: tenant.ID, extra: tenantUpdateBreadcrumb(tenant, req)}
	if req.Status != nil && *req.Status != models.TenantStatusActive && tenantServesRequest(r, tenant.ID) {
		audit.err = ErrTenantServesThisHost
		api.auditWrite(r, audit)
		renderTenantWriteError(w, r, ErrTenantServesThisHost)
		return
	}
	_, err := api.service.UpdateTenant(r.Context(), tenant.ID, update)
	audit.err = err
	api.auditWrite(r, audit)
	if err != nil && !errors.Is(err, adminservice.ErrTenantSessionsNotEnded) {
		renderTenantWriteError(w, r, err)
		return
	}
	api.respondTenant(w, r, tenant.ID)
}

// suspendTenant suspends a tenant.
//
// @Summary Suspend a tenant (admin)
// @Description Sets the tenant's status to suspended and ends the sessions of every user in it, the same teardown blocking a user runs: refresh tokens and personal API tokens are revoked and access tokens are refused.
// @Description While suspended, the tenant's users cannot sign in by any method and every request on the tenant's host is refused. Suspending a suspended tenant ends the sessions again and returns 200.
// @Description Suspending the tenant that serves this request's host is refused with 422 `admin.tenant.serves_this_host`.
// @Description A missing or blank reason is 422 `admin.tenant.reason_required`, one over 500 characters `admin.tenant.reason_too_long`.
// @Description Requires the platform_admin role.
// @Tags admin
// @Accept json
// @Produce json-api
// @Param tenantID path string true "Tenant ID"
// @Param data body AdminTenantStatusRequest true "Reason"
// @Success 200 {object} jsonapi.AdminTenantResponse "OK"
// @Failure 400 {object} jsonapi.Errors "Bad Request - invalid body"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Failure 403 {object} jsonapi.Errors "Forbidden - the operator is not a platform admin (admin.role_required)"
// @Failure 404 {object} jsonapi.Errors "Tenant not found"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - invalid reason, or the tenant serves this host"
// @Router /admin/tenants/{tenantID}/suspend [post]
func (api *adminTenantsAPI) suspendTenant(w http.ResponseWriter, r *http.Request) {
	api.changeTenantStatus(w, r, AuditActionAdminTenantSuspend, api.service.SuspendTenant)
}

// reactivateTenant reactivates a tenant.
//
// @Summary Reactivate a tenant (admin)
// @Description Sets the tenant's status back to active. Sessions that the suspension ended stay ended: the users sign in again. Reactivating an active tenant returns 200 and changes nothing.
// @Description A missing or blank reason is 422 `admin.tenant.reason_required`, one over 500 characters `admin.tenant.reason_too_long`.
// @Description Requires the platform_admin role.
// @Tags admin
// @Accept json
// @Produce json-api
// @Param tenantID path string true "Tenant ID"
// @Param data body AdminTenantStatusRequest true "Reason"
// @Success 200 {object} jsonapi.AdminTenantResponse "OK"
// @Failure 400 {object} jsonapi.Errors "Bad Request - invalid body"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Failure 403 {object} jsonapi.Errors "Forbidden - the operator is not a platform admin (admin.role_required)"
// @Failure 404 {object} jsonapi.Errors "Tenant not found"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - invalid reason"
// @Router /admin/tenants/{tenantID}/reactivate [post]
func (api *adminTenantsAPI) reactivateTenant(w http.ResponseWriter, r *http.Request) {
	api.changeTenantStatus(w, r, AuditActionAdminTenantReactivate, api.service.ReactivateTenant)
}

// changeTenantStatus runs suspend or reactivate, which differ only in the
// audit action and the service call.
func (api *adminTenantsAPI) changeTenantStatus(
	w http.ResponseWriter, r *http.Request, action string,
	change func(ctx context.Context, idOrSlug string) (*models.Tenant, error),
) {
	tenant, ok := api.loadTenant(w, r, action)
	if !ok {
		return
	}
	reason, ok := decodeTenantStatusReason(w, r)
	if !ok {
		return
	}
	audit := tenantWriteAudit{
		action:   action,
		tenantID: tenant.ID,
		reason:   reason,
		extra:    map[string]any{"slug": tenant.Slug, "status_from": tenant.Status},
	}
	if action == AuditActionAdminTenantSuspend && tenantServesRequest(r, tenant.ID) {
		audit.err = ErrTenantServesThisHost
		api.auditWrite(r, audit)
		renderTenantWriteError(w, r, ErrTenantServesThisHost)
		return
	}
	_, err := change(r.Context(), tenant.ID)
	audit.err = err
	api.auditWrite(r, audit)
	if err != nil && !errors.Is(err, adminservice.ErrTenantSessionsNotEnded) {
		renderTenantWriteError(w, r, err)
		return
	}
	api.respondTenant(w, r, tenant.ID)
}

// deleteTenant hard-deletes a tenant and everything in it.
//
// @Summary Delete a tenant (admin)
// @Description Deletes the tenant, its users, groups, items and files, with the same rules as `inventario tenants delete`.
// @Description The stored files go first, so an unreachable file store stops the deletion before any row is removed. It cannot be undone.
// @Description `confirm_slug` must be the tenant's slug, the same confirmation the CLI prompt asks for; anything else is 422 `admin.tenant.confirmation_mismatch`.
// @Description Deleting the tenant that serves this request's host is refused with 422 `admin.tenant.serves_this_host`.
// @Description Requires the platform_admin role.
// @Tags admin
// @Produce json-api
// @Param tenantID path string true "Tenant ID"
// @Param confirm_slug query string true "The tenant's slug, typed to confirm"
// @Success 204 "No Content"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Failure 403 {object} jsonapi.Errors "Forbidden - the operator is not a platform admin (admin.role_required)"
// @Failure 404 {object} jsonapi.Errors "Tenant not found"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - the confirmation does not match, or the tenant serves this host"
// @Router /admin/tenants/{tenantID} [delete]
func (api *adminTenantsAPI) deleteTenant(w http.ResponseWriter, r *http.Request) {
	tenant, ok := api.loadTenant(w, r, AuditActionAdminTenantDelete)
	if !ok {
		return
	}
	audit := tenantWriteAudit{
		action:   AuditActionAdminTenantDelete,
		tenantID: tenant.ID,
		extra:    map[string]any{"slug": tenant.Slug, "name": tenant.Name},
	}
	err := adminservice.ConfirmTenantDeletion(tenant, r.URL.Query().Get("confirm_slug"))
	if err == nil && tenantServesRequest(r, tenant.ID) {
		err = ErrTenantServesThisHost
	}
	if err == nil {
		err = api.service.DeleteTenant(r.Context(), tenant.ID)
	}
	audit.err = err
	api.auditWrite(r, audit)
	if err != nil {
		renderTenantWriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// loadTenant reads the tenant the route names. On a failure it writes the
// response and the audit row for action, and returns false.
func (api *adminTenantsAPI) loadTenant(w http.ResponseWriter, r *http.Request, action string) (*models.Tenant, bool) {
	if appctx.AdminActorFromContext(r.Context()) == nil {
		// Defence-in-depth: RequireBackofficeAuth should have caught this.
		_ = unauthorizedError(w, r, ErrMissingUserContext)
		return nil, false
	}
	tenantID := chi.URLParam(r, "tenantID")
	if strings.TrimSpace(tenantID) == "" {
		api.auditWrite(r, tenantWriteAudit{action: action, err: registry.ErrNotFound})
		_ = renderEntityError(w, r, registry.ErrNotFound)
		return nil, false
	}
	tenant, err := api.factorySet.TenantRegistry.Get(r.Context(), tenantID)
	if err != nil {
		api.auditWrite(r, tenantWriteAudit{action: action, tenantID: tenantID, err: err})
		_ = renderEntityError(w, r, err)
		return nil, false
	}
	return tenant, true
}

// respondTenant renders the tenant's detail row, the shape GET
// /admin/tenants/{tenantID} returns.
func (api *adminTenantsAPI) respondTenant(w http.ResponseWriter, r *http.Request, tenantID string) {
	if detail, ok := api.readTenantDetail(w, r, tenantID); ok {
		api.renderTenant(w, r, detail)
	}
}

// respondCreatedTenant renders the detail row of a tenant just created, with
// 201.
func (api *adminTenantsAPI) respondCreatedTenant(w http.ResponseWriter, r *http.Request, tenantID string) {
	if detail, ok := api.readTenantDetail(w, r, tenantID); ok {
		api.renderTenant(w, r, adminTenantCreatedResponse{Data: detail.Data})
	}
}

// readTenantDetail reads the tenant's detail row back after a write. It
// renders the error and returns false when that fails.
func (api *adminTenantsAPI) readTenantDetail(w http.ResponseWriter, r *http.Request, tenantID string) (*jsonapi.AdminTenantResponse, bool) {
	item, err := api.factorySet.TenantRegistry.GetAdmin(r.Context(), tenantID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to read the tenant back after an admin write", "tenant_id", tenantID, "error", err)
		_ = renderEntityError(w, r, err)
		return nil, false
	}
	return jsonapi.NewAdminTenantResponse(item), true
}

func (api *adminTenantsAPI) renderTenant(w http.ResponseWriter, r *http.Request, resp render.Renderer) {
	if err := render.Render(w, r, resp); err != nil {
		_ = internalServerError(w, r, err)
	}
}

// tenantWriteAudit is one audit row of a tenant write.
type tenantWriteAudit struct {
	action   string
	tenantID string
	reason   string
	err      error
	extra    map[string]any
}

// auditWrite records a tenant write. A status change whose sessions could not
// all be ended is recorded as a failure with the cause, like a user block
// whose cascade failed.
func (api *adminTenantsAPI) auditWrite(r *http.Request, ev tenantWriteAudit) {
	if api.auditService == nil {
		return
	}
	subject := nullableString(ev.tenantID)
	api.auditService.LogAdmin(r.Context(), services.AdminEvent{
		Action:      ev.action,
		ActorID:     actorIDFromRequest(r),
		TenantID:    subject,
		SubjectType: stringPtr("tenant"),
		SubjectID:   subject,
		Success:     ev.err == nil,
		Request:     r,
		ErrMsg:      strPtrFromErr(ev.err),
		Reason:      ev.reason,
		Extra:       ev.extra,
	})
}

// tenantUpdateBreadcrumb lists what an update asks to change, for the audit
// row.
func tenantUpdateBreadcrumb(tenant *models.Tenant, req AdminTenantUpdateRequest) map[string]any {
	extra := map[string]any{"slug": tenant.Slug}
	changed := make([]string, 0, 4)
	if req.Name != nil {
		changed = append(changed, "name")
	}
	if req.Slug != nil {
		changed = append(changed, "slug")
	}
	if req.Domain != nil {
		changed = append(changed, "domain")
	}
	if req.Status != nil {
		changed = append(changed, "status")
		extra["status_from"] = tenant.Status
		extra["status_to"] = *req.Status
	}
	extra["changed"] = changed
	return extra
}

// tenantServesRequest reports whether the request resolved to the tenant.
func tenantServesRequest(r *http.Request, tenantID string) bool {
	resolved := TenantFromContext(r.Context())
	return resolved != nil && resolved.ID == tenantID
}

// renderTenantWriteError maps the errors of the tenant writes to responses.
func renderTenantWriteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, registry.ErrSlugAlreadyExists):
		_ = codedConflictError(w, r, err, TenantSlugTakenCode, nil)
	case errors.Is(err, adminservice.ErrTenantDomainTaken):
		_ = codedConflictError(w, r, err, TenantDomainTakenCode, nil)
	case errors.Is(err, adminservice.ErrTenantDeleteNotConfirmed):
		_ = codedUnprocessableEntityError(w, r, err, AdminTenantConfirmationMismatchCode)
	case errors.Is(err, ErrTenantServesThisHost):
		_ = codedUnprocessableEntityError(w, r, err, AdminTenantServesThisHostCode)
	default:
		if verrs, ok := errors.AsType[validation.Errors](err); ok {
			// Under data.attributes, where the form fields are, so the
			// back office can show each message next to its field.
			_ = unprocessableEntityError(w, r, validation.Errors{"data": validation.Errors{"attributes": verrs}})
			return
		}
		_ = renderEntityError(w, r, err)
	}
}

// decodeAdminBody decodes a JSON body into dst, refusing unknown fields and
// trailing data. It writes a 400 and returns false when the body is not
// acceptable.
func decodeAdminBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Body == nil {
		_ = badRequest(w, r, errors.New("missing request body"))
		return false
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		_ = badRequest(w, r, err)
		return false
	}
	if !decoderAtEOF(dec) {
		_ = badRequest(w, r, errors.New("invalid JSON body — trailing tokens"))
		return false
	}
	return true
}

// decodeTenantStatusReason decodes the suspend and reactivate body and
// checks its reason.
func decodeTenantStatusReason(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req AdminTenantStatusRequest
	if !decodeAdminBody(w, r, &req) {
		return "", false
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		_ = codedUnprocessableEntityError(w, r, errors.New("reason is required"), AdminTenantReasonRequiredCode)
		return "", false
	}
	if utf8.RuneCountInString(reason) > adminTenantReasonMaxLen {
		_ = codedUnprocessableEntityError(w, r, errors.New("reason is too long"), AdminTenantReasonTooLongCode)
		return "", false
	}
	return reason, true
}

// trimmedPtr trims the string a pointer holds, keeping nil as nil.
func trimmedPtr(s *string) *string {
	if s == nil {
		return nil
	}
	return new(strings.TrimSpace(*s))
}
