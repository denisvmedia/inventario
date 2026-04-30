import type { ListCommoditiesOptions } from "./api"

// Stringify list-query options into a stable key suffix. URLSearchParams
// preserves multi-value keys; we explicitly sort `types` and `statuses`
// before serialising so that two options objects with the same filters
// but different array order produce identical keys (TanStack Query
// re-uses the cache instead of issuing a duplicate request).
function listKeySuffix(opts: ListCommoditiesOptions | undefined): string {
  if (!opts) return ""
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  for (const t of [...(opts.types ?? [])].sort()) params.append("type", t)
  for (const s of [...(opts.statuses ?? [])].sort()) params.append("status", s)
  if (opts.areaId) params.set("area_id", opts.areaId)
  if (opts.search?.trim()) params.set("q", opts.search.trim())
  if (opts.includeInactive) params.set("include_inactive", "1")
  if (opts.sort) params.set("sort", opts.sortDesc ? `-${opts.sort}` : opts.sort)
  return params.toString()
}

// TanStack Query keys for the commodities slice. Scoped by group slug
// because the http client rewrites /commodities -> /g/{slug}/commodities
// — without the slug in the key, navigating from /g/household to
// /g/office would reuse cached household data while the http call goes
// to office, and the mismatch would only resolve on the next refetch.
export const commodityKeys = {
  all: ["commodity"] as const,
  group: (slug: string) => [...commodityKeys.all, slug] as const,
  list: (slug: string, opts?: ListCommoditiesOptions) =>
    [...commodityKeys.group(slug), "list", listKeySuffix(opts)] as const,
  detail: (slug: string, id: string) => [...commodityKeys.group(slug), "detail", id] as const,
  values: (slug: string) => [...commodityKeys.group(slug), "values"] as const,
}
