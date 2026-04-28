# Illustration Prompts (drop-in for ChatGPT Images 2.0)

These are ready-to-paste prompts for generating the full illustration set described in `06-iconography-and-illustration.md`. They use a **shared style preamble** so every output looks like part of one coherent set.

Generation tool: **ChatGPT Images 2.0** (`gpt-image-2`), launched 21 April 2026, replaced DALL-E. The instructions and prompt structure below assume its capabilities — Thinking mode, multi-image consistency, reference images, inpainting.

## What ChatGPT Images 2.0 changes for our workflow

The new model has four capabilities that materially change how to generate this set:

1. **Multi-image consistency in one prompt — up to 8 images** with shared style and character coherence. Use this to generate **batches** instead of one-at-a-time. Result: dramatically tighter visual cohesion across the set.
2. **Reference images, up to 10 per prompt.** Feed the existing logo, a palette swatch, and one strong already-generated illustration as references for every subsequent batch. The model anchors the style.
3. **Thinking mode** — model researches, plans the composition, reasons through layout before drawing. Use it for everything in this set; Instant mode is for throwaway exploration only.
4. **Inpainting with masks** — when one element of an output is wrong (e.g., a stray gradient, a misaligned shape), mask it and regenerate just that region. Don't re-roll the whole image.

These are not just speed wins — they enable a level of cohesion that DALL-E 3 couldn't reach.

## Recommended workflow

### Step 1 — Build a "style anchor" image

Generate **one** strong image first using only the style preamble (no scene). Call it the **anchor**. Save it. Every subsequent prompt feeds this anchor as a reference image. The model will inherit color, line weight, texture, mood automatically — much more reliably than re-describing the style each time.

Anchor prompt (use Thinking mode, no reference images):

> [STYLE PREAMBLE — see below]
> Subject: a single rectangular label tag with a corner notch and a small round hole near the corner, hanging slightly tilted. The tag bottom third is filled in terracotta. This is a style-anchor reference: composition is centered, the subject fills 60% of the frame, generous breathing room, paper-grain texture clearly visible. Format: 1:1 square.

Save the result as `anchor.png`. **Use this anchor as a reference image (`image_input`) for all batch prompts below.**

### Step 2 — Generate scenes in batches

ChatGPT Images 2.0 accepts a batch request — describe several scenes in one prompt, ask the model to render them as a sheet of N coherent images. Each batch gets the **anchor** as a reference for style continuity.

Example batch request (light-mode empty states):

> Reference image attached: `anchor.png` (use as the canonical style reference for this set).
>
> [STYLE PREAMBLE — see below]
>
> Generate 6 separate images at 1:1 square format, all in the exact style of the reference. Each is for a different empty-state surface. Use Thinking mode to plan layout and composition. Keep style, color, line weight, and texture consistent across all 6.
>
> Scene 1 — Empty list (no things):
> [scene description from below]
>
> Scene 2 — Empty list (no places):
> [scene description from below]
>
> ... (up to 8 scenes per batch)

ChatGPT Images 2.0 will return all images as a coherent set, each separately downloadable.

### Step 3 — Iterate via inpainting if needed

If a single image in a batch has one wrong element (e.g., the bird in the 404 illustration ended up too cartoonish), don't re-roll the whole image. Use ChatGPT's edit feature with a mask over just that element:

> Edit attached image: replace the bird in the masked region with a simpler silhouette in the same line weight as the rest of the illustration. Keep terracotta accent on the bird only.

This preserves the rest of the image (which was good) and fixes the one defect.

### Step 4 — Generate dark-mode counterparts

After the light-mode set is locked, swap the palette in the preamble (see "Dark-mode preamble override" near the end) and re-batch the same scenes. Use the locked light-mode versions **as reference images** so the dark-mode versions inherit the same composition and just shift color.

## Style preamble (paste before every scene; never modify wording)

> A flat warm illustration with subtle paper-grain texture, in the style of editorial book illustrations and modern archival design. Two-color palette only: warm cream `#FAF6EC` as background, deep navy blue `#1A2238` for primary subjects and outlines (medium-thick lines, 2 to 3 pixels at 2K resolution, with rounded ends), with a single sparing terracotta `#B8451F` accent on one focal detail per image. Mood: calm, considered, archival, slightly nostalgic but not retro. NO human characters, NO faces. NO text or letters anywhere in the image (text rendering is not desired here even though the model is capable — keep illustrations purely iconographic). NO heavy drop shadows. NO gradients except the subtle background paper grain. Composition: centered, generous breathing room, the subject fills about 60% of the frame. Vector-feeling but with a slight risograph / printed-paper grain texture, as if printed with two ink runs on slightly textured cream paper. Square 1:1 format at 2K resolution unless otherwise specified.

