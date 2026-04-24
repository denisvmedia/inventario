# Imports and bans

This document defines what may and may not be imported in `frontend/src/` code, and the ESLint rules that enforce it.

## Banned imports

| Pattern | Reason | Replacement |
|---|---|---|
| `primevue/*` | Being removed in Phase 6 ([#1331](https://github.com/denisvmedia/inventario/issues/1331)). Two-system coexistence is the migration cost; new code must not extend it. | `@design/ui/*` (shadcn-vue) |
| `primeicons` | Same migration; second icon system. | `lucide-vue-next` |
| `@fortawesome/fontawesome-svg-core` | Being removed in Phase 6. | `lucide-vue-next` |
| `@fortawesome/free-*-svg-icons` | Being removed in Phase 6. | `lucide-vue-next` |
| `@fortawesome/vue-fontawesome` | Being removed in Phase 6. | `lucide-vue-next` |

These are enforced as **errors** (build-blocking) by ESLint after Phase 0 PR 0.8 ([#1325](https://github.com/denisvmedia/inventario/issues/1325)).

## Discouraged imports (warning level)

| Pattern | Why discouraged | When acceptable |
|---|---|---|
| `@/components/*` | Old component directory; new patterns live in `@design/patterns/*`. | Existing files that have not yet been migrated. New files must not land there. |
| `@/assets/*.scss` | Legacy SCSS; the design system uses Tailwind tokens via `@theme`. | Legacy view files until they are migrated. |
| `lodash` whole import | Bundle bloat. | Cherry-pick: `import debounce from 'lodash/debounce'`. Better: write a small composable. |

## ESLint rule

`frontend/eslint.config.js` (added in Phase 0 PR 0.8):

```js
{
  rules: {
    '@typescript-eslint/no-restricted-imports': ['error', {
      patterns: [
        { group: ['primevue/*'], message: 'PrimeVue is being removed (#1331). Use @design/ui/* instead.' },
        { group: ['primeicons'],  message: 'PrimeIcons is being removed (#1331). Use lucide-vue-next.' },
        { group: ['@fortawesome/*'], message: 'FontAwesome is being removed (#1331). Use lucide-vue-next or @design/lib/icons during migration.' },
      ],
      paths: [
        // (none currently, room for specific module bans)
      ],
    }],
  },
}
```

When the migration completes (Phase 6), the rule stays — to prevent reintroduction.

## Per-file overrides during migration

Existing legacy files keep working while their view migrates. To keep the lint clean during the transition window, suppress the rule **per import line** with a comment naming the phase that will remove it:

```ts
// eslint-disable-next-line @typescript-eslint/no-restricted-imports -- removed in #1330 PR 5.6
import { useToast } from 'primevue/usetoast'
```

The phase number in the comment is mandatory. CI (eventually) checks for orphaned suppressions whose referenced issue is closed.

## What `@design/lib/icons` is

A bridge module created in Phase 0 PR 0.3 that re-exports `lucide-vue-next` icons under their FontAwesome names:

```ts
// frontend/src/design/lib/icons.ts (created in Phase 0)
export {
  Box as FaBox,
  Pencil as FaEdit,
  Trash2 as FaTrash,
  // ...
} from 'lucide-vue-next'
```

This lets old code be touched in small migrations without fighting the icon swap. **New code must not import from the bridge** — go straight to `lucide-vue-next`. The bridge is deleted in Phase 6 PR 6.2.

## Allowed external imports

Default rule: anything in `package.json` is allowed unless explicitly listed above.

Notable allow-listed libraries (the new stack):

- `vue`, `vue-router`, `pinia`
- `reka-ui`
- `lucide-vue-next`
- `class-variance-authority`, `clsx`, `tailwind-merge`
- `vue-sonner`
- `vee-validate`, `@vee-validate/zod`, `zod`
- `@internationalized/date`
- `axios` (HTTP client; existing)
- `pdfjs-dist` (used by `PDFViewerCanvas` only — do not import elsewhere)

## Adding a new dependency

Adding a new npm dependency is a deliberate decision:

1. Open a discussion in the relevant phase issue (or a dedicated issue if cross-cutting) with: name, size (gzipped), maintenance status, alternatives considered.
2. After approval, the PR adds the dependency *and* updates this document if a new ban or category emerges.
3. Reviewer confirms `package-lock.json` change matches the discussion.

Drive-by dependency adds are rejected at review.

## Deleting a banned dependency

When the last legacy import of a banned library is removed, the *next* PR removes the dependency from `package.json`. This is the Phase 6 program; do not do it ad-hoc earlier or you risk breaking residual imports.
