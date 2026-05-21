// TanStack Query keys for the admin slice. The admin surface is
// platform-wide (not group-scoped), so unlike the locations / commodities
// keys these are NOT keyed by a group slug — there is one global admin
// view per system administrator.
export interface AdminTenantsParams {
  page?: number
  perPage?: number
  q?: string
  sort?: string
  order?: "asc" | "desc"
}

// Per-tenant user listing params (GET /admin/tenants/{id}/users). The
// `isActive` tri-state mirrors the BE's `?is_active` filter: `true`
// (active only), `false` (inactive only), `undefined` (no filter).
export interface AdminTenantUsersParams {
  page?: number
  perPage?: number
  q?: string
  isActive?: boolean
  sort?: string
  order?: "asc" | "desc"
}

// Cross-tenant group listing params (GET /admin/groups). The detail
// page always pins `tenantID`; `status` is the optional exact-match
// filter the Groups tab exposes.
export interface AdminGroupsParams {
  tenantID?: string
  page?: number
  perPage?: number
  q?: string
  status?: "active" | "pending_deletion"
  sort?: string
  order?: "asc" | "desc"
}

export const adminKeys = {
  all: ["admin"] as const,
  tenants: () => [...adminKeys.all, "tenants"] as const,
  tenantList: (params: AdminTenantsParams) => [...adminKeys.tenants(), "list", params] as const,
  tenantDetail: (id: string) => [...adminKeys.tenants(), "detail", id] as const,
  tenantUsers: (id: string, params: AdminTenantUsersParams) =>
    [...adminKeys.tenants(), "detail", id, "users", params] as const,
  groups: () => [...adminKeys.all, "groups"] as const,
  groupList: (params: AdminGroupsParams) => [...adminKeys.groups(), "list", params] as const,
  // Per-user admin detail (GET /admin/users/{id}). Keyed by the user id
  // so block/unblock mutations can invalidate exactly this entry.
  users: () => [...adminKeys.all, "users"] as const,
  userDetail: (id: string) => [...adminKeys.users(), "detail", id] as const,
  impersonation: () => [...adminKeys.all, "impersonation"] as const,
  impersonationCurrent: () => [...adminKeys.impersonation(), "current"] as const,
}
