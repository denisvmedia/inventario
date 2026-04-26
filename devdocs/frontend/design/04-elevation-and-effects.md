# Elevation & Effects

Shadows, opacity, blur, and layering — the depth language of the product.

## Elevation philosophy

Inventario is **mostly flat**. Shadows are used sparingly and intentionally:

- A list of cards on a page: **no shadow at rest**, subtle shadow on hover only
- A floating element (popover, dropdown, toast): clear shadow
- A dialog/modal: strong shadow + overlay
- Inline pills, badges, tags: never shadowed

If everything has a shadow, nothing does. Aim for ~3 elevation levels visible on a typical screen, never more.

## Shadow scale

Five steps. Each shadow is a layered stack (1px hairline + soft drop) — never a single blob, that looks 2010.

```css
@theme {
  /* Light mode shadows */
  --shadow-xs:
    0 1px 0 0 rgb(0 0 0 / 0.04),
    0 1px 2px 0 rgb(0 0 0 / 0.04);

  --shadow-sm:
    0 1px 0 0 rgb(0 0 0 / 0.04),
    0 2px 4px -1px rgb(0 0 0 / 0.06),
    0 1px 2px 0 rgb(0 0 0 / 0.04);

  --shadow-md:
    0 1px 0 0 rgb(0 0 0 / 0.04),
    0 4px 8px -2px rgb(0 0 0 / 0.08),
    0 2px 4px -2px rgb(0 0 0 / 0.06);

  --shadow-lg:
    0 2px 0 0 rgb(0 0 0 / 0.04),
    0 12px 24px -6px rgb(0 0 0 / 0.12),
    0 4px 8px -4px rgb(0 0 0 / 0.08);

  --shadow-xl:
    0 4px 0 0 rgb(0 0 0 / 0.04),
    0 24px 48px -12px rgb(0 0 0 / 0.18),
    0 8px 16px -8px rgb(0 0 0 / 0.1);
}

/* Dark mode shadows — rely on inner-glow or border, not drop-shadow */
.dark {
  --shadow-xs:
    inset 0 1px 0 0 rgb(255 255 255 / 0.04);

  --shadow-sm:
    inset 0 1px 0 0 rgb(255 255 255 / 0.04),
    0 2px 4px 0 rgb(0 0 0 / 0.4);

  --shadow-md:
    inset 0 1px 0 0 rgb(255 255 255 / 0.05),
    0 4px 12px 0 rgb(0 0 0 / 0.5);

  --shadow-lg:
    inset 0 1px 0 0 rgb(255 255 255 / 0.06),
    0 12px 32px 0 rgb(0 0 0 / 0.6);

  --shadow-xl:
    inset 0 1px 0 0 rgb(255 255 255 / 0.08),
    0 24px 64px 0 rgb(0 0 0 / 0.7);
}
```

## Shadow semantic mapping

| Element | At rest | Hover/active |
| --- | --- | --- |
| Card in list | none | `xs` |
| Card on dashboard | `xs` | `sm` |
| Button (default variant) | none | none |
| Button (raised variant, used for primary CTA) | `xs` | `sm` |
| Dropdown, popover, tooltip | `md` | — |
| Toast | `lg` | — |
| Dialog | `xl` + overlay | — |
| Floating action button (rare) | `lg` | `xl` |

**Avoid:** shadow on inputs, on table rows, on accordion panels at rest, on full-bleed page sections.

## Overlay (scrim) tokens

For modals, mobile drawers, lightbox.

```css
--overlay-scrim:        rgb(20 16 12 / 0.55);   /* tinted with palette warmth */
--overlay-scrim-strong: rgb(20 16 12 / 0.75);   /* file viewer, photo lightbox */
--overlay-scrim-light:  rgb(20 16 12 / 0.32);   /* mobile drawer */

.dark {
  --overlay-scrim:        rgb(0 0 0 / 0.7);
  --overlay-scrim-strong: rgb(0 0 0 / 0.85);
  --overlay-scrim-light:  rgb(0 0 0 / 0.5);
}
```

