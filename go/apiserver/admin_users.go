package apiserver

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// adminUsersAPI backs GET /admin/tenants/{tenantID}/users and GET
// /admin/users/{userID}. Holds the FactorySet directly (not the
// per-request user-aware Set) for the same cross-tenant reason
// adminTenantsAPI does.
type adminUsersAPI struct {
	factorySet   *registry.FactorySet
	auditService services.AuditLogger
}

// listTenantUsers returns a paginated, filtered, sorted listing of
// every user in the target tenant. The endpoint crosses tenants — the
// admin caller may not be a member of the target tenant — and the
// registry layer bypasses RLS via `SET LOCAL row_security = off` for
// defense-in-depth.
//
// @Summary List users for a tenant (admin)
// @Description Returns users in the target tenant with computed group_membership_count. Tri-state ?is_active. Pagination via ?page&per_page; ?q matches email/name (ILIKE); ?sort=<field> with `-` prefix or ?order=asc|desc.
// @Tags admin
// @Produce json-api
// @Param tenantID path string true "Tenant ID"
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 50, max 100)"
// @Param q query string false "Search term — ILIKE match on email/name"
// @Param is_active query boolean false "Tri-state active-flag filter: true (active only), false (inactive only), or omit the param entirely for no filter. Unknown values are ignored."
// @Param sort query string false "Sort field: email|name|created_at|last_login_at|is_active (prefix with - for desc)"
// @Param order query string false "Sort direction override: asc|desc (wins over `-` prefix)"
// @Success 200 {object} jsonapi.AdminUsersResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "Tenant not found"
// @Router /admin/tenants/{tenantID}/users [get]
func (api *adminUsersAPI) listTenantUsers(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		api.auditListUsers(r, tenantID, registry.ErrNotFound)
		_ = renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	// Probe the tenant first so a typo in the URL surfaces as 404
	// rather than an empty listing. The probe and the listing share a
	// single audit row keyed on `admin.list_tenant_users`.
	if _, err := api.factorySet.TenantRegistry.Get(r.Context(), tenantID); err != nil {
		api.auditListUsers(r, tenantID, err)
		_ = renderEntityError(w, r, err)
		return
	}

	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))
	sortField, sortDesc := parseAdminSortAndOrder(q.Get("sort"), q.Get("order"))

	opts := registry.AdminUserListOptions{
		Page:      page,
		PerPage:   perPage,
		Query:     q.Get("q"),
		IsActive:  parseTriStateBool(q.Get("is_active")),
		SortField: registry.AdminUserSortField(sortField),
		SortDesc:  sortDesc,
	}

	items, total, err := api.factorySet.UserRegistry.ListAdminByTenant(r.Context(), tenantID, opts)
	api.auditListUsers(r, tenantID, err)
	if err != nil {
		_ = renderEntityError(w, r, err)
		return
	}

	setPaginationHeaders(w, page, perPage, total)
	if renderErr := render.Render(w, r, jsonapi.NewAdminUsersResponse(items, page, perPage, total)); renderErr != nil {
		_ = internalServerError(w, r, renderErr)
	}
}

