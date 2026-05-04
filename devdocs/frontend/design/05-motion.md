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
Tailwind v4. See `../imports-and-bans.md`.

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
| Page transition between routes | none | — |

**No page transitions** — switching between routes is instant. Adding a
fade between pages adds latency the user doesn't want and breaks
keyboard / back-button rhythm.

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
`motion-reduce:` gating, but the WCAG 2.2 AA bar (`14-accessibility.md`)
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
  `16-notifications-and-trust.md`.

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

## Anti-patterns

- `transition: all 300ms` — too broad, transitions properties you
  didn't expect (e.g. `width` becomes animated). Use `transition-colors`,
  `transition-opacity`, `transition-transform`.
- A 600ms hover. Hover is feedback, not theatre.
- `animate-bounce` on the empty-state icon. Empty states are quiet
  (per `00-positioning.md`); a bouncing icon is loud.
- Page-load fade-in. The user clicked a link; the page should appear
  immediately.
- A custom `@keyframes` for a "subtle gradient sweep" on a card.
  Cards don't sweep. Borders, not effects (per
  `04-elevation-and-effects.md`).

## Cross-refs

- Library policy: `../imports-and-bans.md`.
- Toast hierarchy: `16-notifications-and-trust.md`.
- Reduced-motion a11y: `14-accessibility.md`.
- Mock canonical: `denisvmedia/inventario-design/CLAUDE.md` does not
  spell out timings; this doc is the canonical source.
