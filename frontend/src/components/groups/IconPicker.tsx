import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Check, X } from "lucide-react"

import { GROUP_ICON_CATEGORIES, GROUP_ICONS } from "@/features/group/icons"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { cn } from "@/lib/utils"

interface IconPickerProps {
  value: string
  onChange: (next: string) => void
  // data-testid for the trigger button (e.g. "group-create-icon-picker"). The
  // popover panel itself uses the fixed `icon-picker-*` testid family
  // (`icon-picker-panel`, `icon-picker-tab-<category>`,
  // `icon-picker-option-<emoji>`, `icon-picker-clear`, `icon-picker-close`)
  // so e2e selectors don't repeat the call-site's prefix.
  testId?: string
  disabled?: boolean
}

// Group-icon picker rendered as a Popover. Trigger button shows the current
// emoji (or a placeholder); clicking opens a panel with category tabs + an
// emoji grid + a close button. The picker mirrors the curated list from
// features/group/icons.ts.
export function IconPicker({ value, onChange, testId = "icon-picker", disabled }: IconPickerProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [tab, setTab] = useState<(typeof GROUP_ICON_CATEGORIES)[number]>(GROUP_ICON_CATEGORIES[0])
  const filtered = GROUP_ICONS.filter((g) => g.category === tab)

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={t("groups:create.iconLabel")}
          aria-expanded={open}
          data-testid={testId}
          disabled={disabled}
          className={cn(
            "flex h-12 w-full items-center justify-between rounded-md border border-input bg-background px-3 text-sm shadow-xs",
            "transition-[color,box-shadow] outline-none",
            "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50",
            "disabled:cursor-not-allowed disabled:opacity-50"
          )}
        >
          <span className="flex items-center gap-2">
            {value ? (
              <span className="text-2xl leading-none" aria-hidden="true">
                {value}
              </span>
            ) : (
              <span className="text-muted-foreground">{t("groups:create.iconNone")}</span>
            )}
          </span>
          <span className="text-xs text-muted-foreground">
            {value ? t("groups:create.iconChange") : t("groups:create.iconPick")}
          </span>
        </button>
      </PopoverTrigger>
      <PopoverContent
        data-testid="icon-picker-panel"
        className="w-[min(22rem,calc(100vw-2rem))] p-3 space-y-3"
        align="start"
      >
        <div className="flex items-center justify-between gap-2">
          <p className="text-sm font-medium">{t("groups:create.iconLabel")}</p>
          <button
            type="button"
            data-testid="icon-picker-close"
            aria-label={t("common:actions.close")}
            onClick={() => setOpen(false)}
            className="rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <X className="size-4" aria-hidden="true" />
          </button>
        </div>
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
              data-testid={`icon-picker-tab-${cat}`}
              onClick={() => setTab(cat)}
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
        <div className="grid grid-cols-6 gap-2" role="tabpanel">
          {filtered.map(({ emoji, label }) => {
            const selected = value === emoji
            return (
              <button
                key={emoji}
                type="button"
                onClick={() => {
                  onChange(emoji)
                  setOpen(false)
                }}
                aria-pressed={selected}
                aria-label={label}
                data-testid={`icon-picker-option-${emoji}`}
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
            data-testid="icon-picker-clear"
            onClick={() => onChange("")}
          >
            {t("groups:create.iconNone")}
          </Button>
        ) : null}
      </PopoverContent>
    </Popover>
  )
}
