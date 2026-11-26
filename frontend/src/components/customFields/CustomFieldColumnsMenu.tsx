import { useTranslation } from "react-i18next"
import { ChevronDown, Columns3 } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { CustomField } from "@/features/customFields/api"
import { CUSTOM_FIELD_COLUMNS_MAX } from "@/features/customFields/columns"

export interface CustomFieldColumnsMenuProps {
  fields: readonly CustomField[]
  // The ids of the fields shown as columns, in column order.
  columns: readonly string[]
  onChange: (columns: string[]) => void
}

// CustomFieldColumnsMenu picks the custom fields the items list shows as
// columns in its list layout (#2803), up to CUSTOM_FIELD_COLUMNS_MAX.
export function CustomFieldColumnsMenu({ fields, columns, onChange }: CustomFieldColumnsMenuProps) {
  const { t } = useTranslation()
  const shown = columns.filter((id) => fields.some((f) => f.id === id))
  const full = shown.length >= CUSTOM_FIELD_COLUMNS_MAX
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="gap-1.5"
          data-testid="commodities-custom-field-columns"
        >
          <Columns3 className="size-3.5" aria-hidden="true" />
          {t("customFields:columns.button")}
          {shown.length > 0 ? (
            <Badge variant="secondary" className="ml-0.5 h-4 px-1 text-xs">
              {shown.length}
            </Badge>
          ) : null}
          <ChevronDown className="size-3.5" aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="max-h-80 w-56 overflow-y-auto">
        <DropdownMenuLabel>{t("customFields:columns.title")}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {fields.map((f) => {
          const checked = shown.includes(f.id)
          return (
            <DropdownMenuCheckboxItem
              key={f.id}
              checked={checked}
              disabled={!checked && full}
              // Keep the menu open to pick several columns.
              onSelect={(e) => e.preventDefault()}
              onCheckedChange={() =>
                onChange(checked ? shown.filter((id) => id !== f.id) : [...shown, f.id])
              }
              data-testid={`commodities-custom-field-column-${f.id}`}
            >
              <span className="truncate">{f.name}</span>
            </DropdownMenuCheckboxItem>
          )
        })}
        {full ? (
          <p className="px-2 py-1.5 text-xs text-muted-foreground">
            {t("customFields:columns.full", { max: CUSTOM_FIELD_COLUMNS_MAX })}
          </p>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
