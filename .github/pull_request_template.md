<!--
Thanks for opening a PR.

For frontend changes, follow the FE PR checklist:
  → devdocs/frontend/pr-checklist.md

For backend changes, follow the existing project conventions documented in AGENTS.md.
-->

## Summary

<!-- 1-3 bullets describing what this PR does and why. -->

## Related issues

<!-- Closes #..., Refs #... -->

## Test plan

<!-- How to verify this works. Bulleted checklist of manual or automated steps. -->
- [ ] `make lint`
- [ ] `make test`
- [ ] (frontend) `make test-frontend`
- [ ] (frontend) `make test-e2e`

## Frontend PR checklist (only if `frontend/src/` was touched)

<!-- Source: devdocs/frontend/pr-checklist.md
     Tick every box; strike through with ~~text~~ + a one-line reason if N/A. -->

### Stack discipline
- [ ] No new `primevue/*` / `primeicons` / `@fortawesome/*` imports.
- [ ] New components live in `@design/{ui,patterns}` or co-located inside a view; nothing new under `@/components/`.
- [ ] No new `<style scoped>` blocks; styling via Tailwind utilities.
- [ ] No new SCSS files.

### Forms / components
- [ ] Forms use `<Form>` + `<FormField>` from `@design/ui/form` with a zod schema.
- [ ] Props/emits/slots are TypeScript-typed.
- [ ] Variants use `cva` for 2+ visual variants.
- [ ] Vitest spec added/updated for new components/composables.

### Accessibility
- [ ] Every interactive element has an accessible name.
- [ ] Focus is visible on keyboard navigation.
- [ ] Modals/overlays use Reka UI primitives.
- [ ] Color is not the only signal of state.
- [ ] Verified in dark mode.
- [ ] Animations use `motion-safe:` prefix.

### Tests
- [ ] New tests use semantic locators (`getByRole` / `getByLabel` / `getByTestId`) — no CSS selectors.
- [ ] Legacy class anchors preserved on patterns that replace legacy components.
- [ ] `make lint-frontend && make test-frontend && make build-frontend && make test-e2e` all pass.

### Migration hygiene (if part of Epic #1324)
- [ ] PR ≤ ~500 LOC of changes; split if larger.
- [ ] PrimeVue imports removed from any view this PR rewrites.
- [ ] ESLint suppressions cite the phase that will remove them.
- [ ] Bundle delta documented if > 10 KB gzipped.
