# Iconography & Illustration

**Committed.** Inventario uses **`lucide-react`** for every icon and
no other library. Illustration is sparing and lives in empty states /
onboarding only — see [22-illustration-prompts.md](22-illustration-prompts.md) for the sourcing
recipe.

## One library

```tsx
import { ChevronRight, Trash2, Plus } from "lucide-react"
```

Hard rules:

- Lucide is the only icon source. FontAwesome / PrimeIcons / Heroicons
  / Material Icons / Phosphor / react-icons are bans on sight. See
  [../imports-and-bans.md](../imports-and-bans.md).
- Inline SVG paths are not allowed in components. The one exception is
  the brand mark (`AppLogo`); see [19-branding.md](19-branding.md).
- Always import by named export — `import { Trash2 } from "lucide-react"`,
  never the namespace import. Tree-shake matters; the entry-bundle
  budget is 200 KB gzip.

## Why lucide

- Free, MIT, vendor-neutral.
- Excellent coverage of the inventory domain (Package, Folder, Boxes,
  ScanBarcode, Receipt, Wrench, Wallet, FileText, Image, Camera, Tag,
  …).
- One stroke weight, one size convention, one design language —
  consistent across the app at zero engineering cost.
- The design mock uses lucide. Lockstep
  with the canonical visual contract.

PR #1362 proposed migrating to Phosphor. That proposal is no longer
on the table — Lucide stays. The earlier proposal predated cutover,
when icon stylization was being re-litigated; it didn't survive into
the React rewrite.

## Size scale

| Class | px | Use |
| --- | --- | --- |
| `size-3` | 12 | xs button, small badge |
| `size-3.5` | 14 | sm button, tight inline cue |
| `size-4` | 16 | Body default, default button, list-row chevron |
| `size-5` | 20 | lg button, dialog title icon, hero-row icon (in a `size-10` tile) |
| `size-6` | 24 | Sidebar nav (`SidebarMenuButton`) |
| `size-8` | 32 | Stat-card tile (with `size-4` icon inside) |
| `size-10` | 40 | Empty-state hero, first-run onboarding |
| `size-16` | 64 | Hero illustration backdrop (rare) |

Match icon size to the surrounding control's height — see the
button-size-to-icon table in [../icons.md](../icons.md). Don't reach for `size-[18px]`;
the scale is enough.

## Stroke

Lucide ships at stroke-width 2 by default. Don't override:

- Don't write `strokeWidth={1.5}` to "soften" the look. The whole
  app's stroke language is consistent at 2; one weakened icon looks
  off-rhythm.
- Don't compose lucide icons into a bigger glyph. Use the named icon
  closest to the meaning.

## Color

Decorative icons inherit color from `text-*` on the parent. Defaults:

| Use | Class |
| --- | --- |
| Decorative cue | `text-muted-foreground` |
| Primary action / focal | `text-primary` |
| Destructive | `text-destructive` |
| Domain status | `text-status-active` / `text-status-expiring` / `text-status-expired` / `text-status-none` |
| Tag color | `text-tag-amber` / `text-tag-green` / … (closed enum) |

Hard ban: `text-amber-500`, `text-red-600` for icons. Tokens via the
documented utilities.

## Aria

Lucide adds `aria-hidden="true"` to its rendered SVG when no
label-related prop is set. Don't override — that's what you want for
decorative icons next to a text label:

```tsx
<Button>
  <Trash2 className="size-3.5" />
  {t("common:actions.delete")}
</Button>
```

For stand-alone icons (without a text label, not inside a labeled
button), set `aria-label`:

```tsx
<Check className="size-4 text-status-active" aria-label={t("common:status.completed")} />
```

For icon-only buttons, label the button, not the icon:

```tsx
<Button size="icon" aria-label={t("common:actions.delete")}>
  <Trash2 className="size-4" />
</Button>
```

See [14-accessibility.md](14-accessibility.md) for the full rule.

## Domain icon vocabulary

Closed mapping: domain concept → lucide icon. Mirror these across the
codebase rather than picking a synonym:

| Domain | Icon |
| --- | --- |
| Commodity / item | `Package` |
| Location | `Folder` |
| Area | `Boxes` |
| File (unspecified) | `File` |
| File: image / photo | `Image` (or `Camera` for capture flows) |
| File: invoice | `Receipt` |
| File: document | `FileText` |
| File: other | `Paperclip` |
| Tag | `Tag` |
| Search | `Search` |
| Settings | `Settings` |
| Profile | `User` |
| Group | `Users` |
| Logout | `LogOut` |
| Add | `Plus` |
| Edit | `Edit2` (line-edit) — never `Pencil` |
| Delete | `Trash2` |
| Upload | `Upload` |
| Download | `Download` |
| Export | `Download` (when target is the user's disk) / `Box` (the export entity itself) |
| Restore | `RotateCcw` |
| Filter | `Filter` |
| Sort | `ArrowUpDown` |
| More actions | `MoreHorizontal` |
| Calendar / date | `Calendar` |
| Warranty | `ShieldCheck` (active) / `ShieldAlert` (expiring) / `ShieldX` (expired) |
| Currency / value | `Banknote` (or `Coins` for many) |
| Print | `Printer` |
| Share | `Share2` |
| Copy | `Copy` |

If the domain concept doesn't map, file an issue rather than picking a
near match in the moment.

## Illustrations

Used in:

- Empty states (per [20-edge-cases.md](20-edge-cases.md)).
- Onboarding (no-group, post-register).
- The 404 page.

Never used in:

- Marketing-style page heroes (Inventario doesn't have those).
- Stat cards (icons in tiles, not illustrations).
- Confirmation dialogs.

The current production set is **icon-only** — the `Folder` /
`Package` glyphs at `size-10` over a `bg-primary/10 rounded-lg` tile
serve as the empty-state illustration. The brief at
[22-illustration-prompts.md](22-illustration-prompts.md) documents the recipe for richer
illustrations if and when we commission them; they aren't yet in the
shipping bundle.

## Hard rules

1. **Lucide only.** No mixed icon libraries.
2. **Named imports.** No namespace imports of `lucide-react`.
3. **Pick from the size scale.** No `size-[18px]`.
4. **Default stroke (2).** No softening or hand-rolled bolding.
5. **Tokens for color.** No raw Tailwind palettes on icons.
6. **`aria-hidden` is Lucide's default.** Override (with `aria-label`)
   only when the icon is stand-alone meaningful.
7. **Domain vocabulary is closed.** Mirror the table above rather than
   picking a synonym.

## Anti-patterns

- A `Trash` icon and a `Trash2` icon side-by-side — pick one and
  search-and-replace.
- Color-tinted decorative icons (`text-amber-500` on a list-row
  chevron) — chevrons inherit `text-muted-foreground`.
- A 4-color SVG illustration on the dashboard. The dashboard is
  glanceable, not decorative.
- Wrapping an icon in `<span aria-hidden>` next to an unlabeled
  `<button>`. Label the button.
- Using a `Package` icon for files. The mapping is closed.

## Cross-refs

- Engineering rules: [../icons.md](../icons.md).
- A11y for icons: [14-accessibility.md](14-accessibility.md).
- Empty-state usage: [20-edge-cases.md](20-edge-cases.md).
- Brand mark vs. icon: [19-branding.md](19-branding.md), [21-logo-directions.md](21-logo-directions.md).
- Illustration prompts: [22-illustration-prompts.md](22-illustration-prompts.md).
