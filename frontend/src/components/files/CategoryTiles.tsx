import { useTranslation } from "react-i18next"

import { Card } from "@/components/ui/card"
import { FILE_CATEGORY_TILES, type FileCategoryTile } from "@/features/files/constants"
import { useCategoryLabel } from "@/features/files/labels"
import type { FileCategoryCounts } from "@/features/files/api"
import { useIsMobile } from "@/hooks/use-mobile"
import { cn } from "@/lib/utils"

// Five tiles row at the top of the Files list. Selecting a tile filters
// the list to that category (or "all" — synthetic, not part of the BE
// enum). Counts come from GET /files/category-counts and respect the
// active search/tags filters. Below the `sm` breakpoint the row collapses
// into a single full-width <select>; the mock paid for the screen real
// estate the 2-col grid was eating on phones.
export interface CategoryTilesProps {
  active: FileCategoryTile
  counts?: FileCategoryCounts
  loading?: boolean
  onSelect: (key: FileCategoryTile) => void
}

export function CategoryTiles({ active, counts, loading, onSelect }: CategoryTilesProps) {
  const { t } = useTranslation()
  const labelOf = useCategoryLabel()
  const isMobile = useIsMobile()

  if (isMobile) {
    return (
      <label className="block">
        <span className="sr-only">
          {t("files:categoryMobileLabel", { defaultValue: "Category" })}
        </span>
        <select
          data-testid="files-category-select"
          value={active}
          onChange={(e) => onSelect(e.target.value as FileCategoryTile)}
          className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm font-medium shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {FILE_CATEGORY_TILES.map((tile) => {
            const value = countForKey(tile.key, counts)
            const display = loading && value === undefined ? "—" : (value ?? 0)
            return (
              <option key={tile.key} value={tile.key}>
                {labelOf(tile.key)} ({display})
              </option>
            )
          })}
        </select>
      </label>
    )
  }

  return (
    <div
      role="tablist"
      aria-label={t("files:title", { defaultValue: "Files" })}
      className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5"
    >
      {FILE_CATEGORY_TILES.map((tile) => {
        const Icon = tile.icon
        const value = countForKey(tile.key, counts)
        const selected = active === tile.key
        return (
          <Card
            key={tile.key}
            role="tab"
            aria-selected={selected}
            tabIndex={selected ? 0 : -1}
            data-testid={`files-tile-${tile.key}`}
            onClick={() => onSelect(tile.key)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault()
                onSelect(tile.key)
              }
            }}
            className={cn(
              "group flex cursor-pointer flex-col items-start gap-1.5 p-3 text-left transition-all",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
              selected
                ? "border-primary bg-primary/5 shadow-sm"
                : "hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-sm"
            )}
          >
            <div
              className={cn(
                "flex size-7 items-center justify-center rounded-lg transition-colors",
                selected ? cn(tile.activeBg, tile.activeColor) : "bg-muted text-muted-foreground"
              )}
            >
              <Icon className="size-3.5" aria-hidden="true" />
            </div>
            <div className="min-w-0 w-full">
              <p
                className={cn(
                  "truncate text-xs font-semibold",
                  selected ? "text-foreground" : "text-muted-foreground group-hover:text-foreground"
                )}
              >
                {labelOf(tile.key)}
              </p>
              <p
                className={cn(
                  "mt-0.5 text-lg font-bold leading-none tabular-nums",
                  selected ? "text-foreground" : "text-muted-foreground"
                )}
                data-testid={`files-tile-count-${tile.key}`}
              >
                {loading && value === undefined ? "—" : (value ?? 0)}
              </p>
            </div>
          </Card>
        )
      })}
    </div>
  )
}

function countForKey(key: FileCategoryTile, counts: FileCategoryCounts | undefined) {
  if (!counts) return undefined
  switch (key) {
    case "all":
      return counts.all
    case "images":
      return counts.images
    case "documents":
      return counts.documents
    case "other":
      return counts.other
  }
}
