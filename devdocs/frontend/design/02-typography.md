# Typography

Inventario's typography carries 70% of the modernity. Get the scale, weights, and pairings right and the product reads as considered even before color choice lands.

## Pairing recommendation

### Primary: Switzer (display) + Inter (body)

**Why:** Switzer is a free, geometric-humanist sans by Indian Type Foundry with warm character and seven weights. Pairs cleanly with Inter for body — both are open-source, both have wide language coverage including Cyrillic, both ship as variable fonts. Total bundle weight ~80KB woff2 with subsetting. Free for commercial use.

```
Display: Switzer (variable, weights 100–900)
Body:    Inter (variable, weights 100–900)
Mono:    JetBrains Mono (only for IDs, keys, code-ish content)
```

### Alternate A: GT Walsheim (display) + Söhne (body) — **paid, premium**

If you want a more distinctive personality and have a budget for Klim/Grilli Type licenses (~€600–1500 one-time for self-hosted). GT Walsheim brings warmth without sweetness; Söhne is the modern grotesk standard (Stripe, Vercel, OpenAI). The product would feel two notches more premium.

### Alternate B: Inter only — **safe fallback**

If you don't want a separate display face, use Inter for everything with weight + size + tracking variation carrying hierarchy. This is what many shadcn-vue products do. Looks fine, lacks personality. Suitable if Direction B (Quiet Practical) palette is chosen and the goal is "tool-like" not "product-with-character."

## Type scale

A 10-step scale, not a Tailwind default. Mathematical base 1.125 (minor third) on body; display steps drift to 1.2 (minor third upper) for visual punch.

```css
@theme {
  /* Display — only for marketing/empty-state heroes, almost never in-product chrome */
  --text-display-2xl: 4rem;        /* 64px */
  --text-display-xl:  3rem;        /* 48px */
  --text-display-lg:  2.25rem;     /* 36px */

  /* Headings — page titles, section headers */
  --text-heading-xl:  1.875rem;    /* 30px — page title */
  --text-heading-lg:  1.5rem;      /* 24px — major section */
  --text-heading-md:  1.25rem;     /* 20px — sub-section */
  --text-heading-sm:  1.0625rem;   /* 17px — group label */

  /* Body */
  --text-body-lg:     1.0625rem;   /* 17px — long-form content, hero subhead */
  --text-body:        0.9375rem;   /* 15px — default UI body */
  --text-body-sm:     0.8125rem;   /* 13px — secondary, hints, captions */
  --text-body-xs:     0.75rem;     /* 12px — only for badges, metadata, labels */

  /* Mono */
  --text-mono:        0.875rem;    /* 14px — IDs, file paths */
}
```

## Line-height (leading) tokens

Display gets tight leading; body needs loose-ish leading for inventory text (often long product names).

```css
--leading-display: 1.05;   /* display sizes */
--leading-tight:   1.2;    /* headings */
--leading-snug:    1.4;    /* short labels */
--leading-normal:  1.55;   /* body default */
--leading-relaxed: 1.7;    /* body-lg long-form */
```

## Tracking (letter-spacing) tokens

Display tightens; body stays neutral; small uppercase labels widen.

```css
--tracking-display:    -0.04em;    /* display */
--tracking-heading-xl: -0.025em;
--tracking-heading-lg: -0.018em;
--tracking-heading-md: -0.012em;
--tracking-heading-sm: -0.005em;
--tracking-body:        0;
--tracking-body-sm:     0.005em;
--tracking-uppercase:   0.06em;    /* used on small caps labels, status pills */
```

## Weight tokens

Switzer weights map clearly:

```css
--font-weight-light:    300;
--font-weight-regular:  400;
--font-weight-medium:   500;
--font-weight-semibold: 600;
--font-weight-bold:     700;
```

**Weight rules:**
- Body text: `regular` (400). Never lighter for content.
- Body emphasis: `medium` (500), not bold.
- Headings (heading-md and below): `semibold` (600).
- Headings (heading-lg and above): `medium` (500) with tight tracking — modern editorial feel.
- Display: `medium` (500), never bold. Big text + bold = dated/heavy.
- Mono: `regular` (400).

