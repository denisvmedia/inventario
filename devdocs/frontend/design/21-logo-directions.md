# Logo Mark Directions

Historical exploration kept for reference. The shipping mark is the
**bracket-cube** wordmark in `AppLogo.tsx`. The directions below were
considered before settling on it; they exist in this folder as a
record, not as alternatives still on the table.

## The shipping mark

A small bracket glyph wrapping a cube — the bracket signals
"container", the cube signals "the thing inside". Wordmark "Inventario"
to the right in the system sans face.

Live in `frontend/src/components/AppLogo.tsx`. Render rules in
`19-branding.md`.

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

**Verdict:** rejected. Direction 3 (PR #1362's pick) was based on
this, but reads off-tone for the React rewrite.

## Direction 3 — the bracket-cube (shipping)

The current production mark. A square bracket on the left wraps a
small cube; the cube has a faint top facet to read as 3D without
shading.

Pros:

- Domain-resonant — bracket = container, cube = item.
- Reads at favicon size (the bracket alone is recognizable).
- Mode-aware via `currentColor`.
- No third-party trademark / cliché associations.

Cons:

- Requires an SVG, not just a typographic mark.
- The bracket geometry needs hinting for `h-4` rasterization; we live
  with mild aliasing at 16×16.

**Verdict:** committed. See `frontend/src/components/AppLogo.tsx`.

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

## What the shipping mark commits to

- **Bracket-cube glyph.** The single iconographic cue.
- **System sans wordmark.** No custom face. (See `02-typography.md`.)
- **Mode-aware via `currentColor`.** No per-mode SVG variant.
- **No animation.** The mark is static.
- **No watermark / no decorative use** beyond the canonical surfaces
  (`19-branding.md`).

## When to revisit

Re-litigate the mark only when:

- The product positioning (`00-positioning.md`) changes substantially.
- A new product surface emerges where the mark genuinely doesn't
  work (e.g. a native app icon at 1024×1024 — the bracket would need
  a layered treatment).
- A trademark conflict surfaces.

The current cadence: review the mark once a year, ship a refresh PR
only if a substantive reason exists. Don't re-paint just because
fashion shifted.

## Rejected ideas, locked

- **A "playful" mark with a wink.** The product is quiet, not
  playful.
- **A gradient logo.** No gradients (per `04-elevation-and-effects.md`,
  applied to brand assets too).
- **A serif wordmark.** Inventario doesn't have a literary or luxury
  voice.
- **A bilingual mark** ("Inventario / Инвентарь" stacked). The brand
  is one word; locale handles the rest.

## Cross-refs

- Brand rules: `19-branding.md`.
- Component: `frontend/src/components/AppLogo.tsx`.
- Favicon source: `frontend/public/favicon.svg`.
- Voice anchor: `00-positioning.md`.
