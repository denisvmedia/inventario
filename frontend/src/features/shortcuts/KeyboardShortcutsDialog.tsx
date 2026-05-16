import { useEffect, useMemo, useState } from "react"
import { Search } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"

import {
  CATEGORY_ORDER,
  SHORTCUTS,
  type ShortcutCategoryKey,
  type ShortcutDef,
  detectIsMac,
  formatCombo,
} from "./registry"

interface KeyboardShortcutsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

// The cheat-sheet modal for issue #1385. Driven entirely by the
// SHORTCUTS array in registry.ts — no hard-coded list lives here, so a
// new entry in the registry surfaces automatically. The filter input at
// the top is a plain substring match against the (translated) action
// label, the category name, and the literal combo string; press `?`
// anywhere with no input focused to open the dialog, `Esc` to close.
export function KeyboardShortcutsDialog({ open, onOpenChange }: KeyboardShortcutsDialogProps) {
  const { t } = useTranslation()
  const [filter, setFilter] = useState("")
  const isMac = useMemo(() => detectIsMac(), [])

  // Reset the filter when the dialog closes so the next open starts
  // empty. Otherwise the user's last query sits there and silently
  // narrows the list when they reopen the dialog later.
  useEffect(() => {
    // Sync from external open prop → local filter input.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (!open) setFilter("")
  }, [open])

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return SHORTCUTS
    return SHORTCUTS.filter((s) => {
      const label = t(s.labelKey).toLowerCase()
      const cat = t(`common:shortcuts.categories.${s.categoryKey}`).toLowerCase()
      const combo = s.combo.toLowerCase()
      return label.includes(q) || cat.includes(q) || combo.includes(q)
    })
  }, [filter, t])

  const grouped = useMemo(() => {
    const byCategory = new Map<ShortcutCategoryKey, ShortcutDef[]>()
    for (const entry of filtered) {
      const bucket = byCategory.get(entry.categoryKey) ?? []
      bucket.push(entry)
      byCategory.set(entry.categoryKey, bucket)
    }
    // CATEGORY_ORDER defines display order; categories absent from the
    // filtered set drop out, anything not listed there falls through
    // at the end (defensive — every registry entry SHOULD use one of
    // the declared categories).
    const orderedKeys: ShortcutCategoryKey[] = [
      ...CATEGORY_ORDER.filter((k) => byCategory.has(k)),
      ...Array.from(byCategory.keys()).filter((k) => !CATEGORY_ORDER.includes(k)),
    ]
    return orderedKeys.map((key) => [key, byCategory.get(key)!] as const)
  }, [filtered])

  const trimmedFilter = filter.trim()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl sm:max-w-2xl" data-testid="keyboard-shortcuts-dialog">
        <DialogHeader>
          <DialogTitle>{t("common:shortcuts.title")}</DialogTitle>
          <DialogDescription>{t("common:shortcuts.description")}</DialogDescription>
        </DialogHeader>
        <div className="relative">
          <Search
            className="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder={t("common:shortcuts.filterPlaceholder")}
            aria-label={t("common:shortcuts.filterPlaceholder")}
            className="pl-9"
            data-testid="keyboard-shortcuts-filter"
          />
        </div>
        {grouped.length === 0 ? (
          <p
            className="py-6 text-center text-sm text-muted-foreground"
            data-testid="keyboard-shortcuts-empty"
          >
            {t("common:shortcuts.empty", { query: trimmedFilter })}
          </p>
        ) : (
          <div className="space-y-6">
            {grouped.map(([category, items]) => (
              <section key={category} data-testid={`keyboard-shortcuts-section-${category}`}>
                <h3 className="mb-2 text-sm font-medium text-muted-foreground">
                  {t(`common:shortcuts.categories.${category}`)}
                </h3>
                <div className="divide-y divide-border rounded-xl border border-border">
                  {items.map((entry) => (
                    <ShortcutRow key={entry.id} entry={entry} isMac={isMac} />
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

interface ShortcutRowProps {
  entry: ShortcutDef
  isMac: boolean
}

function ShortcutRow({ entry, isMac }: ShortcutRowProps) {
  const { t } = useTranslation()
  const chords = formatCombo(entry.combo, isMac)
  return (
    <div
      className="flex items-center justify-between gap-4 p-3"
      data-testid={`keyboard-shortcut-${entry.id}`}
    >
      <span className="text-sm">{t(entry.labelKey)}</span>
      <div className="flex items-center gap-2">
        {chords.map((chord, chordIndex) => (
          // Chord index is a stable position within the immutable combo
          // string, so using it as a React key is safe here.
          <span key={chordIndex} className="flex items-center gap-1">
            {chord.map((key, keyIndex) => (
              // Same reasoning — fixed position inside an immutable chord.
              <span key={keyIndex} className="flex items-center gap-1">
                <kbd className="rounded-md border border-border bg-muted px-2 py-0.5 font-mono text-xs">
                  {key}
                </kbd>
                {keyIndex < chord.length - 1 ? (
                  <span className="text-xs text-muted-foreground">+</span>
                ) : null}
              </span>
            ))}
            {chordIndex < chords.length - 1 ? (
              <span className="text-xs text-muted-foreground">{t("common:shortcuts.then")}</span>
            ) : null}
          </span>
        ))}
      </div>
    </div>
  )
}
