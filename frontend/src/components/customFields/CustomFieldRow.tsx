import { Pencil, Trash2 } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import type { CustomField } from "@/features/customFields/api"
import { cn } from "@/lib/utils"

export interface CustomFieldRowProps {
  field: CustomField
  // Left out for a member who may not change it; the row then has no
  // edit button. The same holds for onDelete.
  onEdit?: () => void
  onDelete?: () => void
  className?: string
}

// One row of the Custom fields page (#2803). The mock has no custom fields;
// the row follows ItemTypeRow: the name with its type badge, the item
// counts beneath, and edit and delete buttons that fade in on hover.
export function CustomFieldRow({ field, onEdit, onDelete, className }: CustomFieldRowProps) {
  const { t } = useTranslation()
  const items = field.item_count ?? 0
  const trashed = field.trashed_item_count ?? 0
  const usage = [
    field.type === "select" ? t("customFields:list.options", { count: field.options.length }) : "",
    items > 0
      ? t("customFields:list.usageItems", { count: items })
      : t("customFields:list.usageNone"),
    trashed > 0 ? t("customFields:list.usageTrashed", { count: trashed }) : "",
  ].filter(Boolean)

  return (
    <div
      data-testid={`custom-field-row-${field.id}`}
      className={cn("group flex items-center gap-3 px-4 py-3", className)}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-sm font-medium">{field.name}</span>
          <Badge variant="secondary" className="shrink-0 text-[10px]">
            {t(`customFields:types.${field.type}`)}
          </Badge>
        </div>
        <p
          className="mt-0.5 truncate text-xs text-muted-foreground"
          data-testid={`custom-field-row-${field.id}-usage`}
        >
          {usage.join(" · ")}
        </p>
      </div>

      {onEdit || onDelete ? (
        <div className="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
          {onEdit ? (
            <Button
              type="button"
              size="icon"
              variant="ghost"
              className="size-7"
              onClick={onEdit}
              aria-label={t("customFields:list.edit", { name: field.name })}
              data-testid={`custom-field-row-${field.id}-edit`}
            >
              <Pencil aria-hidden="true" className="size-3.5" />
            </Button>
          ) : null}
          {onDelete ? (
            <Button
              type="button"
              size="icon"
              variant="ghost"
              className="size-7 text-destructive hover:bg-destructive/10 hover:text-destructive"
              onClick={onDelete}
              aria-label={t("customFields:list.delete", { name: field.name })}
              data-testid={`custom-field-row-${field.id}-delete`}
            >
              <Trash2 aria-hidden="true" className="size-3.5" />
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
