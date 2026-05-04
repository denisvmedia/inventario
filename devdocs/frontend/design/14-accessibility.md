# Accessibility

**Floor: WCAG 2.2 AA.** Some surfaces (auth, dashboard, settings)
target AAA where reasonable. A11y is a quality bar; the engineering
contract — Radix primitives, focus-visible rings, jest-axe in unit
tests, `@axe-core/playwright` in e2e — is in [../accessibility.md](../accessibility.md).

This doc is the **design** contract: what each surface promises to a
keyboard / screen-reader / low-vision / motor-impaired user.

## Promises per surface

### Keyboard

Every interactive element is reachable by Tab in DOM order. Skip-to-content
link at the top of every page (mounted in `Shell.tsx`). Modal focus
trapped via Radix; closure restores focus to the trigger.

| Shortcut | Action | Where |
| --- | --- | --- |
| `Tab` / `Shift+Tab` | Move focus | Everywhere |
| `Enter` / `Space` | Activate | Buttons, links |
| `Escape` | Close | Dialogs, dropdowns, popovers, sheets |
| `Cmd+K` (mac) / `Ctrl+K` | Open command palette | App-wide |
| `/` | Focus search input | Pages with search |
| `?` | Open keyboard-shortcut help | App-wide (future) |
| Arrow keys | Navigate within menus / lists | Radix-handled |

Don't bind `Cmd+S` / `Cmd+Z` etc. — the user's browser owns those.

### Screen reader

- **Every page has exactly one `<h1>`**, set via `<RouteTitle>`.
- **Heading hierarchy is preserved** — `<h2>` for sections, `<h3>` for
  subsections. Don't skip levels.
- **Landmarks** — `<header>`, `<nav>`, `<main>`, `<aside>`, `<footer>`
  via the Shell.
- **Form fields** are labeled programmatically via `<Label htmlFor>`
  or `aria-label`. See [09-component-patterns.md](09-component-patterns.md) ("Input + Label").
- **Live regions** — toasts use `aria-live="polite"`, errors use
  `aria-live="assertive"` (sonner does this; useAppToast preserves
  it).
- **Loading states** — skeleton placeholders should be wrapped in
  `aria-busy="true"` until data resolves.

### Low-vision

- Contrast targets per [01-palette.md](01-palette.md). Body text AAA, UI cues AA.
- Zoom to 200% should reflow without horizontal scroll. Test at
  `Cmd+` × 2.
- Don't convey state by color alone — pair with icon + text.
- `prefers-contrast: more` is honored — see [01-palette.md](01-palette.md) (TBD;
  not yet wired).

### Motor

- **Tap targets ≥ 44×44 px** on touch surfaces. shadcn `Button` is 36
  by default; touch-first surfaces should use `size="lg"` or
  custom-pad.
- **No double-click required.** Single click everywhere.
- **Hover-revealed actions** (kebab on row hover) are also reachable
  via keyboard focus — the `opacity-0 group-hover:opacity-100`
  pattern needs a `focus-within:opacity-100` mirror.
- **Drag-and-drop has a keyboard fallback.** File upload via drop
  zone always pairs with a "Browse" button.

### Cognitive

- **No timed actions.** Logout-on-idle is a server-side decision; no
  countdown banners.
- **Plain language.** Per [12-tone-of-voice-and-copy.md](12-tone-of-voice-and-copy.md).
- **No unexpected motion.** Respect `prefers-reduced-motion: reduce`
  per [05-motion.md](05-motion.md).

## Focus

The 3px amber ring at 50% opacity is the contract. It's tuned for:

- 3:1 contrast against `--background` (light) — passes AA UI.
- 3:1 contrast against `--card` — passes AA UI.
- 3:1 contrast against `--muted` — passes AA UI.

When you build a custom focusable element, never `outline: none`
without applying `focus-visible:ring-[3px] focus-visible:ring-ring/50`
in its place.

## Color contrast

| Pair | Required ratio | Token combo |
| --- | --- | --- |
| Body text | 4.5:1 (AA) / 7:1 (AAA) | `--foreground` on `--background`, `--card` |
| Large text (≥ 18.5px or 14.5px bold) | 3:1 (AA) / 4.5:1 (AAA) | Same |
| UI cues | 3:1 (AA) | `--ring` on adjacent surface, `--border` on adjacent surface |
| Decorative | none required | Free choice within tokens |

The Lighthouse `accessibility` audit catches drift; jest-axe catches
it earlier. See [../perf.md](../perf.md), [../testing.md](../testing.md).

## ARIA patterns

