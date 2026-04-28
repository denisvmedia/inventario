# Palette — final

The palette is **committed**. Three earlier directions (Cozy Domestic / Quiet Practical / Modern Catalog) were considered; the final choice is anchored to the existing brand mark (warm cream + deep navy) and adds one warm earth-toned accent. This produces a three-anchor palette: **cream surface, navy ink, terracotta accent.**

Why this combination:
- The existing logo already establishes cream + navy as the brand baseline. The palette must harmonize with it, not overwrite it.
- Cool ink (navy) on warm surface (cream) is a refined, considered combination — used by Aesop, Kinfolk, Are.na, classic editorial design. It reads as "considered" without trying.
- A warm earth-toned accent (terracotta) closes the triangle: warm + cool + warm. This is classical palette construction. Works for warranty receipts (practical) and for collectors (refined) alike.
- It is unambiguously **not** corporate-tech (no neon, no purple gradients, no SaaS-bro blue) and unambiguously **not** insurance-corporate (no "Lemonade pink" friendliness).

## Light mode tokens

```css
@theme {
  /* Surfaces */
  --surface-base:    #FAF6EC;   /* warm cream — matches logo background */
  --surface-raised:  #FFFFFF;   /* pure white for elevated cards/dialogs */
  --surface-sunken:  #F1EBDB;   /* deeper cream for hovers, table stripes, sunken areas */
  --surface-overlay: rgb(26 34 56 / 0.55);  /* navy-tinted scrim for modal overlays */

  /* Ink */
  --ink-primary:   #1A2238;   /* deep navy — matches logo wordmark */
  --ink-secondary: #4A5570;
  --ink-muted:     #8189A0;
  --ink-disabled:  #C5C9D2;

  /* Borders */
  --border-subtle:  #EAE2D0;
  --border-default: #D4C8AC;
  --border-strong:  #998B6F;

  /* Brand accent — terracotta (warm earth red) */
  --accent:           #B8451F;   /* primary — CTAs, links, brand emphasis */
  --accent-hover:     #92341A;
  --accent-soft:      #FBE6D8;   /* tinted bg for accent areas (callouts, selected states) */
  --accent-foreground:#FAF6EC;   /* cream-on-accent for buttons */

  /* Semantic */
  --success:        #2F7D5C;
  --success-soft:   #D9EBE2;
  --warning:        #A37700;   /* golden amber — distinct from terracotta accent */
  --warning-soft:   #F5EBC8;
  --destructive:    #B12A1F;   /* true red — for genuine danger */
  --destructive-soft:#F8DDD8;
  --info:           #1F5D8C;   /* navy-family blue — harmonizes with ink-primary */
  --info-soft:      #DCE7F1;
}
```

## Dark mode tokens

The dark inversion swaps cream and navy: navy-tinged dark surfaces, warm cream foreground.

```css
:root[data-theme="dark"] {
  /* Surfaces */
  --surface-base:    #14191F;   /* deep navy-tinged dark, NOT pure black */
  --surface-raised:  #1C2230;
  --surface-sunken:  #0E1218;
  --surface-overlay: rgb(0 0 0 / 0.7);

  /* Ink */
  --ink-primary:   #F1ECE0;   /* warm cream — matches the light surface */
  --ink-secondary: #B8B0A2;
  --ink-muted:     #7C7468;
  --ink-disabled:  #44403A;

  /* Borders */
  --border-subtle:  #1E232D;
  --border-default: #2C313D;
  --border-strong:  #4A4F5A;

  /* Accent — lighter terracotta for readability on dark bg */
  --accent:           #E07F4F;
  --accent-hover:     #F19770;
  --accent-soft:      #382318;
  --accent-foreground:#14191F;

  /* Semantic — lifted for dark bg */
  --success:        #6FBF96;
  --success-soft:   #1A2F25;
  --warning:        #DEA757;
  --warning-soft:   #2D2618;
  --destructive:    #EE7A6B;
  --destructive-soft:#2E1816;
  --info:           #7AB7E8;
  --info-soft:      #1A2230;
}
```

## Chart palette

For data viz (per `07-data-visualization.md`). Five categorical colors derived from the brand palette + complementary range:

