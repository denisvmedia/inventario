# Migration conventions

How the React rewrite happened and the conventions that survived
cutover. This doc supersedes the old strangler-fig recipe under the
deprecated Vue tree (deleted in cutover #1423).

## What's there now

`frontend/` is the only frontend tree. Single Vite app, single Go
embed, single Playwright project per browser. The `INVENTARIO_FRONTEND`
env var no longer exists.

If you're looking at this doc to learn "how do I add a new feature?",
there is nothing migration-specific to know — read the rest of this
folder, starting with [README.md](README.md).

## What used to be there

Until cutover (#1423, PR #1457), two parallel trees coexisted:

```
frontend/         # Vue 3 + PrimeVue (legacy)
frontend-react/   # React 19 + Tailwind v4 + shadcn (the rewrite)
```

The Go binary embedded one or the other based on
`INVENTARIO_FRONTEND={legacy|new}`. CI ran the test matrix on both;
each FE issue (#1407–#1417) shipped its React equivalent and an
e2e tag (`@react-only`) so the new bundle gated separately from
legacy.

The cutover removed the Vue tree, renamed `frontend-react/` →
`frontend/`, removed the env var, collapsed the Playwright matrix,
and switched the Dockerfile to a single-stage frontend builder.

## Why the rewrite happened (epic #1397)

The legacy stack (Vue + PrimeVue) was mid-migration onto shadcn-vue
when a clean React mock was delivered with a coherent visual system
and a four-category file model (Photos / Invoices / Documents / Other).
Continuing on Vue + shadcn-vue would have required re-implementing the
mock against shadcn-vue's smaller primitive set; rewriting in React
matched the mock 1:1 and unblocked design work that the legacy stack
couldn't carry.

The full plan: epic [#1397](https://github.com/denisvmedia/inventario/issues/1397),
28 sub-issues #1398–#1425. Each issue's PR is linked from the epic.

## Conventions that survived

### URL is the source of truth for group context

`/g/:groupSlug/*` is the canonical group identifier. The
`GroupContext` (`features/group/GroupContext.tsx`) reads from
`useParams`; the http wrapper reads from `getCurrentGroupSlug()` which
falls back to `window.location` when the React effect hasn't mirrored.

This was true in the legacy app and the rewrite kept it — tabs stay
isolated even when the user changes groups in another tab.

### One feature slice per domain

`features/<name>/{api,hooks,keys,schemas,constants}.ts`. The shape
predates the rewrite (legacy had it under `src/api/<name>/`) but it
proved out as the right granularity for both stacks. Don't slice
finer (per-page hooks files) unless a slice grows past ~600 lines.

### i18n key shape

`namespace:dot.path` was used in the legacy frontend (vue-i18n) and
carried over to react-i18next. JSON files are 1:1 between the
locales; `i18n:check` enforces that the en bundle is canonical and
cs/ru track it.

### Per-feature query key factory

The `<name>Keys` factory (`features/<name>/keys.ts`) is React-specific
and doesn't have a legacy equivalent. The pattern was set in PR #1428
(#1403) and every slice since has used it.

### Coverage threshold contract

80/70/80/80 (lines/branches/functions/statements). Set in PR #1433
(#1418) and never weakened — every PR that drops coverage finds the
missing test instead. See [testing.md](testing.md).

### Bundle and Lighthouse gates

Set in PR #1437 (#1420). Entry JS ≤ 200 KB gzip; perf ≥ 0.85; a11y ≥
0.95; best-practices ≥ 0.90. See [perf.md](perf.md).

### Color tokens copied from the mock

`src/index.css`'s `:root` and `.dark` blocks are 1:1 with the internal
design mock. Lockstep updates: a token change in one repo lands as a
follow-up in the other, with the PR cross-linked. See
[styles-and-tokens.md](styles-and-tokens.md).

### Smoke test on the embed

`go/apiserver/frontend_embed_test.go` asserts the bundled HTML's
`<title>` and absence of Bolt artifacts. Predates cutover; survives as
the cheapest tripwire for "did the build accidentally inline the wrong
HTML?".

## Cutover criteria (for the historical record)

The cutover issue (#1423) listed pre-conditions that should have been
met before flipping. They were partially met when the cutover landed —
issues #1400, #1412, #1415, #1442, #1448 were still open. The cutover
shipped anyway; those follow-ups merged in subsequent PRs against the
unified tree (PRs #1481/#1477/#1479 and the #1424 docs PR you're
looking at). The official AC on the cutover issue:

- [x] Vue tree deleted from `frontend/`.
- [x] `INVENTARIO_FRONTEND` env var removed (single bundle path).
- [x] `frontend-react/` renamed to `frontend/`.
- [x] Playwright matrix collapsed to one bundle.
- [x] Dockerfile switched to single-stage frontend builder.
- [x] CI workflow names normalized (drop `frontend-react-*` prefix).

The "two-week soak window" listed in the issue was waived. In
hindsight, the rewrite landed cleanly enough that the soak was
unnecessary; future cutovers of similar scale should still default to
soaking before deletion.

## What this doc isn't

This is not a guide for "migrating a screen to React" — there's
nothing left to migrate. If a new design system arrives next year and
the answer is another rewrite, this file gets archived (probably in
`devdocs/history/` or similar) and the new conventions live in their
own doc tree.

## Cross-references

- Strategy memo: epic [#1397](https://github.com/denisvmedia/inventario/issues/1397).
- Cutover PR: [#1457](https://github.com/denisvmedia/inventario/pull/1457).

- Sub-issues: #1398 through #1425 (linked from the epic).
