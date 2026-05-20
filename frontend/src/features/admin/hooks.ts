import { useQuery } from "@tanstack/react-query"

import { useIsSystemAdmin } from "@/features/auth/hooks"

import { getImpersonationState, listAdminTenants, type AdminTenantsResult } from "./api"
import { adminKeys, type AdminTenantsParams } from "./keys"

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
export function useAdminTenants(params: AdminTenantsParams = {}, { enabled = true }: QueryOptions = {}) {
  const isSystemAdmin = useIsSystemAdmin()
  return useQuery<AdminTenantsResult>({
    queryKey: adminKeys.tenantList(params),
    queryFn: ({ signal }) => listAdminTenants(params, signal),
    enabled: enabled && isSystemAdmin,
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
