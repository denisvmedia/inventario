# Migration conventions

This document defines the rules that govern the design-system migration tracked in Epic [#1324](https://github.com/denisvmedia/inventario/issues/1324). Apply them whenever you touch a file as part of a phase PR.

## Strangler-fig

PrimeVue is **not removed in a single PR**. It stays in `package.json` for the entire transition (Phases 0–5). Each view migrates in its own PR; once migrated, that view's PrimeVue imports are removed.

The final Phase 6 ([#1331](https://github.com/denisvmedia/inventario/issues/1331)) deletes PrimeVue *only when* `grep -rn "primevue" frontend/src/` returns empty (or only inside comments). This is a hard pre-flight gate.

Implication for every per-view PR:

- Removing PrimeVue from the file you touched? Good.
- Adding PrimeVue to a new file? Forbidden (ESLint blocks it).
- Leaving PrimeVue in a file you did not touch? Fine — that file's migration belongs to its own PR.

## Per-view PR size

Target: **≤ 500 LOC of changes** (excluding generated files like `package-lock.json`, snapshots, etc.).

If a view is bigger, split:

- Extract the new pattern in PR 1 (no view change).
- Migrate the view to consume the pattern in PR 2.
- Optionally remove the legacy component in PR 3.

The biggest known offenders are listed in Phase 4 ([#1329](https://github.com/denisvmedia/inventario/issues/1329)) — `ExportDetailView` (1493 LOC), `ExportImportView` (361 LOC), `RestoreCreateView` (640 LOC). These are explicitly allowed to be 2–3 sub-PRs.

## View migration recipe

For each view:

1. **Identify the patterns it needs.** If a pattern does not yet exist in `@design/patterns/`, write it first (separate PR within the same phase).
2. **Rewrite the view** to consume only `@design/{ui,patterns}` + composables + services + stores.
3. **Drop the view's legacy imports** — `primevue/*`, `@fortawesome/*`, `primeicons`, anything in `@/components/` that has been replaced.
4. **Update the view's vitest spec** — if it asserts on CSS classes, swap to semantic locators (`getByRole` / `getByLabel`).
5. **Run the e2e suite** — if a Playwright test breaks, prefer fixing it via semantic locators (don't silence with `.skip`).
6. **Open the PR** with the [pr-checklist](./pr-checklist.md) ticked.

## Legacy class anchors

To keep existing Playwright tests green during the migration, certain CSS classes act as **stable selector anchors** on the new patterns. They do **no styling** — Tailwind utilities do. They are pure markers.

The anchors:

| Class | On which new pattern | Used by which legacy e2e |
|---|---|---|
| `.commodity-card` | `CommodityCard` | `commodity-simple-crud.spec.ts`, `draft-inactive-toggle.spec.ts` |
| `.location-card` | `LocationCard` | `commodity-simple-crud.spec.ts` |
| `.file-card` | `FilePreview` (when used in a grid) | `file-uploads.spec.ts` |
| `.file-item` | `FilePreview` (when used in a list) | `file-deletion-cascade.spec.ts` |
| `.export-row` | `ExportListView` row pattern | `exports-crud.spec.ts` |

Rules:

- Keep the class on the **outermost** element of the new pattern.
- Do not strip them as part of a refactor PR.
- If you write a new pattern that has *no* legacy counterpart, it does **not** get a class anchor — modern tests use semantic locators only.
- Removal happens in Phase 6 once each legacy e2e has been rewritten to semantic locators (or, more likely, replaced by a new flow).

## Test contract during migration

Two-sided rule:

1. **Existing e2e tests** (the ones with `.commodity-card`-style selectors) **keep working** because new patterns retain the class anchors above. Do not mass-rewrite them.
2. **New tests** (any spec landing as part of a phase PR) use **only** semantic locators (`getByRole`, `getByLabel`, `getByTestId`). ESLint enforces this on test files.

If you find yourself updating an existing test (e.g. selectors broke because the DOM changed in your refactor), rewrite the broken portion with semantic locators in the same PR. Do not patch with another CSS selector.

## When to add a new primitive

If a phase reveals that `@design/ui/` is missing a primitive your view needs, **add it via the shadcn-vue CLI in a separate PR** before continuing the view's migration:

1. `npx shadcn-vue@latest add <component>` from the `frontend/` directory.
2. Verify the generated files compile.
3. Add a Vitest spec covering render, props, slots, variants, emits.
4. Open a PR titled `[Phase X] design/ui: add <component>`.
5. Once merged, return to your view migration PR and consume the new primitive.

This avoids mixing primitive scaffolding with view migration logic in the same diff.

## When to add a new pattern

If multiple views need the same composite (e.g. `<MediaGallery>` is needed in `CommodityDetailView` and `FileDetailView`), the pattern PR comes first:

1. New pattern in `frontend/src/design/patterns/<Name>.vue`.
2. Vitest spec in `__tests__/`.
3. Pattern PR merged separately.
4. Subsequent view migrations consume it.

If only **one** view needs the composite right now, build it in the view's PR but extract it into `patterns/` the moment a second consumer appears.

## Feature flags

Reserve for visible-behavior changes that warrant a kill switch. Currently planned:

- **Phase 0 PR 0.4** — `<Toaster />` (vue-sonner) and `<Toast />` (PrimeVue) coexist throughout the migration. No flag needed; they don't interfere with each other.
- **Phase 1 PR 1.3** — new `AppHeader` ships behind `?new-header=1` for 1–2 days on staging, then default on.
- **Phase 6 PR 6.4** — dark theme behind a user preference (toggle in Profile).

Other phases ship without flags. The strangler-fig itself is the flag (legacy code paths keep working).

## Deleting a legacy file

The order matters:

1. Migrate every view that imported the legacy file.
2. Verify no production code references it (`grep -rn "<filename>" frontend/src/`).
3. Delete in a focused PR titled `chore: delete <filename> (replaced by …)`.
4. The same PR removes any tests for that file.

Do not delete a legacy file in the same PR as a view migration. Keeps revertibility surgical.

## Bundle budget

Each phase has a soft bundle delta target documented in the phase issue. Track it via `make build-frontend` output and report in the phase's "Definition of Done" comment when closing.

Per-PR rule: deltas > 10 KB gzipped require a one-paragraph justification in the PR description.

## Rollback

- **Per-PR rollback** — `git revert <sha>`. Trivial for Phase 0–2 (additive) and per-view PRs in Phase 3–4.
- **Per-phase rollback** — release tags after each phase (`design-v2-phase-3`, …) make it easy to roll the entire phase back if a regression emerges.
- **Phase 6 rollback** — the riskiest. PR 6.1 (PrimeVue removal) ships only after PR 5.6 (Toast removal) and Phase 4 have soaked in production for **1–2 weeks** without issues. If a problem surfaces post-removal, `git revert` on PR 6.1 brings PrimeVue back into `package.json` (and the lockfile resolves on the next `npm install`).