// getUser returns the full user detail row: identity fields,
// is_active, last_login_at, the resolved group memberships, and the
// active_session_count from refresh_tokens. The handler resolves
// memberships by joining via the GroupMembership registry and
// fetching the matching LocationGroup rows in a small batched lookup
// so the FE doesn't N+1 the group registry per row.
//
// @Summary Get user (admin)
// @Description Returns the user detail across tenants: identity, is_active, last_login_at, group memberships (group_id, group_slug, group_name, role, joined_at), and active_session_count from unrevoked refresh tokens. No password hash.
// @Tags admin
// @Produce json-api
// @Param userID path string true "User ID"
// @Success 200 {object} jsonapi.AdminUserResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "User not found"
// @Router /admin/users/{userID} [get]
func (api *adminUsersAPI) getUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		api.auditGetUser(r, userID, "", registry.ErrNotFound)
		_ = renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	user, err := api.factorySet.UserRegistry.Get(r.Context(), userID)
	if err != nil {
		api.auditGetUser(r, userID, "", err)
		_ = renderEntityError(w, r, err)
		return
	}

	// Memberships are part of the resource identity (the FE renders
	// group access on this page), so a registry failure here is a 500.
	// Contrast with the session-count path below, which degrades
	// silently because active_session_count is derived metadata and
	// the user-detail row is still useful without it.
	memberships, mlErr := api.factorySet.GroupMembershipRegistry.ListByUser(r.Context(), user.TenantID, user.ID)
	if mlErr != nil && !errors.Is(mlErr, registry.ErrNotFound) {
		api.auditGetUser(r, userID, user.TenantID, mlErr)
		_ = renderEntityError(w, r, mlErr)
		return
	}

	// Resolve each membership to (group_id, group_slug, group_name).
	// The membership rows carry only the group_id; the slug + name
	// live on LocationGroup, fetched per row. Tenant-scoped lookup
	// (ListByTenant) is one round-trip with an in-memory join.
	groupsByID := map[string]*models.LocationGroup{}
	if user.TenantID != "" {
		groups, glErr := api.factorySet.LocationGroupRegistry.ListByTenant(r.Context(), user.TenantID)
		if glErr != nil {
			api.auditGetUser(r, userID, user.TenantID, glErr)
			_ = renderEntityError(w, r, glErr)
			return
		}
		for _, g := range groups {
			groupsByID[g.ID] = g
		}
	}

	mems := make([]jsonapi.AdminUserGroupMembership, 0, len(memberships))
	for _, m := range memberships {
		row := jsonapi.AdminUserGroupMembership{
			GroupID:  m.GroupID,
			Role:     m.Role,
			JoinedAt: m.JoinedAt,
		}
		if g, ok := groupsByID[m.GroupID]; ok {
			row.GroupSlug = g.Slug
			row.GroupName = g.Name
		}
		mems = append(mems, row)
	}

	sessionCount, sErr := api.factorySet.UserRegistry.CountSessionsByUser(r.Context(), user.ID)
	if sErr != nil {
		// Count failures degrade to zero rather than 500 so a transient
		// registry hiccup doesn't hide the user detail. The audit row
		// still flags the failure via a separate `admin.get_user_sessions`
		// action so operators can spot a pattern of session-count
		// outages without taking down the primary endpoint — note the
		// primary `admin.get_user` row reports Success: true, so audit
		// readers correlating on ActorID + timestamp see both rows.
		api.auditSessionCountFailure(r, user.ID, user.TenantID, sErr)
		sessionCount = 0
	}

	api.auditGetUser(r, userID, user.TenantID, nil)
	if renderErr := render.Render(w, r, jsonapi.NewAdminUserResponse(jsonapi.AdminUserResponseInput{
		User:               user,
		Memberships:        mems,
		ActiveSessionCount: sessionCount,
	})); renderErr != nil {
		_ = internalServerError(w, r, renderErr)
	}
}

// auditSessionCountFailure records the secondary `admin.get_user_sessions`
// audit row when CountSessionsByUser fails. The primary
// `admin.get_user` row still reports Success: true because the user
// detail itself rendered correctly; this row exists so audit consumers
// can spot a pattern of session-count outages.
func (api *adminUsersAPI) auditSessionCountFailure(r *http.Request, userID, tenantID string, opErr error) {
	if api.auditService == nil {
		return
	}
	api.auditService.LogAdmin(r.Context(), services.AdminEvent{
		Action:      "admin.get_user_sessions",
		ActorID:     actorIDFromRequest(r),
		TenantID:    nullableString(tenantID),
		SubjectType: stringPtr("user"),
		SubjectID:   nullableString(userID),
		Success:     false,
		Request:     r,
		ErrMsg:      strPtrFromErr(opErr),
	})
}

// auditListUsers records an `admin.list_tenant_users` audit row keyed
// to the target tenant.
func (api *adminUsersAPI) auditListUsers(r *http.Request, tenantID string, opErr error) {
	if api.auditService == nil {
		return
	}
	ev := services.AdminEvent{
		Action:      "admin.list_tenant_users",
		ActorID:     actorIDFromRequest(r),
		TenantID:    nullableString(tenantID),
		SubjectType: stringPtr("tenant"),
		SubjectID:   nullableString(tenantID),
		Success:     opErr == nil && !errors.Is(opErr, registry.ErrNotFound),
		Request:     r,
		ErrMsg:      strPtrFromErr(opErr),
	}
	api.auditService.LogAdmin(r.Context(), ev)
}

// auditGetUser records an `admin.get_user` audit row keyed to the
// target user and (when resolved) the user's tenant.
func (api *adminUsersAPI) auditGetUser(r *http.Request, userID, tenantID string, opErr error) {
	if api.auditService == nil {
		return
	}
	ev := services.AdminEvent{
		Action:      "admin.get_user",
		ActorID:     actorIDFromRequest(r),
		TenantID:    nullableString(tenantID),
		SubjectType: stringPtr("user"),
		SubjectID:   nullableString(userID),
		Success:     opErr == nil && !errors.Is(opErr, registry.ErrNotFound),
		Request:     r,
		ErrMsg:      strPtrFromErr(opErr),
	}
	api.auditService.LogAdmin(r.Context(), ev)
}

// nullableString returns nil for empty strings, &s otherwise. Used by
// the admin audit helpers where TenantID/SubjectID columns are
// nullable on the audit_logs schema and we want "missing" represented
// as SQL NULL rather than an empty string.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
}

func stringPtr(s string) *string {
	v := s
	return &v
}

func strPtrFromErr(err error) *string {
	if err == nil {
		return nil
	}
	msg := err.Error()
	return &msg
}
