package apiserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// AdminPingResponse is the body returned by GET /admin/_ping. It is
// intentionally tiny — the endpoint exists so the FE (and the swagger
// route-coverage test) has something to hit while the rest of the
// /api/v1/admin/* surface is being built out under the #1744 umbrella.
type AdminPingResponse struct {
	Ok        bool      `json:"ok"`
	Timestamp time.Time `json:"timestamp"`
}

// adminPing is the placeholder handler behind RequireSystemAdmin.
// Returns 200 with a simple JSON body so the FE can detect "I have
// system-admin" without needing a richer endpoint until later
// admin issues land.
// @Summary System-admin ping
// @Description Returns 200 when the caller has system-admin privileges. Probe endpoint for the admin surface (#1745).
// @Tags admin
// @Produce json
// @Success 200 {object} AdminPingResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Router /admin/_ping [get]
func adminPing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(AdminPingResponse{
		Ok:        true,
		Timestamp: time.Now().UTC(),
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// AdminParams is the set of dependencies the admin subtree needs to
// resolve cross-tenant data. The FactorySet is held directly (not the
// per-request user-aware Set) because admin endpoints intentionally
// cross tenants — using the user-aware registries would limit the
// admin to their own tenant, defeating the surface. AuditService is
// shared with the rest of the apiserver so the admin audit trail
// lands in the same audit_logs table. Blacklist is needed by the
// block/unblock cascade (#1747) to bump the JWT-iat staleness
// threshold so live access tokens minted before the block fail on
// next use.
type AdminParams struct {
	FactorySet   *registry.FactorySet
	Blacklist    services.TokenBlacklister
	AuditService services.AuditLogger
}

// Admin returns the router configurator for /api/v1/admin/*. Mounted
// from apiserver.go behind the standard userMiddlewares (JWT + RLS +
// CSRF) and the RequireSystemAdmin gate. Later admin issues hang their
// endpoints off the same chi.Router this closure receives.
func Admin(params AdminParams) func(r chi.Router) {
	tenantsAPI := &adminTenantsAPI{
		factorySet:   params.FactorySet,
		auditService: params.AuditService,
	}
	usersAPI := &adminUsersAPI{
		factorySet:   params.FactorySet,
		blacklist:    params.Blacklist,
		auditService: params.AuditService,
	}
	groupsAPI := &adminGroupsAPI{
		factorySet:   params.FactorySet,
		auditService: params.AuditService,
	}
	return func(r chi.Router) {
		// RequireSystemAdmin runs as the first per-subtree middleware so
		// every handler below it can assume the caller is a system admin.
		r.Use(RequireSystemAdmin)
		r.Get("/_ping", adminPing)

		// #1746: tenants + users listing endpoints. Each handler
		// audit-logs via params.AuditService and bypasses RLS at the
		// registry layer (correlated COUNT subqueries on RLS-enabled
		// tables run with `SET LOCAL row_security = off` for
		// defense-in-depth).
		r.Get("/tenants", tenantsAPI.listTenants)
		r.Get("/tenants/{tenantID}", tenantsAPI.getTenant)
		r.Get("/tenants/{tenantID}/users", usersAPI.listTenantUsers)
		r.Get("/users/{userID}", usersAPI.getUser)

		// #1747: user block/unblock endpoints. Each transition cascades
		// IsActive=false → refresh-token revoke → blacklist bump and
		// audit-logs reason+forced via the breadcrumb in user_agent.
		r.Post("/users/{userID}/block", usersAPI.blockUser)
		r.Post("/users/{userID}/unblock", usersAPI.unblockUser)

		// #1748: cross-tenant groups admin. List + detail bypass RLS at
		// the registry layer (SET LOCAL row_security = off); DELETE flips
		// status to pending_deletion (idempotent) and the existing
		// group_purge_worker finishes the hard-delete. Each handler
		// audit-logs via params.AuditService.
		r.Get("/groups", groupsAPI.listGroups)
		r.Get("/groups/{groupID}", groupsAPI.getGroup)
		r.Delete("/groups/{groupID}", groupsAPI.deleteGroup)
	}
}

// parseAdminSort splits a `?sort=<field>` query value into the (field, desc)
// pair the registry layer expects. A leading `-` reverses the natural
// order (e.g. `-created_at` → desc by created_at). Unknown fields are
// left as-is and the registry layer falls back to its default sort —
// surface drift across FE/BE versions is intentionally tolerated to
// keep the listing endpoint responsive during deploys.
func parseAdminSort(raw string) (field string, desc bool) {
	if raw == "" {
		return "", false
	}
	if raw[0] == '-' {
		return raw[1:], true
	}
	return raw, false
}

// parseAdminSortAndOrder reconciles the dual sort conventions the
// admin FE may send: the `?sort=-name` shorthand (consistent with the
// rest of the API) or the explicit `?sort=name&order=desc` pair (which
// some FE table libs prefer). An explicit `order=` param always wins
// over a leading `-` prefix so the FE never has to strip the prefix
// before re-sending the current sort with a flipped direction. Unknown
// `order=` values (e.g. `?order=ascending`) are intentionally ignored
// rather than rejected so the FE can drift slightly across versions —
// the registry layer further whitelists the sort field via IsValid().
func parseAdminSortAndOrder(rawSort, rawOrder string) (field string, desc bool) {
	field, desc = parseAdminSort(rawSort)
	switch strings.ToLower(strings.TrimSpace(rawOrder)) {
	case "asc":
		desc = false
	case "desc":
		desc = true
	}
	return field, desc
}
