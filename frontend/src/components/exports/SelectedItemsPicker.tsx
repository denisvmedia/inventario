import { useTranslation } from "react-i18next"

import { Skeleton } from "@/components/ui/skeleton"
import type { ExportSelectedItem } from "@/features/export/api"
import { useLocations } from "@/features/locations/hooks"
import { cn } from "@/lib/utils"

export interface SelectedItemsPickerProps {
  value: ExportSelectedItem[]
  onChange: (next: ExportSelectedItem[]) => void
  // Errors mounted by the form-level validation. Renders below the list
  // so screen readers pick it up after the listbox.
  errorMessage?: string
}

// SelectedItemsPicker lets the user pick whole locations to scope the
// export. Picking a location adds a `{ type: "location", id, include_all: true }`
// entry; picking the same location again removes it. The picker is
// intentionally location-only in this first cut — area / commodity
// granularity is tracked as a follow-up to keep this wizard small. The
// BE accepts the same `selected_items` shape regardless of which leaf
// type the user picks, so adding the other entity types later is purely
// additive.
export function SelectedItemsPicker({ value, onChange, errorMessage }: SelectedItemsPickerProps) {
  const { t } = useTranslation(["exports"])
  const locationsQuery = useLocations()
  const locations = locationsQuery.data ?? []
  const pickedIds = new Set(value.map((item) => item.id).filter((id): id is string => !!id))

  function toggle(locationId: string, locationName: string) {
    if (pickedIds.has(locationId)) {
      onChange(value.filter((entry) => entry.id !== locationId))
    } else {
      onChange([
        ...value,
        { type: "location", id: locationId, name: locationName, include_all: true },
      ])
    }
  }

  if (locationsQuery.isLoading) {
    return (
      <div className="flex flex-col gap-2" data-testid="selected-items-picker-loading">
        {Array.from({ length: 3 }).map((_, idx) => (
          <Skeleton key={idx} className="h-10 w-full" />
        ))}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-2" data-testid="selected-items-picker">
      {locations.length === 0 ? (
        <p className="text-sm text-muted-foreground" data-testid="selected-items-picker-empty">
          {t("exports:wizard.scopePicker.empty")}
        </p>
      ) : (
        <ul className="flex flex-col gap-1.5">
          {locations.map((loc) => {
            const id = loc.id ?? ""
            const checked = pickedIds.has(id)
            return (
              <li key={id}>
                <label
                  className={cn(
                    "flex cursor-pointer items-center justify-between gap-3 rounded-md border bg-card px-3 py-2 text-sm",
                    checked && "border-primary/40 bg-primary/5"
                  )}
                  data-testid={`selected-items-picker-row-${id}`}
                >
                  <span className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      className="size-4"
                      checked={checked}
                      onChange={() => toggle(id, loc.name ?? "")}
                      aria-label={loc.name ?? id}
                    />
                    <span className="font-medium">{loc.name ?? id}</span>
                  </span>
                  {loc.address && (
                    <span className="truncate text-xs text-muted-foreground">{loc.address}</span>
                  )}
                </label>
              </li>
            )
          })}
        </ul>
      )}
      {errorMessage && (
        <p
          className="text-sm text-destructive"
          role="alert"
          data-testid="selected-items-picker-error"
        >
          {errorMessage}
        </p>
      )}
    </div>
  )
}
