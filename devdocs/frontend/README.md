# Frontend developer documentation

This directory holds the standards every Inventario frontend pull request is reviewed against. It is the single source of truth for *how* we write Vue 3 + TypeScript + Tailwind code in this repo.

If you are about to:

- open an FE PR — read [`pr-checklist.md`](./pr-checklist.md) and copy the checklist into your PR description;
- add a new component — read [`components.md`](./components.md);
- add a new form — read [`forms.md`](./forms.md);
- add a new icon — read [`icons.md`](./icons.md);
- write a test — read [`testing.md`](./testing.md);
- migrate a legacy view — read [`migration-conventions.md`](./migration-conventions.md).

## Index

| File | What it covers |
|---|---|
| [`coding-standards.md`](./coding-standards.md) | TypeScript strictness, naming, file structure, import ordering, console policy, formatting. |
| [`components.md`](./components.md) | Where components live (`@design/ui`, `@design/patterns`, `views/`), props/emits/slots typing, variants via `cva`, composition rules. |
| [`styles-and-tokens.md`](./styles-and-tokens.md) | Tailwind utility ordering, token consumption rules, custom CSS policy, dark mode, density. |
| [`forms.md`](./forms.md) | `vee-validate` + `zod` as the only form stack; `<Form>` + `<FormField>` patterns; server error surfacing. |
| [`icons.md`](./icons.md) | `lucide-vue-next` only; size scale; `aria-hidden` defaults; bridge layer during migration. |
| [`accessibility.md`](./accessibility.md) | Accessible names, focus order, modal a11y via Reka UI, contrast targets, reduced motion. |
| [`testing.md`](./testing.md) | Vitest unit specs, semantic Playwright locators, snapshot policy, legacy class-anchor preservation. |
| [`imports-and-bans.md`](./imports-and-bans.md) | ESLint `no-restricted-imports` rules and the rationale behind each ban. |
| [`pr-checklist.md`](./pr-checklist.md) | Copy-paste checklist for every FE PR description. |
| [`migration-conventions.md`](./migration-conventions.md) | Strangler-fig migration recipe, per-view PR size, legacy class anchors, when to add a new primitive. |

## Scope

These standards apply to **all code under `frontend/src/`**. Backend Go standards live elsewhere (`go/`, `devdocs/SYSTEM_DESIGN.md`, etc.).

The standards exist primarily to support the design-system migration tracked in Epic [#1324](https://github.com/denisvmedia/inventario/issues/1324). Once the migration completes (Phase 6 / [#1331](https://github.com/denisvmedia/inventario/issues/1331)), they remain in force for any new frontend work.

## Decision log

When a standard changes, update the relevant file and reference the PR that drove the change in the commit message. The docs are versioned with the code; the latest commit is always the current standard.

If a PR proposes deviating from one of these standards, the deviation must be discussed in the PR description with a rationale, *and* the relevant doc must be updated in the same PR (or a sibling PR merged first). Drift between code and docs defeats the purpose.
