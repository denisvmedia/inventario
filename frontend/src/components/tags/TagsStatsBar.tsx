import { File, FileX, Hash, Package, Tag } from "lucide-react"
import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"

import type { TagStats } from "@/features/tags/api"

export interface TagsStatsBarProps {
  stats?: TagStats
  loading?: boolean
  testId?: string
}

// Visual contract: matches the stats row in
// `design-mocks/src/views/TagsView.tsx` — icon-tile + label + value
// laid out horizontally inside a `rounded-xl border bg-card` shell. Mock
// shows three tiles (Total / Tagged / Untagged items); we keep the
// extra two file tiles so the page also surfaces file-tag adoption
// (the Files page mirrors this number elsewhere but the Tags page is
// the canonical "how are tags being used?" surface). Logged as a
// deviation in devdocs/frontend/design-deviations.md.
interface TileSpec {
  key: keyof TagStats
  labelKey: string
  icon: typeof Tag
}

const TILES: TileSpec[] = [
  { key: "tags_total", labelKey: "tags:stats.tagsTotal", icon: Tag },
  { key: "items_tagged", labelKey: "tags:stats.itemsTagged", icon: Package },
  { key: "items_untagged", labelKey: "tags:stats.itemsUntagged", icon: Hash },
  { key: "files_tagged", labelKey: "tags:stats.filesTagged", icon: File },
  { key: "files_untagged", labelKey: "tags:stats.filesUntagged", icon: FileX },
]

export function TagsStatsBar({ stats, loading, testId }: TagsStatsBarProps) {
  const { t } = useTranslation(["tags"])
  const showPlaceholder = loading && stats === undefined
  return (
    <div
      className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5"
      data-testid={testId ?? "tags-stats"}
    >
      {TILES.map(({ key, labelKey, icon: Icon }) => {
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