The scrim color is **tinted** with palette warmth (per Direction A: rgb(28 23 17), per B: rgb(15 18 23), per C: rgb(20 30 28)) — a true black scrim looks harsh against warm interiors.

## Blur (backdrop-filter)

Used sparingly. Apply to:

- File viewer overlay (when image is showing): scrim 0.55 + `backdrop-filter: blur(8px)` for a focus-pulling effect
- Sticky table headers when scrolled: subtle blur for read-through
- Mobile pull-to-refresh state

```css
--blur-sm: 4px;    /* sticky headers */
--blur-md: 8px;    /* lightbox */
--blur-lg: 16px;   /* deep focus overlays */
```

**Performance gate:** never apply backdrop-filter to elements that animate position. Use a separate non-animated layer.

## Opacity scale

Discrete steps — avoid arbitrary `opacity-43`.

```css
--opacity-disabled: 0.4;
--opacity-muted:    0.6;   /* secondary state UI */
--opacity-faded:    0.8;   /* nearly-fully-opaque, used for hover-out */
--opacity-full:     1;
```

For **disabled** elements, prefer color shifts (`--ink-disabled`, `--surface-sunken`) over opacity reduction. Opacity-only disabled states are hard to read against busy backgrounds.

## Border-as-elevation

In dark mode, prefer a subtle inset highlight + 1px border over drop shadow for raised surfaces:

```css
.card {
  background: var(--surface-raised);
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-xs);  /* inset hairline in dark mode */
  border-radius: var(--radius-lg);
}
```

This pattern (border + inset hairline) is what gives Linear, Vercel, Things 3, etc. their "considered" feel in dark mode. Drop shadows in dark mode read as muddy.

## Focus ring

Focus rings are an elevation/effect concern, not a color one alone. Specified here:

```css
--focus-ring: 0 0 0 2px var(--surface-base), 0 0 0 4px var(--accent);
```

Two-step ring: a 2px halo of the surface color, then 2px of accent. Result: ring "floats" with a thin moat between element and ring — modern and accessible. **Default browser outline disabled globally**, replaced with this token.

**Focus rule:** every focusable element gets the focus ring. No `outline: none` without a replacement. Audit every PR.

## Selection highlight

Custom `::selection` rule using palette accent at low opacity:

```css
::selection {
  background: color-mix(in srgb, var(--accent) 22%, transparent);
  color: var(--ink-primary);
}
```

A small detail that signals product care.

## Glass / frost

Inventario does **not** use glass/frost surfaces in v1. They date quickly (peak 2020) and often fail accessibility contrast. Reserved for a future moment if compelling.

## Gradient usage

Inventario does **not** use decorative gradients in v1. The palette is rich enough flat. Gradient is permitted in exactly two places:

1. **Page background subtle wash** — single linear gradient from `--surface-base` to `--surface-sunken` at 0.5% per 100vh, almost invisible. Optional.
2. **Charts** — area-chart fill gradients from accent to transparent. Documented in `07-data-visualization.md`.

No CTA gradients. No "vibrant hero" gradients. No mesh gradients.

## Noise / grain

Optional, single-direction call: a 0.4% noise overlay applied via SVG-encoded data-URL on the body background can add tactility to the warm-domestic palette (Direction A specifically). If used, it's a constant — not animated, not toggled.

```css
body::before {
  content: "";
  position: fixed; inset: 0; pointer-events: none; z-index: 0;
  background-image: url("data:image/svg+xml,..."); /* tiny noise SVG */
  opacity: 0.04;
  mix-blend-mode: overlay;
}
```

**Verdict:** include only if Direction A is chosen, and ship it from day 1 — adding noise later feels gimmicky. Skip for B and C.

## Cursor language

Default `cursor: default`. Override:

- `cursor: pointer` on every interactive element (button, link, card with @click handler)
- `cursor: text` on text inputs and content-editable
- `cursor: grab` / `grabbing` on draggable thumbnails (file gallery reorder)
- `cursor: zoom-in` on file gallery thumbnails (hint of lightbox)
- `cursor: crosshair` on draw/measure tools (none currently)

Audit pass: any `<button>`, `<a>`, or element with `@click` must have `cursor: pointer` or its replacement.
