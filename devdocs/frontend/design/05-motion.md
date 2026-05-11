# Motion

`tw-animate-css` only. Durations short, easings native. Reduced-motion
is a first-class consumer; the Tailwind utilities respect it
automatically.

## Library: `tw-animate-css`

Imported at the top of `frontend/src/index.css`:

```css
@import "tw-animate-css";
```

Provides the utilities Radix and shadcn primitives use:

- `animate-in`, `animate-out`
- `fade-in-0`, `fade-out-0`
- `slide-in-from-top-2`, `slide-in-from-bottom-2`,
  `slide-in-from-left-2`, `slide-in-from-right-2`
- `zoom-in-95`, `zoom-out-95`

These compose: a Sheet's open animation is
`data-[state=open]:animate-in data-[state=open]:slide-in-from-right
data-[state=open]:duration-300`.

`@tailwindcss/animate` is banned — `tw-animate-css` replaces it for
Tailwind v4. See [../imports-and-bans.md](../imports-and-bans.md).

Framer Motion / react-spring / popmotion / GSAP are also banned. The
animations the design uses are all property transitions (color,
opacity, transform); a runtime animation library is dead weight.

## Durations

| Class | ms | Use |
| --- | --- | --- |
| `duration-75` | 75 | Color transitions on hover |
| `duration-150` (default for `transition-colors`) | 150 | Buttons, links, inputs |
| `duration-200` | 200 | Dropdowns, tooltips, popovers |
| `duration-300` | 300 | Dialogs, sheets, sonner toasts |
| `duration-500` | 500 | Empty-state fade-in (rare) |

Anything > 500ms is "this is making me wait" territory. If you need
500ms+, you probably want a skeleton instead of an animation.

## Easings

Tailwind defaults map to:

- `ease-linear` — never (looks robotic).
- `ease-in` — never on its own; only as part of `ease-in-out`.
- `ease-out` — entries (`animate-in`). Things appear with deceleration.
- `ease-in` — exits (`animate-out`). Things leave with acceleration.
- `ease-in-out` — color/opacity transitions (`transition-colors`,
  `transition-opacity`). The default for hover/focus.

Don't reach for custom cubic-beziers (`ease-[cubic-bezier(...)]`).
The defaults are correct.

## What animates

| Element | Animation | Duration |
| --- | --- | --- |
| Button background on hover | `transition-colors` | 150ms ease-in-out |
| Tooltip open | `fade-in-0 zoom-in-95` | 200ms ease-out |
| Dropdown menu open | `fade-in-0 zoom-in-95 slide-in-from-top-2` | 200ms ease-out |
| Dialog open | `fade-in-0 zoom-in-95` | 300ms ease-out |
| Dialog close | `fade-out-0 zoom-out-95` | 300ms ease-in |
| Sheet open (right) | `slide-in-from-right` | 300ms ease-out |
| Sheet close (right) | `slide-out-to-right` | 300ms ease-in |
| Sonner toast in | `slide-in-from-bottom` | 300ms ease-out |
| Reveal-on-hover (kebab in row) | `transition-opacity` | 150ms |
| Skeleton shimmer | `animate-pulse` | infinite |
| Wizard step swap (container resize) | `transition-[height]` 200ms ease-out | 200ms |
| List row enter (URL `+ Add`, file attached, reveal-on-click section) | `animate-in fade-in slide-in-from-top-1` | 150ms ease-out |
| List row exit (URL × button) | `animate-out fade-out slide-out-to-top-1 fill-mode-forwards` | 150ms ease-in |
| Page transition between routes | none | — |

**No page transitions** — switching between routes is instant. Adding a
fade between pages adds latency the user doesn't want and breaks
keyboard / back-button rhythm.

## Smoothness — no abrupt jerks

