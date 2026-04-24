# Icons

This document defines how icons are used in this codebase: only `lucide-vue-next`, with named imports, sized via Tailwind, and `aria-hidden` by default.

## Stack

- **`lucide-vue-next`** — the only icon library.
- **`@design/lib/icons.ts`** — bridge module that re-exports Lucide icons under FontAwesome names during the migration. To be removed in Phase 6 ([#1331](https://github.com/denisvmedia/inventario/issues/1331)).

Forbidden:

- `@fortawesome/*` — banned via ESLint (see [`imports-and-bans.md`](./imports-and-bans.md)).
- `primeicons` — banned via ESLint.
- Inline SVG copy-pasted from icon catalogues — use Lucide; if Lucide does not have it, raise an issue.
- Icon fonts of any kind.

## Importing

Always named import directly from `lucide-vue-next`:

```ts
import { Box, MapPin, Trash2 } from 'lucide-vue-next'
```

The bridge module exists for transitional code only:

```ts
// During migration only — old code still using FA names:
import { FaBox, FaTrash } from '@design/lib/icons'
```

When you touch a file, prefer flipping its bridge imports to direct Lucide imports as part of the same PR.

## Sizes

We use a small fixed scale, expressed via Tailwind utilities:

| Use | Class | Pixels |
|---|---|---|
| Inline within text | `h-4 w-4` | 16 |
| Buttons, headers, list items | `h-5 w-5` | 20 |
| Empty-state hero, large stat cards | `h-8 w-8` | 32 |
| Splash / illustrations | `h-12 w-12` | 48 |

Do not use arbitrary sizes. If a design needs a new size, add a row to this table first.

## Color

Icons inherit `currentColor`. Set the color via the parent's `text-*` class:

```vue
<span class="text-muted-foreground">
  <MapPin class="h-4 w-4" /> Office
</span>
```

Never set `color` directly on the icon element.

## Accessibility

The default is `aria-hidden="true"` — the icon is decorative and the surrounding text carries the meaning:

```vue
<button>
  <Trash2 class="h-4 w-4" aria-hidden="true" /> Delete
</button>
```

The exception is when the icon is the *only* child of a focusable element:

```vue
<button aria-label="Delete commodity">
  <Trash2 class="h-4 w-4" />
</button>
```

In that case the focusable element carries `aria-label` and the icon stays `aria-hidden`.

The `<IconButton>` pattern enforces this: its `aria-label` prop is required at the type level.

## Lucide vs old FA names

Common mappings used in this codebase:

| FontAwesome name | Lucide name |
|---|---|
| `box` | `Box` |
| `blender` | `CookingPot` |
| `laptop` | `Laptop` |
| `tools` | `Wrench` |
| `couch` | `Sofa` |
| `tshirt` | `Shirt` |
| `map-marker-alt` | `MapPin` |
| `calendar` | `Calendar` |
| `cloud-upload-alt` | `UploadCloud` |
| `file-pdf` | `FileText` |
| `download` | `Download` |
| `trash` | `Trash2` |
| `edit` | `Pencil` |
| `times` | `X` |
| `plus` | `Plus` |
| `chevron-up` / `chevron-down` | `ChevronUp` / `ChevronDown` |
| `user` | `User` |
| `right-from-bracket` | `LogOut` |
| `spinner` | `Loader2` (with `animate-spin`) |
| `check-circle` | `CheckCircle2` |
| `exclamation-triangle` | `AlertTriangle` |
| `exclamation-circle` | `AlertCircle` |
| `redo` | `RotateCw` |
| `upload` | `Upload` |

The full bridge table lives in `frontend/src/design/lib/icons.ts`.

## Adding a new icon

1. Find it on https://lucide.dev/icons.
2. Import it directly: `import { NewIcon } from 'lucide-vue-next'`.
3. Use the standard size class.
4. If the icon is decorative, add `aria-hidden="true"` (or use `<IconButton>`).
5. No registration step needed — Lucide is tree-shaken.

## What is *not* an icon

- Status indicators that need color **and** text — those are `<StatusBadge>` patterns, which combine a Lucide icon, a label, and a color token. Use the badge, not a raw icon.
- Logos — those are `<img>` (or inlined SVG in a dedicated `<AppLogo>` component), not Lucide.
- Country flags — out of scope; if needed in future, raise an issue and pick a flag library.
