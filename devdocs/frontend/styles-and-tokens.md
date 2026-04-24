# Styles and tokens

This document defines how visual style is expressed in this codebase: Tailwind utilities first, design tokens via CSS variables, no SCSS in new code.

## Stack

- **Tailwind CSS v4** — utility classes, `@theme`-block tokens.
- **CSS custom properties** — the actual token implementation. Tailwind classes resolve to `var(--…)`.
- **`class-variance-authority`** (`cva`) — typed variant tables for components.
- **`clsx` + `tailwind-merge`** via the `cn()` helper from `@design/lib/cn` — for combining classes safely.

No SCSS in new code. Existing SCSS in `frontend/src/assets/*.scss` is read-only legacy that gets deleted in Phase 6 ([#1331](https://github.com/denisvmedia/inventario/issues/1331)).

## Tokens

Tokens live in `frontend/src/design/tokens/{colors,spacing,motion}.css` inside `@theme` blocks. They become Tailwind classes automatically:

```css
@theme {
  --color-primary: hsl(122 39% 49%);
  --color-card: hsl(0 0% 100%);
  --color-muted-foreground: hsl(215 16% 47%);
  --radius: 0.5rem;
}
```

```vue
<div class="bg-card text-card-foreground p-4 rounded-md">
  <h3 class="text-foreground">Title</h3>
  <p class="text-muted-foreground">Subtitle</p>
</div>
```

### Token categories

| Category | Tokens (examples) | Where to find |
|---|---|---|
| Color — brand | `--color-brand-50` … `--color-brand-900` | `colors.css` |
| Color — semantic | `--color-background`, `--color-foreground`, `--color-card`, `--color-primary`, `--color-secondary`, `--color-muted`, `--color-destructive`, `--color-success`, `--color-warning`, `--color-border`, `--color-input`, `--color-ring` | `colors.css` |
| Color — status (commodity) | `--color-status-{draft,in-use,sold,lost,disposed,written-off}` | `colors.css` |
| Radius | `--radius` | `colors.css` |
| Spacing scale | `--spacing-card`, `--spacing-section`, `--spacing-page` | `spacing.css` |
| Typography | `--font-sans`, `--text-{xs,sm,base,lg,xl,2xl}` | `spacing.css` |
| Motion | `--ease-out`, `--ease-in-out`, `--duration-fast`, `--duration-normal` | `motion.css` |

### Consuming tokens

- **Inside a component template:** Tailwind utility classes (`bg-card`, `text-foreground`, `rounded-md`).
- **Inside `cva` definitions in `design/ui/`:** Tailwind utility classes; `var(--…)` only when no Tailwind alias exists.
- **Anywhere else (`patterns/`, `views/`, scoped `<style>`):** raw `var(--…)` is forbidden. Add a Tailwind alias if you need one.

## Tailwind utility ordering

The Prettier Tailwind plugin auto-sorts on save. Manual ordering convention (when sorting fails):

1. **Layout** — `flex`, `grid`, `block`, `hidden`, `relative`, `absolute`, `inset-*`.
2. **Box model** — `w-*`, `h-*`, `p-*`, `m-*`, `gap-*`.
3. **Typography** — `text-*`, `font-*`, `leading-*`, `tracking-*`.
4. **Color** — `bg-*`, `text-<color>-*`, `border-<color>-*`.
5. **Border / radius / shadow** — `border-*`, `rounded-*`, `shadow-*`.
6. **State** — `hover:*`, `focus:*`, `disabled:*`, `dark:*`, `motion-safe:*`.
7. **Misc** — `transition-*`, `cursor-*`, `select-*`.

Long class lists (≥ 6 utilities) — break to multi-line in templates:

```vue
<button
  class="
    inline-flex items-center justify-center
    h-10 px-4 gap-2
    text-sm font-medium
    bg-primary text-primary-foreground
    rounded-md
    hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring
    transition-colors
  "
>
```

## Custom CSS

Forbidden in:

- `frontend/src/views/**/*.vue` (no `<style>` block at all).
- `frontend/src/design/patterns/**/*.vue` (no `<style>` block at all).

Allowed in:

- `frontend/src/design/ui/**/*.vue` — only when shadcn-vue's upstream copy includes a `<style>` block. Keep it as-is.
- `frontend/src/design/tokens/*.css` — the token definitions themselves.

If a Tailwind utility is missing for a use case, add the token (and therefore the utility) — do not write inline CSS.

## Dark mode

Dark mode is a runtime CSS-variable swap. Toggled by setting `data-theme="dark"` on `<html>`:

```css
[data-theme="dark"] {
  --color-background: hsl(222 84% 5%);
  --color-foreground: hsl(210 40% 98%);
  /* … */
}
```

Every new component is verified visually in both themes before merge:

1. Open the dev server.
2. In DevTools Console: `document.documentElement.dataset.theme = 'dark'`.
3. Visually inspect: nothing white-on-white, nothing low-contrast, accent borders still visible, status pills readable.
4. Reset: `document.documentElement.dataset.theme = ''`.

The `useTheme()` composable wraps this in production code.

## Density

Density is a runtime token swap on `data-density`:

```css
[data-density="compact"] {
  --spacing-card: 0.75rem;
  --spacing-section: 1rem;
}
```

Components consume `--spacing-card` via Tailwind classes (`p-card`, where `card` is a registered spacing token), not hard-coded values. This way both density modes work without component-level branching.

## State styles

- **Hover** — only when there is a meaningful affordance (clickable card, button). Never on static text.
- **Focus** — every interactive element has `focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2`. The shadcn `<Button>` already includes this; mirror it on custom interactive elements.
- **Disabled** — `disabled:pointer-events-none disabled:opacity-50` is the standard pair.
- **Active** — `active:scale-[0.98]` only when explicitly designed for; default off.

## Animation

- Default duration: 150–200 ms (use `duration-150` or `duration-200`).
- Default easing: `ease-out` for enter, `ease-in` for exit.
- Honor `prefers-reduced-motion`: prefix transitions with `motion-safe:`.

```vue
<div class="motion-safe:transition-shadow hover:shadow-md">…</div>
```

## Common patterns cheatsheet

```vue
<!-- Card -->
<div class="rounded-md border border-border bg-card p-6 shadow-sm">…</div>

<!-- Card hover -->
<div class="rounded-md border border-border bg-card p-6 shadow-sm motion-safe:transition-shadow hover:shadow-md">…</div>

<!-- Two-column form grid -->
<div class="grid grid-cols-1 gap-4 md:grid-cols-2">…</div>

<!-- Page container -->
<main class="mx-auto max-w-7xl px-4 py-6">…</main>

<!-- Sticky form footer -->
<footer class="sticky bottom-0 border-t border-border bg-background/95 backdrop-blur p-4 -mx-4">…</footer>

<!-- Empty state -->
<div class="flex flex-col items-center gap-3 py-12 text-center text-muted-foreground">…</div>
```
