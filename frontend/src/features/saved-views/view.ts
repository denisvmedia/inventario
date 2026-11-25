import {
  AlertCircle,
  Archive,
  Banknote,
  Bookmark,
  Car,
  Clock,
  Gift,
  Heart,
  House,
  MapPin,
  Package,
  Shield,
  Star,
  Tag,
  Wrench,
  Zap,
  type LucideIcon,
} from "lucide-react"

import {
  DEFAULT_COMMODITY_QUERY,
  commodityQueryFromApiQuery,
  commodityQueryToApiQuery,
  sameCommodityQuery,
  writeCommodityQuery,
  type CommodityQuery,
} from "@/features/commodities/query"

import type { SavedView, SavedViewLayout } from "./api"

// The page-URL parameter that names the saved view the items list was
// opened from. The list keeps it while the filters change, so it can tell
// that the view was changed and offer to save or revert.
export const SAVED_VIEW_PARAM = "saved_view"

// The icons a view can carry, by the name the server stores
// (models.ValidSavedViewIcons).
export const SAVED_VIEW_ICONS: Record<string, LucideIcon> = {
  bookmark: Bookmark,
  star: Star,
  heart: Heart,
  tag: Tag,
  shield: Shield,
  clock: Clock,
  alert: AlertCircle,
  wrench: Wrench,
  package: Package,
  home: House,
  car: Car,
  zap: Zap,
  gift: Gift,
  archive: Archive,
  banknote: Banknote,
  "map-pin": MapPin,
}

export const SAVED_VIEW_ICON_NAMES = Object.keys(SAVED_VIEW_ICONS)

// The query a view stores, read into the items-list form.
export function savedViewQuery(view: Pick<SavedView, "query">): CommodityQuery {
  return commodityQueryFromApiQuery(view.query)
}

// The items list showing view: its query and layout, plus the view's ID so
// the list knows which view it shows. A view whose stored query cannot run
// has no link (null): the list would drop the condition it cannot read and
// show more than the view does.
export function savedViewHref(
  slug: string,
  view: Pick<SavedView, "id" | "query" | "layout" | "queryError">
): string | null {
  if (view.queryError) return null
  const params = writeCommodityQuery(new URLSearchParams(), savedViewQuery(view))
  if (view.layout) params.set("view", view.layout)
  params.set(SAVED_VIEW_PARAM, view.id)
  return `/g/${encodeURIComponent(slug)}/commodities?${params.toString()}`
}

// Whether the list, showing query in layout, differs from what view keeps.
// A view without a layout keeps the viewer's own, so the layout only counts
// when the view has one.
export function isSavedViewChanged(
  view: Pick<SavedView, "query" | "layout">,
  query: CommodityQuery,
  layout: Exclude<SavedViewLayout, "">
): boolean {
  if (!sameCommodityQuery(savedViewQuery(view), query)) return true
  return view.layout !== "" && view.layout !== layout
}

// The query string a view stores for query.
export function savedViewQueryString(query: CommodityQuery): string {
  return commodityQueryToApiQuery(query)
}

// What a view's query names that no longer exists: an area that was
// deleted, tags without a tag row, the IDs of deleted sellers and the slugs
// of item types the group no longer has. Only meaningful once the areas and
// tags have loaded.
export interface StaleReferences {
  area: boolean
  tags: string[]
  sellers: string[]
  types: string[]
}

// sellerIds and typeSlugs are undefined while the sellers or the item types
// are not known (loading or failed): none of them is reported deleted then.
export function staleReferences(
  query: CommodityQuery,
  areaIds: ReadonlySet<string>,
  tagSlugs: ReadonlySet<string>,
  sellerIds?: ReadonlySet<string>,
  typeSlugs?: ReadonlySet<string>
): StaleReferences {
  const named = [...query.tags.include, ...query.tags.exclude]
  return {
    area: query.areaId !== "" && !areaIds.has(query.areaId),
    tags: named.filter((slug) => !tagSlugs.has(slug)),
    sellers: sellerIds ? query.sellerIds.filter((id) => !sellerIds.has(id)) : [],
    types: typeSlugs ? query.types.filter((slug) => !typeSlugs.has(slug)) : [],
  }
}

export function hasStaleReferences(stale: StaleReferences): boolean {
  return stale.area || stale.tags.length > 0 || stale.sellers.length > 0 || stale.types.length > 0
}

// A view the dashboard offers to a member who has none yet. Each one runs a
// filter the items list shows; dates stay relative because the warranty
// status is computed on every run.
export interface SuggestedView {
  key: "expiringWarranties" | "lentOut" | "noLocation" | "untagged"
  icon: string
  query: string
}

function suggestion(
  key: SuggestedView["key"],
  icon: string,
  patch: Partial<CommodityQuery>
): SuggestedView {
  return { key, icon, query: commodityQueryToApiQuery({ ...DEFAULT_COMMODITY_QUERY, ...patch }) }
}

export const SUGGESTED_VIEWS: readonly SuggestedView[] = [
  suggestion("expiringWarranties", "shield", { warrantyStatuses: ["expiring"] }),
  suggestion("lentOut", "package", { lentOut: true }),
  suggestion("noLocation", "map-pin", { unassigned: true }),
  suggestion("untagged", "tag", {
    tags: { ...DEFAULT_COMMODITY_QUERY.tags, untagged: true },
  }),
]
