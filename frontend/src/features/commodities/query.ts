// The item query contract, frontend half. A CommodityQuery is one item
// filter plus its sort. The items list keeps it in the page URL, the API
// receives it as query parameters, and every link to a filtered list is
// built from it, so a filter set in one place reads the same everywhere.
//
// The backend half is go/services/commodityquery (parsing, validation) and
// go/registry/commodity_search.go (what a search term and a tag filter
// match). devdocs/item-query-contract.md describes the whole contract,
// including how the page URL names map onto the API names.
import type { Schema } from "@/types"

import type { ListCommoditiesOptions } from "./api"
import { COMMODITY_SORT_OPTIONS, type CommoditySortOption } from "./constants"

export type TagMatchMode = "all" | "any"

// Narrows by the tags items carry, compared by slug. The parts are AND-ed:
// include with mode "all" (default) needs every tag, with "any" one of them;
// exclude drops items carrying any of its tags; untagged keeps items without
// tags and cannot be combined with include or exclude.
export interface CommodityTagFilter {
  include: string[]
  mode: TagMatchMode
  exclude: string[]
  untagged: boolean
}

export interface CommodityQuery {
  // Free-text search. Terms separated by spaces must all match; double
  // quotes keep a phrase together.
  text: string
  types: string[]
  statuses: string[]
  areaId: string
  unassigned: boolean
  // Items bought from one of these sellers, by seller id.
  sellerIds: string[]
  // The list page hides drafts and inactive items unless this is set.
  includeInactive: boolean
  warrantyStatuses: string[]
  lentOut: boolean
  tags: CommodityTagFilter
  sort: CommoditySortOption
  sortDesc: boolean
  // Values of a flag or of tag_mode that the page URL carries but the parser
  // cannot read, by page-URL parameter. They stay in the URL and reach the
  // server as written, so the request fails with query.invalid_parameter and
  // the page names the parameter, as it does for any other unsupported value.
  unreadable: UnreadableValues
}

// The page-URL parameters whose value is parsed into a boolean or a tag mode
// rather than kept as written.
export type UnreadableParam = "unassigned" | "inactive" | "lent_out" | "tag_mode" | "no_tags"
export type UnreadableValues = Partial<Record<UnreadableParam, string>>

// The API name of each of those parameters.
const UNREADABLE_API_PARAM: Record<UnreadableParam, string> = {
  unassigned: "unassigned",
  inactive: "include_inactive",
  lent_out: "lent_out",
  tag_mode: "tag_mode",
  no_tags: "no_tags",
}

export const DEFAULT_COMMODITY_QUERY: CommodityQuery = {
  text: "",
  types: [],
  statuses: [],
  areaId: "",
  unassigned: false,
  sellerIds: [],
  includeInactive: false,
  warrantyStatuses: [],
  lentOut: false,
  tags: { include: [], mode: "all", exclude: [], untagged: false },
  sort: "name",
  sortDesc: false,
  unreadable: {},
}

// Page-URL parameter names. They predate the contract and are kept so saved
// links keep working; listOptionsToParams maps them onto the API names.
const URL_PARAM = {
  text: "q",
  type: "type",
  status: "status",
  area: "area",
  unassigned: "unassigned",
  seller: "seller",
  inactive: "inactive",
  warranty: "warranty",
  lentOut: "lent_out",
  tags: "tags",
  tagMode: "tag_mode",
  tagsExclude: "tags_exclude",
  noTags: "no_tags",
  sort: "sort",
} as const

// Every page-URL parameter the query owns. Anything else in the URL (page,
// view, …) belongs to the page and is left alone by writeCommodityQuery.
export const COMMODITY_QUERY_URL_PARAMS: readonly string[] = Object.values(URL_PARAM)

function unique(values: string[]): string[] {
  return [...new Set(values)]
}

function commaList(params: URLSearchParams, name: string): string[] {
  return unique(
    params
      .getAll(name)
      .flatMap((v) => v.split(","))
      .map((v) => v.trim())
      .filter(Boolean)
  )
}

