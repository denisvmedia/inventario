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

export const adminKeys = {
  all: ["admin"] as const,
  tenants: () => [...adminKeys.all, "tenants"] as const,
  tenantList: (params: AdminTenantsParams) => [...adminKeys.tenants(), "list", params] as const,
  impersonation: () => [...adminKeys.all, "impersonation"] as const,
  impersonationCurrent: () => [...adminKeys.impersonation(), "current"] as const,
}