| Pattern | Primitive |
| --- | --- |
| Modal dialog | Radix `<Dialog>` — already wires `role="dialog"`, `aria-labelledby`, `aria-describedby`, focus trap |
| Alert dialog (destructive) | Radix `<AlertDialog>` |
| Menu | Radix `<DropdownMenu>` |
| Combobox | `cmdk` via shadcn `<Command>` |
| Tabs | Radix `<Tabs>` |
| Disclosure (collapsible) | Radix `<Collapsible>` (when copied in) |
| Tree (locations / areas) | Don't roll your own — use a flat list with hierarchical visual cues; see `frontend/src/components/locations/` |

When a primitive doesn't exist (a sortable table, a kanban column),
read WAI-ARIA Authoring Practices first; don't invent a role.

## Tests

- **Unit / integration**: jest-axe, no critical/serious violations.
  See [../testing.md](../testing.md).
- **End-to-end**: `@axe-core/playwright`, `serious`+`critical` floor.
  See `e2e/utils/axe.ts`.
- **Lighthouse**: `accessibility ≥ 0.95` per [../perf.md](../perf.md).

If an axe violation is intentional (a vendored Radix issue, a known
upstream bug), suppress locally with `runOptions: { rules: { "<rule>":
{ enabled: false } } }` — never lower the global threshold.

## Tooltips

- A tooltip **supplements**, never replaces, a label. Icon-only
  buttons have an `aria-label` *and* a tooltip — the tooltip is for
  sighted hover, the aria-label is for keyboard / SR.
- Tooltips don't fire on focus by default — Radix's tooltip provider
  with `delayDuration={300}` is the standard. Don't lower below 300ms.
- Don't tooltip text that's already visible.

## Forms

- Every field has a `<Label htmlFor>` linked to the input `id`.
- Errors render below the field, with `aria-invalid="true"` on the
  input and `aria-describedby` pointing at the error element's id.
- The submit button stays in tab order even when disabled.
- `autocomplete` attributes set on email / password / name fields.
- Password fields offer a show/hide toggle (see
  `frontend/src/components/auth/PasswordInput.tsx`).

## Modals and overlays

Radix handles:

- Focus trap.
- Escape key closes.
- Backdrop click closes (configurable per dialog).
- Focus restoration to the trigger on close.
- `aria-labelledby` / `aria-describedby` wiring.

Don't override `onPointerDownOutside` / `onEscapeKeyDown` to disable
closure — that breaks user expectations. The legitimate exception: an
unsaved-changes guard that opens a nested confirm AlertDialog before
the parent dialog closes.

## Reduced motion

`prefers-reduced-motion: reduce` disables `tw-animate-css`
animations automatically. Custom transitions opt out via:

```tsx
<div className="transition-colors motion-reduce:transition-none">
```

See [05-motion.md](05-motion.md).

## Reduced data

The auth pages and dashboard render readable on a 3G connection. The
entry-bundle budget (200 KB gzip, see [../perf.md](../perf.md)) is the gate; the
LHCI `accessibility` audit catches "image without alt" and other
classics.

Lighthouse runs with `data-saver` not specifically modeled; the bundle
budget covers it well enough.

## Hard rules

1. **WCAG 2.2 AA** floor everywhere. AAA where it costs nothing.
2. **One `<h1>` per page.**
3. **Every interactive element is keyboard-reachable.**
4. **Every form field has a label.** Programmatic, via `<Label htmlFor>`
   or `aria-label`.
5. **`focus-visible:ring-[3px] focus-visible:ring-ring/50`** is the
   focus ring. Don't redefine.
6. **Color is never alone.** Pair with icon + text.
7. **Tap targets ≥ 44×44** on touch.
8. **No timed actions** the user can't pause / dismiss.

## Anti-patterns

- `<div onClick>` "buttons". Use `<Button>`.
- An icon-only button without `aria-label`.
- A form field with placeholder-as-label.
- Hover-revealed action that's not focus-reachable.
- A modal without `aria-labelledby`.
- "Are you a robot?" CAPTCHAs (use server-side rate limiting).
- Page transitions on route change.
- A timer-driven session expiry banner with no pause.
- A tooltip-only label.

## Cross-refs

- Engineering: [../accessibility.md](../accessibility.md).
- Focus / states: [08-interaction-states.md](08-interaction-states.md).
- Tokens / contrast: [01-palette.md](01-palette.md).
- Reduced motion: [05-motion.md](05-motion.md).
- A11y testing: [../testing.md](../testing.md).
- A11y perf gate: [../perf.md](../perf.md).