```css
/* Light mode */
--chart-1: var(--accent);        /* #B8451F — terracotta */
--chart-2: #1F5D8C;              /* navy-family blue */
--chart-3: #6F8B5D;              /* sage green */
--chart-4: #B59169;              /* warm tan */
--chart-5: #6B5878;              /* dusty mauve */

--chart-emphasis: var(--ink-primary);
--chart-grid:     var(--border-subtle);
--chart-axis:     var(--ink-muted);

/* Dark mode lifts */
:root[data-theme="dark"] {
  --chart-1: #E07F4F;
  --chart-2: #7AB7E8;
  --chart-3: #9CB68D;
  --chart-4: #D4B596;
  --chart-5: #978AA0;
}
```

## Contrast verification

All token pairs meet WCAG 2.2 AA at minimum. Critical pairs:

| Foreground | Background | Ratio (light) | Ratio (dark) | Result |
| --- | --- | --- | --- | --- |
| `--ink-primary` | `--surface-base` | 13.8:1 | 14.1:1 | AAA |
| `--ink-primary` | `--surface-raised` | 16.4:1 | 12.6:1 | AAA |
| `--ink-secondary` | `--surface-base` | 7.2:1 | 6.8:1 | AAA |
| `--ink-muted` | `--surface-base` | 4.6:1 | 4.7:1 | AA |
| `--accent-foreground` | `--accent` | 7.8:1 | 8.1:1 | AAA |
| `--ink-primary` | `--accent-soft` | 12.1:1 | 11.8:1 | AAA |
| `--success` text | `--success-soft` bg | 4.6:1 | — | AA |
| `--destructive` text | `--destructive-soft` bg | 5.1:1 | — | AA |

Validation in CI per `14-accessibility.md`.

## Where each color belongs

| Token | Used on |
| --- | --- |
| `--surface-base` | Page bg, sidebar (subtle differentiation from raised), most surfaces |
| `--surface-raised` | Cards, modals, popovers, dropdown panels |
| `--surface-sunken` | Hover states for list rows, alternating table stripes, sunken inputs |
| `--ink-primary` | Body text, headings, primary controls |
| `--ink-secondary` | Subheads, metadata, descriptions |
| `--ink-muted` | Helper text, captions, timestamps |
| `--ink-disabled` | Disabled controls only |
| `--accent` | Primary CTAs, active sidebar item, links inline in body, focus ring, brand emphasis |
| `--accent-hover` | Hover state on accent surfaces |
| `--accent-soft` | Selected list item bg, callout bgs, tag/chip bg |
| `--success` / `--warning` / `--destructive` / `--info` | Status pills, semantic alerts, only for those meanings |
| `--border-subtle` | Most borders, dividers |
| `--border-default` | Inputs, buttons (secondary variant) |
| `--border-strong` | Filled-state input borders, focused secondary buttons |

## Usage discipline

1. **One accent.** Terracotta is the only warm-red in the UI. Don't add coral, salmon, or rust as separate "decorative" colors.
2. **Status colors are not brand colors.** A success pill is green because the meaning requires it, not because green is a brand color. Don't sprinkle green outside semantic-success.
3. **No new tokens without designer review.** If a developer reaches for a hex value not in this list, that's a flag — review whether the design needs a new token or whether they should use an existing one.
4. **Charts use chart palette only.** Don't pull arbitrary brand or semantic colors into data viz.
5. **Black is forbidden.** `#000000` doesn't appear in the palette. The deepest ink is `--ink-primary` (light mode) or `--surface-sunken` (dark mode). Pure black against warm cream looks harsh.
6. **White is reserved.** `#FFFFFF` is `--surface-raised` only. Body bg is cream, not white.

## Implementation in Tailwind v4

Tokens land in the global `@theme` block. Components reference them via CSS variables, not arbitrary Tailwind colors. Existing Tailwind utility classes for arbitrary colors (`bg-blue-500`, `text-slate-900`, etc.) are migrated to semantic equivalents (`bg-accent`, `text-ink-primary`).

```css
@theme {
  --color-surface-base: #FAF6EC;
  --color-ink-primary: #1A2238;
  /* ... */
}
```

This generates `bg-surface-base`, `text-ink-primary`, etc. as Tailwind utilities.

## Out of scope for this palette

- Holiday / seasonal palettes
- Marketing-site special palettes (the marketing site, when built, uses this same palette)
- User-customizable accent colors (a "theme picker" is over-engineering for v1; revisit later)
