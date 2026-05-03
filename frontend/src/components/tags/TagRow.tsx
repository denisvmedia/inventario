import { Pencil, Trash2 } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

import { TagBadge } from "./TagBadge"
import type { TagColor, TagEntity, TagUsage } from "@/features/tags/api"

export interface TagRowProps {
  tag: TagEntity & { id: string }
  usage?: TagUsage
  onEdit: () => void
  onDelete: () => void
  className?: string
}

export function TagRow({ tag, usage, onEdit, onDelete, className }: TagRowProps) {
  const { t } = useTranslation(["tags"])
  const items = usage?.commodities ?? 0
  const files = usage?.files ?? 0
  const usageNone = items === 0 && files === 0

  return (
    <div
      data-testid={`tag-row-${tag.slug}`}
      className={cn(
        "flex flex-wrap items-center justify-between gap-3 rounded-md border bg-card px-4 py-3",
        className
      )}
    >
      <div className="flex min-w-0 items-center gap-3">
        <TagBadge label={tag.label ?? tag.slug ?? ""} color={(tag.color ?? "muted") as TagColor} />
        <span className="truncate text-xs text-muted-foreground">{tag.slug}</span>
      </div>

      <div
        className="flex items-center gap-3 text-xs text-muted-foreground"
        data-testid={`tag-row-${tag.slug}-usage`}
      >
        {usageNone ? (
          <span>{t("tags:list.usageNone")}</span>
        ) : (
          <>
            <span>{t("tags:list.usageItems", { count: items })}</span>
            <span aria-hidden="true">·</span>
            <span>{t("tags:list.usageFiles", { count: files })}</span>
          </>
        )}
      </div>

      <div className="flex items-center gap-1.5">
        <Button
          type="button"
          size="sm"
          variant="ghost"
          onClick={onEdit}
          aria-label={t("tags:list.edit")}
          data-testid={`tag-row-${tag.slug}-edit`}
        >
          <Pencil className="size-4" aria-hidden="true" />
        </Button>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          onClick={onDelete}
          aria-label={t("tags:list.delete")}
          data-testid={`tag-row-${tag.slug}-delete`}
        >
          <Trash2 className="size-4" aria-hidden="true" />
        </Button>
      </div>
    </div>
  )
}
