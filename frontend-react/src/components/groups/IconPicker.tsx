import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Check } from "lucide-react"

import { GROUP_ICON_CATEGORIES, GROUP_ICONS } from "@/features/group/icons"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface IconPickerProps {
  value: string
  onChange: (next: string) => void
  // data-testid prefix; "<prefix>-button-<emoji>" / "<prefix>-clear" /
  // "<prefix>-tab-<category>". Defaults to "icon-picker".
  testId?: string
  disabled?: boolean
}

// Inline group-icon picker. Shown as a category tab strip + a grid of
// emoji buttons; clicking sets the form value to the emoji string. The
// curated list lives in features/group/icons.ts (mirrors the BE spec).
//
// We deliberately ship a plain inline grid, not a popover/dialog like
// the legacy Vue picker — the surface fits in the create + settings
// forms without needing a dismissible overlay, and skips a chunk of
// shadcn-popover wiring.
export function IconPicker({ value, onChange, testId = "icon-picker", disabled }: IconPickerProps) {
  const { t } = useTranslation()
  const [tab, setTab] = useState<(typeof GROUP_ICON_CATEGORIES)[number]>(GROUP_ICON_CATEGORIES[0])
  const filtered = GROUP_ICONS.filter((g) => g.category === tab)

  return (
    <div className="rounded-lg border border-border bg-card p-3 space-y-3" data-testid={testId}>
      <div
        className="flex flex-wrap gap-1.5"
        role="tablist"
        aria-label={t("groups:create.iconLabel")}
      >
        {GROUP_ICON_CATEGORIES.map((cat) => (
          <button
            key={cat}
            type="button"
            role="tab"
            aria-selected={tab === cat}
            data-testid={`${testId}-tab-${cat}`}
            onClick={() => setTab(cat)}
            disabled={disabled}
            className={cn(
              "rounded-full px-3 py-1 text-xs font-medium transition-colors capitalize",
              tab === cat
                ? "bg-primary text-primary-foreground"
                : "bg-muted text-muted-foreground hover:bg-muted/70"
            )}
          >
            {cat}
          </button>
        ))}
      </div>
      <div className="grid grid-cols-6 gap-2 sm:grid-cols-8" role="tabpanel">
        {filtered.map(({ emoji, label }) => {
          const selected = value === emoji
          return (
            <button
              key={emoji}
              type="button"
              onClick={() => onChange(emoji)}
              disabled={disabled}
              aria-pressed={selected}
              aria-label={label}
              data-testid={`${testId}-button-${emoji}`}
              className={cn(
                "relative flex aspect-square items-center justify-center rounded-lg border-2 text-xl transition-all",
                selected
                  ? "border-primary bg-primary/5"
                  : "border-transparent hover:border-muted-foreground/40"
              )}
            >
              <span aria-hidden="true">{emoji}</span>
              {selected ? (
                <span className="absolute -bottom-1 -right-1 flex size-4 items-center justify-center rounded-full bg-primary">
                  <Check className="size-2.5 text-primary-foreground" aria-hidden="true" />
                </span>
              ) : null}
            </button>
          )
        })}
      </div>
      {value ? (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={() => onChange("")}
          disabled={disabled}
          data-testid={`${testId}-clear`}
        >
          {t("groups:create.iconNone")}
        </Button>
      ) : null}
    </div>
  )
}
