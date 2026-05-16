import { Package, Pencil, Trash2 } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

import { TAG_DOT_TONE } from "./TagBadge"
import type { TagColor, TagEntity, TagUsage } from "@/features/tags/api"

// Visual contract: matches `design-mocks/src/views/TagsView.tsx` row
// shape — leading color dot, `# label` with usage count beneath, an
// inline strip of up to two item-preview chips with a `+N` overflow,
// and edit/delete affordances that only fade in on row hover.
export interface TagRowPreviewItem {
  id: string
  name: string
}

export interface TagRowProps {
  tag: TagEntity & { id: string }
  usage?: TagUsage
  previewItems?: TagRowPreviewItem[]
  onEdit: () => void
  onDelete: () => void
  className?: string
}

const PREVIEW_LIMIT = 2

export function TagRow({
  tag,
  usage,
  previewItems = [],
  onEdit,
  onDelete,
  className,
}: TagRowProps) {
  const { t } = useTranslation(["tags"])
  const items = usage?.commodities ?? 0
  const files = usage?.files ?? 0
  const color = (tag.color ?? "muted") as TagColor
  const label = tag.label ?? tag.slug ?? ""
  const head = previewItems.slice(0, PREVIEW_LIMIT)
  // Overflow counts the *unseen* tagged items beyond the resolved chips.
  // We only have item chips (not file chips), so the figure stays tied
  // to `usage.commodities`. When commodities are zero but files aren't,
  // we don't render a chip strip at all (`head.length === 0`).
  const overflow = Math.max(0, items - head.length)
  // Usage line mirrors the in-use check the delete-confirm dialog uses
  // (`items > 0 || files > 0`) so a tag attached only to files reads as
  // in-use instead of the misleading "Not used yet" the items-only
  // count would produce.
  const usageSegments: string[] = []
  if (items > 0) usageSegments.push(t("tags:list.usageItems", { count: items }))
  if (files > 0) usageSegments.push(t("tags:list.usageFiles", { count: files }))

  return (
    <div
      data-testid={`tag-row-${tag.slug}`}
      className={cn("group flex items-center gap-3 px-4 py-3", className)}
    >
      <div
        className={cn("size-2.5 rounded-full shrink-0", TAG_DOT_TONE[color])}
        data-testid={`tag-row-${tag.slug}-dot`}
        aria-hidden="true"
      />

      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 min-w-0">
          <span className="text-sm font-medium truncate">
            <span aria-hidden="true" className="text-muted-foreground">
              #{" "}
            </span>
            {label}
          </span>
        </div>
        <p
          className="text-xs text-muted-foreground mt-0.5"
          data-testid={`tag-row-${tag.slug}-usage`}
        >
          {usageSegments.length === 0 ? t("tags:list.usageNone") : usageSegments.join(" · ")}
        </p>
      </div>

      {head.length > 0 ? (
        <div
          className="hidden sm:flex items-center gap-1 flex-wrap max-w-48"
          data-testid={`tag-row-${tag.slug}-preview`}
        >
          {head.map((item) => (
            <span
              key={item.id}
              className="inline-flex items-center gap-1 rounded-md bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground"
            >
              <Package aria-hidden="true" className="size-2.5 shrink-0" />
              <span className="truncate max-w-20">{item.name}</span>
            </span>
          ))}
          {overflow > 0 ? (
            <span className="text-[10px] text-muted-foreground">+{overflow}</span>
          ) : null}
        </div>
      ) : null}

      <div className="flex gap-0.5 opacity-0 group-hover:opacity-100 focus-within:opacity-100 transition-opacity shrink-0">
        <Button
          type="button"
          size="icon"
          variant="ghost"
          className="size-7"
          onClick={onEdit}
          aria-label={t("tags:list.edit")}
          data-testid={`tag-row-${tag.slug}-edit`}
        >
          <Pencil aria-hidden="true" className="size-3.5" />
        </Button>
        <Button
          type="button"
          size="icon"
          variant="ghost"
          className="size-7 text-destructive hover:bg-destructive/10 hover:text-destructive"
          onClick={onDelete}
          aria-label={t("tags:list.delete")}
          data-testid={`tag-row-${tag.slug}-delete`}
        >
          <Trash2 aria-hidden="true" className="size-3.5" />
        </Button>
      </div>
    </div>
  )
}
