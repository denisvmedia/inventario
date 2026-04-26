# Iconography & Illustration

## Icon system: Phosphor

**Recommendation: replace lucide-vue-next with `@phosphor-icons/vue`.**

### Why Phosphor over lucide

- **Six weights:** thin / light / regular / bold / fill / duotone. lucide has one (regular). Multi-weight iconography is what makes Linear, Things, 1Password feel "designed" rather than "generic".
- **Humanist, slightly warmer character** — fits Inventario's tone. lucide's icons are technically excellent but emotionally flat.
- **Same coverage** (~1200 icons) and equivalent bundle behavior (tree-shaken, ~250 bytes per icon).
- **Open source, MIT.**

Migration cost: a single sed pass plus per-icon name mapping. ~half-day with audits.

### Icon weight semantics

| Weight | Use case |
| --- | --- |
| `thin` | Decorative — not used in product chrome |
| `light` | Empty state large icons (size ≥48px) |
| `regular` | Default — every UI icon |
| `bold` | Active state, selected state, primary navigation indicator |
| `fill` | Status indicators (success check, error X, warning triangle), notification dots |
| `duotone` | Reserved, do not use without designer review |

**Rule:** within one composition, mix at most two weights (e.g., regular + bold for nav active indicator). Three weights in a single screen is visually noisy.

### Icon size scale

```css
@theme {
  --size-icon-xs:   12px;   /* dense tables, micro hints */
  --size-icon-sm:   14px;   /* tag/badge inline */
  --size-icon-md:   16px;   /* default — buttons, inputs, list items */
  --size-icon-lg:   20px;   /* navigation, large buttons */
  --size-icon-xl:   24px;   /* page title accent, prominent action */
  --size-icon-2xl:  32px;   /* card hero, file-type indicator */
  --size-icon-3xl:  48px;   /* empty state */
  --size-icon-4xl:  64px;   /* hero / onboarding */
}
```

### Icon stroke behavior

Phosphor icons render as outline strokes; `stroke-width` is not adjustable per-instance (weight controls boldness instead). Set color via `currentColor`:

```vue
<Phosphor :icon="House" weight="regular" :size="16" />
<!-- color inherits from text-* class -->
```

### Icon-text alignment

When pairing an icon with text inline (button, list item):

- Icon size = body line-height × 0.8 (e.g., 14px icon next to 1rem body)
- Vertical alignment: optical-center, not strict baseline. Tweak per icon if needed.
- Gap between icon and text: `--space-2` (8px) for body; `--space-1_5` (6px) for body-sm; `--space-3` (12px) for buttons.

### Icon-only buttons

Always paired with `aria-label` and (preferably) `<title>` for tooltip on hover:

```vue
<Button variant="ghost" size="icon" aria-label="Edit location">
  <Phosphor :icon="PencilSimple" />
</Button>
```

**Do not** rely on visual icon meaning alone for primary actions. Critical actions (delete, archive) get text labels even if redundant.

## Illustration strategy

This is the one area where the brief defers most to your decision because it depends on budget.

### Where illustrations are used in Inventario

Illustrations appear in **finite, intentional places**. Not as decoration scattered throughout.

1. **Login / register** — single hero illustration, sets the tone
2. **Onboarding** — 3 illustrations across the use-case selection screens (Home / Collection / Property)
3. **Empty states** — one per major surface (no items, no files, no exports, no search results, no notifications)
4. **Error pages** — 404, 500, "out of storage", "no internet"
5. **Backup/export success** — small celebratory illustration

That's ~12–15 illustrations in total. Manageable.

### Sourcing strategy — confirmed

**Strategy: AI-generated via ChatGPT Images 2.0 (`gpt-image-2`).** Concrete prompts and workflow are in `22-illustration-prompts.md`.

Why this is the right answer for the project:
- ChatGPT Images 2.0's multi-image consistency (up to 8 coherent images per Thinking-mode prompt) gets us a stylistically locked set in 4–5 batches
- Reference-image input (up to 10 per prompt) lets us anchor every batch to a single style-anchor image — variants can't drift
- Inpainting allows surgical fixes on individual elements without re-rolling whole compositions
- Total cost: ~$5–15 for the full 21-illustration set (vs €500–1200 for commissioned)
- Iteration speed: hours, not weeks
- Result quality: with proper anchoring + Thinking mode, materially better than DALL-E 3 era — actually production-grade for a personal-tool product

The illustrator-commissioning path (kept here for reference) is a fine fallback if AI-generated outputs don't reach the desired quality bar after iteration. Cost is ~€500–1200 for 8–15 custom illustrations from a designer on Dribbble/Fiverr.

### Stopgap if generation produces no usable assets

Use large Phosphor icons (`light` weight, 64–96px) inside circular tinted backgrounds as illustration placeholders. Honest, restrained, doesn't pretend to be more than it is.

```
.empty-state-icon {
  width: 96px; height: 96px; border-radius: var(--radius-full);
  background: var(--accent-soft);
  display: grid; place-items: center;
  color: var(--accent);
}
```

This is the "professional minimalism" path. Looks intentional, not unfinished.

### Illustration palette

Whichever source, illustrations must use **only** colors from the chosen palette direction. No green illustrations on a terracotta product. Provide the illustrator with the chosen palette tokens up front.

### File-type icons (special case)

For file representation (PDF / image / video / audio / archive), Phosphor's regular weight `FileText`, `FileImage`, `FilePdf`, etc. are good. For a slightly more characterful version, consider:

- A thin custom set of 8 file-type icons drawn in the same illustration style — ~€100–200 add-on if commissioning illustrations

Default: ship with Phosphor file icons. Upgrade later.

## Logo / brand mark

The current QR-cube favicon-style logo reads as a placeholder, not a brand. Logo design is in scope of `19-branding.md`. The icon system does **not** include the product logo.

## Decisions made

- Phosphor migration: **confirmed**
- Illustration source: **AI-generated via ChatGPT Images 2.0** (prompts in `22-illustration-prompts.md`)

## What ships in sprint 0

1. Replace lucide imports with Phosphor (one PR, mechanical)
2. Define icon size tokens and audit all `<Icon>` usages to use them
3. Implement `EmptyState` primitive that accepts either an illustration slot or falls back to the Phosphor-in-tinted-circle pattern
4. Document the icon weight semantic rules in component docs
5. Generate priority illustrations (login hero, empty-things, 404, backup-completed) per batch workflow in `22`

Sprint 1 backfills the remaining empty-state, onboarding, and edge-case illustrations.
