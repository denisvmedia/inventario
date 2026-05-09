import { useId, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Check, ChevronsUpDown } from "lucide-react"
import { useTranslation } from "react-i18next"

import { http } from "@/lib/http"
import { cn } from "@/lib/utils"
import { currencyMeta } from "@/lib/currency-meta"
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
  // "default" stretches `w-full` (settings forms / standalone fields).
  // "compact" snaps to a fixed code-width — used inline with a price
  // input where the price should take the rest of the row.
  variant?: "default" | "compact"
}

// CurrencyCombobox is a Popover + Command searchable combobox for ISO 4217
// codes. Backed by /api/v1/currencies. Each option displays
// `<symbol>  <code>  —  <name>` (mock design-mocks/src/data/mock.ts
// L511-L542); the backend returns codes only, the symbol + name pair
// is mirrored from `lib/currency-meta.ts`. The trigger button carries
// `role="combobox"` and the requested `id` so e2e specs (and assistive
// tech) can locate it deterministically; each option exposes its
// 3-letter code via `data-currency-code` so tests don't have to
// text-match country names.
export function CurrencyCombobox({
  id,
  value,
  onChange,
  disabled,
  placeholder,
  ariaInvalid,
  variant = "default",
}: CurrencyComboboxProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  // jsx-a11y/role-has-required-aria-props requires aria-controls on
  // role="combobox" — point it at the listbox the popover renders.
  const listboxId = useId()

  const currenciesQuery = useQuery<string[]>({
    queryKey: ["currencies"],
    queryFn: ({ signal }) => http.get<string[]>("/currencies", { signal }),
    staleTime: 60 * 60 * 1000,
  })

  const codes = currenciesQuery.data ?? []
  const selectedCode = (value || "").toUpperCase()
  const label = selectedCode || placeholder || t("groups:create.currencyPlaceholder")
  const isCompact = variant === "compact"

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          id={id}
          type="button"
          role="combobox"
          aria-controls={listboxId}
          aria-expanded={open}
          aria-invalid={ariaInvalid || undefined}
          aria-label={t("groups:create.currencyAriaLabel")}
          disabled={disabled}
          className={cn(
            "flex h-9 items-center justify-between rounded-md border border-input bg-background px-3 py-1 text-sm shadow-xs",
            "transition-[color,box-shadow] outline-none placeholder:text-muted-foreground",
            "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50",
            "aria-invalid:border-destructive aria-invalid:ring-destructive/20",
            "disabled:cursor-not-allowed disabled:opacity-50",
            "font-mono uppercase",
            isCompact ? "w-24 shrink-0" : "w-full"
          )}
        >
          <span className={cn("truncate", !selectedCode && "text-muted-foreground")}>{label}</span>
          <ChevronsUpDown className="ml-2 size-4 shrink-0 opacity-50" aria-hidden="true" />
        </button>
      </PopoverTrigger>
      <PopoverContent
        className={cn("p-0", isCompact ? "w-72" : "w-(--radix-popover-trigger-width)")}
        align="start"
      >
        <Command>
          <CommandInput placeholder={t("groups:create.currencySearchPlaceholder")} />
          <CommandList id={listboxId}>
            <CommandEmpty>{t("groups:create.currencyNoMatch")}</CommandEmpty>
            <CommandGroup>
              {codes.map((code) => {
                const meta = currencyMeta(code)
                // Pass code+name to cmdk's filter so search matches both
                // ("USD" or "Dollar"). data-currency-code stays for tests.
                const filterValue = `${meta.code} ${meta.name}`
                return (
                  <CommandItem
                    key={code}
                    value={filterValue}
                    data-currency-code={code}
                    onSelect={() => {
                      onChange(meta.code)
                      setOpen(false)
                    }}
                    className="gap-3"
                  >
                    <span
                      aria-hidden="true"
                      className="w-8 shrink-0 text-center font-medium text-muted-foreground"
                    >
                      {meta.symbol}
                    </span>
                    <span className="font-mono font-semibold">{meta.code}</span>
                    <span className="truncate text-muted-foreground">— {meta.name}</span>
                    {selectedCode === meta.code ? (
                      <Check className="ml-auto size-4 text-primary" aria-hidden="true" />
                    ) : null}
                  </CommandItem>
                )
              })}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
