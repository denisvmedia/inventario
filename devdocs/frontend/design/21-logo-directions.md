# Logo Mark — Directions and AI Prompts

The wordmark direction (geometric sans-serif, deep navy on warm cream, sentence-cased) is settled. The **icon mark** is where to keep iterating — the current "three rounded squares in a triangular formation" reads as a competent productivity logo, but it's close to generic. This document offers six concrete directions (five icon-mark variants plus one pure-wordmark fallback) to push it more distinctive, each with an AI image-generation prompt ready to drop into ChatGPT Images 2.0 (`gpt-image-2`) or Midjourney.

## ChatGPT Images 2.0 workflow notes for logo iteration

The current existing logo file (the three-block mark + Inventario wordmark on cream) should be **uploaded as a reference image** for every prompt below. This locks the model onto the existing palette, line weight, and overall visual register, so variations feel like evolutions of the existing brand rather than wholly different identities.

Use **Thinking mode** for logo generation. The mark needs careful composition and optical balance — Instant mode produces faster but compositionally weaker results.

When iterating, request **batches of 4–8 variants per direction** in a single Thinking-mode prompt. ChatGPT Images 2.0's multi-image consistency guarantees the variants will share line weight, color, and texture — making them easier to compare side-by-side. Don't generate one at a time; you'll just get drift between attempts.

Resolution: request 2K (2048×2048). The chosen mark gets traced into a vector tool (Figma / Illustrator) anyway, but high resolution helps you spot detail issues.

**Model now renders text reliably** — meaning Direction 6 (pure wordmark with custom letter modification) is now genuinely achievable through generation, where on DALL-E 3 the result was usually unusable. Even custom letterform tweaks render legibly.

**Recommendation: confirmed Direction 3 (catalog tag).** Refine via reference-image-driven iteration. Selected direction marked below.

## Brief for any mark direction

Constraints that all six directions must respect:

- **Single color** in primary form: deep navy `#1A2238` on warm cream `#FAF6EC`
- **Two-color** secondary form allowed: navy + terracotta `#B8451F` accent
- **Vector-friendly** geometry — readable as a 16×16 favicon
- **No internal text** — favicons can't render type at small size
- **Tells a story** — the mark suggests "things kept safely / catalogued / organized" without being literal
- **Ages well** — no hyper-trendy effects (no neumorphism, no aggressive gradients, no thin-line illustration trend)

## Direction 1 — Refined three-block (evolve the current concept)

**Concept:** keep the three rounded squares but add micro-detail that disambiguates the logo from the dozens of "three boxes" productivity logos. Options:
- One block has a tiny horizontal slot near its top edge — suggests a label slot. Tells story: "labelled storage."
- One block is slightly inset / nested — suggests "things inside."
- Blocks have a subtle 1-pixel inner offset (like a ledger or label card edge).

**Why it works:** preserves the recognition you've built up; adds a single specific detail that turns "generic" into "specific."

**ChatGPT Images 2.0 prompt** (use Thinking mode for best composition):

