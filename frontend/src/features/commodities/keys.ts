import type { ListCommoditiesOptions } from "./api"
import { listOptionsToParams } from "./query"

// Stringify list-query options into a stable key suffix. The suffix is the
// API query string itself (listOptionsToParams sorts set-like lists), so
// two option objects that send the same request share one cache entry.
function listKeySuffix(opts: ListCommoditiesOptions | undefined): string {
  if (!opts) return ""
  return listOptionsToParams(opts).toString()
}

// TanStack Query keys for the commodities slice. Scoped by group slug
// because the http client rewrites /commodities -> /g/{slug}/commodities
// — without the slug in the key, navigating from /g/household to
// /g/office would reuse cached household data while the http call goes
// to office, and the mismatch would only resolve on the next refetch.
// commodityEventsKeySuffix mirrors listKeySuffix for the events endpoint.
function commodityEventsKeySuffix(opts: {
  page?: number
  perPage?: number
  kinds?: string[]
}): string {
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  for (const k of [...(opts.kinds ?? [])].sort()) params.append("kind", k)
  return params.toString()
}

export const commodityKeys = {
  all: ["commodity"] as const,
  group: (slug: string) => [...commodityKeys.all, slug] as const,
  list: (slug: string, opts?: ListCommoditiesOptions) =>
    [...commodityKeys.group(slug), "list", listKeySuffix(opts)] as const,
  // Distinct from `list` so the page-loop accumulator (`useAllCommodities`)
  // doesn't collide with the paginated single-page list cache. Named
  // `allList` (not `all`) because `all` above is the base namespace array
  // that `group()` spreads — reusing the name would shadow it and break
  // every commodity query. The suffix omits page/perPage — the hook always
  // pages internally — so two callers with the same filters share one entry.
  allList: (slug: string, opts?: ListCommoditiesOptions) =>
    [
      ...commodityKeys.group(slug),
      "all",
      listKeySuffix({ ...opts, page: undefined, perPage: undefined }),
    ] as const,
  detail: (slug: string, id: string) => [...commodityKeys.group(slug), "detail", id] as const,
  values: (slug: string) => [...commodityKeys.group(slug), "values"] as const,
  events: (
    slug: string,
    id: string,
    opts?: { page?: number; perPage?: number; kinds?: string[] }
  ) => [...commodityKeys.group(slug), "events", id, commodityEventsKeySuffix(opts ?? {})] as const,
}
