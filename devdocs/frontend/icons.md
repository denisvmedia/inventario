# Icons

Lucide only. Strict size scale. Decorative icons are `aria-hidden`,
icon-only buttons get an `aria-label`. The whole convention exists so
focus, contrast, and rhythm stay consistent across pages.

## One library: `lucide-react`

```tsx
import { ChevronRight, Trash2, Plus, Edit2 } from "lucide-react"
```

Hard rules:

- **No FontAwesome, no PrimeIcons, no Heroicons, no Material Icons.** They
  drift from the visual contract and bloat the bundle. The ban is
  documented in [imports-and-bans.md](imports-and-bans.md).
- **No inline SVG paths.** When an icon doesn't exist in Lucide, propose
  it upstream (<https://lucide.dev/guide/contributing>) or pick a near
  match. The one exception is `AppLogo` (`src/components/AppLogo.tsx`),
  which is a brand mark — that's a logo, not an icon.
- **Tree-shake by named import.** Always
  `import { Trash2 } from "lucide-react"`, never
  `import * as Icons from "lucide-react"`.

## Size scale

Tailwind's `size-*` utility maps to width + height in one class. Every
icon picks from this scale — no `w-[18px] h-[18px]`:

| Class | Pixels | Use |
| --- | --- | --- |
| `size-3` | 12 | Inside an `xs` button or a small badge |
| `size-3.5` | 14 | Inside a `sm` button or a tight inline cue |
| `size-4` | 16 | Standard body icon, `default` button, list-row chevron |
| `size-5` | 20 | `lg` button, dialog title icon, hero-row icon (in a `size-10` tile) |
| `size-6` | 24 | Sidebar nav (`SidebarMenuButton` default) |
| `size-8` | 32 | Stat-card tile icon (`size-8` rounded-lg bg-muted, with `size-4` icon inside) |
| `size-10` | 40 | Empty-state hero, first-run onboarding |
| `size-16` | 64 | Hero illustration |

Match the icon size to the surrounding control's height:

| Button size | Icon class |
| --- | --- |
| `xs` (h-6) | `size-3` |
| `sm` (h-8) | `size-3.5` |
| `default` (h-9) | `size-4` |
| `lg` (h-10) | `size-4` |
| `icon` (h-9) | `size-4` |
| `icon-sm` (h-8) | `size-3.5` |

## Decorative vs. labeled icons

| Icon's role | Treatment |
| --- | --- |
| **Decorative** — sits next to a text label that already conveys meaning | `aria-hidden` is **on by default** (Lucide adds it when there's no label prop). Don't override. |
| **Stand-alone in an icon-only button** | Wrap in `<Button size="icon" aria-label="…">`. The button gets the label; the icon stays hidden. |
| **Stand-alone outside a button** (status indicator, badge with no text) | Add `aria-label` to the icon **or** pair with visually hidden text via `<span className="sr-only">…</span>`. |

```tsx
{/* Decorative — has accompanying "Delete" text */}
<Button variant="destructive" size="sm" className="gap-1.5">
  <Trash2 className="size-3.5" />
  {t("common:actions.delete")}
</Button>

{/* Icon-only button — label on the button */}
<Button variant="ghost" size="icon" aria-label={t("common:actions.delete")}>
  <Trash2 className="size-4" />
</Button>

{/* Stand-alone status icon — label on the icon (rare) */}
<Check className="size-4 text-status-active" aria-label={t("common:status.completed")} />
```

The Lucide React component renders `aria-hidden="true"` automatically
when no label-related prop is set. Don't write `aria-hidden={true}`
explicitly — it's noise. If the icon is stand-alone and meaningful, set
`aria-label` (Lucide will drop `aria-hidden` and apply the label).

## Color

Decorative icons inherit color from `text-*` on the parent or `text-muted-foreground`:

```tsx
<Icon className="size-4 text-muted-foreground" />     {/* decorative cue */}
<Icon className="size-4 text-primary" />              {/* primary action */}
<Icon className="size-4 text-destructive" />          {/* destructive */}
<Icon className="size-3 text-status-active" />        {/* domain status */}
```

Hard rules:

- **Don't color icons with raw colors** (`text-amber-500`, `text-red-600`).
  Use the token (`text-status-expiring`, `text-destructive`) — see
  [styles-and-tokens.md](styles-and-tokens.md).
- **Don't paint decorative icons in primary by default.** Decorative
  cues read with `text-muted-foreground`; primary is a deliberate
  emphasis, not a default.

## Hero-row pattern

The "icon tile next to a title + subtitle" appears in dialogs, settings
rows, onboarding banners, empty states. Always the same shape:

```tsx
<div className="flex items-center gap-3">
  <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 shrink-0">
    <Icon className="size-5 text-primary" />
  </div>
  <div>
    <p className="font-semibold text-sm">{t("…title")}</p>
    <p className="text-xs text-muted-foreground">{t("…subtitle")}</p>
  </div>
</div>
```

Vary the `bg-*/10` token (and the icon `text-*`) when the surface should
read warning, destructive, or status — never the icon size.

## Adding a new icon

1. Pick from <https://lucide.dev/icons>. Aim for the most semantically
   close icon; don't proliferate near-synonyms.
2. Import it by name from `lucide-react`.
3. Apply a size class from the scale above.
4. If it's decorative, leave `aria-hidden` to Lucide. If it stands alone,
   add `aria-label`. If it's inside an icon-only button, label the
   button.
5. Color via tokens (`text-muted-foreground`, `text-primary`,
   `text-status-*`) — never raw Tailwind palettes.

## Anti-patterns

- `<i className="fa fa-trash" />` — bans on sight.
- `import { FaTrash } from "react-icons/fa"` — same.
- `<svg ...><path d="…"/></svg>` inline in a component — file an issue
  upstream and use the closest Lucide for now.
- `<Trash2 width={14} height={14} />` — use `className="size-3.5"`
  instead, so density / dark-mode utilities can compose.
- `<Trash2 aria-hidden={true} />` — Lucide already does this; redundant.
- `<button><Trash2 /></button>` with no label — accessibility violation.
  Use `<Button size="icon" aria-label="…">`.
