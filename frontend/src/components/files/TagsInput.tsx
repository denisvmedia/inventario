import { Plus, X } from "lucide-react"
import { useEffect, useRef, useState } from "react"

import { Badge } from "@/components/ui/badge"
import { Label } from "@/components/ui/label"
import { Popover, PopoverAnchor, PopoverContent } from "@/components/ui/popover"
import { cn } from "@/lib/utils"
import { useTagAutocomplete } from "@/features/tags/hooks"

// Lightweight tag chip input — type, hit Enter / comma to commit;
// Backspace on empty draft pops the last chip. Used by the files
// metadata edit form. The same UX pattern as commodities ChipInput,
// kept independent here so we don't reach into a private dialog
// helper from a different feature.
//
// `suggestions` enables a degraded autocomplete via HTML5 `<datalist>`
// — each browser renders the popup natively. Pass an explicit static
// list, or set `autocomplete` to swap the datalist for a Popover-
// portalled dropdown that fetches `/g/:slug/tags/autocomplete` itself
// (top tags surface immediately on focus, the list narrows as the
// user types). Portalling is what lets the dropdown escape ancestor
// `overflow-clip` containers (e.g. the dialog's StepResizeWrapper).
export interface TagsInputProps {
  label?: string
  values: string[]
  onChange: (next: string[]) => void
  placeholder?: string
  testId?: string
  suggestions?: string[]
  autocomplete?: boolean
  // Tighter padding + smaller chips/text. Used inside the per-file
  // row of CommodityFormDialog where the input sits below filename
  // metadata and the default size feels too tall.
  compact?: boolean
}

