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

// adminPing is the placeholder handler behind RequireBackofficeAuth.
// Returns 200 with a simple JSON body so the FE can detect "I am
// signed in to the back-office plane" without needing a richer
// endpoint until later admin issues land. Post-#1785 Phase 3 the gate
// is back-office-only — a tenant JWT with `is_system_admin` no longer
// reaches this route.
// @Summary Back-office ping
// @Description Returns 200 when the caller is authenticated on the back-office plane. Probe endpoint for the admin surface (#1745).
// @Tags admin
// @Produce json
// @Success 200 {object} AdminPingResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Failure 403 {object} jsonapi.Errors "Account disabled"
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
}

// Admin returns the router configurator for /api/v1/admin/*. Mounted
// from apiserver.go. Cross-tenant CRUD endpoints (tenants, users,
// groups, members) and the impersonation-start surface are gated by
// the back-office auth plane: the operator must hold a back-office
// JWT (aud=backoffice, admin_id claim), and start additionally
// requires role=platform_admin (support_agent cannot impersonate).
// GET /admin/impersonation/current accepts EITHER a back-office token
// OR an impersonation token so the FE banner can read its state from
// inside the impersonated session. POST /admin/impersonation/end is
// mounted bare because it must remain reachable after the impersonation
// access token expires (it self-validates the imp token). Phase 5 of
// issue #1785 cut start/end over to the back-office plane (was on the
// tenant-side RequireSystemAdmin gate).
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
	// #1308: background-worker soft-pause control plane. FactorySet +
	// AuditService are guaranteed non-nil by the fail-fast block above.
	workersAPI := &adminWorkersAPI{
		factorySet:   params.FactorySet,
		auditService: params.AuditService,
	}
	// #1785 Phase 5 removed the legacy `RequireSystemAdmin` gate from
	// every admin route: the CRUD subtree is gated by RequireBackofficeAuth
	// (Phase 3), impersonation/start is gated by RequirePlatformAdmin
	// inside the back-office group (this phase), and
	// /admin/impersonation/current uses RequireBackofficeAuthOrImpersonating.
	// The grants registry still exists (#1784) — it backs the per-handler
	// `is_system_admin` advisory flag on /auth/me — just not the admin
	// routing gate.

	backofficeAuth := RequireBackofficeAuth(
		params.JWTSecret,
		params.BackofficeUserRegistry,
		params.Blacklist,
	)

	currentGate := RequireBackofficeAuthOrImpersonating(
		params.JWTSecret,
		params.BackofficeUserRegistry,
		params.FactorySet.UserRegistry,
		params.Blacklist,
	)

	return func(r chi.Router) {
		// POST /admin/impersonation/end is mounted FIRST and OUTSIDE
		// every auth-middleware group on purpose. Any auth middleware
		// would reject an expired access token with 401 before the
		// handler runs — which would make `end` unreachable the moment
		// the (≤30-min) impersonation token lapses, stranding the
		// operator on a re-login and orphaning the return slot until
		// it is TTL-pruned. endImpersonation therefore self-validates
		// the imp token straight off the Authorization header: it
		// verifies the signature and the `imp=true` +
		// non-empty-impersonator_id claims and tolerates ONLY an
		// expired `exp`. The jti-keyed server-side slot plus the
		// `slot.OperatorUserID == impersonator_id` +
		// `slot.OperatorKind == ImpersonationOperatorBackoffice`
		// assertions are the real authorization — a forged/garbage
		// token fails signature verification, and an expired
		// NON-impersonation token fails the imp check, so neither is
		// newly admitted. See endImpersonation.
		r.Post("/impersonation/end", impersonationAPI.endImpersonation)

		// GET /admin/impersonation/current sits on a WIDER gate than
		// every other admin route: the FE banner needs to read its own
		// state from inside the impersonated session whose access token
		// is a tenant JWT and so cannot satisfy RequireBackofficeAuth.
		// RequireBackofficeAuthOrImpersonating peeks at the bearer
		// token's `aud` claim and dispatches: a back-office token goes
		// through the back-office gate (per-request user load, blacklist
		// check), an impersonation token (`imp=true`) is validated as
		// a tenant JWT with the target loaded into context. A token
		// that is neither shape is a 401.
		r.With(currentGate).Get("/impersonation/current", impersonationAPI.currentImpersonation)

		// Cross-tenant CRUD subtree (#1785 Phase 3): gated by the
		// back-office auth plane. RequireBackofficeAuth parses +
		// validates a `aud=backoffice` JWT and attaches the back-office
		// identity to the context via appctx.WithBackofficeUser.
		// Handlers read it back via appctx.AdminActorFromContext for
		// audit-row population. RequireBackofficeAuth REJECTS tenant
		// tokens (and vice versa).
		r.Group(func(r chi.Router) {
			r.Use(backofficeAuth)
			adminBackofficeRoutes(r, tenantsAPI, usersAPI, groupsAPI, groupMembersAPI, impersonationAPI, workersAPI)
		})
	}
}

// adminBackofficeRoutes registers every cross-tenant admin route that
// is gated by RequireBackofficeAuth (issue #1785, Phase 3+5). Extracted
// from Admin() so that closure stays under the funlen budget.
//
// Every handler in this group can assume appctx.AdminActorFromContext
// returns non-nil — RequireBackofficeAuth populates it before
// dispatching.
//
// POST /admin/users/{userID}/impersonate adds a deeper role check via
// RequirePlatformAdmin: support_agent is read-mostly and may not start
// an impersonation session. The check is applied only on that one
// route so support_agent retains read access to every other admin
// surface.
func adminBackofficeRoutes(
	r chi.Router,
	tenantsAPI *adminTenantsAPI,
	usersAPI *adminUsersAPI,
	groupsAPI *adminGroupsAPI,
	groupMembersAPI *adminGroupMembersAPI,
	impersonationAPI *adminImpersonationAPI,
	workersAPI *adminWorkersAPI,
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

	// #1308: background-worker soft-pause control plane. List the
	// canonical worker types with their pause state; pause / resume by
	// worker type. No tenant scope — worker_control is a platform control
	// orthogonal to tenants. Each handler audit-logs via the shared
	// AuditService.
	r.Get("/workers", workersAPI.listWorkers)
	r.Post("/workers/{workerType}/pause", workersAPI.pauseWorker)
	r.Post("/workers/{workerType}/resume", workersAPI.resumeWorker)

	// #1785 Phase 5: impersonation-start is gated on platform_admin —
	// support_agent (the read-mostly persona) cannot borrow a tenant
	// identity. The nested-impersonation guard in the handler is
	// defence-in-depth: RequireBackofficeAuth already rejects
	// impersonation (tenant) tokens, so an active impersonation context
	// at the handler would mean the gate was bypassed.
	r.With(RequirePlatformAdmin).Post("/users/{userID}/impersonate", impersonationAPI.startImpersonation)
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
