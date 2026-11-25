import { useId, useRef, useState, type ComponentProps, type KeyboardEvent } from "react"

import { Input } from "@/components/ui/input"
import { useCommodityBrands } from "@/features/commodities/hooks"
import { useDebouncedValue } from "@/hooks/useDebouncedValue"
import { cn } from "@/lib/utils"

type InputProps = Omit<ComponentProps<typeof Input>, "value" | "onChange">

interface BrandInputProps extends InputProps {
  value: string
  onChange: (value: string) => void
}

// BrandInput is a text input that suggests the brands the group's items
// already carry, most used first, once the user has typed something. It
// mirrors the mock's BrandInput (design-mocks AddItemDialog.tsx): the
// suggestions open under the input and a click puts one into it. They come
// from GET /commodities/brands instead of a fixed list. The arrow keys move
// through them and Enter picks one.
export function BrandInput({ value, onChange, className, onBlur, ...inputProps }: BrandInputProps) {
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  const inputRef = useRef<HTMLInputElement>(null)
  const listId = useId()

  const typed = value.trim()
  const query = useDebouncedValue(typed, 200)
  const { data } = useCommodityBrands(query, { enabled: open && query.length > 0 })
  // The value as typed is not a suggestion of itself.
  const suggestions =
    typed.length > 0
      ? (data ?? [])
          .map((b) => b.brand ?? "")
          .filter((b) => b !== "" && b.toLowerCase() !== typed.toLowerCase())
      : []
  const showList = open && suggestions.length > 0

  function pick(brand: string) {
    onChange(brand)
    setOpen(false)
    setActive(-1)
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (!showList) return
    if (e.key === "ArrowDown") {
      e.preventDefault()
      setActive((i) => (i + 1) % suggestions.length)
    } else if (e.key === "ArrowUp") {
      e.preventDefault()
      setActive((i) => (i <= 0 ? suggestions.length - 1 : i - 1))
    } else if (e.key === "Enter" && active >= 0 && active < suggestions.length) {
      // Picking a suggestion must not submit the form around the input.
      e.preventDefault()
      pick(suggestions[active])
    }
  }

  return (
    <div className="relative">
      <Input
        ref={inputRef}
        {...inputProps}
        value={value}
        autoComplete="off"
        role="combobox"
        aria-autocomplete="list"
        aria-expanded={showList}
        aria-controls={showList ? listId : undefined}
        aria-activedescendant={showList && active >= 0 ? `${listId}-${active}` : undefined}
        className={className}
        onChange={(e) => {
          onChange(e.target.value)
          setOpen(true)
          setActive(-1)
        }}
        onFocus={() => setOpen(true)}
        onBlur={(e) => {
          setTimeout(() => setOpen(false), 150)
          onBlur?.(e)
        }}
        onKeyDown={onKeyDown}
      />
      {showList ? (
        <div
          id={listId}
          role="listbox"
          className="absolute top-full z-50 mt-1 w-full overflow-hidden rounded-lg border border-border bg-popover shadow-md"
          data-testid="brand-suggestions"
        >
          {suggestions.map((brand, i) => (
            <button
              key={brand}
              id={`${listId}-${i}`}
              type="button"
              role="option"
              aria-selected={i === active}
              tabIndex={-1}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => {
                pick(brand)
                inputRef.current?.blur()
              }}
              className={cn(
                "w-full px-3 py-2 text-left text-sm transition-colors hover:bg-accent hover:text-accent-foreground",
                i === active && "bg-accent text-accent-foreground"
              )}
            >
              {brand}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  )
}
