import { useTranslation } from "react-i18next"

import { Card, CardContent } from "@/components/ui/card"
import { cn } from "@/lib/utils"

import type { TagStats } from "@/features/tags/api"

export interface TagsStatsBarProps {
  stats?: TagStats
  loading?: boolean
  testId?: string
}

const FIELDS: Array<{ key: keyof TagStats; labelKey: string }> = [
  { key: "tags_total", labelKey: "tags:stats.tagsTotal" },
  { key: "items_tagged", labelKey: "tags:stats.itemsTagged" },
  { key: "items_untagged", labelKey: "tags:stats.itemsUntagged" },
  { key: "files_tagged", labelKey: "tags:stats.filesTagged" },
  { key: "files_untagged", labelKey: "tags:stats.filesUntagged" },
]

export function TagsStatsBar({ stats, loading, testId }: TagsStatsBarProps) {
  const { t } = useTranslation(["tags"])
  return (
    <div
      className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5"
      data-testid={testId ?? "tags-stats"}
    >
      {FIELDS.map(({ key, labelKey }) => {
        const value = stats?.[key]
        return (
          <Card
            key={key}
            className="border-muted"
            data-testid={`tags-stats-${key.replace(/_/g, "-")}`}
          >
            <CardContent className="flex flex-col gap-1 p-4">
              <span className="text-xs uppercase tracking-wide text-muted-foreground">
                {t(labelKey)}
              </span>
              <span
                className={cn(
                  "text-2xl font-semibold tabular-nums",
                  loading && stats === undefined ? "text-muted-foreground" : ""
                )}
              >
                {loading && stats === undefined ? "—" : (value ?? 0)}
              </span>
            </CardContent>
          </Card>
        )
      })}
    </div>
  )
}
