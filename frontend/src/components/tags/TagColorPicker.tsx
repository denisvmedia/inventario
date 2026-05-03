import { Check } from "lucide-react"
import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"

import { TAG_COLORS, type TagColor } from "@/features/tags/api"

const SWATCH: Record<TagColor, string> = {
  amber: "bg-tag-amber",
  green: "bg-tag-green",
  blue: "bg-tag-blue",
  orange: "bg-tag-orange",
  red: "bg-tag-red",
  muted: "bg-tag-muted",
}

export interface TagColorPickerProps {
  value: TagColor
  onChange: (next: TagColor) => void
  testId?: string
  disabled?: boolean
}

export function TagColorPicker({ value, onChange, testId, disabled }: TagColorPickerProps) {
  const { t } = useTranslation(["tags"])
  return (
    <div
      role="radiogroup"
      aria-label={t("tags:color.selectLabel")}
      className="flex flex-wrap items-center gap-2"
      data-testid={testId}
    >
      {TAG_COLORS.map((color) => {
        const selected = value === color
        return (
          <button
            key={color}
            type="button"
            role="radio"
            aria-checked={selected}
            aria-label={t(`tags:color.${color}`)}
            disabled={disabled}
            onClick={() => onChange(color)}
            data-testid={testId ? `${testId}-${color}` : undefined}
            className={cn(
              "relative size-7 rounded-full ring-1 ring-border transition",
              SWATCH[color],
              selected ? "ring-2 ring-foreground ring-offset-2 ring-offset-background" : "",
              disabled ? "cursor-not-allowed opacity-50" : "cursor-pointer hover:scale-110"
            )}
          >
            {selected ? (
              <Check
                aria-hidden="true"
                className="absolute inset-0 m-auto size-4 text-background drop-shadow"
              />
            ) : null}
          </button>
        )
      })}
    </div>
  )
}
