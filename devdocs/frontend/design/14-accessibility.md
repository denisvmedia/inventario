# Accessibility

Inventario commits to **WCAG 2.2 AA** as the floor. Some surfaces (dashboards, file viewer) target AAA where reasonable. Accessibility is not a feature — it's a quality bar that catches a lot of design defects (low contrast, missing keyboard support, opaque labels) before they ship.

## Minimum bars

| Bar | Standard |
| --- | --- |
| Color contrast | WCAG 2.2 AA — body text 4.5:1, large text (18px+ or 14px bold) 3:1 |
| Focus visibility | Custom focus ring per `04-elevation-and-effects.md`, never `outline: none` without replacement |
| Touch targets | 44×44 CSS pixels minimum, per WCAG 2.5.5 |
| Keyboard support | All interactive elements reachable and operable via keyboard alone |
| Screen reader | Major flows readable by VoiceOver / NVDA / JAWS |
| Motion | `prefers-reduced-motion: reduce` respected |
| Zoom | UI works at 200% browser zoom without horizontal scroll |
| Language | `<html lang>` set, language-switch announced |

## Color contrast rules

Per palette direction, **every token combination is pre-checked** before committing. The contrast pairs that must pass:

| Foreground | Background | Min ratio |
| --- | --- | --- |
| `--ink-primary` | `--surface-base` | 7:1 (AAA) |
| `--ink-primary` | `--surface-raised` | 7:1 |
| `--ink-secondary` | `--surface-base` | 4.5:1 (AA) |
| `--ink-muted` | `--surface-base` | 4.5:1 (text), 3:1 (decorative) |
| `--ink-disabled` | `--surface-base` | 3:1 — explicitly NOT meant to pass body text contrast (signals disabled) |
| `--accent-foreground` | `--accent` | 4.5:1 |
| `--ink-primary` | `--accent-soft` | 7:1 |
| Status text | semantic-soft bg | 4.5:1 each |

**Validation:** ship a CI check that asserts all token pairs pass ratio thresholds. Use `chroma.js` or similar in a unit test.

**The current dark-on-dark section header bug** is a 1:1 violation — basically invisible. Fix this on day one.

## Focus management

### Focus ring

Single token applied universally:

```css
:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}
```

`focus-visible` (not `focus`) so mouse clicks don't show the ring. Keyboard focus does.

### Focus traps

Used in:
- Dialogs (Reka UI handles via `FocusScope`)
- File viewer fullscreen
- Mobile drawers
- Command palette

When the trap is removed, focus returns to the element that opened it (Reka UI default).

### Initial focus

| Surface | Initial focus |
| --- | --- |
| Form view | First field |
| Dialog | First field, or Cancel button if confirmation dialog |
| Search opened | Search input |
| File viewer opened | Close button (so Esc and Tab both work intuitively) |
| Page navigation | Skip-to-content target → `<main>` |

### Skip links

A "Skip to content" link at the top of every page, hidden until focused via Tab:

```html
<a href="#main-content" class="skip-link">Skip to content</a>
```

```css
.skip-link {
  position: absolute; top: -40px; left: var(--space-4);
  padding: var(--space-2) var(--space-3);
  background: var(--accent); color: var(--accent-foreground);
  border-radius: var(--radius-md);
  z-index: 100;
}
.skip-link:focus { top: var(--space-4); }
```

## Keyboard shortcuts

Global:

| Shortcut | Action |
| --- | --- |
| `⌘K` / `Ctrl+K` | Open command palette / global search |
| `?` | Show keyboard shortcut reference |
| `g h` | Go to Home (dashboard) |
| `g t` | Go to Things |
| `g p` | Go to Places |
| `g f` | Go to Files |
| `g b` | Go to Backups |
| `n t` | New thing |
| `n p` | New place |
| `n f` | Upload file |
| `Esc` | Close dialog / drawer / lightbox |

Per-surface:

| Surface | Shortcuts |
| --- | --- |
| List view | `/` focus search, `j/k` next/prev item, `Enter` open item, `e` edit selected, `Del` delete selected |
| File viewer | `←/→` prev/next, `+/-` zoom, `0` reset, `r` rotate, `Space` play (video), `Esc` close |
| Form view | `⌘S` save, `Esc` cancel |
| Multi-select | `Shift+Click` range, `⌘+Click` toggle |

Shortcut reference accessible via `?` key — modal showing all available shortcuts grouped by section.

## Screen reader patterns

### Landmarks