## Hierarchy rules

A page should have **at most**:
- 1 page title (`heading-xl`)
- 3–5 section headers (`heading-lg` or `heading-md`)
- Group labels (`heading-sm` or `body-xs uppercase tracking-uppercase`) as needed
- Body content (`body`)

If you find yourself reaching for `display-*` inside the product, you're decorating an admin page — stop. Display is for empty states, onboarding, and marketing surfaces only.

## Vertical rhythm

Spacing between text elements follows the spacing scale (`03-space-and-layout.md`), but the defaults below produce the right rhythm for most contexts:

| Pair | Default gap |
| --- | --- |
| Heading-xl → body | `space-6` (1.5rem) |
| Heading-lg → body | `space-4` (1rem) |
| Heading-md → body | `space-3` (0.75rem) |
| Heading-sm → body | `space-2` (0.5rem) |
| Body paragraph → body paragraph | `space-3` (0.75rem) |
| Body → caption | `space-1` (0.25rem) |

## Numerals

Inventario displays many prices, counts, and dates. Use **tabular numerals** for any column-aligned numerics:

```css
font-variant-numeric: tabular-nums;
```

Apply via utility class `.tabular` to all `<td>` content, all stat values on dashboard, all "X.XX CZK" displays. Without tabular-nums, prices in lists jitter as users scan. This single rule makes the product feel ten degrees more polished.

## Numerals — currency formatting

Currency values use a non-breaking space between number and code:

```
17 500.00 CZK    not   17500.00CZK or 17,500.00 CZK
```

Decimals always shown for currency, even on whole numbers (`100.00 CZK`, not `100 CZK`). Reasoning: insurance valuations need precision affordance.

## Quotation marks and punctuation

Use **typographic** marks, not ASCII:

- `'` not `'` for apostrophes
- `"…"` not `"…"` for English quotes
- `«…»` for Russian quotes (locale-aware)
- `—` (em dash) not `--` for thought breaks
- `–` (en dash) for ranges (`Apr–May`)
- `…` (ellipsis) not `...`

Implement via a simple sanitizer at render time for user-entered text in display contexts, and never type ASCII versions in microcopy.

## Code/ID display

File IDs, UUIDs, paths use mono. Truncate UUIDs to first 8 + last 4 with middle ellipsis:

```
3fd5fc3e-8e69…b67d
```

Full UUID available on hover via tooltip and copy via icon button.

## Multiline text limits

To prevent jagged edges and runaway lines, set `max-width` ceilings via CSS variable:

```css
--text-measure-prose:  65ch;   /* paragraphs, descriptions */
--text-measure-form:   42ch;   /* form fields, labels */
--text-measure-card:   28ch;   /* card titles, list items */
```

Page content widths in `03-space-and-layout.md` align with these.

## Localization headroom

Russian copy is ~30% longer than English, German is ~20% longer. **Never set fixed widths on text containers** in components that may render translated copy. Test type scale at 130% character density before locking values.

## Accessibility minimums

- No body text below `body-sm` (13px / 0.8125rem). `body-xs` is for badges/metadata only.
- Line-height never below 1.5 for body text.
- Contrast ratios per `14-accessibility.md` apply per palette.
- Never rely on weight alone for emphasis — pair with color shift or marker.

## What gets implemented in sprint 0

1. Tailwind v4 `@theme` block with all `--text-*`, `--leading-*`, `--tracking-*`, `--font-weight-*` tokens above
2. `font-family` setup for Switzer + Inter (or alternate based on your choice) — self-hosted woff2, preloaded
3. Apply `font-variant-numeric: tabular-nums` to global stat/number contexts
4. Replace **all** `text-*` Tailwind utilities in current code with semantic tokens (audit pass)
5. Document the type scale in Storybook/Histoire if/when it lands

## Decision needed

- Switzer + Inter (free, recommended), or
- GT Walsheim + Söhne (paid, premium), or
- Inter-only (safe fallback)
