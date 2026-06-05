import * as React from "react"

import { Checkbox } from "@/components/ui/checkbox"
import { cn } from "@/lib/utils"

interface BulkActionBarSelectAll {
  checked: boolean
  onCheckedChange: () => void
  label: string
  "data-testid"?: string
}

interface BulkActionBarProps extends React.ComponentProps<"div"> {
  // Already-pluralized count label, e.g. "3 selected" / "3 items selected".
  label: React.ReactNode
  // Landmark aria-label. Defaults to `label` when that is a string.
  regionLabel?: string
  // Optional select-all-on-page checkbox shown left of the label.
  selectAll?: BulkActionBarSelectAll
  // Action controls (move / delete / …) rendered on the right.
  children: React.ReactNode
}

// Shared multi-select bulk-action bar. A fixed bottom-centre overlay on
// the `popover` token so toggling the first checkbox never reflows the
// list (no "jolt") and the bar stays visible across scroll. Slides in via
// `tw-animate-css`. This is the canonical mass-action shell for every
// list surface (Items, Files, …) — keeping it in one place is what stops
// the per-page bars from drifting apart again. It is an intentional
// `mock < reality` divergence (no BulkBar in the design mock) logged in
// devdocs/frontend/design-deviations.md.
export function BulkActionBar({
  label,
  regionLabel,
  selectAll,
  children,
  className,
  ...props
}: BulkActionBarProps) {
  return (
    <div
      role="region"
      aria-label={regionLabel ?? (typeof label === "string" ? label : undefined)}
      className={cn(
        "fixed bottom-6 left-1/2 z-40 w-[calc(100vw-2rem)] max-w-xl -translate-x-1/2",
        "flex flex-wrap items-center justify-between gap-3 rounded-xl border bg-popover px-4 py-2.5 text-sm text-popover-foreground",
        "animate-in slide-in-from-bottom-4 fade-in-0 duration-200",
        className
      )}
      {...props}
    >
      <div className="flex items-center gap-3">
        {selectAll ? (
          <Checkbox
            checked={selectAll.checked}
            onCheckedChange={selectAll.onCheckedChange}
            aria-label={selectAll.label}
            data-testid={selectAll["data-testid"]}
          />
        ) : null}
        <span>{label}</span>
      </div>
      <div className="flex items-center gap-2">{children}</div>
    </div>
  )
}
