import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useIsSystemAdmin } from "@/features/auth/hooks"

import {
  blockAdminUser,
  getAdminTenant,
  getAdminUser,
  getImpersonationState,
  listAdminGroups,
  listAdminTenants,
  listAdminTenantUsers,
  unblockAdminUser,
  type AdminBlockRequest,
  type AdminGroupsResult,
  type AdminTenant,
  type AdminTenantsResult,
  type AdminTenantUsersResult,
  type AdminUnblockRequest,
  type AdminUserDetail,
} from "./api"
import {
  adminKeys,
  type AdminGroupsParams,
  type AdminTenantsParams,
  type AdminTenantUsersParams,
} from "./keys"

interface QueryOptions {
  // Gate the query. The admin endpoints 403 for non-admin users, so the
  // hooks default to firing only when the caller is a system admin —
  // pass `enabled: false` to suppress further (e.g. while a parent guard
  // is still resolving).
  enabled?: boolean
}

// Lists tenants for the admin Tenants page. Defaults to enabled only for
// system admins so a non-admin who somehow mounts the page doesn't fire a
// guaranteed-403 request.
export function useAdminTenants(
  params: AdminTenantsParams = {},
  { enabled = true }: QueryOptions = {}
) {
  const isSystemAdmin = useIsSystemAdmin()
  return useQuery<AdminTenantsResult>({
    queryKey: adminKeys.tenantList(params),
    queryFn: ({ signal }) => listAdminTenants(params, signal),
    enabled: enabled && isSystemAdmin,
  })
}

// Reads a single tenant for the admin Tenant detail page header. Like
// useAdminTenants this is gated on is_system_admin — a non-admin who
// somehow deep-links the page never fires a guaranteed-403 request.
export function useAdminTenant(tenantId: string, { enabled = true }: QueryOptions = {}) {
  const isSystemAdmin = useIsSystemAdmin()
  return useQuery<AdminTenant>({
    queryKey: adminKeys.tenantDetail(tenantId),
    queryFn: ({ signal }) => getAdminTenant(tenantId, signal),
    enabled: enabled && isSystemAdmin && !!tenantId,
  })
}

// Lists the users in one tenant for the Tenant detail Users tab.
export function useAdminTenantUsers(
  tenantId: string,
  params: AdminTenantUsersParams = {},
  { enabled = true }: QueryOptions = {}
) {
  const isSystemAdmin = useIsSystemAdmin()
  return useQuery<AdminTenantUsersResult>({
    queryKey: adminKeys.tenantUsers(tenantId, params),
    queryFn: ({ signal }) => listAdminTenantUsers(tenantId, params, signal),
    enabled: enabled && isSystemAdmin && !!tenantId,
  })
}

// Lists location groups for the Tenant detail Groups tab. The caller
// pins `tenantID` in `params` so the listing is tenant-scoped.
export function useAdminGroups(
  params: AdminGroupsParams = {},
  { enabled = true }: QueryOptions = {}
) {
  const isSystemAdmin = useIsSystemAdmin()
  return useQuery<AdminGroupsResult>({
    queryKey: adminKeys.groupList(params),
    queryFn: ({ signal }) => listAdminGroups(params, signal),
    enabled: enabled && isSystemAdmin,
  })
}

// Reads a single user's full admin detail for the Admin user detail page.
// Gated on is_system_admin like the other admin reads — a non-admin who
// somehow deep-links the page never fires a guaranteed-403 request.
export function useAdminUser(userId: string, { enabled = true }: QueryOptions = {}) {
  const isSystemAdmin = useIsSystemAdmin()
  return useQuery<AdminUserDetail>({
    queryKey: adminKeys.userDetail(userId),
    queryFn: ({ signal }) => getAdminUser(userId, signal),
    enabled: enabled && isSystemAdmin && !!userId,
  })
}

// Patches the cached user-detail entry with the authoritative `is_active`
// returned by a block / unblock mutation. Writing the fresh value into
// the cache *before* invalidating closes the window where the detail page
// would otherwise read the stale pre-mutation value between dropping its
// optimistic flag and the refetch settling (no Blocked→Active→Blocked
// badge flash). The block/unblock endpoints return an `AdminUserView`
// (a narrower identity view) — only `is_active` is merged.
function patchUserDetailActive(
  qc: ReturnType<typeof useQueryClient>,
  userId: string,
  isActive: boolean | undefined
) {
  qc.setQueryData<AdminUserDetail>(adminKeys.userDetail(userId), (prev) =>
    prev ? { ...prev, is_active: isActive } : prev
  )
}

// Blocks a user. On success the authoritative `is_active` from the
// mutation response is written straight into the user-detail cache, then
// the entry is invalidated so the identity card also re-fetches the
// session count; the tenant-scoped user listings are invalidated too
// since the blocked row's state changes there as well. The page applies
// an optimistic badge flip only to bridge the in-flight / rollback
// window — these hooks own the post-success cache state.
export function useBlockAdminUser(userId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload: AdminBlockRequest) => blockAdminUser(userId, payload),
    onSuccess: (view) => {
      patchUserDetailActive(qc, userId, view.is_active)
      qc.invalidateQueries({ queryKey: adminKeys.userDetail(userId) })
      qc.invalidateQueries({ queryKey: adminKeys.tenants() })
    },
  })
}

// Unblocks a user — symmetric to useBlockAdminUser.
export function useUnblockAdminUser(userId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload: AdminUnblockRequest) => unblockAdminUser(userId, payload),
    onSuccess: (view) => {
      patchUserDetailActive(qc, userId, view.is_active)
      qc.invalidateQueries({ queryKey: adminKeys.userDetail(userId) })
      qc.invalidateQueries({ queryKey: adminKeys.tenants() })
    },
  })
}

// Reads the active impersonation session. Unlike useAdminTenants this is
// NOT gated on is_system_admin: an impersonated (non-admin) browser still
// needs to see the banner, and GET /admin/impersonation/current is the
// one /admin/* endpoint reachable from inside an impersonation session.
export function useImpersonationState({ enabled = true }: QueryOptions = {}) {
  return useQuery({
    queryKey: adminKeys.impersonationCurrent(),
    queryFn: ({ signal }) => getImpersonationState(signal),
    enabled,
    // Impersonation state changes on the order of minutes and a session
    // has a hard ≤30m expiry, so a 5m staleTime with no refocus refetch
    // is plenty — this avoids hammering the endpoint with a guaranteed
    // 403 on every tab refocus for the non-admin majority.
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
  })
}
