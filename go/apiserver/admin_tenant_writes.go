package apiserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
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
	// through this tenant's host, and the change would take the tenant off
	// that host, which locks the operator out of the back office.
	AdminTenantServesThisHostCode = "admin.tenant.serves_this_host"
	// TenantSlugTakenCode: another tenant has the slug.
	TenantSlugTakenCode = "tenant.slug_taken"
	// TenantDomainTakenCode: another tenant has the domain.
	TenantDomainTakenCode = "tenant.domain_taken"
	// TenantDomainReservedCode: the domain is the tenant base domain or a host
	// under it, where every subdomain names a tenant by its slug.
	TenantDomainReservedCode = "tenant.domain_reserved"
)

const (
	// adminTenantStatusTimeout bounds a status change and the session teardown
	// it runs. Both run detached from the request's cancellation, so neither a
	// client that goes away nor the router's request timeout stops the
	// teardown half-way.
	adminTenantStatusTimeout = 10 * time.Minute
	// adminTenantDeleteTimeout bounds a tenant deletion, which runs in the
	// background after the request is answered.
	adminTenantDeleteTimeout = 2 * time.Hour
)

var (
	// ErrTenantServesThisHost is returned when a change would take the tenant
	// that the back-office request resolved to off the request's host: a
	// deletion, a status away from active, or a new slug or domain the host
	// was resolved by. Every /api/v1 request, the back office included, is
	// refused when its host resolves to no active tenant, so the change would
	// take the back office away with the tenant. The CLI can still make it.
	ErrTenantServesThisHost = errx.NewSentinel("the back office is served through this tenant's host; " +
		"the change would take the tenant off that host and lock the back office out")

	// ErrTenantDomainReserved is returned for a custom domain that is the
	// tenant base domain or a host under it. Hosts are matched against custom
	// domains before slugs, so such a domain would take over the host of the
	// tenant whose slug it spells.
	ErrTenantDomainReserved = errx.NewSentinel("the domain is the tenant base domain or a host under it, where subdomains name tenants by their slug")
)

var tenantReasonCodes = reasonCodes{required: AdminTenantReasonRequiredCode, tooLong: AdminTenantReasonTooLongCode}

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
// A field that is left out, or that equals the stored value, does not change.
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

// adminTenantWriteResponse renders the tenant detail row with a status other
// than 200. It holds the row rather than embedding
// jsonapi.AdminTenantResponse: render walks the fields after the parent and
// the embedded Render would set the status back to 200.
type adminTenantWriteResponse struct {
	Data   *jsonapi.AdminTenantListItem `json:"data"`
	status int
}

func (resp adminTenantWriteResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, resp.status)
	return nil
}

