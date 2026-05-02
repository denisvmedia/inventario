import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Check, ChevronsUpDown } from "lucide-react"
import { useTranslation } from "react-i18next"

import { http } from "@/lib/http"
import { cn } from "@/lib/utils"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"

interface CurrencyComboboxProps {
  id?: string
  value: string
  onChange: (next: string) => void
  disabled?: boolean
  placeholder?: string
  ariaInvalid?: boolean
}

// CurrencyCombobox is a Popover + Command searchable combobox for ISO 4217
// codes. Backed by /api/v1/currencies. The trigger button carries
// `role="combobox"` and the requested `id` so e2e specs (and assistive tech)
// can locate it deterministically; each option exposes its 3-letter code via
// `data-currency-code` so tests don't have to text-match country names.
export function CurrencyCombobox({
  id,
  value,
  onChange,
  disabled,
  placeholder,
  ariaInvalid,
}: CurrencyComboboxProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  const currenciesQuery = useQuery<string[]>({
    queryKey: ["currencies"],
    queryFn: ({ signal }) =>
      http.get<string[]>("/currencies", { signal }).then((r) => r.body ?? []),
    staleTime: 60 * 60 * 1000,
  })

  const codes = currenciesQuery.data ?? []
  const selectedCode = (value || "").toUpperCase()
  const label = selectedCode || placeholder || t("groups:create.currencyPlaceholder")

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          id={id}
          type="button"
          role="combobox"
          aria-expanded={open}
          aria-invalid={ariaInvalid || undefined}
          aria-label={t("groups:create.currencyAriaLabel")}
          disabled={disabled}
          className={cn(
            "flex h-9 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-1 text-sm shadow-xs",
            "transition-[color,box-shadow] outline-none placeholder:text-muted-foreground",
            "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50",
            "aria-invalid:border-destructive aria-invalid:ring-destructive/20",
            "disabled:cursor-not-allowed disabled:opacity-50",
            "font-mono uppercase"
          )}
        >
          <span className={cn("truncate", !selectedCode && "text-muted-foreground")}>{label}</span>
          <ChevronsUpDown className="ml-2 size-4 shrink-0 opacity-50" aria-hidden="true" />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-(--radix-popover-trigger-width) p-0" align="start">
        <Command>
          <CommandInput placeholder={t("groups:create.currencySearchPlaceholder")} />
          <CommandList>
            <CommandEmpty>{t("groups:create.currencyNoMatch")}</CommandEmpty>
            <CommandGroup>
              {codes.map((code) => (
                <CommandItem
                  key={code}
                  value={code}
                  data-currency-code={code}
                  onSelect={(selected) => {
                    onChange(selected.toUpperCase())
                    setOpen(false)
                  }}
                >
                  <span className="font-mono">{code}</span>
                  {selectedCode === code ? (
                    <Check className="ml-auto size-4 text-primary" aria-hidden="true" />
                  ) : null}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