// Reads a boolean the way the server does: 1/true and 0/false in any case,
// absent or empty as false. Anything else is recorded in unreadable and the
// flag reads as false.
function flag(
  params: URLSearchParams,
  name: UnreadableParam,
  unreadable: UnreadableValues
): boolean {
  const v = params.get(name) ?? ""
  switch (v.toLowerCase()) {
    case "1":
    case "true":
      return true
    case "":
    case "0":
    case "false":
      return false
  }
  unreadable[name] = v
  return false
}

function tagMode(params: URLSearchParams, unreadable: UnreadableValues): TagMatchMode {
  const v = params.get(URL_PARAM.tagMode) ?? ""
  if (v === "any") return "any"
  if (v !== "" && v !== "all") unreadable.tag_mode = v
  return "all"
}

// Reads the query from a page URL. Values are kept as written, so a link
// with a filter value the server does not support reaches the server and
// fails with a 400 the page can show, instead of being dropped. The sort is
// the exception: sorting does not filter, and an unknown field falls back to
// name as it does on the server.
export function parseCommodityQuery(params: URLSearchParams): CommodityQuery {
  const sortRaw = params.get(URL_PARAM.sort) ?? ""
  const sortDesc = sortRaw.startsWith("-")
  const sortField = (sortDesc ? sortRaw.slice(1) : sortRaw) as CommoditySortOption
  const validSort = COMMODITY_SORT_OPTIONS.includes(sortField)
  const unreadable: UnreadableValues = {}
  return {
    text: (params.get(URL_PARAM.text) ?? "").trim(),
    types: unique(params.getAll(URL_PARAM.type).filter(Boolean)),
    statuses: unique(params.getAll(URL_PARAM.status).filter(Boolean)),
    areaId: params.get(URL_PARAM.area) ?? "",
    unassigned: flag(params, URL_PARAM.unassigned, unreadable),
    sellerIds: unique(params.getAll(URL_PARAM.seller).filter(Boolean)),
    includeInactive: flag(params, URL_PARAM.inactive, unreadable),
    warrantyStatuses: unique(params.getAll(URL_PARAM.warranty).filter(Boolean)),
    lentOut: flag(params, URL_PARAM.lentOut, unreadable),
    tags: {
      include: commaList(params, URL_PARAM.tags),
      mode: tagMode(params, unreadable),
      exclude: commaList(params, URL_PARAM.tagsExclude),
      untagged: flag(params, URL_PARAM.noTags, unreadable),
    },
    sort: validSort ? sortField : "name",
    sortDesc: validSort && sortDesc,
    unreadable,
  }
}

// Writes query into a copy of params: the query's own parameters are
// replaced, defaults are left out, and every other parameter is kept.
export function writeCommodityQuery(
  params: URLSearchParams,
  query: CommodityQuery
): URLSearchParams {
  const next = new URLSearchParams(params)
  for (const name of COMMODITY_QUERY_URL_PARAMS) next.delete(name)

  if (query.text.trim()) next.set(URL_PARAM.text, query.text.trim())
  for (const v of query.types) next.append(URL_PARAM.type, v)
  for (const v of query.statuses) next.append(URL_PARAM.status, v)
  if (query.areaId) next.set(URL_PARAM.area, query.areaId)
  if (query.unassigned) next.set(URL_PARAM.unassigned, "1")
  for (const v of query.sellerIds) next.append(URL_PARAM.seller, v)
  if (query.includeInactive) next.set(URL_PARAM.inactive, "1")
  for (const v of query.warrantyStatuses) next.append(URL_PARAM.warranty, v)
  if (query.lentOut) next.set(URL_PARAM.lentOut, "1")
  if (query.tags.include.length > 0) next.set(URL_PARAM.tags, query.tags.include.join(","))
  // Written without tags too, so the mode picked before the first tag is
  // kept; the server ignores it until a tag is included.
  if (query.tags.mode === "any") next.set(URL_PARAM.tagMode, "any")
  if (query.tags.exclude.length > 0) next.set(URL_PARAM.tagsExclude, query.tags.exclude.join(","))
  if (query.tags.untagged) next.set(URL_PARAM.noTags, "1")
  if (query.sort !== "name" || query.sortDesc) {
    next.set(URL_PARAM.sort, query.sortDesc ? `-${query.sort}` : query.sort)
  }
  // An unreadable value is kept until the parameter is set to a readable one.
  for (const [name, raw] of unreadableEntries(query.unreadable)) {
    if (!next.has(name)) next.set(name, raw)
  }
  return next
}