// createTenant creates a tenant.
//
// @Summary Create a tenant (admin)
// @Description Creates a tenant with the same rules as `inventario tenants create`: an empty slug is generated from the name, the status defaults to active and registration to closed.
// @Description A slug another tenant has is refused with 409 `tenant.slug_taken`, a domain another tenant has with 409 `tenant.domain_taken`, and invalid fields with 422 naming them under data.attributes.
// @Description A domain that is the tenant base domain or a host under it is refused with 422 `tenant.domain_reserved`.
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
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - invalid fields, or a reserved domain"
// @Router /admin/tenants [post]
func (api *adminTenantsAPI) createTenant(w http.ResponseWriter, r *http.Request) {
	var req AdminTenantCreateRequest
	if !decodeAdminBody(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	req.Domain = strings.TrimSpace(req.Domain)
	// What was asked for, so a refused create still says what it was.
	audit := tenantWriteAudit{
		action: AuditActionAdminTenantCreate,
		extra:  map[string]any{"name": auditValue(req.Name), "slug": auditValue(req.Slug), "domain": auditValue(req.Domain)},
	}

	var domain *string
	if req.Domain != "" {
		domain = new(req.Domain)
	}
	err := api.checkDomainOutsideBase(domain)
	var tenant *models.Tenant
	if err == nil {
		tenant, err = api.service.CreateTenant(r.Context(), adminservice.TenantCreateRequest{
			Name:   req.Name,
			Slug:   req.Slug,
			Domain: domain,
			Status: req.Status,
		})
	}
	if err != nil {
		audit.err = err
		api.auditWrite(r, audit)
		renderTenantWriteError(w, r, err)
		return
	}
	audit.tenantID = tenant.ID
	audit.extra = map[string]any{"name": tenant.Name, "slug": tenant.Slug, "domain": derefString(tenant.Domain), "status": tenant.Status}
	api.auditWrite(r, audit)
	api.respondTenantWithStatus(w, r, tenant.ID, http.StatusCreated)
}

// updateTenant changes a tenant's name, slug, domain or status.
//
// @Summary Update a tenant (admin)
// @Description Changes the fields the body sets to a value other than the stored one, with the same rules as `inventario tenants update`. An empty `domain` removes the custom domain. A body that changes nothing writes nothing.
// @Description A status away from active ends the sessions of every user in the tenant, as the suspend endpoint does.
// @Description A change that takes the tenant serving this request off the request's host is refused with 422 `admin.tenant.serves_this_host`: a status away from active and,
// @Description with a tenant base domain, a new domain when the request came through the tenant's domain or a new slug when it came another way.
// @Description A slug another tenant has is refused with 409 `tenant.slug_taken`, a domain another tenant has with 409 `tenant.domain_taken`, and invalid fields with 422 naming them under data.attributes.
// @Description A domain that is the tenant base domain or a host under it is refused with 422 `tenant.domain_reserved`.
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
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - invalid fields, a reserved domain, or the tenant serves this host"
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
	update, changes := tenantUpdateChanges(tenant, req)
	if len(changes) == 0 {
		// Nothing differs from the stored tenant: no write, no audit row.
		api.respondTenant(w, r, tenant.ID)
		return
	}
	audit := tenantWriteAudit{action: AuditActionAdminTenantUpdate, tenantID: tenant.ID, extra: tenantUpdateBreadcrumb(tenant, changes)}

	err := api.checkDomainOutsideBase(update.Domain)
	if err == nil && tenantServesRequest(r, tenant.ID) && api.movesOffRequestHost(r, tenant, update) {
		err = ErrTenantServesThisHost
	}
	if err == nil {
		ctx, cancel := detachedContext(r, adminTenantStatusTimeout)
		defer cancel()
		_, err = api.service.UpdateTenant(ctx, tenant.ID, update)
	}
	audit.err = err
	api.auditWrite(r, audit)
	if api.sessionsNotEnded(r, tenant, err) {
		api.respondTenant(w, r, tenant.ID)
		return
	}
	if err != nil {
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
	req, ok := decodeReasonBody(w, r, tenantReasonCodes, func(req *AdminTenantStatusRequest) *string { return &req.Reason })
	if !ok {
		return
	}
	audit := tenantWriteAudit{
		action:   action,
		tenantID: tenant.ID,
		reason:   req.Reason,
		extra:    map[string]any{"slug": tenant.Slug, "status_from": tenant.Status},
	}
	if action == AuditActionAdminTenantSuspend && tenantServesRequest(r, tenant.ID) {
		audit.err = ErrTenantServesThisHost
		api.auditWrite(r, audit)
		renderTenantWriteError(w, r, ErrTenantServesThisHost)
		return
	}
	ctx, cancel := detachedContext(r, adminTenantStatusTimeout)
	defer cancel()
	_, err := change(ctx, tenant.ID)
	audit.err = err
	api.auditWrite(r, audit)
	if api.sessionsNotEnded(r, tenant, err) {
		api.respondTenant(w, r, tenant.ID)
		return
	}
	if err != nil {
		renderTenantWriteError(w, r, err)
		return
	}
	api.respondTenant(w, r, tenant.ID)
}

// deleteTenant suspends a tenant and starts deleting it and everything in it.
//
// @Summary Delete a tenant (admin)
// @Description Suspends the tenant and ends its sessions, as the suspend endpoint does, and answers 202 with the suspended tenant.
// @Description It then deletes the tenant, its users, groups, items and files in the background with the rules of `inventario tenants delete`.
// @Description The stored files go after a dry run of the row purge and before the rows, so a purge that would fail or an unreachable file store stops the deletion with the rows in place. It cannot be undone.
// @Description A failed or interrupted deletion leaves the tenant suspended; sending the same request again continues it. The audit log records the request and the outcome.
// @Description `confirm_slug` must be the tenant's slug, the same confirmation the CLI prompt asks for; anything else is 422 `admin.tenant.confirmation_mismatch`.
// @Description Deleting the tenant that serves this request's host is refused with 422 `admin.tenant.serves_this_host`.
// @Description Requires the platform_admin role.
// @Tags admin
// @Produce json-api
// @Param tenantID path string true "Tenant ID"
// @Param confirm_slug query string true "The tenant's slug, typed to confirm"
// @Success 202 {object} jsonapi.AdminTenantResponse "Accepted - the tenant is suspended and its deletion is running"
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
		extra:    map[string]any{"slug": tenant.Slug, "name": tenant.Name, "status_from": tenant.Status, "phase": "accepted"},
	}
	err := adminservice.ConfirmTenantDeletion(tenant, r.URL.Query().Get("confirm_slug"))
	if err == nil && tenantServesRequest(r, tenant.ID) {
		err = ErrTenantServesThisHost
	}
	if err != nil {
		audit.err = err
		api.auditWrite(r, audit)
		renderTenantWriteError(w, r, err)
		return
	}

	// The deletion outlives the request. bg keeps the request's values, so
	// its log lines and audit rows carry the request id, but not its
	// cancellation.
	bg := r.Clone(context.WithoutCancel(r.Context()))

	// Suspend first, the way POST .../suspend does: the users are signed out
	// and stay out while the deletion runs, and a deletion that stops
	// half-way leaves a suspended tenant rather than a working one with part
	// of its data gone. A teardown that missed some sessions does not stop
	// the deletion: the users are refused while the tenant is suspended, and
	// the deletion removes their tokens.
	statusCtx, cancel := context.WithTimeout(bg.Context(), adminTenantStatusTimeout)
	_, err = api.service.SuspendTenant(statusCtx, tenant.ID)
	cancel()
	if api.sessionsNotEnded(r, tenant, err) {
		err = nil
	}
	if err != nil {
		audit.err = err
		api.auditWrite(r, audit)
		renderTenantWriteError(w, r, err)
		return
	}
	detail, ok := api.readTenantDetail(w, r, tenant.ID)
	if !ok {
		return
	}
	api.auditWrite(r, audit)
	api.startTenantDeletion(bg, tenant)
	api.renderTenant(w, r, adminTenantWriteResponse{Data: detail.Data, status: http.StatusAccepted})
}

