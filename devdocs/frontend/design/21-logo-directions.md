# Logo Mark Directions

Historical exploration kept for reference. The shipping mark is the
**house-with-checklist** glyph in `AppLogo.tsx`. The directions below
were considered before settling on it; they exist in this folder as a
record, not as alternatives still on the table.

## The shipping mark

A stylized house silhouette with a small checklist inside it: the
house signals "your place / your belongings", the checklist signals
"catalog". Wordmark "Inventario" to the right in the system sans face.

Authored as inline SVG at an `18×18` viewBox, with the silhouette in
`fill-foreground` and the checklist details cut out via
`fill-background` so the mark inverts cleanly across light / dark.

Live in `frontend/src/components/AppLogo.tsx`. Render rules in
[19-branding.md](19-branding.md).

The directions below were the alternatives weighed during exploration.
None ship today; they're preserved as a record of options considered.

## Direction 1 — sans wordmark only

The simplest possible mark: just "Inventario" in a system semibold,
tracking-tight, slight ligature on the "io".

Pros:

- Zero asset cost.
- Reads at any size.
- Mode-aware via `currentColor`.

Cons:

- No glyph for the favicon.
- No iconographic recognition cue.

**Verdict:** rejected. Favicon + sidebar collapsed need a glyph.

## Direction 2 — the catalog tag

A small tag silhouette (the tag-and-string shape from `Tag` in
lucide). Pairs with the wordmark.

Pros:

- Domain-resonant — Inventario is about labeling.
- Strong recognition at small sizes.

Cons:

- Conflates the brand mark with the in-app `Tag` icon. The `tags`
  feature uses `Tag` heavily; the brand reading "tags" too is
  confusing.
- The tag silhouette reads as e-commerce / pricing. Inventario isn't
  e-commerce.

**Verdict:** rejected. PR #1362 had originally explored a related
catalog-tag direction; the React rewrite picked a different glyph
altogether.

## Direction 3 — the bracket-cube

A square bracket on the left wrapping a small cube; the cube has a
faint top facet to read as 3D without shading.

Pros:

- Domain-resonant — bracket = container, cube = item.
- Reads at favicon size (the bracket alone is recognizable).
- No third-party trademark / cliché associations.

Cons:

- Requires careful SVG hinting at small sizes.
- Reads slightly cold / generic next to the warm-neutral palette.

**Verdict:** rejected. Direction 7 (the house-with-checklist) was
chosen instead.

## Direction 4 — the stacked-shelf

Three horizontal lines with rounded caps, suggesting shelves. Wordmark
to the right.

Pros:

- Clean, geometric, reads at small sizes.

Cons:

- Reads as "to-do list" or "menu hamburger" — both UI-mark cliches.
- Conflicts visually with the `Menu` icon in lucide.

**Verdict:** rejected.

## Direction 5 — the inventory grid

A 2×2 grid of small squares.

Pros:

- Direct domain reference (a catalog grid).
- Geometric.

Cons:

- Reads as Excel / Google Workspace / generic-app-icon.
- Loses character at small sizes.

**Verdict:** rejected.

## Direction 6 — the warm rectangle

A solid warm-cream rounded rectangle with the wordmark inside,
centered. The "Things 3 / Aesop" container shape.

Pros:

- Reads as considered, editorial.
- Pairs with the warm palette.

Cons:

- Doesn't scale to a favicon (the rectangle is the mark; at 16×16 the
  text is illegible).
- Looks like a button.

**Verdict:** rejected.

## Direction 7 — house-with-checklist (shipping)

A stylized house silhouette with a small checklist composed inside
it. Both the silhouette and the cut-out details resolve through
theme tokens (`fill-foreground` / `fill-background`).

Pros:

- Domain-resonant — the house reads as "your place"; the checklist
  reads as "catalog".
- Warmer than abstract geometric marks; pairs with the warm-neutral
  palette.
- Reads at favicon size (the silhouette alone is recognizable).
- Mode-aware without two SVG sources — the cut-out detail
  automatically inverts.
- No third-party trademark / cliché associations.

Cons:

- More illustrative than a pure geometric mark — slightly less
  abstract, slightly more on-the-nose.

**Verdict:** committed. See `frontend/src/components/AppLogo.tsx`.

## What the shipping mark commits to

- **House-with-checklist glyph.** The single iconographic cue.
- **System sans wordmark.** No custom face. (See [02-typography.md](02-typography.md).)
- **Mode-aware via `fill-foreground` / `fill-background` tokens.** No
  per-mode SVG variant.
- **No animation.** The mark is static.
- **No watermark / no decorative use** beyond the canonical surfaces
  ([19-branding.md](19-branding.md)).

## When to revisit

Re-litigate the mark only when:

- The product positioning ([00-positioning.md](00-positioning.md))
  changes substantially.
- A new product surface emerges where the mark genuinely doesn't
  work (e.g. a native app icon at 1024×1024 — the silhouette would
  need a layered treatment).
- A trademark conflict surfaces.

The current cadence: review the mark once a year, ship a refresh PR
only if a substantive reason exists. Don't re-paint just because
fashion shifted.

## Rejected ideas, locked

- **A "playful" mark with a wink.** The product is quiet, not
  playful.
- **A gradient logo.** No gradients (per [04-elevation-and-effects.md](04-elevation-and-effects.md),
  applied to brand assets too).
- **A serif wordmark.** Inventario doesn't have a literary or luxury
  voice.
- **A bilingual mark** ("Inventario / Инвентарь" stacked). The brand
  is one word; locale handles the rest.

## Cross-refs

- Brand rules: [19-branding.md](19-branding.md).
- Component: `frontend/src/components/AppLogo.tsx`.
- Favicon source: `frontend/public/favicon.svg`.
- Voice anchor: [00-positioning.md](00-positioning.md).
