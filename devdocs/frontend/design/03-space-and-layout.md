# Space & Layout

The spacing rhythm and the standard layout shells. **Committed.**
Where these patterns disagree with a one-off design, the patterns win
— a one-off PR can either match or open a follow-up to update this
doc.

## Scale

Tailwind's default 4px-step scale. Always pick from this set; never
write `p-[14px]`:

| Class | px | Use |
| --- | --- | --- |
| `gap-1`, `space-y-1` | 4 | Inline cue + label |
| `gap-1.5`, `space-y-1.5` | 6 | Field label + input pairs |
| `gap-2`, `space-y-2` | 8 | Tight inline groups |
| `gap-3`, `space-y-3` | 12 | Stat cards, dialog body rows |
| `gap-4`, `space-y-4` | 16 | Default card-internal rhythm |
| `gap-5`, `space-y-5` | 20 | Card sections that need extra air |
| `gap-6`, `space-y-6` | 24 | Page sections |

`p-3.5` (14px) and `py-3.5` are the one half-step we use — for list
rows and divide-y settings rows. Anywhere else, stick to whole steps.

## Page wrapper

Every full-page view starts with this outer shell:

```tsx
<div className="flex flex-col gap-6 p-6 max-w-2xl mx-auto w-full">
  {/* content */}
</div>
```

Decisions encoded:

- **`p-6` (24px) page padding.** Consistent across every surface so
  the user never sees a "tighter" or "wider" frame depending on which
  page they landed on.
- **`max-w-2xl` for settings / detail / form pages**, `max-w-4xl` for
  list / data pages, `max-w-6xl` for dashboards with multiple wide
  cards. Don't introduce new max-widths.
- **`mx-auto w-full`** centers the column up to the max.
- **`flex flex-col gap-6`** spaces sections at 24px without per-child
  margins.

A side-rail surface (sidebar + main) is the Shell's job — see
[11-page-layouts-and-flows.md](11-page-layouts-and-flows.md) and `frontend/src/app/Shell.tsx`. The
page wrapper above only describes the main column.

## Card

```tsx
<div className="rounded-xl border border-border bg-card p-6 space-y-5">
  {/* card content */}
</div>
```

- `rounded-xl` (12px) for cards. `rounded-lg` (8px) for inputs and
  small chips. `rounded-md` (6px) for tags. `rounded-full` for circular
  avatars / dot indicators.
- `border-border` (the token) — not `border-gray-200`.
- `bg-card` — the token swaps to a slightly lifted dark surface in
  dark mode.
- `p-6` inside, `space-y-5` between sections, `space-y-4` between
  rows of the same shape.

A card never gets a drop shadow — see [04-elevation-and-effects.md](04-elevation-and-effects.md).

## Stat card row

```tsx
<div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
  <StatCard icon={Package} label="Items" value={114} />
  <StatCard icon={Folder} label="Locations" value={4} />
  {/* … */}
</div>
```

Each stat card:

```tsx
<div className="rounded-xl border border-border bg-card px-4 py-3 flex items-center gap-3">
  <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
    <Icon className="size-4 text-muted-foreground" />
  </div>
  <div>
    <p className="text-xs text-muted-foreground">{label}</p>
    <p className="text-lg font-semibold leading-tight">{value}</p>
  </div>
</div>
```

The 8×8 icon tile + 4×4 icon ratio is canonical — don't drift to 10×10
or 6×6.

## Divide list (settings rows)

```tsx
<div className="divide-y divide-border">
  <div className="flex items-center justify-between py-3.5">
    <div>
      <p className="text-sm font-medium">Warranty expiring alerts</p>
      <p className="text-xs text-muted-foreground">Push and email.</p>
    </div>
    <Switch checked={val} onCheckedChange={setVal} />
  </div>
</div>
```

- `py-3.5` (14px) per row. The half-step exists because 12px feels
  cramped and 16px feels loose for settings rows specifically.
- `divide-y divide-border` instead of borders on each row — keeps the
  first / last rows clean.

## Dialog body

```tsx
<DialogContent className="sm:max-w-md">
  <DialogHeader>
    <DialogTitle>…</DialogTitle>
    <DialogDescription>…</DialogDescription>
  </DialogHeader>
  <div className="space-y-4">{/* fields */}</div>
  <DialogFooter className="gap-2">…</DialogFooter>
</DialogContent>
```

- `sm:max-w-md` (448px) for confirmations and short forms.
- `sm:max-w-lg` (512px) for ≤5-field dialogs.
- `sm:max-w-2xl` (672px) for multi-step wizards.
- Dialogs above `max-w-2xl` are a smell — use a Sheet or a full page.

## Sheet (slide-over)

The slide-over for item preview, file detail, and contextual edits.
Width:

| Sheet purpose | Width |
| --- | --- |
| Quick preview (item card, file card) | `sm:max-w-md` |
| Edit form (most cases) | `sm:max-w-xl` |
| Long form / multi-section | `sm:max-w-2xl` |

Sheets always slide in from the right on desktop; on mobile they
become a full-screen modal. Radix `<Sheet>` handles the breakpoints.

## Spacing primitives at scale

| Primitive | Outer | Inner | Between siblings |
| --- | --- | --- | --- |
| Page | `p-6` page wrapper | `space-y-6` between sections | — |
| Card | `p-6` (or `px-4 py-3` for stat cards) | `space-y-5` between blocks, `space-y-4` between rows | `gap-4` between cards in a grid |
| Form field | — | `space-y-1.5` between label, input, error | `space-y-4` between fields |
| List | `divide-y` for settings; bare for tables | `py-3.5` per row | — |
| Stat row | — | `gap-3` icon + text | `gap-4` between stat cards |

## Grids

Use grids for stat rows, gallery layouts, and card decks. Tables
remain `<table>`-shaped.

```tsx
<div className="grid grid-cols-2 gap-4 lg:grid-cols-4">…</div>
<div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">…</div>
```

Breakpoints (`md`, `lg`, `xl`, `2xl`) follow Tailwind defaults. Don't
introduce custom breakpoints.

## Hard rules

1. **`gap-*` between siblings, `space-y-*` inside containers.** Mixing
   `mt-*` / `mb-*` on individual children to get the same effect is a
   regression.
2. **Pick from the scale.** `p-[13px]` gets caught in review.
3. **`rounded-xl` for cards, `rounded-lg` for inputs.** Don't drift.
4. **Page padding is `p-6` everywhere.** Don't tighten the auth pages
   to `p-4` because "they feel cramped" — they aren't, the form is
   centered with `max-w-md`.
5. **No fixed-pixel max-widths.** Use Tailwind's `max-w-*` set:
   `max-w-md`, `max-w-xl`, `max-w-2xl`, `max-w-4xl`, `max-w-6xl`.

## Anti-patterns

- `mt-6 mb-6` on alternating children. Use the parent's
  `space-y-6` / `gap-6`.
- A custom 14px gap. Round to 12 (`gap-3`) or 16 (`gap-4`).
- `p-4` cards next to `p-6` cards on the same page. Pick one density.
- Hand-rolled grid columns (`grid-cols-[1fr_2fr_1fr]`). Use the
  preset Tailwind columns; if you need a custom split, the design is
  off rhythm.
- Adding a 7th max-width to the set. The five we have are enough.

## Cross-refs

- Card / dialog / sheet anatomy: [09-component-patterns.md](09-component-patterns.md).
- Page templates per surface: [11-page-layouts-and-flows.md](11-page-layouts-and-flows.md).
- Density adjustments: [17-density-and-modes.md](17-density-and-modes.md).