// startTenantDeletion runs the deletion of the tenant in the background,
// unless this process is already deleting it.
func (api *adminTenantsAPI) startTenantDeletion(r *http.Request, tenant *models.Tenant) {
	if _, running := api.deletions.LoadOrStore(tenant.ID, struct{}{}); running {
		return
	}
	go api.runTenantDeletion(r, tenant)
}

// runTenantDeletion deletes the tenant and records the outcome. r is the
// detached copy of the request that started it.
func (api *adminTenantsAPI) runTenantDeletion(r *http.Request, tenant *models.Tenant) {
	defer api.deletions.Delete(tenant.ID)
	ctx, cancel := context.WithTimeout(r.Context(), adminTenantDeleteTimeout)
	defer cancel()

	err := api.deleteTenantData(ctx, tenant.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Tenant deletion failed; the tenant stays suspended until the deletion is sent again",
			"tenant_id", tenant.ID, "tenant_slug", tenant.Slug, "error", err)
	} else {
		slog.InfoContext(ctx, "Tenant deleted", "tenant_id", tenant.ID, "tenant_slug", tenant.Slug)
	}
	api.auditWrite(r, tenantWriteAudit{
		action:   AuditActionAdminTenantDelete,
		tenantID: tenant.ID,
		err:      err,
		extra:    map[string]any{"slug": tenant.Slug, "name": tenant.Name, "phase": "finished"},
	})
}

