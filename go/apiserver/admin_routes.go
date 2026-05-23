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
	FactorySet *registry.FactorySet
	// BackofficeUserRegistry backs the RequireBackofficeAuth middleware
	// that gates every cross-tenant admin route (issue #1785, Phase 3).
	// Held separately from the tenant FactorySet entry so the
	// middleware constructor can take only what it needs.
	BackofficeUserRegistry registry.BackofficeUserRegistry
	Blacklist              services.TokenBlacklister
	AuditService           services.AuditLogger
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
// from apiserver.go. Cross-tenant CRUD endpoints (tenants, users,
// groups, members) are gated by RequireBackofficeAuth so the operator
// must hold a back-office JWT (aud=backoffice, admin_id claim). The
// legacy impersonation subtree (start/current/end) remains on the
// tenant-side RequireSystemAdmin gate because the impersonation
// lifecycle (return-slot restore of the operator's tenant refresh
// cookie, fresh tenant access token mint) is intrinsically tied to a
// tenant operator identity; a back-office redesign of impersonation is
// deferred to a later phase of issue #1785.
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
	// The #1784 grant store backs RequireSystemAdmin; we dereference
	// params.FactorySet a few lines below to resolve it. A nil FactorySet
	// would NPE before any middleware ran, so trip the startup-guard
	// invariant here for a clear failure message.
	if params.FactorySet == nil {
		panic("apiserver.Admin requires non-nil AdminParams.FactorySet")
	}
	// BackofficeUserRegistry is mandatory for the new back-office gate
	// (#1785 Phase 3). Fail at startup so a wiring miss surfaces here
	// rather than as a per-request nil deref inside the middleware.
	if params.BackofficeUserRegistry == nil {
		panic("apiserver.Admin requires non-nil AdminParams.BackofficeUserRegistry")
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
	// (#1784) but instances bound to the deployment's grant store. Used
	// by the legacy impersonation subtree below; the back-office gate
	// replaces them for the CRUD subtree (#1785 Phase 3). The FactorySet
	// nil-guard above runs before this dereference, so a missing wiring
	// fails at startup with a clear message rather than NPE'ing here.
	// A nil SystemAdminGrantRegistry inside a non-nil FactorySet is caught
	// downstream by RequireSystemAdmin's own nil-check, which panics at
	// construction time — fail-fast on a misconfigured deployment rather
	// than letting the process boot and 500-on-every-admin-request.
	requireSystemAdmin := RequireSystemAdmin(params.FactorySet.SystemAdminGrantRegistry)
	requireSystemAdminOrImpersonating := RequireSystemAdminOrImpersonating(params.FactorySet.SystemAdminGrantRegistry)

	backofficeAuth := RequireBackofficeAuth(
		params.JWTSecret,
		params.BackofficeUserRegistry,
		params.Blacklist,
	)

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

		// Cross-tenant CRUD subtree (#1785 Phase 3): gated by the
		// back-office auth plane. RequireBackofficeAuth parses + validates
		// a `aud=backoffice` JWT and attaches the back-office identity to
		// the context via appctx.WithBackofficeUser. Handlers read it back
		// via appctx.AdminActorFromContext for audit-row population.
		// Distinct from the impersonation group below — RequireBackofficeAuth
		// REJECTS tenant tokens (and vice versa), so the two groups are
		// mutually exclusive on the wire.
		r.Group(func(r chi.Router) {
			r.Use(backofficeAuth)
			adminBackofficeRoutes(r, tenantsAPI, usersAPI, groupsAPI, groupMembersAPI, impersonationAPI)
		})

		// Legacy impersonation subtree stays on the tenant-side JWT +
		// RequireSystemAdmin gate. The impersonation start/end lifecycle
		// captures and restores the operator's tenant refresh cookie and
		// mints a tenant access token in `restoreAdminSession`; none of
		// that has a back-office analogue yet. Redesign is deferred to a
		// later phase of issue #1785. UserMiddlewares may be nil in
		// isolated unit tests, in which case no middleware is applied and
		// the caller is responsible for wrapping.
		r.Group(func(r chi.Router) {
			for _, mw := range params.UserMiddlewares {
				r.Use(mw)
			}
			adminImpersonationRoutes(r, requireSystemAdmin, requireSystemAdminOrImpersonating, impersonationAPI)
		})
	}
}

// adminBackofficeRoutes registers every cross-tenant admin route that
// is gated by RequireBackofficeAuth (issue #1785, Phase 3). The legacy
// impersonation subtree lives in adminImpersonationRoutes — those routes
// retain the tenant JWT + RequireSystemAdmin gate until a back-office
// redesign of the impersonation lifecycle lands. Extracted from Admin()
// so that closure stays under the funlen budget.
//
// Every handler in this group can assume appctx.AdminActorFromContext
// returns non-nil — RequireBackofficeAuth populates it before
// dispatching.
func adminBackofficeRoutes(
	r chi.Router,
	tenantsAPI *adminTenantsAPI,
	usersAPI *adminUsersAPI,
	groupsAPI *adminGroupsAPI,
	groupMembersAPI *adminGroupMembersAPI,
	_ *adminImpersonationAPI, // reserved for a future back-office impersonation surface
) {
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

	// #1749 / #1756: group-membership admin endpoints. add / remove /
	// role-change route through GroupService so the per-group
	// invariants (cap, ≥1 owner, ≥1 member) stay single-sourced,
	// bypassing only the per-group requireGroupAdmin middleware —
	// the back-office gate above is authorization enough.
	r.Get("/groups/{groupID}/members", groupMembersAPI.listMembers)
	r.Post("/groups/{groupID}/members", groupMembersAPI.addMember)
	r.Delete("/groups/{groupID}/members/{userID}", groupMembersAPI.removeMember)
	r.Patch("/groups/{groupID}/members/{userID}", groupMembersAPI.updateMemberRole)
}

// adminImpersonationRoutes registers the #1750 impersonation start /
// current endpoints. These intentionally stay on the tenant JWT +
// RequireSystemAdmin gate (rather than RequireBackofficeAuth): the
// impersonation lifecycle captures and restores the operator's tenant
// refresh cookie and mints a tenant access token in
// restoreAdminSession, none of which has a back-office analogue yet.
// A back-office-native impersonation surface is a follow-up of issue
// #1785.
//
// POST /impersonation/end is registered separately by Admin() because
// it must remain reachable AFTER the (≤30 min) impersonation access
// token expires — see Admin() for the full rationale.
func adminImpersonationRoutes(
	r chi.Router,
	requireSystemAdmin func(http.Handler) http.Handler,
	requireSystemAdminOrImpersonating func(http.Handler) http.Handler,
	impersonationAPI *adminImpersonationAPI,
) {
	// Starting an impersonation session is a privileged operation — it
	// stays behind the strict gate. The nested-impersonation guard in
	// the handler rejects a request already inside an impersonation
	// session.
	r.With(requireSystemAdmin).Post("/users/{userID}/impersonate", impersonationAPI.startImpersonation)

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
