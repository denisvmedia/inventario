package apiserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/csrf"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// defaultAdminImpersonationStore lets Admin() fall back to an in-memory
// return-slot store when AdminParams.ImpersonationStore is unset (tests,
// memory-mode bootstrap). Production wiring passes one explicitly so the
// store lifetime is owned by the caller.
func defaultAdminImpersonationStore(s services.ImpersonationStore) services.ImpersonationStore {
	if s != nil {
		return s
	}
	return services.NewInMemoryImpersonationStore()
}

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
	// GroupService backs the #1749 group-membership admin endpoints
	// (add / remove / role-change). The same instance the rest of the
	// apiserver uses — its registries are the tenant-scoped (not
	// user-aware) FactorySet ones, which is exactly what the admin
	// surface needs to cross tenants. Reusing the service keeps the
	// membership invariants (cap, ≥1 owner, ≥1 member) single-sourced
	// instead of forking a parallel code path for admins.
	GroupService *services.GroupService

	// JWTSecret signs the impersonation access tokens issued by the
	// #1750 impersonate endpoint. Same secret the rest of the apiserver
	// uses so an impersonation token validates through the standard
	// JWTMiddleware path.
	JWTSecret []byte

	// RateLimiter enforces the per-admin impersonation-start rate limit
	// (#1750). The same AuthRateLimiter instance the auth endpoints use;
	// a nil value disables the limit (the handler fails open).
	RateLimiter services.AuthRateLimiter

	// ImpersonationStore records the server-side return slots for active
	// impersonation sessions (#1750). When nil, Admin() falls back to an
	// in-memory store — fine for single-replica deployments and tests.
	ImpersonationStore services.ImpersonationStore

	// CSRFService mints a fresh CSRF token for the new effective user when
	// an impersonation session starts or ends (#1750). CSRF validation is
	// per-user, so the identity swap (admin↔target) must rotate the token
	// or the SPA's first mutating request under the swapped identity 403s.
	// The same csrf.Service the rest of the apiserver uses; a nil value
	// leaves the impersonation responses' csrf_token empty.
	CSRFService csrf.Service

	// ImpersonationTTL is the lifetime of an impersonation session
	// (#1750). Zero falls back to the 30-min default; values above the
	// 30-min ceiling are clamped down inside the handler.
	ImpersonationTTL time.Duration

	// UserMiddlewares is the standard authenticated-route middleware chain
	// (JWT + RLS + RegistrySet + CSRF). Admin() applies it to every admin
	// route EXCEPT POST /admin/impersonation/end. That one endpoint is
	// mounted bare so it stays reachable after the impersonation access
	// token has expired — JWTMiddleware would otherwise 401 an expired
	// token before endImpersonation runs, stranding the operator and
	// orphaning the return slot. endImpersonation self-validates the imp
	// token (signature + imp=true), tolerating only an expired `exp`, and
	// the server-side slot is the real authorization. When nil (tests that
	// build the admin router in isolation), Admin() applies no middleware
	// — callers must wrap the result themselves.
	UserMiddlewares []func(http.Handler) http.Handler
}

// Admin returns the router configurator for /api/v1/admin/*. Mounted
// from apiserver.go behind the standard userMiddlewares (JWT + RLS +
// CSRF) and the RequireSystemAdmin gate. Later admin issues hang their
// endpoints off the same chi.Router this closure receives.
func Admin(params AdminParams) func(r chi.Router) {
	// Fail fast on a misconfigured wiring: the #1749 group-membership
	// endpoints dereference GroupService on every request, so a nil
	// here would otherwise surface as a confusing per-request panic
	// rather than a clear startup failure.
	if params.GroupService == nil {
		panic("apiserver.Admin requires non-nil AdminParams.GroupService")
	}
	// The #1750 impersonation endpoints sign tokens with JWTSecret and
	// blacklist them on `end` via Blacklist — an AdminParams literal that
	// omits either turns a wiring mistake into a runtime failure on
	// impersonate start/end. Fail at startup instead. RateLimiter and
	// ImpersonationStore are deliberately NOT required: a nil RateLimiter
	// fails the limit open by design, and ImpersonationStore has the
	// defaultAdminImpersonationStore in-memory fallback.
	if len(params.JWTSecret) == 0 {
		panic("apiserver.Admin requires non-empty AdminParams.JWTSecret")
	}
	if params.Blacklist == nil {
		panic("apiserver.Admin requires non-nil AdminParams.Blacklist")
	}
	if params.AuditService == nil {
		panic("apiserver.Admin requires non-nil AdminParams.AuditService")
	}

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
	groupMembersAPI := &adminGroupMembersAPI{
		factorySet:   params.FactorySet,
		groupService: params.GroupService,
		auditService: params.AuditService,
	}
	impersonationAPI := &adminImpersonationAPI{
		factorySet:   params.FactorySet,
		store:        defaultAdminImpersonationStore(params.ImpersonationStore),
		rateLimiter:  params.RateLimiter,
		blacklist:    params.Blacklist,
		auditService: params.AuditService,
		csrfService:  params.CSRFService,
		jwtSecret:    params.JWTSecret,
		ttl:          params.ImpersonationTTL,
	}
	// Resolve the grants registry from the FactorySet — RequireSystemAdmin
	// and RequireSystemAdminOrImpersonating are no longer static funcs
	// (#1784) but instances bound to the deployment's grant store. A nil
	// FactorySet is a clear startup misconfiguration; let RequireSystemAdmin
	// panic loudly so the operator notices.
	requireSystemAdmin := RequireSystemAdmin(params.FactorySet.SystemAdminGrantRegistry)
	requireSystemAdminOrImpersonating := RequireSystemAdminOrImpersonating(params.FactorySet.SystemAdminGrantRegistry)

	return func(r chi.Router) {
		// POST /admin/impersonation/end is mounted FIRST and OUTSIDE the
		// authenticated-middleware group on purpose. JWTMiddleware (part of
		// UserMiddlewares) rejects an expired access token with 401 before
		// any handler runs — which would make `end` unreachable the moment
		// the (≤30-min) impersonation token lapses, stranding the operator
		// on a re-login and orphaning the return slot until it is
		// TTL-pruned. endImpersonation therefore self-validates the imp
		// token straight off the Authorization header: it verifies the
		// signature and the `imp=true` claim and tolerates ONLY an expired
		// `exp`. The jti-keyed server-side slot plus the
		// `slot.AdminUserID == impersonated_by` assertion are the real
		// authorization — a forged/garbage token fails signature
		// verification, and an expired NON-impersonation token fails the
		// imp check, so neither is newly admitted. See endImpersonation.
		r.Post("/impersonation/end", impersonationAPI.endImpersonation)

		// Everything else lives behind the standard authenticated
		// middleware chain (JWT + RLS + RegistrySet + CSRF). UserMiddlewares
		// may be nil in isolated unit tests, in which case no middleware is
		// applied and the caller is responsible for wrapping.
		r.Group(func(r chi.Router) {
			for _, mw := range params.UserMiddlewares {
				r.Use(mw)
			}
			adminAuthenticatedRoutes(r, requireSystemAdmin, requireSystemAdminOrImpersonating,
				tenantsAPI, usersAPI, groupsAPI, groupMembersAPI, impersonationAPI)
		})
	}
}

