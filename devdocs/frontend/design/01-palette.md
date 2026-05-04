# Palette

**Committed.** Warm-neutral OKLCH with amber accents in light mode;
warm-dark with light amber in dark mode. No purple. No raw color
names. The full token set lives at `frontend/src/index.css` (`:root`
and `.dark`); duplicate it in `denisvmedia/inventario-design`'s
`index.css` in lockstep when you change anything here.

## Why OKLCH

OKLCH is perceptually uniform: a 10% lightness step looks like the
same step everywhere on the wheel. That matters for status colors —
green / amber / red / gray need to carry the same visual weight, and
HSL fails that bar (HSL green at L=72 looks brighter than HSL red at
L=72). It also dark-modes cleanly — most tokens swap by adjusting the
lightness component while keeping chroma + hue.

`hsl(...)` wrappers are banned. Tokens hold raw OKLCH; components
consume them via Tailwind utilities. See
`../styles-and-tokens.md` for the layer-by-layer shape.

## Roles

```css
:root {
  /* Surfaces */
  --background: oklch(0.985 0.004 75);     /* warm off-white page bg */
  --foreground: oklch(0.18 0.012 60);      /* near-black text */
  --card: oklch(1 0 0);                    /* pure white card */
  --card-foreground: oklch(0.18 0.012 60);
  --popover: oklch(1 0 0);
  --popover-foreground: oklch(0.18 0.012 60);

  /* Brand */
  --primary: oklch(0.26 0.02 60);          /* dark amber-tinted primary */
  --primary-foreground: oklch(0.985 0.004 75);
  --accent: oklch(0.85 0.12 75);           /* THE amber accent */
  --accent-foreground: oklch(0.22 0.04 60);

  /* Supporting */
  --secondary: oklch(0.95 0.008 70);
  --secondary-foreground: oklch(0.26 0.02 60);
  --muted: oklch(0.945 0.008 70);
  --muted-foreground: oklch(0.5 0.018 60);
  --destructive: oklch(0.577 0.245 27.325);
  --destructive-foreground: oklch(0.985 0 0);

  /* Structural */
  --border: oklch(0.9 0.008 70);
  --input: oklch(0.9 0.008 70);
  --ring: oklch(0.65 0.08 75);             /* amber focus ring */
}

.dark {
  --background: oklch(0.155 0.01 55);
  --foreground: oklch(0.96 0.006 70);
  --card: oklch(0.195 0.012 55);
  --primary: oklch(0.88 0.09 75);          /* light amber in dark */
  --accent: oklch(0.72 0.14 75);
  --border: oklch(1 0 0 / 10%);            /* white at 10% opacity */
  --input: oklch(1 0 0 / 12%);
  --ring: oklch(0.72 0.1 75);
}
```

The full set (sidebar, chart, status, tag) is at
`frontend/src/index.css`. The list above is the irreducible core a new
surface must respect.

## Semantic tokens

These encode business meaning and **always** beat a generic color:

| Token | Light value | Use |
| --- | --- | --- |
| `--status-active` | `oklch(0.72 0.17 145)` | Green — in use, valid warranty |
| `--status-expiring` | `oklch(0.78 0.18 75)` | Amber — within 60 days |
| `--status-expired` | `oklch(0.65 0.22 25)` | Red — past due |
| `--status-none` | `oklch(0.6 0 0)` | Gray — no warranty |
| `--chart-1` … `--chart-5` | warm amber → red | Data viz; see `07-data-visualization.md` |
| `--tag-amber`, `--tag-green`, `--tag-blue`, `--tag-orange`, `--tag-red`, `--tag-muted` | tag-only palette | TagBadge pills (closed enum mirrored on the BE — `models.TagColor`) |

`--tag-*` and `--chart-*` look superficially similar but are different
sets — chart slots are for data viz density (5 distinguishable hues),
tag colors are for closed-enum pills (6 hues that include a `muted`
gray slot). Don't mix.

## Why this combination

A warm off-white surface (~oklch(0.985 0.004 75)) reads as "considered"
without trying — it's the editorial / Aesop / Kinfolk neutral. A
near-black warm-tinted foreground (oklch(0.18 0.012 60)) keeps body
text high-contrast without the harshness of pure `#000`. The amber
accent is the one note of warmth — used for primary actions, focus
rings, hover-reveal states. Everything else defers to muted gray.

In dark mode the primary inverts: amber becomes the *light* color
(used for foregrounds and primary surfaces), and the canvas is a deep
warm gray (oklch(0.155 0.01 55)) — never pure `#000`, which would lose
all the warmth.

## Contrast targets

| Pair | Ratio | Standard |
| --- | --- | --- |
| `--foreground` on `--background` | ≥ 14:1 | AAA |
| `--foreground` on `--card` | ≥ 14:1 | AAA |
| `--muted-foreground` on `--background` | ≥ 4.5:1 | AA |
| `--muted-foreground` on `--card` | ≥ 4.5:1 | AA |
| `--primary-foreground` on `--primary` | ≥ 7:1 | AAA |
| `--ring` on adjacent surface | ≥ 3:1 | AA (UI) |
| `--destructive` text on `--card` | ≥ 4.5:1 | AA |
| `--destructive-foreground` on `--destructive` (filled button) | ≥ 4.5:1 | AA |

The Lighthouse `accessibility` gate (`≥ 0.95`, see `../perf.md`)
catches drift; `jest-axe` catches it earlier in unit tests. See
`14-accessibility.md`.

## Hard rules

1. **Never hardcode a color.** Not in `style={}`, not in CSS, not in
   inline SVG `fill=`. Tokens via Tailwind utilities or nothing.
2. **Never use Tailwind named palettes** (`text-amber-500`,
   `bg-green-100`). Use the token (`text-status-expiring`,
   `text-chart-1`).
3. **No purple, indigo, violet, magenta** anywhere — the theme has no
   purple. If a future feature needs a new hue, add a new token, don't
   reach for a Tailwind named color.
4. **Both modes are first-class.** Every new token gets a value in
   `:root` *and* `.dark` in the same PR. A token that only exists in
   one mode is a regression.
5. **Lockstep with the mock.** A token change in
   `frontend/src/index.css` must mirror in
   `denisvmedia/inventario-design`'s `src/index.css` and be cross-linked
   in the PR.

## Anti-patterns

- `text-red-500` for an error label. Use `text-destructive`.
- `bg-amber-100 text-amber-700` for a status pill. Use
  `bg-status-expiring/10 text-status-expiring`.
- `dark:text-amber-200` to swap a color in dark mode. The token does
  the swap automatically; `dark:` is reserved for layout tweaks, not
  color tokens.
- Adding `--color-purple-*` to `index.css` "in case we need it". We
  don't.
- Hardcoding `oklch(...)` inside a component file. Tokens live in
  `index.css`; components consume them.

## Cross-refs

- Implementation: `../styles-and-tokens.md`.
- Status / state usage: `08-interaction-states.md`.
- Chart usage: `07-data-visualization.md`.
- A11y contrast specifics: `14-accessibility.md`.
- Tag color enum: `frontend/src/features/tags/constants.ts` mirrors
  `go/models.TagColor`.
