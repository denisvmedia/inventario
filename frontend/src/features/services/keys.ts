import type { ListGroupServicesOptions } from "./api"

function listKeySuffix(opts: ListGroupServicesOptions | undefined): string {
  if (!opts) return ""
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("limit", String(opts.perPage))
  if (opts.state) params.set("state", opts.state)
  return params.toString()
}

// TanStack Query keys for the services slice. Same group-slug-scoping
// rationale as loanKeys — see that file's comment for the full reasoning.
export const serviceKeys = {
  all: ["service"] as const,
  group: (slug: string) => [...serviceKeys.all, slug] as const,
  groupList: (slug: string, opts?: ListGroupServicesOptions) =>
    [...serviceKeys.group(slug), "groupList", listKeySuffix(opts)] as const,
  byCommodity: (slug: string, commodityID: string) =>
    [...serviceKeys.group(slug), "byCommodity", commodityID] as const,
  counts: (slug: string, commodityIDs: readonly string[]) =>
    [...serviceKeys.group(slug), "counts", [...commodityIDs].sort().join(",")] as const,
}
