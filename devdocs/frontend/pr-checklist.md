# PR checklist

Copy-paste into the body of every frontend PR. Tick the ones you
actually verified. Lines marked **(N/A)** can be deleted if they
genuinely don't apply.

```markdown
### Frontend PR checklist (devdocs/frontend/pr-checklist.md)

#### Code

- [ ] No `forwardRef`. Components accept `ref` via `React.ComponentProps<>` (see devdocs/frontend/coding-standards.md).
- [ ] Named exports only. No default exports outside framework-required files.
- [ ] No new CSS file. Styling is via `index.css` tokens + Tailwind utilities (devdocs/frontend/styles-and-tokens.md).
- [ ] No raw colors (`#hex`, `text-amber-500`, …). Tokens only.
- [ ] No drop shadows beyond `shadow-xs` on inputs.
- [ ] No `console.*` in shipping code (devdocs/frontend/coding-standards.md).
- [ ] Imports follow the documented order (external → `@/` → relative).
- [ ] No banned dependency added (devdocs/frontend/imports-and-bans.md).

#### Components

- [ ] New shadcn primitives (if any) added via `npx shadcn@latest add <name>`, not hand-edited.
- [ ] Cross-feature components live in `src/components/` (or `src/components/<feature>/`); page-local helpers stay in the page file until reused.
- [ ] Variants via `cva`, not prop-driven `if/else` class concatenation.
- [ ] `cn(base, className)` merge — no template-literal class lists.

#### Data layer

- [ ] No direct `fetch` calls. The `lib/http.ts` wrapper is the only HTTP surface (devdocs/frontend/data.md).
- [ ] Query keys factored through `<feature>Keys` (`features/<name>/keys.ts`).
- [ ] Group-scoped query keys include the slug.
- [ ] `useEffect`-based fetching not introduced; `useQuery` / `useMutation` instead.
- [ ] Mutations use `invalidateQueries` (default) or the documented optimistic-update pattern.

#### Forms

- [ ] `react-hook-form` + `zodResolver` (devdocs/frontend/forms.md).
- [ ] Zod schemas in `features/<name>/schemas.ts` carry **i18n keys**, not English strings.
- [ ] Server errors normalized via `parseServerError(err, fallback)`.
- [ ] Submit gated on `mutation.isPending || form.formState.isSubmitting`.
- [ ] Banner resets on field edit when the page surfaces `serverError`.

#### Routing

- [ ] Real pages added as `lazy()` imports in `src/app/router.tsx` (devdocs/frontend/routing.md).
- [ ] `<RouteTitle title={t("…")} />` mounted at the top of the page.
- [ ] Guard placement correct (`<ProtectedRoute>`, `<GroupRequiredRoute>`).
- [ ] Sidebar entry added in `AppSidebar.tsx` if the route is navigable.
- [ ] Command-palette entry added in `CommandPalette.tsx` if the route is Cmd-K-reachable.

#### i18n

- [ ] Every user-visible string goes through `t("namespace:key")` (devdocs/frontend/i18n.md).
- [ ] New keys added via `npm run i18n:extract`; `npm run i18n:check` is clean.
- [ ] New dynamic-key patterns documented in `frontend/i18next.config.ts`'s `preservePatterns` with a *why* comment.
- [ ] Pluralization uses `_one` / `_other` and the `count` arg, not hand-rolled branching.

#### Accessibility

- [ ] All form fields have `<Label htmlFor>` or `aria-label` (devdocs/frontend/accessibility.md).
- [ ] Icon-only buttons have `aria-label`.
- [ ] Modals use `<Dialog>` / `useConfirm()` / `<Sheet>` — never `window.confirm()` or hand-rolled overlays.
- [ ] Focus-visible rings preserved (no naked `outline: none`).
- [ ] Color is paired with icon + text for any error / success state.
- [ ] Page-level test runs `axe(container)` and is clean.

#### Icons

- [ ] Icons from `lucide-react` only (devdocs/frontend/icons.md).
- [ ] Sizes from the documented scale (`size-3`, `size-3.5`, `size-4`, `size-5`, `size-8`, `size-10`).
- [ ] Decorative icons inherit color from `text-muted-foreground` or token.

#### Tests

- [ ] Unit tests use `renderWithProviders` (devdocs/frontend/testing.md).
- [ ] MSW handlers come from `src/test/handlers/` factories — `server.ts` base set untouched.
- [ ] Page-level tests assert against rendered text, not snapshots.
- [ ] Coverage didn't drop. **(N/A)** if no source files changed.

#### Performance

- [ ] `npm run size` passes. **(N/A)** if no `src/**` change.
- [ ] `npm run lhci` passes locally on at least one URL the change touches. **(N/A)** if not on a public route.
- [ ] No new big dependency landed without justification in the PR body.

#### Smoke

- [ ] `npm run typecheck` clean.
- [ ] `npm run lint` clean.
- [ ] `npm run test` green.
- [ ] `npm run build` succeeds.
- [ ] If the PR touches the embed (`frontend.go`, `index.html`): Go embed smoke test passes (`go test ./go/apiserver -run FrontendEmbed`).

#### Description

- [ ] PR body links the relevant issue (`Fixes #NNNN` or `Closes #NNNN`).
- [ ] Screenshots / GIFs attached for visual changes (light + dark, plus dense + comfortable when relevant).
- [ ] Bundle / Lighthouse deltas called out for non-trivial changes.
- [ ] Cross-stack changes (BE + FE in the same PR) note the contract and link the BE issue.
```

## Tips

- **Don't wait until you push** to walk the list. Run `npm run
  typecheck && npm run lint && npm run test` locally before opening a
  draft.
- **Screenshots are worth the time.** Light + dark + dense + comfortable
  catches a third of all visual regressions before the reviewer sees
  them. The harness in [screenshots.md](screenshots.md) automates the
  capture.
- **A red CI gate is a question, not a verdict.** If `i18n:check` fails,
  you might have intended the new keys (`extract` and commit) or you
  might have left a `t("debug.foo")` (delete the call). Don't paper over
  by tweaking the gate.
