# Interaction States

The eight states every interactive surface has, and how each one looks.
**Committed.** When you build a new component, walk this list — if a
state is missing, the component isn't done.

## The eight states

1. **Default** — at rest, no input.
2. **Hover** — pointer over.
3. **Focus-visible** — keyboard arrived.
4. **Active** — pressed but not released.
5. **Disabled** — not interactive (greyed, but reachable by Tab? See below).
6. **Loading** — request in flight.
7. **Empty** — the data is genuinely empty.
8. **Error** — the request, or input validation, failed.

A ninth, **Selected**, applies to multi-select surfaces (rows in a
list, items in a gallery). It's a long-lived state, not a transient
one — see "Selection" below.

## Default

The lowest-energy state. Body uses `text-foreground` (or `text-muted-foreground`
for secondary cues), surfaces use `bg-card` / `bg-background`, borders
use `border-border`. No tints, no fills.

## Hover

Hover is feedback that the surface is interactive — never a transform.

| Element | Hover |
| --- | --- |
| Filled button | `hover:bg-primary/90` (default) / `hover:bg-destructive/90` |
| Outline button | `hover:bg-accent/10` |
| Ghost button | `hover:bg-muted` |
| Link | `hover:text-foreground` (when starting from `text-muted-foreground`) |
| List row | `hover:bg-muted/40` |
| Reveal-on-hover action (kebab in row) | `opacity-0 group-hover:opacity-100 transition-opacity` |
| Card-as-button | `hover:bg-muted/30 hover:border-foreground/15 transition-colors` |

`transition-colors duration-150` is the default transition. No
`hover:translate-*`, no `hover:scale-*`. See [04-elevation-and-effects.md](04-elevation-and-effects.md).

## Focus-visible

The keyboard arrived. shadcn primitives bake in:

```css
focus-visible:ring-[3px] focus-visible:ring-ring/50
```

— a 3px amber ring at 50% opacity, against the adjacent surface. This
is the only depth cue we use.

Hard rules:

- **`focus-visible:`, never `focus:`.** Mouse-clickers shouldn't see a
  ring after pressing a button.
- **Never `outline: none`** without `focus-visible:ring-*`. A removed
  outline + no ring is a critical a11y bug.
- **Don't tone the ring down** to `ring-[1px]` or `ring/20`. The 3px
  amber is the contract.

## Active

Pressed but not released. A color shift, not a transform:

```tsx
className="active:bg-primary/95"   // filled
className="active:bg-muted/60"     // ghost / list row
className="active:bg-destructive/95"
```

`active:` fires on mouse-down on desktop, and on touch-press on mobile —
both correct.

## Disabled

The control is not interactive. Two flavors:

| Flavor | When | How |
| --- | --- | --- |
| **Reachable disabled** | The user should know it exists; an input that's about to become valid as they fix another field. | `disabled` prop on the element + the visual state shadcn ships (50% opacity, cursor-not-allowed). Stays in tab order. |
| **Unreachable disabled** | The action genuinely doesn't apply; pre-launch feature flag. | Hide it. A control that can never be reached is clutter. |

Don't fake-disable with `pointer-events: none` or `opacity-50` alone —
that breaks ARIA and screen-reader announcement. Use the `disabled`
prop on `<button>` / `<input>`; for non-form elements, use
`aria-disabled="true"` and skip the click handler.

## Loading

Three sub-flavors:

- **In-flight mutation** — gate the submit button on
  `mutation.isPending` and show a spinner *inside* the button (`<Loader2
  className="size-4 animate-spin" />`). Don't replace the label —
  keep both.
- **In-flight query** — render a skeleton-shaped placeholder. See
  [05-motion.md](05-motion.md) ("Skeletons over spinners").
- **Indefinite long-running** (export running on the server) — show
  the actual server-side state via SSE / polling, not a spinner. The
  UI says "Running… 12 of 184 items".

