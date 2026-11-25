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
import {
  COMMODITY_SORT_OPTIONS,
  type CommoditySortOption,
  type CommodityStatusValue,
  type CommodityTypeValue,
  type CommodityWarrantyStatus,
} from "./constants"
import { commodityKeys } from "./keys"

export const COMMODITIES_PAGE_SIZE = 24

export interface CommoditiesListParams {
  page: number
  search: string
  types: CommodityTypeValue[]
  statuses: CommodityStatusValue[]
  areaId: string
  warranty: CommodityWarrantyStatus[]
  lentOutOnly: boolean
  unassignedOnly: boolean
  includeInactive: boolean
  /** The raw `sort` value, e.g. `-purchase_date`. */
  sortRaw: string
  sort: CommoditySortOption
  sortDesc: boolean
}

export function parseCommoditiesListParams(params: URLSearchParams): CommoditiesListParams {
  const sortRaw = params.get("sort") ?? "name"
  const sortDesc = sortRaw.startsWith("-")
  const sortField = (sortDesc ? sortRaw.slice(1) : sortRaw) as CommoditySortOption
  return {
    page: parsePageParam(params.get("page")),
    search: params.get("q") ?? "",
    types: params.getAll("type") as CommodityTypeValue[],
    statuses: params.getAll("status") as CommodityStatusValue[],
    areaId: params.get("area") ?? "",
    warranty: params.getAll("warranty") as CommodityWarrantyStatus[],
    lentOutOnly: params.get("lent_out") === "1",
    unassignedOnly: params.get("unassigned") === "1",
    includeInactive: params.get("inactive") === "1",
    sortRaw,
    sort: COMMODITY_SORT_OPTIONS.includes(sortField) ? sortField : "name",
    sortDesc,
  }
}

/** The list request for a parsed Items list URL. */
export function commoditiesListOptions(params: CommoditiesListParams): ListCommoditiesOptions {
  return {
    page: params.page,
    perPage: COMMODITIES_PAGE_SIZE,
    search: params.search,
    types: params.types,
    statuses: params.statuses,
    areaId: params.areaId || undefined,
    // The list always opts in to the active-only view unless the toggle is
    // on. An explicit boolean (not undefined) makes the server apply the
    // implicit filter.
    includeInactive: params.includeInactive,
    // Single-state chips: only ever send true.
    lentOut: params.lentOutOnly ? true : undefined,
    unassigned: params.unassignedOnly ? true : undefined,
    sort: params.sort,
    sortDesc: params.sortDesc,
    // Server-side, so total and page count describe the filtered list (#2128).
    warrantyStatuses: params.warranty.length > 0 ? params.warranty : undefined,
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
