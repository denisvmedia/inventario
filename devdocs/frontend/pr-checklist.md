# Frontend PR checklist

Copy this checklist into the description of every PR that touches `frontend/src/`. Tick each box before requesting review. The reviewer will not merge a PR with unchecked items unless a justification is in the PR thread.

```markdown
## Frontend PR checklist

### Stack discipline
- [ ] No new `primevue/*` / `primeicons` / `@fortawesome/*` imports.
- [ ] All new components live in `@design/{ui,patterns}` or are co-located inside a view; nothing new under `@/components/`.
- [ ] No new `<style scoped>` blocks; styling done via Tailwind utilities.
- [ ] No new SCSS files.

### Forms
- [ ] If the PR adds/edits a form: it uses `<Form>` + `<FormField>` from `@design/ui/form` with a zod schema.
- [ ] Server validation errors are surfaced via `setErrors()` (not toasts).

### Components
- [ ] Props/emits/slots are TypeScript-typed (`defineProps<Props>()`, etc.).
- [ ] Variants use `cva` if there are 2+ visual variants.
- [ ] Each new pattern has a Vitest spec covering render, props, slots, emits.

### Accessibility
- [ ] Every interactive element has an accessible name (label, `aria-label`, or text child).
- [ ] Focus is visible on keyboard navigation (`focus-visible:ring-2 focus-visible:ring-ring`).
- [ ] If a modal/overlay was added: it uses Reka UI primitives (focus trap is automatic).
- [ ] Color is not the only signal of state (icon + text + color for status).
- [ ] Verified visually in dark mode (`document.documentElement.dataset.theme = 'dark'`).
- [ ] Animations use `motion-safe:` prefix.

### Tests
- [ ] Vitest specs for new components/composables added or updated.
- [ ] Playwright e2e (existing or new) green: `make test-e2e`.
- [ ] New tests use semantic locators (`getByRole` / `getByLabel` / `getByTestId`) — no CSS selectors.
- [ ] Legacy class anchors (`.commodity-card`, `.location-card`, `.file-card`, `.file-item`, `.export-row`) preserved on new patterns that replace legacy components.

### Migration hygiene (if part of Epic #1324)
- [ ] PR ≤ ~500 LOC of changes (excluding generated files); split if larger.
- [ ] PrimeVue imports removed from any view this PR rewrites.
- [ ] Eslint suppression comments cite the phase that will remove the suppressed import.

### CI gates
- [ ] `make lint-frontend` passes.
- [ ] `make test-frontend` passes.
- [ ] `make build-frontend` passes.
- [ ] `make test-e2e` passes.
- [ ] Bundle delta documented in the PR description if > 10 KB gzipped.
```

## When to skip an item

Use a one-line justification under the unchecked box:

```markdown
- [x] Verified visually in dark mode.
- [ ] ~Bundle delta documented~ — N/A, no production code changed (docs only).
```

Strikethrough with `~~` shows the reviewer that you considered the item and decided it does not apply, rather than overlooking it.

## Reviewer's job

The reviewer checks every ticked box on a sampled basis:

- Run `make lint-frontend && make test-frontend && make build-frontend && make test-e2e` locally on the PR branch.
- Open one new component and verify the dark-mode swap doesn't break it.
- Skim the diff for forbidden patterns (raw `primevue/*` import, new SCSS, `<style scoped>` in a view).
- Confirm semantic locators in new tests.

If the reviewer finds an undocumented issue, they request changes and ask the author to update both the code and (when it reflects a missed standard) the relevant doc in `devdocs/frontend/`.

## Where this checklist is referenced

- `.github/pull_request_template.md` includes a link to this file.
- `AGENTS.md` (project-level CLAUDE.md) points contributors here.
- The Epic ([#1324](https://github.com/denisvmedia/inventario/issues/1324)) Definition of Done references this file.
