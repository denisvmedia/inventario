package apiserver

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// adminGroupsAPI backs GET /admin/groups, GET /admin/groups/{groupID} and
// DELETE /admin/groups/{groupID}. Holds the FactorySet directly (not the
// per-request user-aware Set) because the admin surface intentionally
// crosses tenants — using the user-aware registries would limit the admin
// to their own tenant, defeating the surface.
type adminGroupsAPI struct {
	factorySet   *registry.FactorySet
	auditService services.AuditLogger
}

// listGroups returns a paginated, filtered, sorted listing of every
// location group in the deployment along with each group's computed
// member_count.
//
// The endpoint crosses tenants by design — see RequireSystemAdmin gate.
// The registry layer runs the listing as the background-worker role,
// which carries RLS bypass policies on location_groups; the cross-tenant
// read is therefore unconditional, not gated on a fail-loud trip-wire.
//
// @Summary List groups (admin)
// @Description Returns every location group with computed member_count and an owning-tenant chip (id, name, slug). Paginate via ?page&per_page; ?q ILIKE-matches name/slug; ?tenantID/?status filter exactly; ?sort/?order set ordering.
// @Tags admin
// @Produce json-api
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 50, max 100)"
// @Param q query string false "Search term — ILIKE match on name/slug"
// @Param tenantID query string false "Filter to groups belonging to this tenant ID (exact match)"
// @Param status query string false "Filter to groups in this status (exact match)" Enums(active,pending_deletion)
// @Param sort query string false "Sort field: name|slug|created_at|status (prefix with - for desc)"
// @Param order query string false "Sort direction override (wins over `-` prefix)" Enums(asc,desc)
// @Success 200 {object} jsonapi.AdminGroupsResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Router /admin/groups [get]
func (api *adminGroupsAPI) listGroups(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))

	sortField, sortDesc := parseAdminSortAndOrder(q.Get("sort"), q.Get("order"))
	opts := registry.AdminGroupListOptions{
		Page:    page,
		PerPage: perPage,
		Query:   q.Get("q"),
		// The tenant filter is read from `tenantID`. The global
		// ValidateNoUserProvidedTenantID middleware normally rejects any
		// query parameter whose name contains "tenant", but it exempts the
		// /api/v1/admin/* subtree from that check by design — see the
		// rationale in isAdminSubtreePath / ValidateNoUserProvidedTenantID.
		TenantID: q.Get("tenantID"),
		// Status is an exact-match filter. An unknown value (a status the
		// model doesn't define) is passed straight through to the SQL
		// `g.status = $N` predicate and simply matches no rows — the page
		// comes back empty rather than 4xx. This is intentional and
		// consistent with the ?sort "tolerate FE drift" rationale in
		// parseAdminSortAndOrder: a FE drifting across versions never gets
		// a hard error from a filter param.
		Status:    q.Get("status"),
		SortField: registry.AdminGroupSortField(sortField),
		SortDesc:  sortDesc,
	}

	items, total, err := api.factorySet.LocationGroupRegistry.ListAdmin(r.Context(), opts)
	if err != nil {
		api.auditList(r, err)
		_ = renderEntityError(w, r, err)
		return
	}

	setPaginationHeaders(w, page, perPage, total)
	// Audit AFTER render so a JSON-encoding / response-writer failure
	// turns into a Success=false row instead of silently claiming the
	// client got their data — see adminTenantsAPI.listTenants for the
	// full rationale.
	renderErr := render.Render(w, r, jsonapi.NewAdminGroupsResponse(items, page, perPage, total))
	api.auditList(r, renderErr)
	if renderErr != nil {
		_ = internalServerError(w, r, renderErr)
	}
}

