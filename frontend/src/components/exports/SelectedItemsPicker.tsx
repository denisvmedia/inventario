import { Search } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"

import { Input } from "@/components/ui/input"
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
  const locations = useMemo(() => locationsQuery.data ?? [], [locationsQuery.data])
  const pickedIds = useMemo(
    () => new Set(value.map((item) => item.id).filter((id): id is string => !!id)),
    [value]
  )

  const [query, setQuery] = useState("")

  // Already-picked rows stay visible even when they don't match the
  // query, so the user doesn't lose track of what they have selected
  // while narrowing the list.
  const visibleLocations = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return locations
    return locations.filter((loc) => {
      const id = loc.id ?? ""
      if (pickedIds.has(id)) return true
      if ((loc.name ?? "").toLowerCase().includes(q)) return true
      if ((loc.address ?? "").toLowerCase().includes(q)) return true
      return false
    })
  }, [locations, pickedIds, query])

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

  // Distinguish a load failure from an actually-empty list: react-query
  // returns `data: undefined` when `isError` is true, which would
  // otherwise fall through to the same empty-state copy.
  if (locationsQuery.isError) {
    return (
      <div className="flex flex-col gap-2" data-testid="selected-items-picker">
        <p
          className="text-sm text-destructive"
          role="alert"
          data-testid="selected-items-picker-load-error"
        >
          {t("exports:wizard.scopePicker.loadError")}
        </p>
      </div>
    )
  }

  const hasLocations = locations.length > 0
  const trimmedQuery = query.trim()
  // Three explicit conditions, so the rule reads the same way the issue
  // describes it: search-empty fires only when the user has typed
  // something AND there are no picks AND no rows match. Without the
  // `pickedIds.size === 0` gate, a picked id that's no longer present in
  // `locations` (e.g., the location was deleted server-side) would
  // wrongly trigger the "no matches" copy even though the user's
  // selection is non-empty.
  const showSearchEmpty =
    hasLocations && trimmedQuery.length > 0 && pickedIds.size === 0 && visibleLocations.length === 0

  return (
    <div className="flex flex-col gap-2" data-testid="selected-items-picker">
      {!hasLocations ? (
        <p className="text-sm text-muted-foreground" data-testid="selected-items-picker-empty">
          {t("exports:wizard.scopePicker.empty")}
        </p>
      ) : (
        <>
          <div className="relative">
            <Search
              className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground"
              aria-hidden="true"
            />
            <Input
              type="search"
              placeholder={t("exports:wizard.scopePicker.searchPlaceholder")}
              aria-label={t("exports:wizard.scopePicker.searchPlaceholder")}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="pl-9"
              data-testid="selected-items-picker-search"
            />
          </div>
          {showSearchEmpty ? (
            <p
              className="text-sm text-muted-foreground"
              data-testid="selected-items-picker-search-empty"
            >
              {t("exports:wizard.scopePicker.searchEmpty", { query: trimmedQuery })}
            </p>
          ) : (
            <ul className="flex flex-col gap-1.5">
              {visibleLocations.map((loc) => {
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
                        <span className="truncate text-xs text-muted-foreground">
                          {loc.address}
                        </span>
                      )}
                    </label>
                  </li>
                )
              })}
            </ul>
          )}
        </>
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
