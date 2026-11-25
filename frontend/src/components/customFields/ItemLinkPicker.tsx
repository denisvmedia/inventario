import { useId, useState } from "react"
import { useTranslation } from "react-i18next"
import { ChevronsUpDown } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { useCommodities } from "@/features/commodities/hooks"
import { useDebouncedValue } from "@/hooks/useDebouncedValue"
import { cn } from "@/lib/utils"

export interface LinkedItem {
  name?: string
  trashed?: boolean
}

export interface ItemLinkPickerProps {
  id?: string
  // The linked item's id, "" for none.
  value: string
  onChange: (id: string) => void
  // Names of items the value may point at, from the item response's
  // meta.linked_items, so a stored link shows its name before any search.
  linkedItems?: Record<string, LinkedItem>
  // The item being edited, which cannot link to itself.
  selfId?: string
  disabled?: boolean
  ariaInvalid?: boolean
  ariaLabel?: string
}

// ItemLinkPicker picks another item of the group for a "link to another
// item" custom field (#2803). It follows CurrencyCombobox: a Popover with
// a Command list, here searching the group's live items through the items
// endpoint as the member types.
export function ItemLinkPicker({
  id,
  value,
  onChange,
  linkedItems,
  selfId,
  disabled,
  ariaInvalid,
  ariaLabel,
}: ItemLinkPickerProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")
  const debounced = useDebouncedValue(search.trim())
  const listboxId = useId()
  // Names picked in this form, for a value the response did not name.
  const [picked, setPicked] = useState<Record<string, string>>({})

  const results = useCommodities({ search: debounced || undefined, perPage: 20 }, { enabled: open })
  const options = (results.data?.commodities ?? []).filter((c) => c.id && c.id !== selfId)

  const known = value ? linkedItems?.[value] : undefined
  const label = !value
    ? t("customFields:section.itemPlaceholder")
    : picked[value]
      ? picked[value]
      : known?.name
        ? known.trashed
          ? t("customFields:section.itemTrashed", { name: known.name })
          : known.name
        : t("customFields:section.itemUnknown")

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          id={id}
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          aria-controls={listboxId}
          aria-invalid={ariaInvalid}
          aria-label={ariaLabel}
          disabled={disabled}
          className={cn("w-full justify-between font-normal", !value && "text-muted-foreground")}
        >
          <span className="truncate">{label}</span>
          <ChevronsUpDown aria-hidden="true" className="ml-2 size-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-(--radix-popover-trigger-width) p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput
            value={search}
            onValueChange={setSearch}
            placeholder={t("customFields:section.itemSearch")}
          />
          <CommandList id={listboxId}>
            <CommandEmpty>{t("customFields:section.itemNone")}</CommandEmpty>
            <CommandGroup>
              {options.map((c) => (
                <CommandItem
                  key={c.id}
                  value={c.id}
                  onSelect={() => {
                    setPicked((prev) => ({ ...prev, [c.id as string]: c.name ?? "" }))
                    onChange(c.id as string)
                    setOpen(false)
                  }}
                  data-testid="custom-field-item-option"
                >
                  <span className="truncate">{c.name}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