function unreadableEntries(values: UnreadableValues | undefined): [UnreadableParam, string][] {
  return Object.entries(values ?? {}).filter(
    (entry): entry is [UnreadableParam, string] => entry[1] !== undefined
  )
}

// The href of the items list showing query, a partial query filled in with
// the defaults. Used by every link that opens a filtered list.
export function commodityListHref(slug: string, query: Partial<CommodityQuery>): string {
  const params = writeCommodityQuery(new URLSearchParams(), {
    ...DEFAULT_COMMODITY_QUERY,
    ...query,
    tags: { ...DEFAULT_COMMODITY_QUERY.tags, ...query.tags },
    unreadable: { ...query.unreadable },
  })
  const qs = params.toString()
  const base = `/g/${encodeURIComponent(slug)}/commodities`
  return qs ? `${base}?${qs}` : base
}

// Link to the items list showing every item of one type, by the type's
// slug, inactive ones included, so the list matches the type's item count.
export function commodityTypeHref(slug: string, typeSlug: string): string {
  return commodityListHref(slug, { includeInactive: true, types: [typeSlug] })
}

// Link to the items list showing every item that carries one tag, inactive
// ones included, so the list matches the tag's item count.
export function commodityTagHref(slug: string, tagSlug: string): string {
  return commodityListHref(slug, {
    includeInactive: true,
    tags: { ...DEFAULT_COMMODITY_QUERY.tags, include: [tagSlug] },
  })
}

// Link to the items list showing every item bought from one seller,
// inactive ones included, so the list matches the seller's item count.
export function commoditySellerHref(slug: string, sellerId: string): string {
  return commodityListHref(slug, { includeInactive: true, sellerIds: [sellerId] })
}

// The tag-filter parameters whose value can be unreadable.
const UNREADABLE_TAG_PARAMS: readonly UnreadableParam[] = ["tag_mode", "no_tags"]

// query with its tag filter replaced by tags. An unreadable tag_mode or
// no_tags is dropped, because tags now says what those parameters hold.
export function withTagFilter(query: CommodityQuery, tags: CommodityTagFilter): CommodityQuery {
  const unreadable = { ...query.unreadable }
  for (const name of UNREADABLE_TAG_PARAMS) delete unreadable[name]
  return { ...query, tags, unreadable }
}

// query narrowed to items that also carry tagSlug. The tag leaves the
// excluded tags and "No tags" is turned off, since a required tag combines
// with neither.
export function withIncludedTag(query: CommodityQuery, tagSlug: string): CommodityQuery {
  const { include, exclude } = query.tags
  return withTagFilter(query, {
    ...query.tags,
    include: include.includes(tagSlug) ? include : [...include, tagSlug],
    exclude: exclude.filter((s) => s !== tagSlug),
    untagged: false,
  })
}

// How many tag-filter parameters in the page URL hold a value the parser
// cannot read.
export function unreadableTagParamCount(query: CommodityQuery): number {
  return UNREADABLE_TAG_PARAMS.filter((name) => query.unreadable[name] !== undefined).length
}

// Whether query narrows the list at all; the sort does not count.
export function hasCommodityFilters(query: CommodityQuery): boolean {
  return (
    query.text !== "" ||
    query.types.length > 0 ||
    query.statuses.length > 0 ||
    query.areaId !== "" ||
    query.unassigned ||
    query.sellerIds.length > 0 ||
    query.includeInactive ||
    query.warrantyStatuses.length > 0 ||
    query.lentOut ||
    query.tags.include.length > 0 ||
    query.tags.exclude.length > 0 ||
    query.tags.untagged ||
    unreadableEntries(query.unreadable).length > 0
  )
}