Every page has:
- `<header>` (top app bar / sidebar)
- `<nav>` for primary navigation
- `<main id="main-content">` for primary content (skip-link target)
- `<aside>` for sidebars, related content
- `<footer>` if present

### Headings

- One `<h1>` per page = page title
- `<h2>` for top-level sections
- `<h3>` for subsections
- No skipping levels (no `<h1>` → `<h3>`)

### Live regions

Async updates announced via:

```html
<div aria-live="polite" aria-atomic="true">
  Saved.
</div>
```

For toasts: `role="status"` (info/success), `role="alert"` (warning/error).

For loading: `aria-busy="true"` on the container being loaded.

### Hidden content

- `aria-hidden="true"` on decorative icons (Phosphor icons that have a label nearby)
- Off-screen labels: `<span class="sr-only">Edit location</span>` for icon-only buttons that already have visible context

```css
.sr-only {
  position: absolute; width: 1px; height: 1px;
  padding: 0; margin: -1px; overflow: hidden;
  clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0;
}
```

## Form accessibility

- Every input has a `<label>` programmatically associated (`for=` + `id=`)
- Required fields: `aria-required="true"` (`*` symbol is decorative)
- Errors: `aria-invalid="true"` on field, error message has `id` and field has `aria-describedby` pointing to it
- Helper text follows the same pattern (field `aria-describedby` points to helper)
- Field groups (radio, checkbox lists) wrapped in `<fieldset><legend>...`

## Status communication

Color is **never** the only signal:
- Success: green + check icon + text
- Error: red + X icon + text
- Warning: amber + warning icon + text

Color-blind users (~8% of men, ~0.5% of women) get the icon/text. AAA aim.

## Touch and mobile

- Tap targets 44×44 CSS pixels minimum
- Spacing between targets at least 8px to prevent fat-finger errors
- Drag handles visible enough to spot (dotted/grip icon, ≥24px target)
- Swipe-to-dismiss confirmable — irreversible swipes get a confirmation toast with Undo

## Reduced motion

Per `05-motion.md`, `prefers-reduced-motion: reduce` respected globally. Critical UI states must be readable in static form (no element invisible until animation completes).

## Reduced transparency

`prefers-reduced-transparency: reduce` (newer media query) considered:
- Backdrop-blur effects fall back to solid `--surface-base`
- Frosted overlays go opaque

## Animation thresholds

Per WCAG 2.2.2:
- No element flashes >3 times per second
- No autoplay video with audio
- Looping animations have a way to pause (or auto-pause after 5s)

## Zoom and reflow

UI must function at:
- 200% browser zoom
- 320×256 effective viewport (1280px viewport at 400% zoom)

This means: no fixed-width layouts, no fixed font sizes (use rem/em), no horizontal scrolling at desktop sizes after zoom.

## Language switching

`<html lang>` updates instantly when locale changes. Language switch announced via `aria-live` region.

## Forms with sensitive data

Insurance values, addresses, identification fields:
- `autocomplete` attribute set correctly per WCAG 1.3.5
- `inputmode` for numeric / decimal / email
- Password fields with reveal toggle (per `09-component-patterns.md`)
- Never auto-zoom on iOS (use `font-size: 16px` minimum on inputs)

## Testing strategy

- **Lint:** `eslint-plugin-vuejs-accessibility` for static checks
- **Unit:** axe-core integration in Vitest for component test suites
- **E2E:** axe-core run as part of Playwright tests (`@axe-core/playwright`)
- **Manual:** quarterly screen reader pass on golden-path flows (sign in, add thing, upload file, view file)
- **Manual:** quarterly keyboard-only navigation pass

## Anti-patterns

- ❌ "Click here" link text (give the link a meaningful name)
- ❌ Icon-only buttons without `aria-label`
- ❌ `placeholder` as the only label
- ❌ Color as the only state indicator
- ❌ `outline: none` without focus replacement
- ❌ `tabindex="-1"` on naturally focusable elements just to avoid a focus ring
- ❌ Modals that don't trap focus
- ❌ Auto-focus on page load on non-form pages
- ❌ Toasts that disappear before screen readers can finish reading them
- ❌ Required fields marked only by red color, no asterisk or text
- ❌ "Read more" / "Continue" buttons with no context (out of context, screen readers can't tell what's continuing)

## Compliance reporting

Maintain `docs/accessibility-statement.md` listing:
- Standard claimed (WCAG 2.2 AA)
- Known gaps (with planned fixes)
- Contact for accessibility issues
- Last audit date

This is required in some jurisdictions (EU, UK, US public sector); good practice everywhere.
