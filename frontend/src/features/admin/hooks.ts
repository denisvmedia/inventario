import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useIsSystemAdmin } from "@/features/auth/hooks"

import {
  getAdminGroup,
  getAdminTenant,
  getImpersonationState,
  listAdminGroups,
  listAdminTenants,
  listAdminTenantUsers,
  softDeleteAdminGroup,
  type AdminGroupDetail,
  type AdminGroupsResult,
  type AdminTenant,
  type AdminTenantsResult,
  type AdminTenantUsersResult,
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
      // above and flipping the page back to the pre-delete row. The
      // `"list"` segment scopes the invalidation to the list keys.
      qc.invalidateQueries({
        predicate: (q) => q.queryKey[0] === "admin" && q.queryKey[1] === "groups" && q.queryKey[2] === "list",
      })
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
