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

/** The page sizes the Items list offers, smallest first. */
export const COMMODITIES_PAGE_SIZES = [24, 48, 96] as const
export type CommoditiesPageSize = (typeof COMMODITIES_PAGE_SIZES)[number]
/** The page size until the user picks one. */
export const COMMODITIES_PAGE_SIZE: CommoditiesPageSize = 24

// The page size is the `per_page` URL parameter, and a page number counts
// pages of that size: 24 when the URL names none, so a link to page 3 shows
// the same items on every device. The size the user last picked on this
// device applies only where the list starts, at page 1, which begins with
// the same item whatever the size. The list page then writes it into its
// URL, so the list URL, and the list context an item opened from it
// carries, names the size whenever it is not 24.
export const COMMODITIES_PAGE_SIZE_PARAM = "per_page"
const PAGE_SIZE_STORAGE_KEY = "commodities:pageSize"

// The filter half of the URL is the item query contract (./query); the page
// and its size belong to the list.
export interface CommoditiesListParams {
  page: number
  perPage: CommoditiesPageSize
  query: CommodityQuery
}

/**
 * Reads an Items list URL. `remembered: false` ignores the size picked on
 * this device, for a list context: the list page wrote its size into the
 * URL, and another tab may have picked a different one since.
 */
export function parseCommoditiesListParams(
  params: URLSearchParams,
  { remembered = true }: { remembered?: boolean } = {}
): CommoditiesListParams {
  const page = parsePageParam(params.get("page"))
  const fallback = remembered && page === 1 ? storedCommoditiesPageSize() : COMMODITIES_PAGE_SIZE
  return {
    page,
    perPage: parsePageSize(params.get(COMMODITIES_PAGE_SIZE_PARAM)) ?? fallback,
    query: parseCommodityQuery(params),
  }
}

/** The list request for a parsed Items list URL. */
export function commoditiesListOptions(params: CommoditiesListParams): ListCommoditiesOptions {
  return {
    page: params.page,
    perPage: params.perPage,
    ...commodityQueryToListOptions(params.query),
  }
}

/** Remembers the page size picked on the list page, for this device. */
export function storeCommoditiesPageSize(size: CommoditiesPageSize): void {
  try {
    localStorage.setItem(PAGE_SIZE_STORAGE_KEY, String(size))
  } catch {
    // Storage can be unavailable (private mode, blocked site data); the URL
    // still carries the size.
  }
}

function storedCommoditiesPageSize(): CommoditiesPageSize {
  try {
    return parsePageSize(localStorage.getItem(PAGE_SIZE_STORAGE_KEY)) ?? COMMODITIES_PAGE_SIZE
  } catch {
    return COMMODITIES_PAGE_SIZE
  }
}

/** A page size the list offers, or undefined for anything else. */
export function parsePageSize(raw: string | null): CommoditiesPageSize | undefined {
  const parsed = Number(raw)
  return COMMODITIES_PAGE_SIZES.find((size) => size === parsed)
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
  const base = parseCommoditiesListParams(listContextParams(ctx), { remembered: false })
  return {
    pageSize: base.perPage,
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