> A minimalist vector-style logo mark consisting of three rounded square shapes arranged in a triangular formation (one centered above, two below at the corners). Deep navy blue color (hex #1A2238) on a warm cream background (hex #FAF6EC). Each square has clean rounded corners. The bottom-left square has a small horizontal notch or slot cut into its top edge, suggesting a label slot on a storage box. Geometric, calm, modern. No shadows, no gradients, no text. Square format, centered composition, generous breathing room. Flat vector illustration.

**Variations to try in iterations:**
- Move the slot to a different square
- Try the slot as a thin negative-space line instead of a notch
- Try one square with a smaller square nested inside (two-tone — small inner square in terracotta)
- Try the three blocks with rounded corners of slightly different radii (subtle hierarchy)

## Direction 2 — Drawer / vessel (object metaphor)

**Concept:** a single container shape, viewed from above, suggesting "where things are kept." Could be:
- An open box top-down with a smaller offset rectangle inside (an item placed in)
- A flat tray with three small dividers (organized compartments)
- An open drawer pulled out from a unit

**Why it works:** more specific to the inventory metaphor than three blocks; tells a clearer story; still abstract.

**ChatGPT Images 2.0 prompt** (use Thinking mode for best composition):

> A minimalist vector logo mark depicting a square-shaped open box viewed from directly above (top-down geometric view). Deep navy blue (hex #1A2238) outline on warm cream background (hex #FAF6EC). Inside the box, two smaller rounded rectangles of different sizes sit at slight angles, suggesting items neatly placed inside. The outer box has rounded corners, clean 2px line weight. The smaller inner shape on the right is filled with terracotta orange (hex #B8451F) for a single accent. Flat vector style, no shadows, no gradient, no text. Square format, generous padding around the mark. Calm, archival, modern.

**Variations:**
- Try drawer-pulled-out (rectangular, with a small handle)
- Try a tray with three compartments (one filled, two empty)
- Try a top-down "open lid" reveal (corner of a square folded/lifted)

## Direction 3 — Catalog tag / index card  ✅ **SELECTED**

**Concept:** the iconography of organized cataloguing — a label tag with a string hole, a library index card with a corner notch. Quietly archival, museum-adjacent.

**Why it works:** evokes the cataloguing intent of the product directly; reads as "every thing has a tag." Familiar imagery (library, museum) without being cliché.

**ChatGPT Images 2.0 prompt** (use Thinking mode for best composition):

> A minimalist vector logo mark depicting a rectangular label tag with rounded corners. The tag has one diagonally-cut corner on the upper-left (the classic library tag shape) and a small circular hole punched near the cut corner for a string. Deep navy blue (hex #1A2238) outline with subtle 2px stroke, on warm cream background (hex #FAF6EC). The tag is shown straight-on, slightly tilted at maybe 5 degrees for character. The bottom third of the tag interior is filled with terracotta (hex #B8451F) suggesting a labeled section, while the rest stays cream. Flat vector style, no shadows, no gradients. The tag is the only object in the composition. Square format, centered, generous padding. No text on the tag.

**Variations:**
- Try multiple tags overlapping (set of three slightly fanned)
- Try a single index card (square with a horizontal line near the top, the line in terracotta)
- Try a tag with a wraparound thread/string visible

## Direction 4 — Letter-mark "i" (or "I")

**Concept:** the letter "i" stylized into a glyph that doubles as object iconography. Things 3 took this approach with an italicized lowercase "i" that reads as both letter and pencil.

**Why it works:** instantly identifiable favicon; ties to the wordmark; can incorporate domain symbolism (the dot becoming a label, the stem becoming a drawer).

**ChatGPT Images 2.0 prompt** (use Thinking mode for best composition):

> A minimalist vector logo mark of a single lowercase letter "i" set in a geometric sans-serif construction. Deep navy blue (hex #1A2238) on warm cream background (hex #FAF6EC). The dot of the "i" is replaced with a small rounded square (matching the rest of the mark in stroke weight) — this small square is filled in terracotta (hex #B8451F) for a single accent. The stem of the "i" has rounded ends. The letter is centered with generous padding. Flat vector style, no shadows. Square format. No additional text or decoration.

**Variations:**
- Try with capital "I" and a small horizontal serif at top and bottom that reads as a label slot
- Try the dot as a tiny rectangle instead of a square (more "label-like")
- Try negative space — the "i" cut out of a filled rounded square shape

## Direction 5 — Stacked/nested rectangles (vertical drawer)

**Concept:** three or four horizontal rectangles stacked vertically — reads as "drawer cabinet" or "filing system." More dimensional than the current triangular composition.

**Why it works:** unambiguously domain-relevant (filing/drawer system), distinctive shape (vertical rectangle is rarer than horizontal triangle), still simple.

**ChatGPT Images 2.0 prompt** (use Thinking mode for best composition):

> A minimalist vector logo mark of a vertical filing-cabinet-like icon, viewed straight-on. Three horizontal rounded rectangles stacked vertically with small gaps between them, contained loosely within an implied vertical frame. The middle rectangle has a small horizontal handle/tab in its center suggesting a drawer pull. Deep navy blue (hex #1A2238) on warm cream background (hex #FAF6EC). The handle/tab on the middle drawer is filled with terracotta (hex #B8451F) as a single accent. Clean rounded corners, 2px line weight where strokes appear. Flat vector style, no shadows, no gradient. Square format, centered with generous padding. Geometric, calm, archival.

**Variations:**
- Try four drawers instead of three
- Try one drawer slightly pulled out (offset to the right)
- Try a wider "cabinet" composition (more rectangular than square)

## Direction 6 — Pure type with one distinctive letter

**Concept:** drop the icon mark entirely; let a wordmark with one custom letterform carry the brand. Favicon = the custom letter alone.

**Why it works:** lowest cost, most timeless, very on-trend with editorial brands (think Are.na, Cabin, Kinfolk wordmark-only approach). Risks blending in if the type isn't distinctive enough.

**ChatGPT Images 2.0 prompt** (use Thinking mode for best composition):

> A wordmark logo (typography only, no icon) for a personal-inventory brand. Set in a refined geometric sans-serif typeface, weight medium, sentence-cased. Deep navy blue (hex #1A2238) on warm cream background (hex #FAF6EC). The letter "k" in the wordmark has a custom modification: its diagonal arm extends slightly past the leg with a small rounded terminal, suggesting a tiny label slot or pull-tab. Terracotta (hex #B8451F) on this single modified element only. The rest of the wordmark is straightforward, calm, evenly tracked. Centered composition. Flat vector style, no shadows.

(Substitute the actual brand wordmark when you generate; the description focuses on what to do with one letter.)

## How to use these prompts

1. **Attach the existing logo as a reference image** in every prompt. Locks palette/line weight automatically.
2. **Iterate batch-style.** Generate 4–8 variations per prompt in Thinking mode. ChatGPT Images 2.0's multi-image consistency means siblings stay coherent — pick the strongest 1–2 from each direction.
3. **If one variant is almost-right, use inpainting.** Mask the off element and ask for a regeneration of just that region. Don't re-roll the entire mark.
4. **Bring the strongest to a vector tool** (Figma, Illustrator, Sketch). AI gen is rasterized; redraw the chosen mark as clean vector for production. This is also where you fine-tune optical balance.
5. **Test at every size** — mark must work at 16×16 favicon and 256×256 app icon. Direction 4 (letter-mark) typically scales best; Direction 3 (tag) and Direction 5 (drawer cabinet) need careful detail tuning at 16px.
6. **Test on dark mode** by inverting cream and navy in a follow-up prompt (use the chosen light-mode mark as the reference image for the dark-mode generation). The mark should read both ways.
7. **Test in monochrome** (single-color print). The terracotta accent must be optional — the mark should still read in pure navy or pure black. Use inpainting to remove the accent for the monochrome test.

## Selected direction

**Direction 3 — Catalog tag** is confirmed. Refinement happens via the prompt above with reference-image-driven iteration in ChatGPT Images 2.0.

Why this choice over the others:
- The current three-block mark reads as generic productivity (could be Kanban / CRM / project manager). Tag-based mark specifically tells the cataloguing story.
- Strong scalability: the silhouette reads at 16×16 favicon size when geometry is kept clean.
- Distinct from competitors (1Password's key, Things' "i", Notion's "N", Airtable's grid) — no other personal-tool product owns the tag/label visual identity.
- Pairs cleanly with the existing wordmark direction without competing.

## What ships in sprint 0

The current placeholder mark stays in the app. Logo iteration is a side workstream — generate variations on your own time, pick a direction, then a one-line CSS/asset swap brings the new mark in.

Sprint 0 logo work limits to:
1. Wordmark cleanup — kerning, weight, sizing, lockup with mark
2. Favicon set generated from current mark (ICO + SVG + apple-touch-icon)
3. App theme-color meta tags using `--accent`
4. OG card template with current mark + tagline
