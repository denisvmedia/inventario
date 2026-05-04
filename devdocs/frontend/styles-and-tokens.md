# Styles and tokens

Tailwind v4 + OKLCH tokens. The visual contract is canonical at
`denisvmedia/inventario-design/CLAUDE.md`; this doc covers the Inventario
codebase's translation and the rules every PR must respect.

## Tailwind v4 setup

`frontend/src/index.css` is the **single source of truth** for CSS:

```css
@import "tailwindcss";
@import "tw-animate-css";

@custom-variant dark (&:is(.dark *));

@theme inline {
  --radius-sm: calc(var(--radius) - 4px);
  --radius-md: calc(var(--radius) - 2px);
  --radius-lg: var(--radius);
  --radius-xl: calc(var(--radius) + 4px);
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-primary: var(--primary);
  /* … one --color-* per token … */
}

:root { /* light mode token values */ }
.dark { /* dark mode token values */ }
```

Three layers:

1. `:root { --foo: <oklch …>; }` — the literal value per mode. Light is
   on `:root`, dark is on `.dark`.
2. `@theme inline { --color-foo: var(--foo); }` — registers the token
   with Tailwind so `text-foo`, `bg-foo`, `border-foo` exist.
3. Components consume the token via Tailwind utilities only. **Never**
   write `color: oklch(...)` or `color: var(--foo)` directly inside a
   component's CSS or `style={}`.

## OKLCH only

Every color is in OKLCH. Why: perceptually uniform across light/dark,
plays well with token-based dark mode, no `#hex` / `hsl()` drift.

```css
--background: oklch(0.985 0.004 75);   /* warm off-white */
--foreground: oklch(0.18 0.012 60);    /* near-black */
--primary:    oklch(0.26 0.02 60);     /* dark amber-tinted */
--accent:     oklch(0.85 0.12 75);     /* THE amber accent */
```

Hard rules:

- **No `hsl()` wrappers.** The token holds raw OKLCH; we never wrap it.
  This is a deliberate break from the legacy shadcn template.
- **No `#hex` anywhere** — not in CSS, not in `style={}`, not in inline
  SVG fills. Use the token (`fill-current`, `text-chart-1`, …).
- **No Tailwind named colors.** `text-amber-500`, `bg-green-100` are
  bans on sight — use the domain token (`text-status-expiring`) or chart
  color (`text-chart-1`).

## Dark mode

Dark mode toggles via the `.dark` class on `<html>`, written by
`useTheme()` (`src/components/theme-provider.tsx`). All `--color-*`
references resolve to the dark variant automatically — components write
the same Tailwind class in either mode.

```css
.dark {
  --background: oklch(0.155 0.01 55);
  --foreground: oklch(0.96 0.006 70);
  --primary:    oklch(0.88 0.09 75);   /* light amber in dark */
  --border:     oklch(1 0 0 / 10%);    /* white at 10% opacity */
}
```

Rules:

- **Both modes are first-class.** Every new token gets a value in both
  `:root` and `.dark`.
- **Never test only one mode.** When you take screenshots
  ([screenshots.md](screenshots.md)), capture both themes.
- **No `dark:` Tailwind variants on color tokens.** Tokens already swap
  via the `.dark` class. `dark:` is reserved for one-off layout tweaks
  (e.g. a border that should disappear in dark) and used sparingly.

## Density

Density is "how tight should rows / cards / lists be?" — three steps:

```ts
type Density = "comfortable" | "cozy" | "compact"
```

