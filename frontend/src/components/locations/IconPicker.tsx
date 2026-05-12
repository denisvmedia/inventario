import { Label } from "@/components/ui/label"
import { cn } from "@/lib/utils"

interface IconPickerProps {
  // Currently selected glyph. Empty string means "no icon picked" — no
  // tile is highlighted in that branch and the consumer is expected to
  // fall back to a generic Lucide icon at render time.
  value: string
  onChange: (value: string) => void
  // The palette to render. Co-located with the consuming dialog so the
  // locations / areas pickers can ship distinct emoji sets per the mock.
  icons: readonly string[]
  // Pre-translated label rendered above the grid + used as the
  // radiogroup aria-label. Callers translate at the call site so the
  // i18n key extractor sees a literal string instead of a prop value.
  label: string
  // Test id prefix used for the field label + the per-icon buttons —
  // each button gets `${testIdPrefix}-${index}` so e2e specs can pin
  // selection deterministically.
  testIdPrefix?: string
  disabled?: boolean
}

// IconPicker mirrors the mock dialogs' inline icon grid (see
// `design-mocks/src/components/LocationDialog.tsx` L102-L122 and
// `AreaDialog.tsx` L62-L82): a `flex-wrap` row of `size-9` `rounded-lg`
// emoji buttons with a `bg-primary/10` + `scale-110` selected state and
// a `bg-muted` resting state. The wider gap rhythm is matched
// (`gap-1.5`) so the dialogs visually match the mock 1:1.
export function IconPicker({
  value,
  onChange,
  icons,
  label,
  testIdPrefix = "icon-picker",
  disabled = false,
}: IconPickerProps) {
  return (
    <div className="flex flex-col gap-2">
      <Label data-testid={`${testIdPrefix}-label`}>{label}</Label>
      <div
        className="flex flex-wrap gap-1.5"
        role="radiogroup"
        aria-label={label}
        data-testid={testIdPrefix}
      >
        {icons.map((ic, idx) => {
          const selected = ic === value
          return (
            <button
              key={ic}
              type="button"
              role="radio"
              aria-checked={selected}
              aria-label={ic}
              onClick={() => onChange(ic)}
              disabled={disabled}
              className={cn(
                "flex size-9 items-center justify-center rounded-lg border text-xl transition-all",
                selected
                  ? "border-primary bg-primary/10 scale-110"
                  : "border-border bg-muted hover:border-primary/40",
                disabled && "cursor-not-allowed opacity-50"
              )}
              data-testid={`${testIdPrefix}-${idx}`}
            >
              <span aria-hidden="true">{ic}</span>
            </button>
          )
        })}
      </div>
    </div>
  )
}

// Default palette for Location.icon — ports `LOCATION_ICONS` from
// `design-mocks/src/components/LocationDialog.tsx` L19 1:1.
export const LOCATION_ICONS = [
  "🏠",
  "🏡",
  "🏢",
  "🏗️",
  "🏚️",
  "🚗",
  "🌿",
  "🌊",
  "⛺",
  "🏕️",
  "🔑",
  "📦",
] as const

// Default palette for Area.icon — ports `AREA_ICONS` from
// `design-mocks/src/components/AreaDialog.tsx` L16-L19 1:1.
export const AREA_ICONS = [
  "🍳",
  "🛋️",
  "💼",
  "🧺",
  "🪣",
  "🛏️",
  "🚿",
  "🎮",
  "📚",
  "🔧",
  "🪴",
  "🍷",
  "🎨",
  "🏋️",
  "🧹",
  "📦",
  "🚗",
  "🌿",
  "🔑",
  "⚡",
] as const