Why this preamble works on `gpt-image-2`:
- Specific hex values let the model match palette exactly
- Pixel-weight specification keeps line consistency across batches
- "Two ink runs on textured paper" is a concrete printing metaphor the model understands
- Explicit "no text" instruction (the model can render text well now, but we don't want it here)
- 2K resolution requested explicitly — defaults vary

## Scenes — Onboarding (batch together, 5 images)

### 01 · Login hero
> A quiet domestic scene — a small wooden writing desk in cross-section view, slightly isometric, with a single open notebook on it, a desk lamp with warm light pooling on the page, and to the side a small set of three drawer-style storage compartments. Terracotta accent on the lampshade glow only. Composition emphasizes warmth and quiet record-keeping. The desk has a single small label tag attached to one drawer.

### 02 · Onboarding choice — "Stuff in my home"
> A cross-section view of a stylized small house showing four rooms (living, kitchen, bedroom, storage), each room containing one or two iconic objects (a chair, a stove, a bed, a box). Each object has a tiny tag attached suggesting it is catalogued. Terracotta accent on the house roof only. Calm, ordered, like a museum diorama.

### 03 · Onboarding choice — "My collection"
> A flat top-down view of a wooden display surface with five varied collectible objects laid out in a deliberate arrangement — an open book, a single coin, a small framed postage stamp, a wine bottle laid on its side, and a vintage pocket watch. Each object rendered with calm restraint. Terracotta accent on the wine bottle's neck label only. Symmetrical, considered composition, like a curated vitrine.

### 04 · Onboarding choice — "A property's documentation"
> A flat architectural blueprint-style view of a small building (top-down floor plan), with simple labeled rooms and a thick folder of papers placed beside it. The folder has a small terracotta accent strip on its spine. Subtle dimension lines around the floor plan. Calm, technical without being cold.

### 05 · Welcome / first thing prompt
> A single lit candle on a small wooden table next to an empty open journal/notebook with blank pages. Composition suggests "beginning a record." Candle flame is the only terracotta accent. Quiet, inviting, slightly warm.

## Scenes — Empty states (batch together, 6 images)

### 06 · Empty list (no things yet)
> A single empty wooden drawer viewed from above at a slight angle, pulled open from a dark cabinet. Drawer interior is a soft cream color, completely empty, ready to receive items. Visible woodgrain rendered as simple line strokes. A small terracotta tag attached to the front edge. Calm, inviting, NOT lonely.

### 07 · Empty list (no places yet)
> An empty room in cross-section (floor + walls visible, no ceiling), with empty floor and walls, completely uninhabited but warmly lit. A single key with a terracotta tag rests on the floor center. Suggests "a place ready to be defined." Architectural lines, calm.

### 08 · Empty list (no files yet)
> An empty manila folder with a single ribbon tie, slightly open showing it's empty inside, sitting on a cream surface. Ribbon tie in terracotta. Calm, organized.

### 09 · Empty list (no backups yet)
> A small wooden chest with its lid slightly ajar, sitting on a cream surface. Chest has a brass-like clasp rendered in terracotta. Inside the chest, a soft glow suggests "ready to keep things safe." Reassuring, NOT magical or fantasy.

### 10 · Search — no results
> A magnifying glass laid flat on top of a single empty piece of paper or card. Magnifying glass handle has a terracotta accent ring. Suggests "looked, nothing here." Patient, NOT failure.

### 11 · Filter — no results
> A row of three small filing folder tabs in cream tone, with the middle tab slightly raised as if currently selected. No content visible behind the tabs — area is empty. The middle (selected) tab has a small terracotta dot.

## Scenes — Error and edge (batch together, 4 images)

### 12 · 404 page
> A small empty drawer pulled fully out from a storage unit, with a label slot on its front that's blank (rectangular outline only). Drawer empty. A small bird (sparrow silhouette, simplified) perches on the corner of the drawer. The bird is the only terracotta accent. Calm, gently melancholic.

### 13 · 500 page
> A small stack of papers slightly toppled, with three or four sheets fanning out from the stack on a cream surface. Bottom sheet has a tiny terracotta corner. Suggests "something tipped over" without being chaotic. Restrained.

### 14 · Maintenance / paused
> A wooden door with a small hanging tag from the doorknob, the tag completely blank. Beside the door, a small lit candle in a holder. Candle flame is terracotta. Door closed but warmly lit. Mood: "briefly closed, will return."

### 15 · Offline / disconnected
> A single unplugged power cord lying on a cream surface, plug end gently curled. Plug prongs have small terracotta tips. Suggests "disconnected" without alarm.

## Scenes — Success and reassurance (batch together, 3 images)

### 16 · Backup completed
> A small wooden box with its lid closed and a brass clasp latched, sitting on a cream surface. A folded paper or document peeks slightly from under the lid. Clasp is terracotta. Subtle glow surrounds the box. Reassuring, slightly celebratory but quiet.

### 17 · Storage warning
> A wooden crate filled almost to the brim with stacked papers, books, and small parcels. One sheet sticks slightly out the top. Topmost item has a terracotta tag. Suggests "getting full." Practical.

### 18 · First item added (celebration)
> A single small folded-paper origami crane resting on top of a closed wooden box, on a cream surface. Crane has a small terracotta marking. Quiet gentle delight, "one thing in and you've started." NOT cartoonish.

## Scenes — File / media (batch together, 2 images)

### 19 · Document preview unavailable
> A single document or paper rolled up like a scroll, tied with a small ribbon. Ribbon is terracotta. Scroll rests on a cream surface. Suggests "wrapped up, can't see inside." Calm, archival.

### 20 · File too large
> A small parcel that is clearly too big for the small wooden crate beside it. Parcel has a terracotta tape strip across it. Suggests "doesn't fit." Practical, NOT alarming.

## OG / share card (separate prompt — different format)

### 21 · Marketing OG card / social share
> Different format for this one: 16:9 wide composition, NOT 1:1 square. Same style preamble. Subject: a layered scene of warmly-lit small storage objects — a stack of two boxes, an open notebook in front, a small floating label tag, and a single drawer pulled slightly out. Composition occupies the right two-thirds of the frame; left third is empty cream space (where a wordmark and tagline would overlay in production). Terracotta accents on one tag and one box latch. 1920×1080 resolution.

## Suggested batch organization (4 batches total)

This minimizes drift and maximizes cohesion. Run each batch as a single multi-image request to ChatGPT Images 2.0 in Thinking mode with `anchor.png` attached as reference.

| Batch | Scenes | Count |
| --- | --- | --- |
| Batch A | Onboarding (01–05) | 5 |
| Batch B | Empty states (06–11) | 6 |
| Batch C | Error & edge (12–15) | 4 |
| Batch D | Success + media (16–20) | 5 |
| Solo | OG card (21) | 1 |

Total: 21 illustrations across 5 generation cycles. Realistically 1–2 hours of work end-to-end including review and inpainting fixes.

## Dark-mode preamble override

For dark-mode versions, replace the preamble's color description block with:

> ...two-color palette only: deep navy `#14191F` as background, warm cream `#F1ECE0` for primary subjects and outlines, with a single sparing terracotta `#E07F4F` accent on one focal detail per image.

Generate dark-mode versions in a second pass, **using the locked light-mode versions as reference images** for each scene. The model will preserve composition and shift only the color treatment.

## Quality control — before committing the set

Lay all 20+ illustrations side-by-side in Figma (or your screen tool of choice). Check:

1. **Line weight is consistent.** All outlines look like the same pen.
2. **Texture is consistent.** Paper grain visible to the same degree across all.
3. **Terracotta usage is consistent.** Each illustration has exactly one terracotta accent on one focal element.
4. **No accidental text** in any illustration (model occasionally adds it).
5. **No accidental humans / faces.** Reject any with hands, partial figures, etc.
6. **Mood reads as one product.** No outliers that feel cartoony, photoreal, or stylistically off.

Regenerate any outliers using **inpainting** (mask the off element) rather than re-rolling the full image. If an outlier is wholesale wrong, re-batch with a new reference image (the strongest sibling in the batch).

## API workflow (optional, for automation)

If you want to script generation:

```bash
# Pseudo-API call structure
POST https://api.openai.com/v1/images/generate
{
  "model": "gpt-image-2",
  "mode": "thinking",
  "prompt": "[full prompt with preamble + scene]",
  "reference_images": ["anchor.png"],
  "size": "2048x2048",
  "n": 6
}
```

Pricing (approximate, as of 21 April 2026):
- Input: $8 per million tokens
- Output: $30 per million tokens (image)
- Generation cost per illustration: rough order of $0.10–0.30 each in Thinking mode

For a 21-image set with a few iterations: ~$5–15 total. Cheaper than commissioning, infinitely cheaper than a stock subscription.

## Out of scope

- Animated illustrations (Lottie / Rive) — v2
- Marketing / landing page illustrations beyond OG card — when marketing site exists
- Hero illustrations for blog posts / docs — when content surfaces exist
- Per-locale illustration variants — illustrations are language-neutral by design (no text in them)
