# Elevation & Effects

**Committed.** Inventario uses **borders, not shadows**. The single
exception is `shadow-xs` on inputs (built into the shadcn `Input`
primitive). This rule is older than the React rewrite and survived
cutover; don't re-litigate without an issue.

## The rule

Elevation is communicated by **bordered surface contrast**, not
shadow:

- Page background = `--background` (warm off-white).
- Card surface = `--card` (pure white in light, lifted dark in dark).
- Card border = `--border` (subtle warm gray).

The visual difference between page and card is the surface color
plus a 1px border. No shadow. No `border-2`. Subtle but unmistakable.

## Why no shadows

1. **Honesty.** A drop shadow says "this thing is floating above the
   page" — a visual lie about a flat document. Borders say "this is a
   region", which is what cards actually are.
2. **Density.** Stacked shadows compound into noise; stacked borders
   stay crisp. Inventario's surfaces (settings rows, list rows, stat
   cards) stack densely.
3. **Dark-mode parity.** Shadows look heavy in dark mode and require
   per-mode tuning. Borders use opacity (`oklch(1 0 0 / 10%)`) and
   render the same way in both modes.
4. **System aesthetic.** Things 3, Linear (early), Notion's recent
   editions all moved away from shadows toward borders. The references
   in [00-positioning.md](00-positioning.md) set the tone.

## What's allowed

| Effect | Where | Token |
| --- | --- | --- |
| `shadow-xs` | shadcn `Input` (built in) | Tailwind default |
| 1px border | every card, dialog, sheet, popover surface | `--border` (or `--sidebar-border` etc.) |
| `border-current/20` | tag pills, status badges | derived from the foreground |
| Backdrop blur | none | — |
| Glassmorphism | none | — |
| Inset shadow / inner glow | none | — |
| Hover-lift transform | none | — |

The `shadow-xs` exception exists because input fields without a hint
of depth read as flat label + caret on the page, which makes them
hard to spot for new users. It's a 1px-blur shadow — barely visible,
purely a focus aid.

## What's banned

- `shadow-sm`, `shadow-md`, `shadow-lg`, `shadow-xl`, `shadow-2xl` on
  any surface.
- `drop-shadow-*` filters.
- `backdrop-blur-*`.
- Box-shadow for glow rings (use `--ring` via `focus-visible:ring`
  instead).
- `transform: translateY(-2px)` on hover.

If you reach for one of these, the underlying need is probably:

| Reaching for | Want | Use |
| --- | --- | --- |
| `shadow-sm` to mark a card | "this is a card" | The card's existing border + bg-card. |
| `shadow-lg` on a dialog | "this is on top" | Radix `<DialogContent>` already has the right backdrop; don't add shadow. |
| `backdrop-blur` on a top bar | "scrollable content under" | A solid `bg-background/80` border + `backdrop-blur-sm` is allowed *only* on the Shell's top bar. Borders elsewhere. |
| `drop-shadow` on an icon | "this icon stands out" | Pair the icon with a tinted background tile (`bg-primary/10`). |

## Borders

| Style | Token / class | Use |
| --- | --- | --- |
| 1px subtle | `border border-border` | Card, dialog, sheet, popover |
| 1px stronger | `border border-foreground/20` | Hover/active states (rare) |
| Tinted | `border-current/20` | Tag pills, status badges |
| 0 | `border-0` | Removed-default-border surfaces |
| 1px destructive | `border-destructive/40` | Inline error highlights |
| 0.5px | not supported | Use 1px |
| Dashed | rare; only on drop-zone targets | `border-dashed border-2 border-border` |

The drop-zone exception (`border-dashed border-2`) is the one place
where a thicker border earns its weight — see [10-file-and-media.md](10-file-and-media.md).

## Focus

The focus ring **is** an effect. shadcn primitives bake in:

```css
focus-visible:ring-[3px] focus-visible:ring-ring/50
```

— a 3px amber ring at 50% opacity. Don't override. See
[14-accessibility.md](14-accessibility.md) for the keyboard-only contract.

## Hover state

Hover is a visual cue, not a transform. Patterns:

| Element | Hover |
| --- | --- |
| Button | `hover:bg-*/90` (filled) / `hover:bg-accent/10` (ghost) |
| Card-row | `hover:bg-muted/40` |
| Link | `hover:text-foreground` (when starts at `text-muted-foreground`) |
| Icon-only ghost button | `hover:bg-muted` |
| Reveal action (kebab in row) | `opacity-0 group-hover:opacity-100 transition-opacity` |

`transition-colors` is the default transition utility. No
`transition-transform`, no `transition-shadow`. See [05-motion.md](05-motion.md)
for durations.

## Active state

The user just pressed it but hasn't released:

```tsx
className="active:bg-primary/95"   // filled
className="active:bg-muted/60"     // ghost
```

Active is the depth cue you'd otherwise reach for an inset shadow
for. Color shift, not transform.

## Inset / outset / cutout

None of these. Inventario is a flat document with regions. Surfaces
have edges (borders), not three-dimensional cues.

## Hard rules

1. **No drop shadows.** Anywhere except `shadow-xs` on inputs.
2. **No transforms on hover** — `hover:translate-y-[-2px]` is a ban
   on sight.
3. **Border or color, never glow.** Focus rings via the `--ring`
   token's amber via shadcn defaults; "highlighted" via tinted bg
   (`bg-primary/10`), not via `box-shadow`.
4. **No backdrop blur** beyond the Shell's top bar (and even that's
   optional — the bar can be a solid `bg-background border-b`).
5. **No "elevation tiers".** Material's `shadow-1` … `shadow-24`
   model is a different product's choice. We have one tier: bordered.

## Anti-patterns

- A stat card with `shadow-md` because "it's important". Importance
  is conveyed by the value's typography (`text-2xl font-bold`), not
  by floating it.
- Hover-lifting a row with `hover:translate-y-[-1px]`. Use
  `hover:bg-muted/40`.
- A glassmorphism login background. The auth pages are quiet; a
  blurred hero contradicts the tone.
- `box-shadow: 0 0 0 4px var(--ring)` to hand-roll a focus ring. The
  shadcn primitive already does it via `ring-*` utilities.

## Cross-refs

- Tokens: [01-palette.md](01-palette.md).
- Spacing rhythm: [03-space-and-layout.md](03-space-and-layout.md).
- Hover / focus / active states: [08-interaction-states.md](08-interaction-states.md).
- Focus ring contrast: [14-accessibility.md](14-accessibility.md).
