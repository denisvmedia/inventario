import { File, FileX, Hash, Package, Tag } from "lucide-react"
import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"

import type { TagKind, TagStats } from "@/features/tags/api"

export interface TagsStatsBarProps {
  stats?: TagStats
  kind: TagKind
  loading?: boolean
  testId?: string
}

// Visual contract: matches the stats row in
// `design-mocks/src/views/TagsView.tsx` — icon-tile + label + value laid
// out horizontally inside a `rounded-xl border bg-card` shell. Item-tags
// and file-tags are separate entities, so the bar shows only the three
// tiles relevant to the active view (tag count + tagged/untagged for that
// entity), never a combined total.
interface TileSpec {
  key: keyof TagStats
  labelKey: string
  icon: typeof Tag
}

const COMMODITY_TILES: TileSpec[] = [
  { key: "commodity_tags_total", labelKey: "tags:stats.commodityTagsTotal", icon: Tag },
  { key: "items_tagged", labelKey: "tags:stats.itemsTagged", icon: Package },
  { key: "items_untagged", labelKey: "tags:stats.itemsUntagged", icon: Hash },
]

const FILE_TILES: TileSpec[] = [
  { key: "file_tags_total", labelKey: "tags:stats.fileTagsTotal", icon: Tag },
  { key: "files_tagged", labelKey: "tags:stats.filesTagged", icon: File },
  { key: "files_untagged", labelKey: "tags:stats.filesUntagged", icon: FileX },
]

export function TagsStatsBar({ stats, kind, loading, testId }: TagsStatsBarProps) {
  const { t } = useTranslation(["tags"])
  const showPlaceholder = loading && stats === undefined
  const tiles = kind === "file" ? FILE_TILES : COMMODITY_TILES
  return (
    <div className="grid grid-cols-3 gap-3" data-testid={testId ?? "tags-stats"}>
      {tiles.map(({ key, labelKey, icon: Icon }) => {
        const value = stats?.[key]
        return (
          <div
            key={key}
            className="rounded-xl border border-border bg-card px-4 py-3 flex items-center gap-3"
            data-testid={`tags-stats-${key.replace(/_/g, "-")}`}
          >
            <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
              <Icon aria-hidden="true" className="size-4 text-muted-foreground" />
            </div>
            <div className="min-w-0">
              <p className="text-xs text-muted-foreground truncate">{t(labelKey)}</p>
              <p
                className={cn(
                  "text-lg font-semibold leading-tight tabular-nums",
                  showPlaceholder && "text-muted-foreground"
                )}
              >
                {showPlaceholder ? "—" : (value ?? 0)}
              </p>
            </div>
          </div>
        )
      })}
    </div>
  )
}