Layout changes in place — wizard step swaps, content reveals
(`+ Add part numbers`-style toggles), list rows being added or
removed, dynamic field appearance — must **not snap**. A snap reads
to the user as a bug ("did the page reload?") even when the new
state is correct. Every state change that touches container height
or list membership gets a transition.

Three patterns cover almost everything we ship:

### Container resize (wizard steps, dynamic-content sections)

When children add or remove rows that change the container's
natural height — and especially when the *whole* step swaps inside
a Dialog — wrap the variable region in a `ResizeObserver`-driven
explicit-height wrapper:

```tsx
function StepResizeWrapper({ children }) {
  const innerRef = useRef<HTMLDivElement>(null)
  const [height, setHeight] = useState<number | null>(null)
  // First measurement commits without animation; subsequent ones
  // transition between two pixel values.
  const [transitionsReady, setTransitionsReady] = useState(false)
  useEffect(() => {
    const node = innerRef.current
    if (!node) return
    const obs = new ResizeObserver(([entry]) => setHeight(entry.contentRect.height))
    obs.observe(node)
    return () => obs.disconnect()
  }, [])
  useEffect(() => {
    if (height === null || transitionsReady) return
    const id = requestAnimationFrame(() => setTransitionsReady(true))
    return () => cancelAnimationFrame(id)
  }, [height, transitionsReady])
  return (
    <div
      style={height === null ? undefined : { height: `${height}px` }}
      className={cn(
        "overflow-hidden",
        transitionsReady && "transition-[height] duration-200 ease-out"
      )}
    >
      <div ref={innerRef}>{children}</div>
    </div>
  )
}
```

Why pixel-driven and not `interpolate-size: allow-keywords`?
Auto-to-auto transitions need two distinct *resolved* sizes per
frame; React commits the new step's children synchronously so the
browser never sees the "old" auto-height before the swap. Pixel
values give the transition something concrete on both ends, and
work on every browser instead of Chrome 129+ / Firefox 124+ /
Safari 18+.

Reference implementation: `StepResizeWrapper` inside
`frontend/src/components/items/CommodityFormDialog.tsx`.

### List row enter / exit

Use the `tw-animate-css` utilities — they're
`prefers-reduced-motion`-gated for free.

**Enter** (e.g. clicking "+ Add" on a URL list, revealing a
`ChipInput` after `+ Item has additional serial numbers`):

```tsx
<div className="animate-in fade-in slide-in-from-top-1 duration-150">…</div>
```

**Exit** is harder — React unmounts immediately, so a fading row
with `animate-out` still vanishes in the same frame. The pattern
is: hold the row in DOM with a "leaving" state, run the exit
animation for `EXIT_MS`, then commit the actual removal.

```tsx
const EXIT_MS = 150
const [leavingId, setLeavingId] = useState<string | null>(null)
function remove(id: string) {
  setLeavingId(id)
  setTimeout(() => {
    onChange(items.filter((it) => it.id !== id))
    setLeavingId(null)
  }, EXIT_MS)
}
// In render:
<li className={cn(
  base,
  isLeaving && "animate-out fade-out slide-out-to-top-1 fill-mode-forwards duration-150"
)} />
```

`fill-mode-forwards` keeps the element at end-state (transparent +
slid up) until unmount, so the user doesn't see a flash of full
opacity right before the row vanishes. Reference implementation:
`UrlList` inside `CommodityFormDialog.tsx`.

### Toggle reveal (chevron-down "+ X" affordances)

When a click swaps a toggle for the actual control, animate the
appearance of the control (the toggle disappearing on the same
click is fine instantly — it's the *new* element's arrival that
the user reads as a state change):

```tsx
{revealed ? (
  <div className="animate-in fade-in slide-in-from-top-1 duration-200">
    <RevealedField …/>
  </div>
) : (
  <button className="text-xs text-muted-foreground hover:text-foreground">
    <ChevronDown className="size-3.5" />
    Reveal copy
  </button>
)}
```

