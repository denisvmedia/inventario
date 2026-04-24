# Accessibility

This document defines the a11y rules every Inventario frontend PR is reviewed against. The bar is WCAG 2.1 AA, AAA where feasible.

## Accessible names

Every interactive non-text element must expose an accessible name to assistive tech:

- A text child counts: `<button>Save</button>`.
- An `aria-label` counts: `<button aria-label="Close"><X /></button>`.
- An `aria-labelledby` pointing at visible text counts.
- An `aria-describedby` is *additional* description, not the name.

Never:

- A bare icon button without `aria-label`. The `<IconButton>` pattern requires `aria-label` at the TS type level.
- Reusing visual placeholder text as the only label (`<input placeholder="Email">` is not labelled).

## Forms

- `<FormLabel>` is paired with `<FormControl>` via the `<FormField>` plumbing — do not write `<label for="id">` by hand.
- Required fields: schema-driven (`z.string().min(1)`) → vee-validate sets `aria-required="true"` and `<FormLabel required>` renders the visual asterisk.
- Errors: `<FormMessage>` is `role="alert"` (provided by shadcn) and is associated with the input via `aria-describedby`. Do not pop toasts for field errors; let `<FormMessage>` carry them.
- Help text: `<FormDescription>` is `aria-describedby`-linked to the input.

## Focus

### Order

DOM order = focus order. No `tabindex` greater than 0. The only acceptable values are `0` (default) and `-1` (programmatic-only focus, e.g. inside a managed roving tabindex group).

### Visible ring

Every focusable element shows a focus ring on keyboard focus. The shadcn `<Button>` ships:

```
focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2
```

Mirror this on any custom interactive element. Never set `outline: none` without providing an alternative ring.

### Modal focus trap

Modals (`<Dialog>`, `<AlertDialog>`, `<Drawer>`, `<Sheet>`) get focus trap, escape-to-close, and return-focus for free from Reka UI.

Forbidden:

- Roll-your-own modal markup (`<div class="modal">…</div>`). Use Reka UI primitives.
- The legacy `FocusOverlay.vue` — to be deleted in Phase 4 PR 4.7 ([#1329](https://github.com/denisvmedia/inventario/issues/1329)).

## Color and state

Color must never be the only signal of state.

- **Status pills** (`<StatusBadge>`, `<CommodityStatusPill>`) always include both an icon and a text label. Color is the third signal.
- **Form errors** combine red border, red text, and an icon (Lucide `AlertCircle` inside `<FormMessage>`).
- **Required marker** is the asterisk (text), not a colored field border.
- **Selected card** has a 2 px ring + a check icon, not just a background tint.

## Contrast

Targets:

- **Body text:** 7:1 (AAA) where feasible; 4.5:1 (AA) minimum.
- **UI text** (button labels, badges): 4.5:1 minimum.
- **Large text** (≥ 18.66 px regular or 14 px bold): 3:1 minimum.
- **Non-text contrast** (icons, focus rings, borders): 3:1 minimum.

Verify in dev:

1. Open the page.
2. DevTools → Lighthouse → Accessibility audit.
3. Or install axe-core DevTools extension and run the panel.

CI runs `axe-core` via Playwright on a sampled subset of routes (added in Phase 5 — see [#1330](https://github.com/denisvmedia/inventario/issues/1330)).

## Reduced motion

Honour `prefers-reduced-motion`. Use Tailwind's `motion-safe:` prefix:

```vue
<div class="motion-safe:transition-shadow hover:shadow-md">…</div>
<div class="motion-safe:hover:-translate-y-0.5 hover:shadow-md">…</div>
```

`motion-safe:transition-*`, `motion-safe:translate-*`, `motion-safe:rotate-*`, `motion-safe:scale-*` are all acceptable.

Animations applied without `motion-safe:` (e.g. a sonner toast slide-in) should be < 200 ms and avoid large translation distances. Long, parallax, or autoplay animations need `motion-safe:` mandatorily.

## Keyboard

Every interactive element is reachable and operable with the keyboard alone. Manual smoke-test before merging a new pattern:

- `Tab` reaches the element in a logical order.
- `Enter` (and `Space` for button-like roles) activates it.
- `Esc` closes overlays.
- For composite widgets (Tabs, Menu, Combobox) — Reka UI handles arrow-key navigation correctly out of the box; do not override.

## Page structure

- One `<h1>` per page (the page title).
- `<h2>` for major sections; `<h3>` for sub-sections. Do not skip levels.
- `<main>` wraps the route content (already the case in `App.vue` after Phase 1).
- `<nav>` for navigation regions (header nav, breadcrumb).
- Landmarks (`<header>`, `<footer>`, `<aside>`) used for their intended purpose only.

## Images

- `<img>` always has an `alt` attribute.
- Decorative images: `alt=""`.
- Informative images: `alt="…descriptive text…"`.
- Image previews in `<FilePreview>` use the file's display title as `alt` when present.

## Documents and PDFs

- The PDF viewer (`PDFViewerCanvas`) is canvas-based — text inside the PDF is not selectable by screen readers from that canvas.
- Always provide a "Download" affordance next to the viewer so users can read the PDF in their preferred reader.

## ARIA — use sparingly

ARIA is a fallback when native semantics are insufficient. Reka UI's primitives already supply correct ARIA for dialogs, tabs, menus, comboboxes, popovers — do not re-decorate them.

If you find yourself adding a `role` or `aria-*` to a Reka UI primitive, you are probably wrong. Open a discussion before merging.
