# Accessibility

The shadcn primitives ship with sensible a11y defaults; the rules below
keep the rest of the codebase from undoing them. Lighthouse, jest-axe,
and `@axe-core/playwright` all run in CI — see [perf.md](perf.md) and
[testing.md](testing.md) for the gates.

## Targets

| Surface | Tool | Threshold |
| --- | --- | --- |
| Unit / integration | `jest-axe` | No violations at `critical` or `serious` |
| End-to-end | `@axe-core/playwright` | Same — see `e2e/utils/axe.ts` |
| In-browser audit | Lighthouse `accessibility` | ≥ 0.95 |

If a violation is genuinely intentional (a vendored Radix primitive's
known issue), suppress it locally with `runOptions` rather than skipping
the test or weakening the threshold globally.

## Focus

- **Never `outline: none` without a replacement.** shadcn primitives use
  `focus-visible:ring-[3px] focus-visible:ring-ring/50` — keep it. The
  ring color (`--ring`) is amber in light mode and a brighter amber in
  dark; both meet 3:1 contrast against their adjacent surfaces.
- **`focus-visible:`, not `focus:`.** Visible focus rings appear only for
  keyboard users; mouse-clickers don't see the ring around a button they
  just pressed. shadcn does this correctly out of the box; mirror it
  when you write a new interactive element.
- **Trap focus inside modals.** Radix `<Dialog>`, `<AlertDialog>`,
  `<Sheet>`, `<DropdownMenu>` already do this. Don't roll your own
  modal — use the primitive.
- **Initial focus.** Radix returns focus to the trigger when the modal
  closes. If you need a specific control to focus on open, use Radix's
  `onOpenAutoFocus` / `<DialogContent autoFocus={false}>` and ref the
  target manually.

## Labels

- **Every form field has a programmatic label.** Either:
  ```tsx
  <Label htmlFor="email">{t("auth:login.email")}</Label>
  <Input id="email" {...form.register("email")} />
  ```
  or use `aria-label` when the visual label is intentionally absent
  (e.g. a search box where the placeholder is the only cue).
- **`<label>` and `htmlFor` are always paired.** A floating `<label>`
  above an input without `htmlFor` is a11y debt. ESLint's
  `jsx-a11y/label-has-associated-control` catches the obvious cases —
  see `frontend/eslint.config.js`.
- **Icon-only controls need an explicit label.** `<Button size="icon">`
  must include an `aria-label`. See [icons.md](icons.md).
- **Form errors associate via `aria-invalid` + `aria-describedby`.** The
  shadcn `Field`/`FieldError` primitives wire this; for plain
  `Input + Label`, set `aria-invalid={!!error}` and render the error
  text in an element with the same `id` referenced via
  `aria-describedby`.

## Modals

Use Radix primitives:

| Need | Primitive |
| --- | --- |
| Generic dialog with form | `Dialog` (`@/components/ui/dialog`) |
| Destructive confirmation | `AlertDialog` (`@/components/ui/alert-dialog` via `useConfirm()`) |
| Side-sheet (item detail, settings preview) | `Sheet` (`@/components/ui/sheet`) |
| Popover (combobox, date picker) | `Popover` |
| Dropdown menu (kebab actions) | `DropdownMenu` |

Rules:

- **Title + description.** Always render `<DialogTitle>` and
  `<DialogDescription>`. Radix wires them to `aria-labelledby` /
  `aria-describedby` on the dialog content. If you don't render a
  title, axe complains.
- **Close on Esc and backdrop click.** Radix does this; don't override
  `onPointerDownOutside` / `onEscapeKeyDown` to disable closure unless
  you have a reason (an unsaved-changes guard, e.g.).
- **No `window.confirm()` / `alert()`.** Use `<AlertDialog>` or
  `useConfirm()`. Native dialogs aren't styleable, aren't translatable,
  and break in tests.

## Color and contrast

The OKLCH tokens are tuned for WCAG AA contrast (4.5:1 for body text,
3:1 for UI cues):

- `text-foreground` on `bg-background` — body text.
- `text-muted-foreground` on `bg-card` — secondary text. Verified at
  light + dark.
- `text-destructive` on `bg-card` — error text.

Hard rules:

- **Never convey error / success state by color alone.** Pair with
  icon + text:
  ```tsx
  <div className="flex items-center gap-2">
    <CheckCircle2 className="size-4 text-status-active" />
    <span className="text-sm">{t("…success")}</span>
  </div>
  ```
- **Don't drop opacity below ~0.6 for active text.**
  `text-muted-foreground/70` reads as decorative; below that you've
  failed contrast in dark mode.
- **Tokens, not raw colors.** A new contrast pair is a token PR —
  update both `:root` and `.dark`. See
  [styles-and-tokens.md](styles-and-tokens.md).

## Reduced motion

The user's `prefers-reduced-motion: reduce` preference disables the
animations that ship with `tw-animate-css` (it gates them on
`@media (prefers-reduced-motion: no-preference)` automatically).
When you write a new transition:

```tsx
<div className="transition-colors motion-reduce:transition-none">…</div>
```

— or just lean on `tw-animate-css` utilities (`animate-in`, `fade-in-0`,
`zoom-in-95`), which already respect the preference.

Don't fall back to JS-driven animations (Framer Motion, react-spring) —
none of them are in the bundle, and Tailwind v4 + `tw-animate-css`
covers every animation the design uses.

## Disabled state

- Use the `disabled` prop on buttons / inputs — never `pointer-events:
  none` or `opacity-50` to fake-disable. Radix and shadcn handle the
  attribute, ARIA state, and focus semantics together.
- A disabled submit button still must be reachable by Tab. Don't move
  focus over it; don't `tabindex={-1}` it.

## Tooltips

- **Tooltips supplement, never replace, a label.** A `Tooltip` on an
  icon-only button is an enhancement; the `aria-label` is still required.
- **Don't tooltip text that's already visible.** Lighthouse and axe
  flag redundant tooltips.

## Tests

Every page-level component test runs `axe(container)`:

```tsx
import { axe } from "jest-axe"

it("has no axe violations", async () => {
  const { container } = renderWithProviders({ children: <Page /> })
  expect(await axe(container)).toHaveNoViolations()
})
```

End-to-end: import `axeAudit` from `e2e/utils/axe.ts` and call it
(`await axeAudit(page)`) inside the relevant `test()` block. The default
severity floor is `serious` + `critical`. See [testing.md](testing.md).

## Anti-patterns

- `outline: none` on `:focus` without a `:focus-visible` replacement.
- A new modal built from `<div className="fixed inset-0">` instead of
  `<Dialog>`.
- `onClick` on a `<div>` to make it "clickable". Use `<Button>` or a
  real `<a>`.
- Color-only error indication (red border, no icon, no text).
- `tabindex={-1}` on a focus target the user is supposed to reach.
- Skipping a label "because the placeholder says it" — placeholders
  disappear on focus and aren't read by screen readers as labels.
