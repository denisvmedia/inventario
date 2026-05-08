import { useState, useRef, useEffect } from "react"
import { ChevronDown, Search } from "lucide-react"
import { CURRENCIES } from "@/data/mock"
import { cn } from "@/lib/utils"

type Currency = (typeof CURRENCIES)[number]

interface CurrencyComboboxProps {
  value: string
  onValueChange: (value: string) => void
  className?: string
  variant?: "compact" | "full"
}

export function CurrencyCombobox({
  value,
  onValueChange,
  className,
  variant = "full",
}: CurrencyComboboxProps) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const containerRef = useRef<HTMLDivElement>(null)
  const searchRef = useRef<HTMLInputElement>(null)

  const selected = CURRENCIES.find((c) => c.code === value)

  const filtered = query.trim()
    ? CURRENCIES.filter((c) =>
        `${c.code} ${c.name} ${c.symbol}`.toLowerCase().includes(query.toLowerCase())
      )
    : CURRENCIES

  function openDropdown() {
    setOpen(true)
    setQuery("")
    setTimeout(() => searchRef.current?.focus(), 0)
  }

  function selectCurrency(c: Currency) {
    onValueChange(c.code)
    setOpen(false)
    setQuery("")
  }

  // Close on outside click
  useEffect(() => {
    if (!open) return
    function handleClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
        setQuery("")
      }
    }
    document.addEventListener("mousedown", handleClick)
    return () => document.removeEventListener("mousedown", handleClick)
  }, [open])

  return (
    <div ref={containerRef} className={cn("relative", className)}>
      {/* Trigger button */}
      <button
        type="button"
        onClick={openDropdown}
        className={cn(
          "flex items-center justify-between gap-1.5 h-9 rounded-md border border-input bg-background px-3 text-sm shadow-xs transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          variant === "compact" ? "w-24 min-w-0" : "w-full"
        )}
      >
        {variant === "compact" ? (
          <span className="font-medium truncate">{selected?.code ?? "USD"}</span>
        ) : (
          <span className="flex items-center gap-2 min-w-0">
            <span className="w-5 shrink-0 font-mono text-xs text-muted-foreground">{selected?.symbol ?? "$"}</span>
            <span className="font-medium">{selected?.code ?? "USD"}</span>
            <span className="text-muted-foreground truncate text-xs">— {selected?.name ?? "US Dollar"}</span>
          </span>
        )}
        <ChevronDown className="size-3.5 text-muted-foreground shrink-0" />
      </button>

      {/* Dropdown */}
      {open && (
        <div className="absolute z-50 top-full mt-1 right-0 w-72 rounded-lg border border-border bg-popover shadow-lg overflow-hidden">
          {/* Search */}
          <div className="flex items-center gap-2 border-b border-border px-3 py-2">
            <Search className="size-3.5 text-muted-foreground shrink-0" />
            <input
              ref={searchRef}
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search currency…"
              className="flex-1 text-sm bg-transparent outline-none placeholder:text-muted-foreground"
            />
          </div>
          {/* List */}
          <div className="max-h-60 overflow-y-auto p-1">
            {filtered.length === 0 ? (
              <p className="py-3 text-center text-xs text-muted-foreground">No currency found.</p>
            ) : (
              filtered.map((c) => (
                <button
                  key={c.code}
                  type="button"
                  onClick={() => selectCurrency(c)}
                  className={cn(
                    "flex w-full items-center gap-2 rounded-md px-2.5 py-1.5 text-sm transition-colors hover:bg-accent hover:text-accent-foreground",
                    c.code === value && "bg-accent text-accent-foreground font-medium"
                  )}
                >
                  <span className="w-6 shrink-0 font-mono text-xs text-muted-foreground">{c.symbol}</span>
                  <span className="font-medium">{c.code}</span>
                  <span className="text-muted-foreground truncate text-xs">— {c.name}</span>
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
