import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useIsSystemAdmin } from "@/features/auth/hooks"
import { clearAuth } from "@/lib/auth-storage"
import { hardRedirect } from "@/lib/navigation"

import {
  addAdminGroupMember,
  blockAdminUser,
  endImpersonation,
  getAdminGroup,
  getAdminTenant,
  getAdminUser,
  getImpersonationState,
  listAdminGroupMembers,
  listAdminGroups,
  listAdminTenants,
  listAdminTenantUsers,
  removeAdminGroupMember,
  softDeleteAdminGroup,
  startImpersonation,
  unblockAdminUser,
  updateAdminGroupMemberRole,
  type AdminAddMemberRequest,
  type AdminBlockRequest,
  type AdminGroupDetail,
  type AdminGroupMember,
  type AdminGroupsResult,
  type AdminTenant,
  type AdminTenantsResult,
  type AdminTenantUsersResult,
  type AdminUnblockRequest,
  type AdminUserDetail,
  type EndImpersonationResult,
  type GroupRole,
  type LoginResponse,
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

// Reads a single location group for the admin Group detail page header.
// Like useAdminTenant this is gated on is_system_admin so a non-admin who
// somehow deep-links the page never fires a guaranteed-403 request.
export function useAdminGroup(groupId: string, { enabled = true }: QueryOptions = {}) {
  const isSystemAdmin = useIsSystemAdmin()
  return useQuery<AdminGroupDetail>({
    queryKey: adminKeys.groupDetail(groupId),
    queryFn: ({ signal }) => getAdminGroup(groupId, signal),
    enabled: enabled && isSystemAdmin && !!groupId,
  })
}

// Soft-deletes a location group from the admin Group detail page. On
// success the BE echoes the post-transition (pending_deletion) row; we
// write it straight into the detail cache so the page flips read-only
// without a refetch, and invalidate the cross-tenant group LIST queries so
// the status badge there refreshes too. The mutation surfaces errors to
// the caller (via `isError` / `error`) so the page can show a failure
// notice; an idempotent re-delete returns HTTP 200 and resolves cleanly.
export function useDeleteAdminGroup() {
  const qc = useQueryClient()
  return useMutation<AdminGroupDetail, Error, string>({
    mutationFn: (groupId) => softDeleteAdminGroup(groupId),
    onSuccess: (group) => {
      if (group.id) {
        qc.setQueryData(adminKeys.groupDetail(group.id), group)
      }
      // Invalidate only the LIST queries, not the whole `groups()`
      // subtree: a blanket invalidate would also mark the detail query
      // (a child key) stale and — because the detail page is an active
      // observer — immediately refetch it, racing the `setQueryData`
      // above and flipping the page back to the pre-delete row. Using
      // the shared key factory plus the `"list"` prefix scopes the
      // invalidation to list keys without hardcoding queryKey positions.
      qc.invalidateQueries({
        queryKey: [...adminKeys.groups(), "list"],
      })
    },
  })
}

// Lists the members of one location group for the membership editor on
// the admin Group detail page. Gated on is_system_admin like the sibling
// reads — a non-admin who somehow deep-links the page never fires a
// guaranteed-403 request.
export function useAdminGroupMembers(groupId: string, { enabled = true }: QueryOptions = {}) {
  const isSystemAdmin = useIsSystemAdmin()
  return useQuery<AdminGroupMember[]>({
    queryKey: adminKeys.groupMembers(groupId),
    queryFn: ({ signal }) => listAdminGroupMembers(groupId, signal),
    enabled: enabled && isSystemAdmin && !!groupId,
  })
}

// Invalidates the two queries an add / remove / role-change mutation
// affects: the members list itself, and the group-detail header (whose
// `member_count` stat drifts the moment the roster changes). Both keys
// sit under the `groupDetail(groupId)` subtree, so a single invalidate of
// that prefix would catch them — but invalidating the precise keys keeps
// the intent explicit and matches the targeted style of useDeleteAdminGroup.
function invalidateGroupMembership(qc: ReturnType<typeof useQueryClient>, groupId: string) {
  qc.invalidateQueries({ queryKey: adminKeys.groupMembers(groupId) })
  qc.invalidateQueries({ queryKey: adminKeys.groupDetail(groupId) })
}

// Adds a user to a group. On success the members list and the group-detail
// header are invalidated so the roster and the `member_count` stat both
// re-fetch. Errors surface to the caller (`isError` / `error`) so the
// editor's add dialog can branch on the typed 422 codes.
export function useAddAdminGroupMember(groupId: string) {
  const qc = useQueryClient()
  return useMutation<void, Error, AdminAddMemberRequest>({
    mutationFn: (payload) => addAdminGroupMember(groupId, payload),
    onSuccess: () => invalidateGroupMembership(qc, groupId),
  })
}

// Removes a user from a group — symmetric to useAddAdminGroupMember. The
// mutation variable is the target user's id.
export function useRemoveAdminGroupMember(groupId: string) {
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (userId) => removeAdminGroupMember(groupId, userId),
    onSuccess: () => invalidateGroupMembership(qc, groupId),
  })
}

// Changes a member's role. The mutation variable carries both the target
// user id and the new role; on success the same two queries are
// invalidated (a role change does not move `member_count`, but the
// detail-query invalidation is harmless and keeps the helper uniform).
export function useUpdateAdminGroupMemberRole(groupId: string) {
  const qc = useQueryClient()
  return useMutation<void, Error, { userId: string; role: GroupRole }>({
    mutationFn: ({ userId, role }) => updateAdminGroupMemberRole(groupId, userId, role),
    onSuccess: () => invalidateGroupMembership(qc, groupId),
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

// Starts an impersonation session for a target user. No cache work — a
// successful start is immediately followed by a full-page reload (the app
// re-mounts as the target user, rebuilding every cache), so invalidating
// React Query keys here would be pointless. The page consumes `isPending`
// to gate the confirm dialog and `error` to surface a typed 422/429 banner.
//
// The post-success hard reload lives in the hook-level `onSuccess` (not a
// call-site one): hook-level callbacks always run, whereas a call-site
// `mutate(vars, { onSuccess })` is silently skipped if the calling
// component unmounts before the mutation settles — which would leave the
// app live under the target's tokens but still rendering the admin UI.
export function useStartImpersonation() {
  return useMutation<LoginResponse, Error, string>({
    mutationFn: (userId) => startImpersonation(userId),
    onSuccess: () => hardRedirect("/"),
  })
}

// Ends the active impersonation session. Like the start hook this does no
// cache work — the hook-level callbacks perform a full-page reload, which
// rebuilds the cache from scratch.
//
// Both side effects are hook-level for the same reason as
// useStartImpersonation: a call-site callback would be skipped on unmount,
// stranding the operator on a half-swapped identity. On success we route
// to the impersonated user's admin detail page (or the list if the slot
// was missing); on failure the admin session is unrecoverable from the FE,
// so we clear auth and bounce to /login with the `session_expired` reason —
// consistent with the auto-expiry recovery path (review item M1).
export function useEndImpersonation() {
  return useMutation<EndImpersonationResult, Error, void>({
    mutationFn: () => endImpersonation(),
    onSuccess: (result) =>
      hardRedirect(
        result.targetUserId
          ? "/admin/users/" + encodeURIComponent(result.targetUserId)
          : "/admin/users"
      ),
    onError: () => {
      clearAuth()
      hardRedirect("/login?reason=session_expired")
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
