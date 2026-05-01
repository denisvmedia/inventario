import { useTranslation } from "react-i18next"

import { Card } from "@/components/ui/card"
import {
  FILE_CATEGORY_TILES,
  type FileCategoryTile,
} from "@/features/files/constants"
import type { FileCategoryCounts } from "@/features/files/api"
import { cn } from "@/lib/utils"

// Static label resolver — i18next-cli can't see template-literal keys,
// so a switch with explicit t() calls keeps the en/files.json catalogue
// in sync without manual key bookkeeping.
function useCategoryLabel(): (key: FileCategoryTile) => string {
  const { t } = useTranslation()
  return (key) => {
    switch (key) {
      case "all":
        return t("files:categoryAll", { defaultValue: "All" })
      case "photos":
        return t("files:categoryPhotos", { defaultValue: "Photos" })
      case "invoices":
        return t("files:categoryInvoices", { defaultValue: "Invoices" })
      case "documents":
        return t("files:categoryDocuments", { defaultValue: "Documents" })
      case "other":
        return t("files:categoryOther", { defaultValue: "Other" })
    }
  }
}

// Five tiles row at the top of the Files list. Selecting a tile filters
// the list to that category (or "all" — synthetic, not part of the BE
// enum). Counts come from GET /files/category-counts and respect the
// active search/tags filters.
export interface CategoryTilesProps {
  active: FileCategoryTile
  counts?: FileCategoryCounts
  loading?: boolean
  onSelect: (key: FileCategoryTile) => void
}

export function CategoryTiles({ active, counts, loading, onSelect }: CategoryTilesProps) {
  const { t } = useTranslation()
  const labelOf = useCategoryLabel()
  return (
    <div
      role="tablist"
      aria-label={t("files:title", { defaultValue: "Files" })}
      className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5"
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
              "flex cursor-pointer items-center gap-3 p-4 transition-colors",
              "hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
              selected && "border-primary bg-primary/5"
            )}
          >
            <div
              className={cn(
                "flex size-10 items-center justify-center rounded-md",
                selected ? "bg-primary text-primary-foreground" : "bg-muted text-foreground/70"
              )}
            >
              <Icon className="size-5" aria-hidden="true" />
            </div>
            <div className="flex min-w-0 flex-col">
              <span className="text-sm font-medium leading-tight">
                {labelOf(tile.key)}
              </span>
              <span
                className="text-xs text-muted-foreground"
                data-testid={`files-tile-count-${tile.key}`}
              >
                {loading && value === undefined ? "—" : value ?? 0}
              </span>
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
    case "photos":
      return counts.photos
    case "invoices":
      return counts.invoices
    case "documents":
      return counts.documents
    case "other":
      return counts.other
  }
}
