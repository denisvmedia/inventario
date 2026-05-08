import * as React from "react"
import { Check, ChevronDown } from "lucide-react"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Command, CommandEmpty, CommandInput, CommandItem, CommandList } from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"

// ── Context ───────────────────────────────────────────────────────────────────

interface ComboboxContextValue {
  open: boolean
  setOpen: (open: boolean) => void
  value: string | null
  onValueChange: (value: string | null) => void
  items: string[]
  placeholder?: string
}

const ComboboxContext = React.createContext<ComboboxContextValue | null>(null)

function useCombobox() {
  const ctx = React.useContext(ComboboxContext)
  if (!ctx) throw new Error("Combobox components must be used within <Combobox>")
  return ctx
}

// ── Root ──────────────────────────────────────────────────────────────────────

interface ComboboxProps {
  items: string[]
  value: string | null
  onValueChange: (value: string | null) => void
  children: React.ReactNode
}

function Combobox({ items, value, onValueChange, children }: ComboboxProps) {
  const [open, setOpen] = React.useState(false)

  return (
    <ComboboxContext.Provider value={{ open, setOpen, value, onValueChange, items }}>
      <Popover open={open} onOpenChange={setOpen}>
        {children}
      </Popover>
    </ComboboxContext.Provider>
  )
}

// ── Input (trigger) ───────────────────────────────────────────────────────────

interface ComboboxInputProps {
  placeholder?: string
  className?: string
  showClear?: boolean
}

function ComboboxInput({ placeholder = "Select…", className, showClear: _showClear }: ComboboxInputProps) {
  const { value, open } = useCombobox()

  return (
    <PopoverTrigger asChild>
      <Button
        variant="outline"
        role="combobox"
        aria-expanded={open}
        data-slot="combobox-input"
        className={cn(
          "w-full justify-between font-normal text-sm",
          !value && "text-muted-foreground",
          className,
        )}
      >
        <span className="truncate">{value ?? placeholder}</span>
        <ChevronDown className="ml-2 size-4 shrink-0 text-muted-foreground" />
      </Button>
    </PopoverTrigger>
  )
}

// ── Content ───────────────────────────────────────────────────────────────────

interface ComboboxContentProps {
  children: React.ReactNode
  className?: string
}

function ComboboxContent({ children, className, searchPlaceholder }: ComboboxContentProps & { searchPlaceholder?: string }) {
  return (
    <PopoverContent
      align="start"
      sideOffset={6}
      className={cn("w-(--radix-popover-trigger-width) p-0", className)}
      data-slot="combobox-content"
    >
      <Command>
        <CommandInput placeholder={searchPlaceholder ?? "Search…"} />
        {children}
      </Command>
    </PopoverContent>
  )
}

// ── Empty ─────────────────────────────────────────────────────────────────────

function ComboboxEmpty({ children }: { children?: React.ReactNode }) {
  return <CommandEmpty>{children ?? "No results found."}</CommandEmpty>
}

// ── List ──────────────────────────────────────────────────────────────────────

interface ComboboxListProps {
  children: ((item: string) => React.ReactNode) | React.ReactNode
  className?: string
}

function ComboboxList({ children, className }: ComboboxListProps) {
  const { items, value, onValueChange, setOpen } = useCombobox()

  // Support both render-prop (item) => ... and static children
  if (typeof children === "function") {
    return (
      <CommandList className={className}>
        {items.map((item) => {
          const child = (children as (item: string) => React.ReactNode)(item)
          if (!React.isValidElement(child)) return null
          return React.cloneElement(child as React.ReactElement<ComboboxItemProps>, {
            onSelect: (v: string) => {
              onValueChange(v === value ? null : v)
              setOpen(false)
            },
            selected: item === value,
          })
        })}
      </CommandList>
    )
  }

  return <CommandList className={className}>{children}</CommandList>
}

// ── Item ──────────────────────────────────────────────────────────────────────

interface ComboboxItemProps {
  value: string
  children?: React.ReactNode
  className?: string
  // injected by ComboboxList render-prop path
  onSelect?: (value: string) => void
  selected?: boolean
}

function ComboboxItem({ value, children, className, onSelect, selected }: ComboboxItemProps) {
  const ctx = React.useContext(ComboboxContext)

  const isSelected = selected ?? ctx?.value === value

  function handleSelect() {
    if (onSelect) {
      onSelect(value)
    } else if (ctx) {
      ctx.onValueChange(value === ctx.value ? null : value)
      ctx.setOpen(false)
    }
  }

  return (
    <CommandItem
      value={value}
      onSelect={handleSelect}
      data-slot="combobox-item"
      className={cn("pr-8", className)}
    >
      {children ?? value}
      {isSelected && (
        <Check className="absolute right-2 size-4 shrink-0 text-foreground" />
      )}
    </CommandItem>
  )
}

// ── Separator ─────────────────────────────────────────────────────────────────

function ComboboxSeparator({ className }: { className?: string }) {
  return <div className={cn("-mx-1 my-1 h-px bg-border", className)} />
}

// ── Group ─────────────────────────────────────────────────────────────────────

function ComboboxGroup({ children, className }: { children: React.ReactNode; className?: string }) {
  return <div data-slot="combobox-group" className={cn(className)}>{children}</div>
}

function ComboboxLabel({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div
      data-slot="combobox-label"
      className={cn("px-2 py-1.5 text-xs text-muted-foreground", className)}
    >
      {children}
    </div>
  )
}

// ── Anchor helper (no-op, kept for API compat) ────────────────────────────────

function useComboboxAnchor() {
  return React.useRef<HTMLDivElement | null>(null)
}

export {
  Combobox,
  ComboboxInput,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxList,
  ComboboxItem,
  ComboboxGroup,
  ComboboxLabel,
  ComboboxSeparator,
  useComboboxAnchor,
}