// The list-endpoint options for query. includeInactive is always sent, so
// the server applies the active-only default the list page shows.
export function commodityQueryToListOptions(query: CommodityQuery): ListCommoditiesOptions {
  return {
    search: query.text || undefined,
    types: query.types,
    statuses: query.statuses,
    areaId: query.areaId || undefined,
    unassigned: query.unassigned || undefined,
    sellerIds: query.sellerIds.length > 0 ? query.sellerIds : undefined,
    includeInactive: query.includeInactive,
    warrantyStatuses: query.warrantyStatuses.length > 0 ? query.warrantyStatuses : undefined,
    lentOut: query.lentOut ? true : undefined,
    tags: query.tags.include.length > 0 ? query.tags.include : undefined,
    tagMode: query.tags.include.length > 0 && query.tags.mode === "any" ? "any" : undefined,
    tagsExclude: query.tags.exclude.length > 0 ? query.tags.exclude : undefined,
    noTags: query.tags.untagged || undefined,
    sort: query.sort,
    sortDesc: query.sortDesc,
    unreadable: apiUnreadable(query.unreadable),
  }
}

// The unreadable values under their API names, or undefined when there are
// none, so a query without them keeps its list-options shape.
function apiUnreadable(values: UnreadableValues): Record<string, string> | undefined {
  const entries = unreadableEntries(values)
  if (entries.length === 0) return undefined
  return Object.fromEntries(entries.map(([name, raw]) => [UNREADABLE_API_PARAM[name], raw]))
}

// The API wire format of the list options: the one place that names the
// query parameters GET /commodities reads. Set-like lists are sorted, so
// two option objects that select the same items produce the same string
// (the TanStack Query key relies on that).
export function listOptionsToParams(opts: ListCommoditiesOptions): URLSearchParams {
  const params = new URLSearchParams()
  const sorted = (values?: string[]) => [...(values ?? [])].sort()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  for (const t of sorted(opts.types)) params.append("type", t)
  for (const s of sorted(opts.statuses)) params.append("status", s)
  if (opts.areaId) params.set("area_id", opts.areaId)
  if (opts.search?.trim()) params.set("q", opts.search.trim())
  // Absent means the server default (true, no filter); the list page
  // sends an explicit boolean.
  if (opts.includeInactive !== undefined) {
    params.set("include_inactive", opts.includeInactive ? "true" : "false")
  }
  if (opts.sort) params.set("sort", opts.sortDesc ? `-${opts.sort}` : opts.sort)
  for (const w of sorted(opts.warrantyStatuses)) params.append("warranty_status", w)
  if (opts.warrantyExpiresBefore?.trim()) {
    params.set("warranty_expires_before", opts.warrantyExpiresBefore.trim())
  }
  if (opts.lentOut !== undefined) params.set("lent_out", opts.lentOut ? "true" : "false")
  if (opts.unassigned) params.set("unassigned", "true")
  for (const id of sorted(opts.sellerIds)) params.append("seller_id", id)
  if (opts.tags && opts.tags.length > 0) {
    params.set("tags", sorted(opts.tags).join(","))
    if (opts.tagMode === "any") params.set("tag_mode", "any")
  }
  if (opts.tagsExclude && opts.tagsExclude.length > 0) {
    params.set("tags_exclude", sorted(opts.tagsExclude).join(","))
  }
  if (opts.noTags) params.set("no_tags", "true")
  if (opts.trashed) params.set("trashed", "true")
  // Sent as written, replacing the readable default written above, so the
  // server refuses the request instead of applying a filter the URL did not
  // ask for.
  const unreadable = opts.unreadable ?? {}
  for (const name of Object.keys(unreadable).sort()) params.set(name, unreadable[name])
  return params
}

// Why an item matched the search text: the field and the value in it.
// For a tag the value is its slug; for a long note it is an excerpt.
export type CommoditySearchMatch = Schema<"registry.CommoditySearchMatch">
export type CommoditySearchField = Schema<"registry.CommoditySearchField">

// The match worth showing next to a result. The name is already the
// result's title, so a match there needs no hint; otherwise the first
// match in field order (asset ID first) is the one shown.
export function primarySearchMatch(
  matches: CommoditySearchMatch[] | undefined
): CommoditySearchMatch | undefined {
  if (!matches || matches.length === 0) return undefined
  if (matches.some((m) => m.field === "name")) return undefined
  return matches[0]
}