These three patterns compose: a wizard step swap inside the
dialog → `StepResizeWrapper` animates the dialog height; the new
step contains a list with rows → row enter/exit handles list
churn; a row's reveal-on-click affordance → toggle reveal pattern.

## Reduced motion

`prefers-reduced-motion: reduce` disables `tw-animate-css` utilities
automatically (they're gated on
`@media (prefers-reduced-motion: no-preference)`).

For your own transitions, gate explicitly with the `motion-reduce:`
prefix:

```tsx
<div className="transition-colors motion-reduce:transition-none">…</div>
```

`motion-safe:` is the inverse — apply only when motion is *not*
reduced. Use it for non-essential animations:

```tsx
<div className="motion-safe:animate-pulse">…</div>
```

The Lighthouse `accessibility` audit doesn't currently flag missing
`motion-reduce:` gating, but the WCAG 2.2 AA bar ([14-accessibility.md](14-accessibility.md))
asks for it. Treat it as required.

## Skeletons over spinners

While data is loading, prefer a **skeleton** that matches the page's
shape over a centered spinner:

```tsx
<div className="animate-pulse space-y-3">
  <div className="h-4 w-1/3 rounded-md bg-muted" />
  <div className="h-4 w-1/2 rounded-md bg-muted" />
</div>
```

`animate-pulse` is a Tailwind built-in (an `@keyframes` opacity
shimmer). Use it on bones-shaped placeholders; don't roll your own
`@keyframes`.

The `<Skeleton>` shadcn primitive (`src/components/ui/skeleton.tsx`)
is the wrapper — it just sets `bg-muted` + `animate-pulse` so the call
site stays terse.

## Sonner toasts

Sonner ships in/out animations tuned to its drawer:

- Slide-in from bottom-right.
- Auto-dismiss at 4s for `info` / `success`, 6s for `warning`,
  manual-dismiss for `error`.
- `useAppToast()` is the wrapper that enforces the role + duration
  contract. Don't call `sonner.toast(...)` directly — see
  [16-notifications-and-trust.md](16-notifications-and-trust.md).

## Hard rules

1. **`tw-animate-css` only.** No Framer Motion, no react-spring, no
   GSAP, no `@keyframes` defined in `index.css` outside what shadcn
   primitives need.
2. **No page transitions.** Routes swap instantly.
3. **Durations from the table.** `duration-[473ms]` is a smell.
4. **Reduced-motion respect.** Always use `motion-reduce:` /
   `motion-safe:` or rely on `tw-animate-css`'s built-in gate.
5. **Skeletons, not spinners.** A spinner is a fallback for genuinely
   indefinite waits (e.g. an export that's running on the server);
   in-flight data goes to a shape skeleton.
6. **No abrupt jerks.** Any state change that affects container
   height or list membership gets a transition (see "Smoothness —
   no abrupt jerks" above). A snap reads as a bug. The patterns
   are paste-ready; reach for them, don't reinvent.

## Anti-patterns

- `transition: all 300ms` — too broad, transitions properties you
  didn't expect (e.g. `width` becomes animated). Use `transition-colors`,
  `transition-opacity`, `transition-transform`.
- A 600ms hover. Hover is feedback, not theatre.
- `animate-bounce` on the empty-state icon. Empty states are quiet
  (per [00-positioning.md](00-positioning.md)); a bouncing icon is loud.
- Page-load fade-in. The user clicked a link; the page should appear
  immediately.
- A custom `@keyframes` for a "subtle gradient sweep" on a card.
  Cards don't sweep. Borders, not effects (per
  [04-elevation-and-effects.md](04-elevation-and-effects.md)).

## Cross-refs

- Library policy: [../imports-and-bans.md](../imports-and-bans.md).
- Toast hierarchy: [16-notifications-and-trust.md](16-notifications-and-trust.md).
- Reduced-motion a11y: [14-accessibility.md](14-accessibility.md).

  spell out timings; this doc is the canonical source.
