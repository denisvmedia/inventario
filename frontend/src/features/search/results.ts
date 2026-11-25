// Where a search result leads. The command palette and the search page link
// a result the same way, so a result opened from either lands on the same
// page, and "show its items" opens the same filtered list.
import type { TFunction } from "i18next"

import { searchMatchText } from "@/components/items/SearchMatchHint"
import { formatAssetId } from "@/features/commodities/asset-id"
import {
  commodityListHref,
  commodityTagHref,
  type CommoditySearchMatch,
} from "@/features/commodities/query"

import type { CommodityAttrs, TagAttrs } from "./api"

function groupPath(slug: string): string {
  return `/g/${encodeURIComponent(slug)}`
}

export function commodityHref(slug: string, id: string): string {
  return `${groupPath(slug)}/commodities/${encodeURIComponent(id)}`
}

export function locationHref(slug: string, id: string): string {
  return `${groupPath(slug)}/locations/${encodeURIComponent(id)}`
}

export function areaHref(slug: string, id: string): string {
  return `${groupPath(slug)}/areas/${encodeURIComponent(id)}`
}

export function fileHref(slug: string, id: string): string {
  return `${groupPath(slug)}/files/${encodeURIComponent(id)}`
}

// A tag has no page of its own: it opens on the Tags page, filtered to it,
// under its kind.
export function tagHref(slug: string, tag: Pick<TagAttrs, "slug" | "kind">): string {
  const params = new URLSearchParams()
  params.set("kind", tag.kind ?? "commodity")
  params.set("q", tag.slug ?? "")
  return `${groupPath(slug)}/tags?${params.toString()}`
}

// The items in an area, on the items list.
export function areaItemsHref(slug: string, areaId: string): string {
  return commodityListHref(slug, { areaId })
}

// The items carrying an item tag, or the files carrying a file tag, as the
// Tags page links them.
export function tagListHref(slug: string, tag: Pick<TagAttrs, "slug" | "kind">): string {
  const tagSlug = tag.slug ?? ""
  if (tag.kind === "file") {
    return `${groupPath(slug)}/files?${new URLSearchParams({ tags: tagSlug })}`
  }
  return commodityTagHref(slug, tagSlug)
}

// The line under an item result: where the search text was found when the
// name does not show it, otherwise the asset ID and short name.
export function commoditySubtitle(
  t: TFunction,
  attrs: CommodityAttrs,
  match: CommoditySearchMatch | undefined,
  tagLabel: (slug: string) => string
): string | undefined {
  if (match) return searchMatchText(t, match, tagLabel)
  return (
    [formatAssetId(attrs.asset_number), attrs.short_name].filter(Boolean).join(" · ") || undefined
  )
}
