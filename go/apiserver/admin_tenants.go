package apiserver

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// adminTenantsAPI backs GET /admin/tenants and GET
// /admin/tenants/{tenantID}. Holds the FactorySet directly (not the
// per-request user-aware Set) because the admin surface intentionally
// crosses tenants — using the user-aware registries would limit the
// admin to their own tenant, defeating the surface.
type adminTenantsAPI struct {
	factorySet   *registry.FactorySet
	auditService services.AuditLogger
}

// listTenants returns a paginated, filtered, sorted listing of every
// tenant in the deployment along with per-tenant computed user_count
// and group_count.
//
// The endpoint crosses tenants by design — see RequireSystemAdmin gate
// above — and the registry layer bypasses RLS via `SET LOCAL
// row_security = off` on the join tx so the count subqueries don't
// silently return zero when the connection role's BYPASSRLS attribute
// is revoked.
//
// @Summary List tenants (admin)
// @Description Returns every tenant with computed user_count and group_count. Pagination via ?page&per_page; ?q matches name/slug/domain (ILIKE); ?sort=<field> with optional `-` prefix for desc, or explicit ?order=asc|desc.
// @Tags admin
// @Produce json-api
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 50, max 100)"
// @Param q query string false "Search term — ILIKE match on name/slug/domain"
// @Param sort query string false "Sort field: name|slug|created_at|status (prefix with - for desc)"
// @Param order query string false "Sort direction override: asc|desc (wins over `-` prefix)"
// @Success 200 {object} jsonapi.AdminTenantsResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Router /admin/tenants [get]
func (api *adminTenantsAPI) listTenants(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))

	sortField, sortDesc := parseAdminSortAndOrder(q.Get("sort"), q.Get("order"))
	opts := registry.AdminTenantListOptions{
		Page:      page,
		PerPage:   perPage,
		Query:     q.Get("q"),
		SortField: registry.AdminTenantSortField(sortField),
		SortDesc:  sortDesc,
	}

	items, total, err := api.factorySet.TenantRegistry.ListAdmin(r.Context(), opts)
	api.auditList(r, "admin.list_tenants", err)
	if err != nil {
		_ = renderEntityError(w, r, err)
		return
	}

	setPaginationHeaders(w, page, perPage, total)
	if renderErr := render.Render(w, r, jsonapi.NewAdminTenantsResponse(items, page, perPage, total)); renderErr != nil {
		_ = internalServerError(w, r, renderErr)
	}
}

// getTenant returns a single tenant detail row (same shape as a list
// item — the issue spec is explicit: "No nested users / groups").
//
// @Summary Get tenant (admin)
// @Description Returns the tenant row with computed user_count and group_count. No nested users or groups list — those live behind GET /admin/tenants/{tenantID}/users (#1746).
// @Tags admin
// @Produce json-api
// @Param tenantID path string true "Tenant ID"
// @Success 200 {object} jsonapi.AdminTenantResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "Tenant not found"
// @Router /admin/tenants/{tenantID} [get]
func (api *adminTenantsAPI) getTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		api.auditGet(r, tenantID, registry.ErrNotFound)
		_ = renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	// GetAdmin runs a single tx with row_security=off plus COUNT
	// subqueries for user_count + group_count — semantics identical to
	// the listing endpoint and O(1) regardless of tenant size. The
	// previous "ListByTenant + len()" implementation scaled linearly
	// with tenant data and skipped the fail-loud RLS guard.
	item, err := api.factorySet.TenantRegistry.GetAdmin(r.Context(), tenantID)
	if err != nil {
		api.auditGet(r, tenantID, err)
		_ = renderEntityError(w, r, err)
		return
	}

	api.auditGet(r, tenantID, nil)
	if renderErr := render.Render(w, r, jsonapi.NewAdminTenantResponse(item)); renderErr != nil {
		_ = internalServerError(w, r, renderErr)
	}
}

// auditList records an `admin.list_tenants` audit row regardless of
// success/failure. Cross-tenant calls have no single tenant subject so
// SubjectType/SubjectID stay nil — the admin user (ActorID) and the
// success/error pair are what compliance reviewers want from the
// listing audit.
func (api *adminTenantsAPI) auditList(r *http.Request, action string, opErr error) {
	if api.auditService == nil {
		return
	}
	ev := services.AdminEvent{
		Action:  action,
		ActorID: actorIDFromRequest(r),
		Success: opErr == nil,
		Request: r,
		ErrMsg:  strPtrFromErr(opErr),
	}
	api.auditService.LogAdmin(r.Context(), ev)
}

// auditGet records an `admin.get_tenant` audit row. Captures the
// target tenant ID so the audit trail tracks "who looked at what".
func (api *adminTenantsAPI) auditGet(r *http.Request, tenantID string, opErr error) {
	if api.auditService == nil {
		return
	}
	subject := nullableString(tenantID)
	ev := services.AdminEvent{
		Action:      "admin.get_tenant",
		ActorID:     actorIDFromRequest(r),
		TenantID:    subject,
		SubjectType: stringPtr("tenant"),
		SubjectID:   subject,
		Success:     opErr == nil && !errors.Is(opErr, registry.ErrNotFound),
		Request:     r,
		ErrMsg:      strPtrFromErr(opErr),
	}
	api.auditService.LogAdmin(r.Context(), ev)
}

// actorIDFromRequest pulls the authenticated user ID off the context
// for use as the AdminEvent.ActorID field. Returns nil if the user is
// missing — RequireSystemAdmin should have rejected the request before
// reaching the handler, so a missing user here is a wiring bug, not a
// data-loss path.
func actorIDFromRequest(r *http.Request) *string {
	user := appctx.UserFromContext(r.Context())
	if user == nil || user.ID == "" {
		return nil
	}
	id := user.ID
	return &id
}

// parseTriStateBool decodes a ?key=true / ?key=false / unset query
// param into the *bool tri-state filter the registry layer expects.
// Unknown values are treated as "unset" rather than 4xx — the FE may
// send legacy variants ("True", "1") during a multi-version rollout;
// the strict subset is documented in the swagger annotation so the FE
// codegen knows the canonical form.
func parseTriStateBool(raw string) *bool {
	switch raw {
	case "true":
		t := true
		return &t
	case "false":
		f := false
		return &f
	}
	return nil
}
