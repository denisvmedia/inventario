import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import type { CommodityQuery } from "@/features/commodities/query"
import { cn } from "@/lib/utils"

import type { StaleReferences } from "./view"

interface QueryConditionsProps {
  query: CommodityQuery
  // Area names by ID, for the area condition.
  areaNames: ReadonlyMap<string, string>
  // Tag labels by slug.
  tagLabel: (slug: string) => string
  stale?: StaleReferences
}

// QueryConditions spells out a view's filter and sort as small badges, so
// a list of views says what each one shows. A condition naming something
// that no longer exists is marked as such.
export function QueryConditions({ query, areaNames, tagLabel, stale }: QueryConditionsProps) {
  const { t } = useTranslation()
  const staleTags = new Set(stale?.tags ?? [])
  const chips: { key: string; label: string; stale?: boolean }[] = []

  if (query.text) {
    chips.push({
      key: "text",
      label: t("commodities:savedViews.conditions.text", { text: query.text }),
    })
  }
  for (const type of query.types) {
    chips.push({ key: `type-${type}`, label: t(`commodities:type.${type}`) })
  }
  for (const status of query.statuses) {
    chips.push({ key: `status-${status}`, label: t(`commodities:status.${status}`) })
  }
  if (query.areaId) {
    chips.push({
      key: "area",
      label: stale?.area
        ? t("commodities:savedViews.conditions.deletedArea")
        : (areaNames.get(query.areaId) ?? t("commodities:filter.area")),
      stale: stale?.area,
    })
  }
  if (query.unassigned) {
    chips.push({ key: "unassigned", label: t("commodities:filter.unassigned") })
  }
  for (const w of query.warrantyStatuses) {
    chips.push({ key: `warranty-${w}`, label: t(`commodities:warrantyStatus.${w}`) })
  }
  if (query.lentOut) chips.push({ key: "lent", label: t("commodities:filter.lentOut") })
  // A deleted tag says so in words, not only in color.
  const tagChip = (key: string, slug: string, condition: string) => {
    const stale = staleTags.has(slug)
    chips.push({
      key,
      label: stale ? t("commodities:savedViews.conditions.deletedTag", { condition }) : condition,
      stale,
    })
  }
  for (const slug of query.tags.include) {
    tagChip(
      `tag-${slug}`,
      slug,
      query.tags.mode === "any" && query.tags.include.length > 1
        ? t("commodities:savedViews.conditions.tagAny", { tag: tagLabel(slug) })
        : t("commodities:savedViews.conditions.tag", { tag: tagLabel(slug) })
    )
  }
  for (const slug of query.tags.exclude) {
    tagChip(
      `not-tag-${slug}`,
      slug,
      t("commodities:savedViews.conditions.notTag", { tag: tagLabel(slug) })
    )
  }
  if (query.tags.untagged) {
    chips.push({ key: "untagged", label: t("commodities:savedViews.conditions.untagged") })
  }
  if (query.includeInactive) {
    chips.push({ key: "inactive", label: t("commodities:savedViews.conditions.inactive") })
  }
  chips.push({
    key: "sort",
    label: t("commodities:savedViews.conditions.sort", {
      field: t(`commodities:sort.${query.sort}`),
      direction: query.sortDesc
        ? t("commodities:savedViews.conditions.descending")
        : t("commodities:savedViews.conditions.ascending"),
    }),
  })

  return (
    <ul className="flex flex-wrap gap-1" data-testid="saved-view-conditions">
      {chips.map((chip) => (
        <li key={chip.key}>
          <Badge
            variant="outline"
            className={cn(
              "font-normal",
              chip.stale && "border-status-expiring/40 bg-status-expiring/10 text-status-expiring"
            )}
            data-stale={chip.stale ? "true" : undefined}
          >
            {chip.label}
          </Badge>
        </li>
      ))}
    </ul>
  )
}