Set on `<html data-density="cozy">` by `DensityProvider`
(`src/hooks/useDensity.tsx`). Persisted to `localStorage` and mirrored
across tabs via the `storage` event. The Settings page (#1414) writes
through `PATCH /settings` and mirrors back to `localStorage` on boot.

When you build a row-heavy surface (lists, tables, settings rows), wire
density via the `data-density` attribute selector:

```css
[data-density="cozy"] .row    { padding-block: 0.75rem; }
[data-density="compact"] .row { padding-block: 0.5rem; }
```

Or — preferred — use Tailwind's `data-[density=*]:` arbitrary variant
when the rule is one-off:

```tsx
<div className="py-3.5 data-[density=compact]:py-2.5 data-[density=cozy]:py-3">…</div>
```

## Domain tokens

Beyond shadcn defaults, the design encodes domain semantics. Always use
the domain token over a generic chart or status color:

| Token | Light value | Use |
| --- | --- | --- |
| `--status-active` | `oklch(0.72 0.17 145)` | Green — in use / valid warranty |
| `--status-expiring` | `oklch(0.78 0.18 75)` | Amber — within 60 days |
| `--status-expired` | `oklch(0.65 0.22 25)` | Red — past due |
| `--status-none` | `oklch(0.6 0 0)` | Gray — no warranty |
| `--chart-1` … `--chart-5` | warm amber → red | Data viz, tag pills |
| `--tag-amber` … `--tag-muted` | tag-only palette | Tag pills (closed enum, not chart colors) |
| `--sidebar` etc. | warm off-white | Sidebar surface — distinct from page bg |

Resolve the domain via `commodityKeys`-style config maps; never inline
the ternary at render:

```ts
WARRANTY_STATUS_CONFIG[status].color   // "text-status-active"
WARRANTY_STATUS_CONFIG[status].bg      // "bg-status-active/10"
WARRANTY_STATUS_CONFIG[status].label   // "Active" (i18n key in real code)
```

## Spacing patterns

| Surface | Outer | Inner |
| --- | --- | --- |
| Page | `flex flex-col gap-6 p-6 max-w-2xl mx-auto w-full` (or `max-w-4xl` for list) | `space-y-5` inside cards |
| Card | `rounded-xl border border-border bg-card p-6 space-y-5` | tight rows: `py-3.5` |
| Stat card | `rounded-xl border border-border bg-card px-4 py-3 flex items-center gap-3` | icon `size-8` |
| Stats row | `grid grid-cols-2 gap-4 lg:grid-cols-4` | — |
| Settings rows | `divide-y divide-border` | each row `flex items-center justify-between py-3.5` |

Rules:

- **`gap-*` between siblings, `space-y-*` inside containers.** Never mix
  `mt-*` and `mb-*` on individual children to get the same spacing — use
  the parent's `gap` / `space-y`.
- **`p-6` for cards, `py-3.5` for list rows.** These are the magic
  numbers from the mock; don't drift.
- **Don't introduce new spacing scales.** Tailwind's default scale (with
  `0.5` increments) is enough. If you need 14px, use `py-3.5`, not a
  custom token.

## Typography

shadcn provides no default element styles. Apply Tailwind classes
explicitly:

| Role | Classes |
| --- | --- |
| Page title (h1) | `scroll-m-20 text-3xl font-semibold tracking-tight` |
| Section heading (h2) | `text-base font-semibold` |
| Body | `text-sm leading-relaxed` |
| Muted | `text-sm text-muted-foreground` |
| Overline | `text-xs font-semibold uppercase tracking-widest text-muted-foreground` |
| Stat value | `text-2xl font-bold tracking-tight` |
| Code / mono | `font-mono text-xs` |

Use them in this exact form so the visual rhythm stays consistent across
pages. The mock at `denisvmedia/inventario-design` lists more — mirror them
when adding a new role.

## `tw-animate-css`, not `@tailwindcss/animate`

The mock left `@tailwindcss/animate` behind. We use `tw-animate-css`
(imported at the top of `index.css`) for `animate-in`, `animate-out`,
`fade-in-0`, `slide-in-from-top-2`, etc. Never install the legacy package.

## No drop shadows

The visual language uses borders, not shadows. The one exception is
`shadow-xs` on inputs (built into the shadcn `Input` primitive). Don't
add `shadow-md` / `shadow-lg` anywhere — when you reach for one, you
probably want a heavier border or a `border-border/60` subtle outline.

## Do / don't

| Do | Don't |
| --- | --- |
| `text-status-active` | `text-green-500` |
| `bg-chart-1/15 text-chart-1` | `bg-amber-100 text-amber-700` |
| `text-destructive` | `text-red-500` |
| Add a token in `index.css` (both modes) | Hardcode `oklch(...)` in a component |
| `cn(base, conditional)` | `` `${base} ${cond ? "…" : ""}` `` |
| `gap-6` between sections | `mt-6` then `mb-6` |
| `rounded-lg` (card), `rounded-xl` (large) | Pixel-perfect `rounded-[10px]` |
| Borders for elevation | Drop shadows |
| One color from the warm-amber palette | Purple / indigo / violet — **the theme has no purple** |

## Adding a new token

When the design genuinely needs a new color (a new status, a new chart
slot):

1. Add the OKLCH value to `:root` in `frontend/src/index.css`.
2. Add the dark-mode value to `.dark` in the same file.
3. Register it in `@theme inline` as `--color-<name>: var(--<name>);`.
4. Mirror the change in `denisvmedia/inventario-design/src/index.css` so
   the mock and the app stay in lockstep. Reference the mock PR in the
   Inventario PR.
5. Use the new token via the Tailwind utility (`text-<name>`,
   `bg-<name>`, `border-<name>`).
