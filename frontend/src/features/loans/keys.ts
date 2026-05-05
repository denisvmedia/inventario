import type { ListGroupLoansOptions } from "./api"

function listKeySuffix(opts: ListGroupLoansOptions | undefined): string {
  if (!opts) return ""
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("limit", String(opts.perPage))
  if (opts.state) params.set("state", opts.state)
  return params.toString()
}

// TanStack Query keys for the loans slice. Scoped by group slug because
// the http client rewrites /loans -> /g/{slug}/loans; without the slug
// in the key, navigating between groups would reuse the wrong cache.
//
// The `counts` key intentionally takes a sorted-stringified id list so
// the same set of commodity ids in different orders shares a cache
// entry (the BE always returns the same map regardless of input order).
export const loanKeys = {
  all: ["loan"] as const,
  group: (slug: string) => [...loanKeys.all, slug] as const,
  groupList: (slug: string, opts?: ListGroupLoansOptions) =>
    [...loanKeys.group(slug), "groupList", listKeySuffix(opts)] as const,
  byCommodity: (slug: string, commodityID: string) =>
    [...loanKeys.group(slug), "byCommodity", commodityID] as const,
  counts: (slug: string, commodityIDs: readonly string[]) =>
    [...loanKeys.group(slug), "counts", [...commodityIDs].sort().join(",")] as const,
}
