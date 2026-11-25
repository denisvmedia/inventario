import { Pencil, Trash2 } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import type { ItemType } from "@/features/itemTypes/api"
import { itemTypeLabel } from "@/features/itemTypes/display"
import { ITEM_TYPE_FALLBACK_ICON, ITEM_TYPE_ICON_LOOKUP } from "@/features/itemTypes/icons"
import { cn } from "@/lib/utils"

export interface ItemTypeRowProps {
  itemType: ItemType
  // The items list of every item of the type.
  href?: string
  onEdit: () => void
  onDelete: () => void
  // The group's last type cannot be deleted.
  canDelete: boolean
  className?: string
}

// One row of the Item types page (#2802). The mock has no item types; the
// row follows TagRow (design-mocks TagsView), as the seller row does: the
// type's icon, its name with the item count beneath, and edit and delete
// buttons that fade in on hover.
export function ItemTypeRow({
  itemType,
  href,
  onEdit,
  onDelete,
  canDelete,
  className,
}: ItemTypeRowProps) {
  const { t } = useTranslation()
  const items = itemType.item_count ?? 0
  const trashed = itemType.trashed_item_count ?? 0
  const name = itemTypeLabel(t, itemType)
  const Icon = ITEM_TYPE_ICON_LOOKUP[itemType.icon ?? ""] ?? ITEM_TYPE_FALLBACK_ICON
  const usage = [
    items > 0 ? t("itemTypes:list.usageItems", { count: items }) : t("itemTypes:list.usageNone"),
    trashed > 0 ? t("itemTypes:list.usageTrashed", { count: trashed }) : "",
  ].filter(Boolean)

  return (
    <div
      data-testid={`item-type-row-${itemType.slug}`}
      className={cn("group flex items-center gap-3 px-4 py-3", className)}
    >
      <Icon aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          {href ? (
            <Link
              to={href}
              className="truncate text-sm font-medium hover:underline"
              data-testid={`item-type-row-${itemType.slug}-link`}
            >
              {name}
            </Link>
          ) : (
            <span className="truncate text-sm font-medium">{name}</span>
          )}
          {itemType.builtin ? (
            <Badge variant="secondary" className="shrink-0 text-[10px]">
              {t("itemTypes:list.builtin")}
            </Badge>
          ) : null}
        </div>
        <p
          className="mt-0.5 truncate text-xs text-muted-foreground"
          data-testid={`item-type-row-${itemType.slug}-usage`}
        >
          {usage.join(" · ")}
        </p>
      </div>

      <div className="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
        <Button
          type="button"
          size="icon"
          variant="ghost"
          className="size-7"
          onClick={onEdit}
          aria-label={t("itemTypes:list.edit", { name })}
          data-testid={`item-type-row-${itemType.slug}-edit`}
        >
          <Pencil aria-hidden="true" className="size-3.5" />
        </Button>
        <Button
          type="button"
          size="icon"
          variant="ghost"
          className="size-7 text-destructive hover:bg-destructive/10 hover:text-destructive"
          onClick={onDelete}
          disabled={!canDelete}
          title={canDelete ? undefined : t("itemTypes:list.lastType")}
          aria-label={t("itemTypes:list.delete", { name })}
          data-testid={`item-type-row-${itemType.slug}-delete`}
        >
          <Trash2 aria-hidden="true" className="size-3.5" />
        </Button>
      </div>
    </div>
  )
}