```tsx
<Button disabled={mutation.isPending}>
  {mutation.isPending && <Loader2 className="size-4 animate-spin" />}
  {t("common:actions.save")}
</Button>
```

## Empty

The user is genuinely seeing no data. This is a first-class state, not
an afterthought:

```tsx
<div className="flex flex-col items-center justify-center gap-3 py-16">
  <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10">
    <Package className="size-5 text-primary" />
  </div>
  <div className="text-center">
    <p className="text-base font-semibold">No items yet</p>
    <p className="text-sm text-muted-foreground">When you add one, it'll show up here.</p>
  </div>
  <Button size="sm" onClick={onAdd}>
    <Plus className="size-3.5" />
    Add item
  </Button>
</div>
```

The shape is consistent across pages: tile + title + body + (optional)
CTA. See [20-edge-cases.md](20-edge-cases.md) for the full empty-state taxonomy.

## Error

Granularity, in order of severity:

| Level | UI | Example |
| --- | --- | --- |
| Field validation | Inline below the input, `text-destructive text-xs` | "Email is required" |
| Form-level | `<Alert variant="destructive">` above the form | "Invalid credentials" |
| Page-level (data didn't load) | Inline replacement of the data area | "Couldn't load items. Retry." |
| App-level (route blew up) | The 500 page ([20-edge-cases.md](20-edge-cases.md)) | "Something went wrong" |

Color is paired with icon + text. `text-destructive` alone never
communicates an error. See [14-accessibility.md](14-accessibility.md).

## Selection

A long-lived state on a multi-select surface (list rows, gallery
tiles, file picker):

- Visual: `bg-accent/10 ring-1 ring-accent` for tiles; `bg-muted/60
  border-l-2 border-l-accent` for list rows.
- Affordance: a checkbox visible on hover (`opacity-0
  group-hover:opacity-100`); always visible once a single item is
  selected (mode change).
- Keyboard: `Space` toggles, `Shift+ArrowDown` extends.
- Bulk actions appear in a sticky toolbar below the list header — not
  inline next to each row. See [09-component-patterns.md](09-component-patterns.md).

## State precedence

When two states co-exist, this is the rendering order:

1. Disabled wins everything.
2. Focus-visible wins over hover.
3. Active wins over hover (on mouse-down).
4. Selection wins over hover (selection persists).
5. Error wins over default (a field with a red border + destructive
   error text overrides the default border).

## Tests

Every component test exercises at least the four states a button has:
default, hover (`userEvent.hover`), focus (`userEvent.tab` to the
control), active (`userEvent.pointer({ keys: '[MouseLeft>]', target })`),
and — if relevant — error and loading. See
[../testing.md](../testing.md).

## Hard rules

1. **Eight states or it's not done.** Every interactive surface
   answers each.
2. **Color shift, not transform.** Hover and active never translate.
3. **`focus-visible` over `focus`.** The ring is for keyboard arrival.
4. **Disabled is real.** Use the `disabled` attr, not `pointer-events:
   none` + opacity.
5. **Empty has a shape.** Tile + title + body + (optional) CTA.

## Anti-patterns

- A button that gets a 1px shadow on hover. Use a color shift.
- A focus ring that's *just* a thin border darkening (`focus-visible:border-foreground/40`).
  The ring is a 3px amber.
- An empty state that says "Loading…". Empty and loading are
  different states; render different copy.
- A disabled-looking button that fires the click handler anyway.
  ("Look, I disabled it visually." — no, you didn't.)
- Inline error styling in a form (`className="border-red-500"`). Use
  `aria-invalid` + the destructive token via the field primitive.

## Cross-refs

- Component anatomy: [09-component-patterns.md](09-component-patterns.md).
- Edge-case states (404 / 500 / offline / no-group): [20-edge-cases.md](20-edge-cases.md).
- Form-state UX: [15-form-and-data-ux.md](15-form-and-data-ux.md).
- A11y: [14-accessibility.md](14-accessibility.md).
- Loading skeletons: [05-motion.md](05-motion.md).
