// The Items list as a list context (lib/list-context.ts): how its URL turns
// into a query, and how an item opened from it finds its way back and to its
// neighbors. CommoditiesListPage and the item page both read the list URL
// through `parseCommoditiesListParams`, so they ask the server the same
// question under the same query key and share the cached pages.
import { listPageQuery, type ListPageSource } from "@/hooks/useListNavigation"
import {
  listContextPage,
  listContextParams,
  withListContext,
  type ListContext,
} from "@/lib/list-context"

import { listCommodities, type ListCommoditiesOptions } from "./api"
import { commodityKeys } from "./keys"
import { commodityQueryToListOptions, parseCommodityQuery, type CommodityQuery } from "./query"

export const COMMODITIES_PAGE_SIZE = 24

// The filter half of the URL is the item query contract (./query); the page
// belongs to the list.
export interface CommoditiesListParams {
  page: number
  query: CommodityQuery
}

export function parseCommoditiesListParams(params: URLSearchParams): CommoditiesListParams {
  return { page: parsePageParam(params.get("page")), query: parseCommodityQuery(params) }
}

/** The list request for a parsed Items list URL. */
export function commoditiesListOptions(params: CommoditiesListParams): ListCommoditiesOptions {
  return {
    page: params.page,
    perPage: COMMODITIES_PAGE_SIZE,
    ...commodityQueryToListOptions(params.query),
  }
}

export function commoditiesListPath(slug: string): string {
  return `/g/${encodeURIComponent(slug)}/commodities`
}

/** True when the context is the group's Items list, the one an item can walk. */
export function isCommoditiesListContext(ctx: ListContext, slug: string): boolean {
  return !!slug && ctx.pathname === commoditiesListPath(slug)
}

/** The Items list behind a context, as a source for useListNavigation. */
export function commoditiesListSource(ctx: ListContext, slug: string): ListPageSource {
  const base = parseCommoditiesListParams(listContextParams(ctx))
  return {
    pageSize: COMMODITIES_PAGE_SIZE,
    page: listContextPage(ctx),
    pageQuery: (page) => {
      const opts = commoditiesListOptions({ ...base, page })
      return listPageQuery({
        queryKey: commodityKeys.list(slug, opts),
        queryFn: ({ signal }) => listCommodities({ ...opts, signal }),
        select: (data) => ({
          ids: data.commodities.map((c) => c.id ?? ""),
          total: data.total,
        }),
      })
    },
  }
}

/** An item's page, or its edit form, carrying the list it was opened from. */
export function commodityHref(
  slug: string,
  id: string,
  ctx: ListContext | null,
  opts: { edit?: boolean } = {}
): string {
  const path = `${commoditiesListPath(slug)}/${encodeURIComponent(id)}${opts.edit ? "/edit" : ""}`
  return withListContext(path, ctx)
}

// `Number("abc")` is NaN, which would poison the list query key and the
// pager; parseInt with a finite-and-positive guard collapses any garbage to
// page 1, the same way listContextPage reads it.
function parsePageParam(raw: string | null): number {
  if (!raw) return 1
  const parsed = Number.parseInt(raw, 10)
  if (!Number.isFinite(parsed) || parsed < 1) return 1
  return parsed
}
