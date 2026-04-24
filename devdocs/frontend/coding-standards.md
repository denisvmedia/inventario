# Coding standards

This document defines the language-level rules every TypeScript / Vue file in `frontend/src/` follows.

## TypeScript

### Strictness

`tsconfig.json` runs in `strict` mode. The following are non-negotiable:

- `strict: true` — all sub-flags on (`strictNullChecks`, `noImplicitAny`, `strictFunctionTypes`, `strictBindCallApply`, `strictPropertyInitialization`, `noImplicitThis`, `alwaysStrict`).
- `noUnusedLocals: true`, `noUnusedParameters: true`. Use a leading underscore (`_arg`) only when an unused parameter is required by a callback signature.
- `noImplicitReturns: true`.
- `noFallthroughCasesInSwitch: true`.

### `any` is banned

Use `unknown` and narrow it. Or pick a precise type.

```ts
// bad
function parse(payload: any) { return payload.foo }

// good
function parse(payload: unknown): string {
  if (typeof payload === 'object' && payload !== null && 'foo' in payload) {
    return String(payload.foo)
  }
  throw new TypeError('payload missing foo')
}
```

The single allowed exception is interop with an untyped third-party library — and even then, isolate it inside one wrapper module and export a typed surface.

### `type` vs `interface`

- Prefer `type` for unions, intersections, mapped/conditional types, and props/emits.
- Use `interface` only when you specifically want declaration merging (rare in app code; common in `*.d.ts`).
- Pick one per file when both work; do not mix styles for the same shape.

### Explicit return types

- Required on **exported** functions and on **composables** (`use*`).
- Optional inside a single file when the type is obvious from the body.
- Required on every `defineProps`, `defineEmits`, `defineSlots` (TS-typed form, not the array form).

### Imports

Use the type-only import syntax when only the type is needed:

```ts
import type { Commodity } from '@/types'
import { commodityService } from '@/services/commodityService'
```

This lets the bundler drop the import in production.

## Naming

| What | Convention | Example |
|---|---|---|
| Component file | `PascalCase.vue` matching the component name | `LocationCard.vue` exports `LocationCard` |
| Composable file | `camelCase.ts` starting with `use` | `useSignedUrl.ts` |
| Service / utility module | `camelCase.ts` | `currencyService.ts`, `formatPrice.ts` |
| Type module | `camelCase.ts` | `commodity.ts` |
| Test file | `<source>.spec.ts` next to source | `LocationCard.spec.ts` |
| Constant | `SCREAMING_SNAKE_CASE` for true constants; `camelCase` for derived/lookup tables | `MAX_UPLOAD_BYTES`, `commodityTypes` |
| Pinia store id | `camelCase` ending with `Store` | `useGroupStore` returns store with id `'group'` |
| CSS class anchor (legacy bridge) | `kebab-case` | `.commodity-card` |

## File structure (Single-File Component)

Order:

```vue
<script setup lang="ts">
// imports
// type / props / emits / slots
// composables / state
// computed / watchers
// methods (event handlers etc.)
</script>

<template>
  <!-- markup -->
</template>

<!-- <style scoped> only as a last resort; prefer Tailwind. See styles-and-tokens.md. -->
```

A component file should rarely exceed ~250 LOC. If it grows beyond that, extract a child component or a composable.

## Import ordering

ESLint's `import/order` enforces:

1. Built-in / Vue / Pinia / Vue Router.
2. Other external libraries (`reka-ui`, `lucide-vue-next`, `vee-validate`, `vue-sonner`, …).
3. `@design/*` (design system).
4. `@/*` (app source — `@/services`, `@/stores`, `@/views`, `@/types`, `@/utils`).
5. Relative imports.

Blank line between groups. Type-only imports stay grouped with their value siblings (TS handles dedupe).

```ts
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

import { Box, MapPin } from 'lucide-vue-next'

import { Button } from '@design/ui/button'
import { LocationCard } from '@design/patterns/LocationCard'

import { useGroupStore } from '@/stores/groupStore'
import type { Location } from '@/types'

import { localHelper } from './helpers'
```

## Console / logging

- No `console.log` in committed code. ESLint warns; CI fails.
- `console.warn` and `console.error` are allowed for genuine warnings/errors that aid debugging in production.
- For user-facing error feedback use `useAppToast()` (`design/composables/useAppToast.ts`).

## Comments

Default to writing none. Code identifiers should explain *what*. Add a comment only when:

- Encoding a non-obvious *why* (constraint, workaround, surprise).
- Pointing at a referenced ticket or upstream bug whose fix will eventually delete the code.
- Documenting a type that is used by external consumers.

Multi-line block comments are forbidden. One short line max.

## Formatting

- Indentation: 2 spaces. No tabs.
- Single quotes for strings; backticks for templates.
- Trailing commas in multi-line literals.
- Semicolons: yes.
- Max line length 120 (soft); the linter will not enforce, but reviewers will flag long ad-hoc lines.

ESLint + Prettier are the source of truth — `make lint-frontend` must pass before requesting review.

## TODO comments

`// TODO(<github-handle> #<issue>): <action>` — must reference an open GitHub issue. Bare `// TODO` without an issue number is rejected at review.
