import type { ListGroupMaintenanceOptions } from "./api"

function listKeySuffix(opts: ListGroupMaintenanceOptions | undefined): string {
  if (!opts) return ""
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("limit", String(opts.perPage))
  if (opts.dueBefore) params.set("due_before", opts.dueBefore)
  if (opts.enabledOnly) params.set("enabled_only", "true")
  return params.toString()
}

// TanStack Query keys for the maintenance slice (#1368). Scoped by
// group slug because the http client rewrites /maintenance ->
// /g/{slug}/maintenance; without the slug in the key, navigating
// between groups would reuse the wrong cache.
export const maintenanceKeys = {
  all: ["maintenance"] as const,
  group: (slug: string) => [...maintenanceKeys.all, slug] as const,
  groupList: (slug: string, opts?: ListGroupMaintenanceOptions) =>
    [...maintenanceKeys.group(slug), "groupList", listKeySuffix(opts)] as const,
  byCommodity: (slug: string, commodityID: string) =>
    [...maintenanceKeys.group(slug), "byCommodity", commodityID] as const,
}
