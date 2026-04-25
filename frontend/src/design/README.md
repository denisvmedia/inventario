# Design system (`src/design/`)

Scaffolded under Epic [#1324](https://github.com/denisvmedia/inventario/issues/1324). Co-exists with the legacy `src/components/` (PrimeVue) until the strangler-fig migration is complete.

## Layout

```
src/design/
  ui/            # shadcn-vue copy-in primitives (Button, Input, Dialog, ...).
  patterns/      # domain composites (DataTable, FormField, StatusBadge, ...).
  composables/   # shared logic (useAppToast, useSignedUrl, ...).
  lib/
    utils.ts     # cn() — the only cross-cutting utility.
  tokens/        # @theme CSS — colors, spacing, motion.
```

Path aliases:

| Alias | Target |
|---|---|
| `@design/*` | `src/design/*` |
| `@design/lib/utils` | `src/design/lib/utils.ts` |
| `@design/ui/*` | shadcn-vue primitives |

## Standards

All code here follows the frontend developer documentation in [`devdocs/frontend/`](../../../devdocs/frontend/README.md). Every FE PR must tick the checklist in [`devdocs/frontend/pr-checklist.md`](../../../devdocs/frontend/pr-checklist.md).

Key rules:

- **Imports**: `@design/ui/...` for primitives, `lucide-vue-next` for icons, `@design/lib/utils` for `cn()`. Never import from `primevue/*`, `primeicons`, or `@fortawesome/*` in new code (enforced by ESLint in PR 0.8).
- **Variants**: every primitive that has visual variants uses `class-variance-authority` and exposes a `ButtonVariants`-style `VariantProps<...>` type.
- **Merging classes**: always `cn(variants(), props.class)` so caller overrides win via `tailwind-merge`.
- **Tokens**: reference semantic tokens (`bg-primary`, `text-foreground`, `border-border`) rather than raw color scales inside primitives. Status tokens (`--color-status-*`) are reserved for `CommodityStatusPill` (Phase 2 [#1327](https://github.com/denisvmedia/inventario/issues/1327)).

## Adding a new shadcn-vue primitive

1. `npx shadcn-vue@latest add <name>` — the CLI respects [`components.json`](../../components.json) and writes to `src/design/ui/<name>/`.
2. Review the generated file: replace hard-coded color classes with semantic tokens if needed; confirm `cn()` is used for class merging.
3. Write a Vitest spec alongside it under `__tests__/<Name>.spec.ts` covering default render, each variant, each size, and `class` prop merging.
4. Re-export from a barrel `index.ts` if the primitive ships multiple sub-components (e.g. `Card`, `CardHeader`, `CardContent`).
