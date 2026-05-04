# Illustration Prompts

Recipe for sourcing the empty-state and onboarding illustrations,
when and if they're commissioned beyond the icon-only minimum that
ships today.

## Status

**Proposal.** Inventario today uses **icon-only empty states** —
`Package` / `Folder` / `Image` glyphs at `size-10` over `bg-primary/10`.
That set is sufficient and ships in production. Richer
illustrations are a future enhancement, not a current dependency.

When the time comes, this doc is the recipe.

## Style

Anchored to the visual contract:

- **Warm-neutral palette** — warm off-white surfaces, near-black warm
  ink, amber accents. No purples, no electric blues, no pastels. (See
  `01-palette.md`.)
- **Line-weight 2px** matching lucide's stroke.
- **Editorial, not playful.** Per `00-positioning.md`.
- **Spot illustrations**, ~120×120 to 160×160 in the layout. No
  full-bleed hero illustrations.
- **No people.** Inventario is about *things* — illustrations show
  containers, items, labels, shelves. Avoid characters.
- **No emoji-style faces on objects.** The cardboard box doesn't
  have eyes.

## Anchor image

Once one illustration ships, use it as the anchor reference for every
subsequent one — consistency-by-anchor. Tools like ChatGPT Images 2.0
(`gpt-image-2`) accept reference images and produce variants in the
same style.

## Surfaces (in priority order)

| # | Surface | What it shows | Tone |
| --- | --- | --- | --- |
| 1 | No items (commodities list empty) | An open cardboard box with a label tag | Considered, mid-light |
| 2 | No locations | A simple house outline | Quiet |
| 3 | No areas | A floor plan rectangle | Neutral |
| 4 | No files (commodity has no files) | A loose receipt + an envelope | Neutral |
| 5 | No tags | A tag pin / hangtag | Neutral |
| 6 | Empty search | A magnifier over a faded grid | Quiet |
| 7 | 404 (page not found) | A loose page on a drift | Quiet |
| 8 | 500 (something went wrong) | A snapped pencil | Apologetic, mid |
| 9 | No-group | A door, slightly ajar | Inviting, neutral |
| 10 | Account deleted | An empty drawer | Final, mid |
| 11 | Onboarding step 1 — welcome | A set of cardboard boxes, neatly arranged | Inviting |
| 12 | Onboarding step 2 — first location | A house with one label | Inviting |
| 13 | Onboarding step 3 — first item | A box with a tag | Inviting |
| 14 | Empty inbox / notifications | A clean desk | Neutral |
| 15 | Empty exports list | A folder, shut, on a shelf | Neutral |
| 16 | No restores yet | An hourglass, paused | Neutral |
| 17 | Maintenance / scheduled downtime | A sign on a rope | Apologetic |
| 18 | Offline | A loose plug | Apologetic |
| 19 | Permission denied | A locked drawer | Final |
| 20 | Verify email | A sealed envelope | Inviting |
| 21 | Forgot password | A loose key | Neutral |

The list captures the surfaces that *would* benefit; the actual
shipped set might be much smaller (1–6).

## Prompt template

For ChatGPT Images 2.0 / equivalent generative tool, use the
following recipe:

```
Style: Editorial line illustration, 2px stroke, warm off-white background
(#FAF7EF, OKLCH equivalent in 01-palette.md). Near-black warm ink for
strokes (#2B2520). One amber accent (#D9933D) used sparingly — at most
one element per illustration. No fills except the amber accent.
No drop shadows. No gradients. No people. No emoji faces on objects.

Subject: <e.g., "an open cardboard box with a paper label tag tied
by twine, viewed from a 3/4 angle">

Composition: Centered subject within a 160×160 viewport, ~30% padding
on all sides. The subject reads at 64×64 (favicon-style decimation).

Reference: Use the anchor image at frontend/src/assets/illustrations/anchor.svg
to match line weight, perspective, and corner-radius treatment.

Output: SVG (preferred) or PNG at 320×320 for crisp 2× rendering.
```

Tweak the `Subject:` line per surface. Keep everything else verbatim.

## Sourcing channels

Three viable paths, in descending order of preference:

1. **Commission from a human illustrator.** Most consistent, most
   expensive. Vendor brief: this doc + the anchor image.
2. **Generative AI (ChatGPT Images 2.0, Midjourney, similar)** with
   the anchor-driven workflow. Cheap, mid consistency. The PR #1362
   recipe estimated $5–15 for 21 illustrations; the same likely
   holds.
3. **Pick from an open library** (unDraw, Open Doodles, lucide
   Studio's illustration kit when it lands). Free, lowest
   consistency.

Whichever channel, all illustrations go through the same review:

- Color values match the tokens (eyedrop with the spec at
  `01-palette.md`).
- Stroke weight matches the anchor.
- No people, no emoji-faces, no drop shadows.
- Reads at 64×64.
- License is clear and assignable.

## File format

- **SVG** preferred — small, mode-aware via `currentColor`,
  arbitrary scaling.
- **PNG** at 1×, 2×, 3× when SVG isn't viable (e.g. AI-generated
  raster).
- Color tokens **inlined as CSS variables** (`var(--accent)`) when the
  SVG is hand-authored, so dark mode swaps automatically.

## Storage

`frontend/src/assets/illustrations/`. One file per illustration, named
after the surface (`empty-items.svg`, `not-found.svg`, …).

## Hard rules

1. **Anchor image is the constraint.** Every new illustration is a
   variant of the same line / palette / tone — never a one-off.
2. **No people, no faces on objects.**
3. **Tokens, not raw colors.** The amber accent is `--accent`, not
   `#D9933D`. (The hex above is approximate for prompt purposes —
   the canonical OKLCH lives in `01-palette.md`.)
4. **Commission, generate, or borrow — pick one channel per batch.**
   Mixing channels per surface guarantees inconsistency.
5. **Optional, not required.** The icon-only empty state is the
   shipping minimum. Don't block features on illustration delivery.

## Anti-patterns

- A one-off "fun" illustration on the empty state for one feature.
- Illustrations with characters / mascots.
- Drop shadows / glows.
- Gradient fills inside illustrations.
- Color outside the tokens (a "purple receipt" on the empty-files
  state).
- Animated illustrations. (Per `05-motion.md`, the empty state is
  quiet.)

## Cross-refs

- Anchor: `00-positioning.md` (tone), `01-palette.md` (color).
- Iconography: `06-iconography-and-illustration.md`.
- Empty-state placement: `08-interaction-states.md`,
  `20-edge-cases.md`.
- Brand: `19-branding.md`.
- Future component: `frontend/src/components/illustrations/` (TBD).