export function TagsInput({
  label,
  values,
  onChange,
  placeholder,
  testId,
  suggestions,
  autocomplete,
  compact,
}: TagsInputProps) {
  const [draft, setDraft] = useState("")
  // Remote suggestions live in local state and are populated via the
  // `<AutocompleteSink>` sub-component below. The sink is only rendered
  // when `autocomplete` is on, so consumers that don't opt in (e.g. the
  // existing FileEditPage path, the standalone TagsInput tests) never
  // pull in the `useTagAutocomplete` hook and don't need a
  // QueryClientProvider in their wrapper.
  const [remoteSuggestions, setRemoteSuggestions] = useState<string[]>([])
  // Open-state for the autocomplete dropdown. Radix Popover handles
  // outside-click + Escape close on its own; we drive `open` from
  // input focus / click events.
  const [open, setOpen] = useState(false)
  // Ref on the anchor wrapper so we can tell Radix's outside-click
  // detection to ignore clicks that landed on the input/chips —
  // otherwise clicking the input fires `onPointerDownOutside`
  // (PopoverAnchor is NOT auto-included as "inside" by Radix), the
  // popover closes, and our `onClick`/`onFocus` handlers immediately
  // reopen it — producing a fade-out + fade-in flicker.
  const anchorRef = useRef<HTMLDivElement>(null)

  // Merge static + remote into a unique set; the consumer's static
  // list takes precedence on label collisions.
  const mergedSuggestions =
    suggestions || (autocomplete && remoteSuggestions.length > 0)
      ? Array.from(new Set([...(suggestions ?? []), ...remoteSuggestions]))
      : undefined
  // Datalist is used by the non-autocomplete path (static `suggestions`
  // consumers, e.g. FileEditPage if it ever opts in). When `autocomplete`
  // is on we render an explicit Popover dropdown below instead, so the
  // datalist is suppressed to avoid a confusing double-popup.
  const datalistId =
    !autocomplete && mergedSuggestions && mergedSuggestions.length > 0
      ? `${testId ?? "tags"}-suggestions`
      : undefined
  const filteredSuggestions = mergedSuggestions
    ? mergedSuggestions.filter((s) => !values.includes(s))
    : undefined
  // For the dropdown path, narrow the visible list with a local
  // case-insensitive prefix filter so the list reacts immediately as
  // the user types (the BE already filters server-side, but the local
  // filter masks the round-trip latency).
  const visibleSuggestions =
    autocomplete && filteredSuggestions
      ? filteredSuggestions.filter(
          (s) => draft.trim() === "" || s.toLowerCase().includes(draft.trim().toLowerCase())
        )
      : []
  const dropdownOpen = !!autocomplete && open && visibleSuggestions.length > 0

  function commit() {
    const trimmed = draft.trim()
    if (!trimmed) return
    if (values.includes(trimmed)) {
      setDraft("")
      return
    }
    onChange([...values, trimmed])
    setDraft("")
  }

  function pick(slug: string) {
    if (values.includes(slug)) return
    onChange([...values, slug])
    setDraft("")
    // Leave dropdown open so the user can keep picking — outside
    // click / Escape / Tab will close it.
  }

  const inputAndChips = (
    <div
      ref={anchorRef}
      className={cn(
        "flex flex-wrap items-center rounded-md border border-input",
        compact ? "gap-1 px-1.5 py-1" : "gap-1.5 px-2 py-1.5"
      )}
    >
      {values.map((v) => (
        <Badge
          key={v}
          variant="secondary"
          className={cn("gap-1", compact ? "h-4 px-1 text-[11px]" : "h-5 px-1.5 text-xs")}
          data-testid={testId ? `${testId}-chip` : undefined}
        >
          {v}
          <button
            type="button"
            className="ml-0.5 inline-flex items-center"
            aria-label={`remove ${v}`}
            onClick={() => onChange(values.filter((x) => x !== v))}
          >
            <X className="size-3" aria-hidden="true" />
          </button>
        </Badge>
      ))}
      <input
        value={draft}
        // Hide CTA placeholder once any chip exists — the placeholder
        // is the "first tag" prompt; it should reappear automatically
        // when the user clears all chips.
        placeholder={values.length === 0 ? placeholder : undefined}
        list={datalistId}
        onChange={(e) => setDraft(e.target.value)}
        onFocus={() => {
          if (autocomplete) setOpen(true)
        }}
        // After the user picks a suggestion we keep focus on the
        // input (mousedown preventDefault), so a second click on
        // the same input does NOT fire onFocus again — without
        // this onClick, the dropdown wouldn't reopen until the
        // input lost focus first. Same handler also reopens after
        // Escape closes it.
        onClick={() => {
          if (autocomplete) setOpen(true)
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === ",") {
            e.preventDefault()
            commit()
          } else if (e.key === "Escape" && open) {
            e.preventDefault()
            setOpen(false)
          } else if (e.key === "Backspace" && draft === "" && values.length > 0) {
            onChange(values.slice(0, -1))
          }
        }}
        onBlur={commit}
        className={cn(
          "min-w-24 flex-1 bg-transparent outline-none placeholder:text-muted-foreground",
          compact ? "text-xs" : "text-sm"
        )}
        data-testid={testId ? `${testId}-input` : undefined}
      />
      {datalistId ? (
        <datalist id={datalistId} data-testid={`${testId}-datalist`}>
          {filteredSuggestions!.map((s) => (
            <option key={s} value={s} />
          ))}
        </datalist>
      ) : null}
      {draft.trim() ? (
        <button
          type="button"
          className="text-muted-foreground hover:text-foreground"
          aria-label="add"
          onClick={commit}
        >
          <Plus className="size-3.5" aria-hidden="true" />
        </button>
      ) : null}
    </div>
  )

  return (
    <div className="flex flex-col gap-1.5" data-testid={testId}>
      {label ? <Label>{label}</Label> : null}
      {autocomplete ? (
        <Popover open={dropdownOpen} onOpenChange={setOpen}>
          <PopoverAnchor asChild>{inputAndChips}</PopoverAnchor>
          <PopoverContent
            align="start"
            sideOffset={4}
            // Width matches the anchor so the dropdown lines up with
            // the input wrapper. `--radix-popover-trigger-width` is
            // populated by Radix from the anchor's bounding box.
            //
            // The `!zoom-in-100` / `!slide-in-from-*-0` overrides
            // neutralise the default Popover zoom-in-95 + slide so
            // the autocomplete list just fades in. The default zoom
            // makes a fixed-width dropdown look like it's "settling"
            // (briefly narrower, then widens) — wrong cue for a
            // typeahead.
            className={cn(
              "w-[var(--radix-popover-trigger-width)] max-h-48 overflow-auto rounded-md border border-border bg-popover p-0 py-1 text-popover-foreground shadow-md",
              "data-[state=open]:!zoom-in-100 data-[state=closed]:!zoom-out-100",
              "data-[side=bottom]:!slide-in-from-top-0 data-[side=top]:!slide-in-from-bottom-0 data-[side=left]:!slide-in-from-right-0 data-[side=right]:!slide-in-from-left-0",
              compact ? "text-xs" : "text-sm"
            )}
            // The default Popover steals focus into PopoverContent; we
            // want the input to keep typing focus instead.
            onOpenAutoFocus={(e) => e.preventDefault()}
            // Same on close — don't yank focus to the anchor.
            onCloseAutoFocus={(e) => e.preventDefault()}
            // PopoverAnchor isn't included in Radix's "inside" check.
            // Tell pointer-down-outside to skip events that landed on
            // the anchor — otherwise the click on the input closes the
            // popover and our onClick/onFocus immediately reopen it,
            // producing a visible fade-out + fade-in flicker.
            onPointerDownOutside={(e) => {
              if (anchorRef.current?.contains(e.target as Node)) e.preventDefault()
            }}
            onFocusOutside={(e) => {
              if (anchorRef.current?.contains(e.target as Node)) e.preventDefault()
            }}
            data-testid={testId ? `${testId}-dropdown` : undefined}
            role="listbox"
          >
            {visibleSuggestions.map((s) => (
              <button
                key={s}
                type="button"
                // `onMouseDown` (not onClick) so we fire BEFORE the
                // input's blur handler runs and tears the dropdown
                // down. preventDefault keeps the input focused.
                onMouseDown={(e) => {
                  e.preventDefault()
                  pick(s)
                }}
                className="block w-full cursor-pointer px-2 py-1 text-left hover:bg-accent hover:text-accent-foreground"
                role="option"
                aria-selected="false"
              >
                {s}
              </button>
            ))}
          </PopoverContent>
        </Popover>
      ) : (
        inputAndChips
      )}
      {autocomplete ? <AutocompleteSink draft={draft} onChange={setRemoteSuggestions} /> : null}
    </div>
  )
}

// AutocompleteSink owns the call to `useTagAutocomplete` so the hook
// is only invoked when the parent opted in via `autocomplete`. Render-
// gating it here avoids forcing every TagsInput consumer to wrap their
// test harness in a QueryClientProvider just because the prop exists.
// The component returns `null` — its only job is to push fetched tag
// slugs back to the parent via `onChange`.
//
// `enabled: true` (no draft-length gate) so the dropdown can show top
// tags on focus before the user types anything — the BE returns its
// usage-ranked default list when `q` is empty.
function AutocompleteSink({
  draft,
  onChange,
}: {
  draft: string
  onChange: (slugs: string[]) => void
}) {
  const remote = useTagAutocomplete(draft, 8, { enabled: true })
  const data = remote.data
  useEffect(() => {
    onChange(data ? data.map((t) => t.slug) : [])
  }, [data, onChange])
  return null
}
