import { RadioGroup as RadioGroupPrimitive } from "radix-ui"
import { useTranslation } from "react-i18next"

import { ITEM_TYPE_ICONS, ITEM_TYPE_ICON_KEYS } from "@/features/itemTypes/icons"
import { cn } from "@/lib/utils"

export interface ItemTypeIconPickerProps {
  // The selected icon key. A key the picker does not offer selects nothing.
  value: string
  onChange: (key: string) => void
  // The id of the element that labels the group.
  labelledBy: string
  disabled?: boolean
}

// ItemTypeIconPicker picks the icon of an item type (#2802) from a grid of
// tiles. It is a Radix radio group, so the arrow keys move between the
// tiles and the group takes one tab stop. The mock has no icon picker; the
// tiles use the outline button and focus ring tokens of UIShowcaseView.
export function ItemTypeIconPicker({
  value,
  onChange,
  labelledBy,
  disabled = false,
}: ItemTypeIconPickerProps) {
  const { t } = useTranslation()
  return (
    <RadioGroupPrimitive.Root
      value={value}
      onValueChange={onChange}
      aria-labelledby={labelledBy}
      disabled={disabled}
      className="grid grid-cols-6 gap-1.5 sm:grid-cols-8"
      data-testid="item-type-icon-picker"
    >
      {ITEM_TYPE_ICON_KEYS.map((key) => {
        const Icon = ITEM_TYPE_ICONS[key]
        const label = t(`itemTypes:icons.${key}`)
        return (
          <RadioGroupPrimitive.Item
            key={key}
            value={key}
            aria-label={label}
            title={label}
            className={cn(
              "flex h-9 items-center justify-center rounded-md border border-border text-muted-foreground outline-none transition-colors",
              "hover:bg-accent hover:text-foreground",
              "data-[state=checked]:border-primary data-[state=checked]:bg-primary/10 data-[state=checked]:text-foreground",
              "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50",
              "disabled:cursor-not-allowed disabled:opacity-50"
            )}
            data-testid={`item-type-icon-${key}`}
          >
            <Icon aria-hidden="true" className="size-4" />
          </RadioGroupPrimitive.Item>
        )
      })}
    </RadioGroupPrimitive.Root>
  )
}