// adminAuthenticatedRoutes registers every /api/v1/admin/* route that runs
// behind the authenticated middleware chain (everything except POST
// /admin/impersonation/end, which is mounted bare by Admin()). Extracted
// from Admin() so that closure stays under the funlen budget. The two
// admin gates are passed in already bound to the grants registry
// (#1784) so this function stays unaware of which registry the
// deployment uses.
func adminAuthenticatedRoutes(
	r chi.Router,
	requireSystemAdmin func(http.Handler) http.Handler,
	requireSystemAdminOrImpersonating func(http.Handler) http.Handler,
	tenantsAPI *adminTenantsAPI,
	usersAPI *adminUsersAPI,
	groupsAPI *adminGroupsAPI,
	groupMembersAPI *adminGroupMembersAPI,
	impersonationAPI *adminImpersonationAPI,
) {
	// The bulk of the admin surface is gated by the strict
	// RequireSystemAdmin middleware in a nested group so every handler
	// inside it can assume the caller is a genuine system admin. GET
	// /admin/impersonation/current is registered OUTSIDE this group with
	// the wider RequireSystemAdminOrImpersonating gate — see below for why.
	r.Group(func(r chi.Router) {
		r.Use(requireSystemAdmin)
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

		// #1749: group-membership admin endpoints. add / remove /
		// role-change route through GroupService so the per-group
		// invariants (cap, ≥1 owner, ≥1 member) stay single-sourced,
		// bypassing only the per-group requireGroupAdmin middleware —
		// the RequireSystemAdmin gate above is authorization enough.
		// #1756: GET /members lists the group's members joined with
		// each member's identity for the admin membership editor;
		// cross-tenant via the registry's RLS-bypass path.
		r.Get("/groups/{groupID}/members", groupMembersAPI.listMembers)
		r.Post("/groups/{groupID}/members", groupMembersAPI.addMember)
		r.Delete("/groups/{groupID}/members/{userID}", groupMembersAPI.removeMember)
		r.Patch("/groups/{groupID}/members/{userID}", groupMembersAPI.updateMemberRole)

		// #1750: starting an impersonation session is a privileged
		// operation — it stays behind the strict gate. The
		// nested-impersonation guard in the handler rejects a request
		// already inside an impersonation session.
		r.Post("/users/{userID}/impersonate", impersonationAPI.startImpersonation)
	})

	// #1750: GET /admin/impersonation/current must be reachable from
	// INSIDE an impersonation session, whose access token deliberately
	// carries is_system_admin=false — the strict RequireSystemAdmin gate
	// would reject it. The wider RequireSystemAdminOrImpersonating gate
	// admits both a genuine admin and an active (non-expired) impersonation
	// token; the handler re-validates the impersonation claim. Unlike
	// `end`, `current` is a harmless read and is fine to keep behind
	// JWTMiddleware — an expired token simply means the FE banner reads
	// inactive, which is the correct answer.
	r.With(requireSystemAdminOrImpersonating).Get("/impersonation/current", impersonationAPI.currentImpersonation)
}

// parseAdminSortAndOrder reconciles the dual sort conventions the
// admin FE may send: the `?sort=-name` shorthand (consistent with the
// rest of the API) or the explicit `?sort=name&order=desc` pair (which
// some FE table libs prefer).
//
// A leading `-` on `?sort` reverses the natural order (e.g. `-created_at`
// → desc by created_at). An explicit `order=` param always wins over that
// prefix so the FE never has to strip the prefix before re-sending the
// current sort with a flipped direction. Unknown `order=` values (e.g.
// `?order=ascending`) and unknown sort fields are intentionally tolerated
// rather than rejected so the FE can drift slightly across versions — the
// registry layer falls back to its default sort and whitelists the field
// via IsValid().
func parseAdminSortAndOrder(rawSort, rawOrder string) (field string, desc bool) {
	if rawSort != "" {
		if rawSort[0] == '-' {
			field, desc = rawSort[1:], true
		} else {
			field, desc = rawSort, false
		}
	}
	switch strings.ToLower(strings.TrimSpace(rawOrder)) {
	case "asc":
		desc = false
	case "desc":
		desc = true
	}
	return field, desc
}