// deleteTenantData runs the service deletion. A panic is reported as an
// error, so it reaches the audit log instead of taking the server down.
func (api *adminTenantsAPI) deleteTenantData(ctx context.Context, tenantID string) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("tenant deletion panicked: %v", p)
		}
	}()
	return api.service.DeleteTenant(ctx, tenantID)
}

// sessionsNotEnded reports whether err says that a status change was saved but
// ending some of the tenant's sessions failed, and logs it when so. The users
// are still refused, because their tenant is not active.
func (api *adminTenantsAPI) sessionsNotEnded(r *http.Request, tenant *models.Tenant, err error) bool {
	if !errors.Is(err, adminservice.ErrTenantSessionsNotEnded) {
		return false
	}
	slog.ErrorContext(r.Context(), "Tenant status saved, but ending the sessions of some of its users failed",
		"tenant_id", tenant.ID, "tenant_slug", tenant.Slug, "error", err)
	return true
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

// checkDomainOutsideBase refuses a custom domain that is the tenant base
// domain or a host under it. Without a base domain (single-tenant mode) no
// host names a tenant by its slug, so any domain is accepted.
func (api *adminTenantsAPI) checkDomainOutsideBase(domain *string) error {
	if api.baseDomain == "" || domain == nil || *domain == "" {
		return nil
	}
	host := normalizeHost(*domain)
	if host == api.baseDomain || strings.HasSuffix(host, "."+api.baseDomain) {
		return errxtrace.Classify(ErrTenantDomainReserved, errx.Attrs("domain", *domain, "base_domain", api.baseDomain))
	}
	return nil
}

// respondTenant renders the tenant's detail row, the shape GET
// /admin/tenants/{tenantID} returns.
func (api *adminTenantsAPI) respondTenant(w http.ResponseWriter, r *http.Request, tenantID string) {
	if detail, ok := api.readTenantDetail(w, r, tenantID); ok {
		api.renderTenant(w, r, detail)
	}
}

// respondTenantWithStatus renders the tenant's detail row with status.
func (api *adminTenantsAPI) respondTenantWithStatus(w http.ResponseWriter, r *http.Request, tenantID string, status int) {
	if detail, ok := api.readTenantDetail(w, r, tenantID); ok {
		api.renderTenant(w, r, adminTenantWriteResponse{Data: detail.Data, status: status})
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
// whose cascade failed. The row is written even when the client has gone
// away or the request timed out: it is detached from the request's
// cancellation.
func (api *adminTenantsAPI) auditWrite(r *http.Request, ev tenantWriteAudit) {
	if api.auditService == nil {
		return
	}
	subject := nullableString(ev.tenantID)
	api.auditService.LogAdmin(context.WithoutCancel(r.Context()), services.AdminEvent{
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

// tenantFieldChange is one field an update changes, with the stored value
// and the new one.
type tenantFieldChange struct {
	field    string
	from, to any
}

// tenantUpdateChanges keeps the fields of req whose trimmed value differs
// from the stored tenant, and returns them as a service request and as a
// list for the audit row.
func tenantUpdateChanges(tenant *models.Tenant, req AdminTenantUpdateRequest) (adminservice.TenantUpdateRequest, []tenantFieldChange) {
	var update adminservice.TenantUpdateRequest
	var changes []tenantFieldChange
	if name := trimmedPtr(req.Name); name != nil && *name != tenant.Name {
		update.Name = name
		changes = append(changes, tenantFieldChange{"name", tenant.Name, auditValue(*name)})
	}
	if slug := trimmedPtr(req.Slug); slug != nil && *slug != tenant.Slug {
		update.Slug = slug
		changes = append(changes, tenantFieldChange{"slug", tenant.Slug, auditValue(*slug)})
	}
	if domain := trimmedPtr(req.Domain); domain != nil && *domain != derefString(tenant.Domain) {
		update.Domain = domain
		changes = append(changes, tenantFieldChange{"domain", derefString(tenant.Domain), auditValue(*domain)})
	}
	if req.Status != nil && *req.Status != tenant.Status {
		update.Status = req.Status
		changes = append(changes, tenantFieldChange{"status", tenant.Status, *req.Status})
	}
	return update, changes
}

// tenantUpdateBreadcrumb records what an update changes, from which value to
// which, for the audit row.
func tenantUpdateBreadcrumb(tenant *models.Tenant, changes []tenantFieldChange) map[string]any {
	extra := map[string]any{"slug": tenant.Slug}
	fields := make([]string, 0, len(changes))
	for _, change := range changes {
		fields = append(fields, change.field)
		extra[change.field+"_from"] = change.from
		extra[change.field+"_to"] = change.to
	}
	extra["changed"] = fields
	return extra
}

// movesOffRequestHost reports whether the update takes the tenant off the
// host this request reached it on. A status away from active always does.
// With a base domain, a new domain does when the request came through the
// tenant's domain, and a new slug does when it came any other way: through
// <slug>.<base domain> or as the catch-all tenant. In single-tenant mode no
// host is resolved by a slug, and a host whose domain no tenant has falls
// back to the default tenant, so neither edit can lock the back office out.
func (api *adminTenantsAPI) movesOffRequestHost(r *http.Request, tenant *models.Tenant, update adminservice.TenantUpdateRequest) bool {
	if update.Status != nil && *update.Status != models.TenantStatusActive {
		return true
	}
	if api.baseDomain == "" {
		return false
	}
	byDomain := tenant.Domain != nil && normalizeHost(*tenant.Domain) == normalizeHost(r.Host)
	if byDomain {
		return update.Domain != nil
	}
	return update.Slug != nil
}

// tenantServesRequest reports whether the request resolved to the tenant.
func tenantServesRequest(r *http.Request, tenantID string) bool {
	resolved := TenantFromContext(r.Context())
	return resolved != nil && resolved.ID == tenantID
}

// detachedContext returns a context that keeps the request's values, the
// request id among them, but not its cancellation, bounded by timeout.
func detachedContext(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(r.Context()), timeout)
}

// renderTenantWriteError maps the errors of the tenant writes to responses.
func renderTenantWriteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, registry.ErrSlugAlreadyExists):
		_ = codedConflictError(w, r, err, TenantSlugTakenCode, nil)
	case errors.Is(err, adminservice.ErrTenantDomainTaken):
		_ = codedConflictError(w, r, err, TenantDomainTakenCode, nil)
	case errors.Is(err, ErrTenantDomainReserved):
		_ = codedUnprocessableEntityError(w, r, err, TenantDomainReservedCode)
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

// auditValueMaxLen caps a value the operator typed in the audit breadcrumb.
// It is above every field's own limit, so it only shortens values the write
// refuses.
const auditValueMaxLen = 255

// auditValue shortens s to auditValueMaxLen characters for the audit row.
func auditValue(s string) string {
	if utf8.RuneCountInString(s) <= auditValueMaxLen {
		return s
	}
	return string([]rune(s)[:auditValueMaxLen]) + "…"
}

// trimmedPtr trims the string a pointer holds, keeping nil as nil.
func trimmedPtr(s *string) *string {
	if s == nil {
		return nil
	}
	return new(strings.TrimSpace(*s))
}

// derefString returns the string s points to, or "" for nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