// getGroup returns a single group detail row: name, slug, status,
// currency, created_by, member_count and the owning-tenant chip.
//
// @Summary Get group (admin)
// @Description Returns the group row with computed member_count and an owning-tenant chip (tenant id, name, slug). Crosses tenants by design.
// @Tags admin
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Success 200 {object} jsonapi.AdminGroupResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "Group not found"
// @Router /admin/groups/{groupID} [get]
func (api *adminGroupsAPI) getGroup(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupID")
	if groupID == "" {
		api.auditGet(r, groupID, "", registry.ErrNotFound)
		_ = renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	item, err := api.factorySet.LocationGroupRegistry.GetAdmin(r.Context(), groupID)
	if err != nil {
		api.auditGet(r, groupID, "", err)
		_ = renderEntityError(w, r, err)
		return
	}

	// Audit AFTER render — see listGroups for the rationale.
	renderErr := render.Render(w, r, jsonapi.NewAdminGroupResponse(item))
	api.auditGet(r, groupID, tenantIDOfGroup(item), renderErr)
	if renderErr != nil {
		_ = internalServerError(w, r, renderErr)
	}
}

// deleteGroup soft-deletes a group by flipping its status to
// pending_deletion. The existing group_purge_worker then finishes the
// hard-delete on its next sweep — there is no parallel purge code path;
// the admin DELETE only owns the status transition, identical to
// GroupService.InitiateGroupDeletion.
//
// Idempotent: re-deleting an already-pending group returns 200 with the
// current status rather than erroring, so a retried request (or a second
// operator clicking delete) is not surprised by a 4xx.
//
// @Summary Soft-delete a group (admin)
// @Description Sets the group's status to `pending_deletion`; the group purge worker finishes the hard-delete asynchronously. Idempotent — re-deleting an already-pending group returns 200. Returns the post-transition group row.
// @Tags admin
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Success 200 {object} jsonapi.AdminGroupResponse "OK - group marked pending_deletion (or already was)"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "Group not found"
// @Router /admin/groups/{groupID} [delete]
func (api *adminGroupsAPI) deleteGroup(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupID")
	if groupID == "" {
		api.auditDelete(r, groupID, "", registry.ErrNotFound)
		_ = renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	// MarkPendingDeletionAdmin owns the status transition (identical to
	// GroupService.InitiateGroupDeletion) and returns the post-transition
	// detail row directly — group row + member_count + tenant chip,
	// computed in the SAME transaction as the status write. There is no
	// second GetAdmin round-trip: re-fetching after the soft-delete
	// commits would race the group_purge_worker, which could hard-delete
	// the now-pending row and turn a DELETE that actually succeeded into
	// a spurious 404. alreadyPending reports whether the group was
	// already pending so the idempotent re-delete renders a plain 200; it
	// is data, not control flow — either way the post-state is
	// pending_deletion and we render `item`.
	item, alreadyPending, err := api.factorySet.LocationGroupRegistry.MarkPendingDeletionAdmin(r.Context(), groupID)
	if err != nil {
		api.auditDelete(r, groupID, "", err)
		_ = renderEntityError(w, r, err)
		return
	}

	// Audit AFTER render. The audit row captures the admin actor, the
	// group as subject, and the group's tenant — the spec's required
	// trio. `already_pending` rides the breadcrumb so an operator can
	// tell a genuine soft-delete from an idempotent no-op without
	// re-deriving it from timestamps. The soft-delete already committed,
	// so the audit row records Success=true on the happy path; only a
	// render blip flips it to false.
	renderErr := render.Render(w, r, jsonapi.NewAdminGroupResponse(item))
	api.auditDeleteResult(r, groupID, tenantIDOfGroup(item), alreadyPending, renderErr)
	if renderErr != nil {
		_ = internalServerError(w, r, renderErr)
	}
}

// tenantIDOfGroup pulls the tenant ID off a resolved admin group detail
// for the audit row's TenantID field. Returns "" when the detail or its
// group is nil (a failed lookup path) so the audit helper records SQL
// NULL rather than a bogus tenant.
func tenantIDOfGroup(item *registry.AdminGroupDetail) string {
	if item == nil || item.Group == nil {
		return ""
	}
	return item.Group.TenantID
}

// auditList records an `admin.list_groups` audit row regardless of
// success/failure. The cross-tenant listing has no single tenant or group
// subject so SubjectType/SubjectID/TenantID stay nil — the admin actor and
// the success/error pair are what the listing audit captures.
func (api *adminGroupsAPI) auditList(r *http.Request, opErr error) {
	if api.auditService == nil {
		return
	}
	api.auditService.LogAdmin(r.Context(), services.AdminEvent{
		Action:  "admin.list_groups",
		ActorID: actorIDFromRequest(r),
		Success: opErr == nil,
		Request: r,
		ErrMsg:  strPtrFromErr(opErr),
	})
}

// auditGet records an `admin.get_group` audit row. Captures the target
// group as subject and, when resolved, the group's tenant.
func (api *adminGroupsAPI) auditGet(r *http.Request, groupID, tenantID string, opErr error) {
	if api.auditService == nil {
		return
	}
	api.auditService.LogAdmin(r.Context(), services.AdminEvent{
		Action:      "admin.get_group",
		ActorID:     actorIDFromRequest(r),
		TenantID:    nullableString(tenantID),
		SubjectType: stringPtr("group"),
		SubjectID:   nullableString(groupID),
		Success:     opErr == nil && !errors.Is(opErr, registry.ErrNotFound),
		Request:     r,
		ErrMsg:      strPtrFromErr(opErr),
	})
}

// auditDelete records an `admin.delete_group` failure row for the early
// error paths (missing ID, transition failure) where the post-transition
// state was never observed. The success path uses auditDeleteResult so
// the breadcrumb can carry `already_pending`.
func (api *adminGroupsAPI) auditDelete(r *http.Request, groupID, tenantID string, opErr error) {
	if api.auditService == nil {
		return
	}
	api.auditService.LogAdmin(r.Context(), services.AdminEvent{
		Action:      "admin.delete_group",
		ActorID:     actorIDFromRequest(r),
		TenantID:    nullableString(tenantID),
		SubjectType: stringPtr("group"),
		SubjectID:   nullableString(groupID),
		Success:     opErr == nil && !errors.Is(opErr, registry.ErrNotFound),
		Request:     r,
		ErrMsg:      strPtrFromErr(opErr),
	})
}

// auditDeleteResult records the `admin.delete_group` row for the path that
// actually reached the soft-delete. It carries the required actor + group
// + tenant trio and tucks `already_pending` into the breadcrumb so audit
// consumers can distinguish a genuine transition from an idempotent
// no-op. opErr here is the render error only — the soft-delete itself
// already committed, so a render blip is a Success=false row.
func (api *adminGroupsAPI) auditDeleteResult(r *http.Request, groupID, tenantID string, alreadyPending bool, opErr error) {
	if api.auditService == nil {
		return
	}
	api.auditService.LogAdmin(r.Context(), services.AdminEvent{
		Action:      "admin.delete_group",
		ActorID:     actorIDFromRequest(r),
		TenantID:    nullableString(tenantID),
		SubjectType: stringPtr("group"),
		SubjectID:   nullableString(groupID),
		Success:     opErr == nil,
		Request:     r,
		ErrMsg:      strPtrFromErr(opErr),
		Extra:       map[string]any{"already_pending": alreadyPending},
	})
}
