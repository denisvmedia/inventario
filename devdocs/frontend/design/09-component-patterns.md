# Component Patterns

Anatomy and rules for every reusable component primitive in Inventario. Engineering reference; if it's here, it's specced. If a sprint task needs a component not on this list, add it here first.

## Primitives covered

1. Button
2. Input (text, email, password, number, date)
3. Textarea
4. Select / Combobox
5. Checkbox
6. Radio group
7. Switch
8. Dialog (modal)
9. Drawer (mobile / side-sheet)
10. Toast
11. Tooltip
12. Popover
13. Card
14. Badge
15. Status pill
16. Tag / Chip
17. Avatar
18. Tabs
19. Accordion
20. Breadcrumb
21. Pagination
22. Search input
23. Filter bar
24. Table / Data grid
25. List
26. Empty state
27. Skeleton
28. Loader / Spinner
29. Progress bar
30. Notification (severity callout)
31. Form section
32. Form footer

## 1. Button

### Variants
- **primary** — main CTA per page (max one), `bg=accent`
- **secondary** — common actions, `bg=transparent border=border-default`
- **ghost** — tertiary, `bg=transparent` no border, used in toolbars
- **destructive** — delete, irreversible, `bg=destructive`
- **link** — looks like a link, sits inline in text

### Sizes
- **sm** — `h-8 (32px)  px-3  text-body-sm`
- **md** — `h-10 (40px)  px-4  text-body` (default)
- **lg** — `h-12 (48px)  px-5  text-body-lg`
- **icon** — `h-10 w-10` square (or sm/lg variants)

### Anatomy
```
[leading-icon?]  Label  [trailing-icon? | counter?]
```

### Rules
- **One** primary button per surface — no exceptions
- Icon-only buttons require `aria-label`
- Loading state replaces leading icon with spinner; preserves width
- Destructive actions require **confirmation dialog** on click (per `15-form-and-data-ux.md`)
- Buttons sit at logical end of dialogs/forms — primary on the right (LTR), secondary to its left, ghost-cancel on the far left

### Anti-patterns
- ❌ "Save" + "Save and Continue" + "Save Draft" all primary
- ❌ Mixing button + link styling on the same row
- ❌ Buttons without explicit size — gets default from CSS reset, looks broken

## 2. Input

### Anatomy
```
[label]                              [optional hint]
┌────────────────────────────────┐
│ [leading-icon?]  value         │
│                  [trailing-action?] │
└────────────────────────────────┘
[helper-text  |  error-message  |  character-count]
```

### Rules
- Label always present and **above** the field, never floating-label (those fail accessibility and i18n)
- Labels in `body-sm font-medium`, color `--ink-primary`
- Required indicator: `*` in `--destructive` after label text — no asterisk explanation needed
- Helper text in `body-xs` `--ink-muted` below; replaced by error message when invalid
- Error state: border `--destructive`, message in `body-xs` `--destructive`, plus an error-icon trailing the field
- Filled state (has value) gets `border-strong` to differentiate from empty

### Sizes match Button (sm/md/lg)

### Trailing actions
- Clear button (X) appears when field has value and is hovered/focused
- Reveal/hide for password fields
- Calendar icon for date fields
- Currency code suffix for amount fields (read-only)

## 3. Textarea

Same as Input but `min-height: 5rem` (~80px), expandable via drag handle in bottom-right corner. Character counter optional — if present, in `body-xs` `--ink-muted` aligned bottom-right inside the field.

## 4. Select / Combobox

Use Reka UI's `Combobox` primitive. Always typeahead-searchable when option count >= 6.

### Anatomy
```
┌────────────────────────────────┐
│ [leading-icon?]  Selected option │
│                       [chevron] │
└────────────────────────────────┘
```

### Dropdown panel
- Max height `360px`, then scrollable
- Search input pinned at top when option count >= 6
- Each option: ghost-list-item style, hover bg `--surface-sunken`, selected bg `--accent-soft`, check icon trailing
- Empty state inside dropdown: "No options match" + body-sm
- Click-outside or Escape closes

### Multi-select
Renders selected values as chips inside the trigger. Beyond 3 chips: "+N more".

## 5. Checkbox

Standard pattern. 16px box, accent-bg when checked, white check icon (Phosphor Check, weight bold).

Indeterminate: dash icon (Phosphor Minus). Used for "select all" when partial selection exists.

## 6. Radio group

Vertical stack default, horizontal allowed for ≤3 options that fit one line. Each item: 16px circle + 8px gap + label. Active radio shows accent-filled center dot.

## 7. Switch

Use for **immediate-effect toggles** (no save button required). Track 32×18px, thumb 14px.

When the switch toggles a setting that *requires* a save action, use a Checkbox instead. Switches imply "applied immediately."

## 8. Dialog (modal)

Use Reka UI Dialog primitive. Fullscreen overlay + centered content.

### Sizes
- **sm** — `max-w-md` (448px) — confirmations, simple forms
- **md** — `max-w-2xl` (672px) — standard forms, settings panels (default)
- **lg** — `max-w-4xl` (896px) — multi-section forms, file pickers
- **xl** — `max-w-6xl` (1152px) — file viewer, large content
- **fullscreen** — `inset-4` with `radius-xl` — file viewer (per `10-file-and-media.md`)

### Anatomy
```
┌─────────────────────────────────────────┐
│ [Title]                          [X]    │  header  --space-5  border-b
├─────────────────────────────────────────┤
│                                         │
│ [body content, scrollable if tall]      │  body  --space-5
│                                         │
├─────────────────────────────────────────┤
│       [ghost cancel] [secondary] [primary] │  footer  --space-5  border-t
└─────────────────────────────────────────┘
```

### Rules
- Title in `heading-md` weight medium
- X button top-right is **redundant with Escape and overlay click** — keep it for discoverability, but those mechanisms must work
- Body scrolls when tall; header and footer remain visible
- Mobile: dialogs >sm become full-screen drawers (use Drawer primitive instead)
- Focus trap inside dialog (Reka UI handles)
- Initial focus on first form input or primary button
- Background overlay: `--overlay-scrim` + backdrop-blur(8px)
- Z-index: 50 (above sticky headers)

## 9. Drawer (mobile / side-sheet)

Slides from edge. On mobile: from bottom; on desktop side-panel: from right.

### Sizes
- **bottom mobile**: full width, max-height 90vh, swipe-down to dismiss, drag handle visible at top
- **right side**: 400px desktop, full-width mobile

### Header includes title + close X. Footer pinned. Body scrolls.

## 10. Toast

Use Sonner-vue or build on Reka UI primitives.

### Variants
- success, info (default), warning, destructive

### Anatomy
```
┌──────────────────────────────────────┐
│ [icon]  Message                  [X] │
│         [optional action button]     │
└──────────────────────────────────────┘
```

### Rules
- Position: bottom-right desktop, bottom-center mobile
- Stack vertically, max 3 visible — older ones expire
- Auto-dismiss: 4s success, 6s info/warning, never auto-dismiss destructive (require explicit action)
- Action button (Undo, Retry, View) preferred when context calls for it
- Slide-in from right with spring easing, slide-out to right
- ARIA: `role="status"` for info/success, `role="alert"` for warning/destructive

## 11. Tooltip

Reka UI Tooltip. Per `08-interaction-states.md`:
- 300ms hover delay
- Max 3 words
- Position: above by default, flip to below if no room
- Background: `--surface-overlay-strong`, ink `--ink-primary` (inverted in dark mode)
- Arrow pointing to anchor
- Z-index: 60 (above dialogs)

## 12. Popover

Larger than tooltip, contains interactive content. Used for:
- Filter quick-edit
- User menu
- Date picker
- Color picker

Reka UI Popover. Max width 320px default, expandable. Background `--surface-raised`, shadow `md`, radius `lg`. Closes on click-outside or Escape.

## 13. Card

The primary surface unit. Anatomy:
```
┌────────────────────────────────┐
│ [optional thumbnail / banner] │
│                                │
│ [Title]                        │
│ [Subtitle — optional]          │
│                                │
│ [Body / details]               │
│                                │
│ [Footer actions — optional]    │
└────────────────────────────────┘
```

### Rules
- `bg=surface-raised  border=border-subtle  radius=lg  padding=padding-card`
- Hover (if interactive): per `08-interaction-states.md`
- Title in `heading-md` weight semibold
- Subtitle in `body-sm` muted
- Cards in a grid: equal heights, content baseline-aligned

## 14. Badge

Small, inline, decorative or numeric.
```
[●] 12        ↳ count badge (bg=accent, ink=accent-foreground, ~18px)
[Draft]       ↳ text badge (bg=surface-sunken, body-xs uppercase tracking-uppercase)
```

Difference from Status pill: badges are decorative (counts, labels); pills convey semantic state.

## 15. Status pill

Semantic state communication — "In use", "Completed", "Draft", "Archived", "Expired".

### Anatomy
```
[●] Status text
```

Leading dot in semantic color, label `body-xs` `font-medium`.

### Variants
- **success** (`--success`) — completed, active, valid, in use
- **warning** (`--warning`) — expiring soon, action needed
- **info** (`--info`) — draft, in progress
- **destructive** (`--destructive`) — expired, failed
- **muted** — archived, inactive

### Rules
- Always use the same word for the same state across the product (per [`12-tone-of-voice-and-copy.md`](./12-tone-of-voice-and-copy.md))
- Pills are non-interactive (no hover); to filter by status, use a separate filter bar

## 16. Tag / Chip

User-applied labels (like "outdoor", "seasonal" on commodity).

### Anatomy
```
[Tag text]   [×]    ↳ removable
[Tag text]          ↳ static
```

### Rules
- bg `--accent-soft`, ink `--accent`, radius `full`, body-xs medium, padding `space-1 space-2`
- Removable variant has trailing X (Phosphor X, weight bold, size icon-xs)
- Multiple tags wrap with `space-1` gap

## 17. Avatar

User identity. Sizes: 24, 32, 40, 48, 64. Initials fallback when no photo.

```css
.avatar {
  background: var(--accent-soft);
  color: var(--accent);
  border-radius: var(--radius-full);
  display: grid; place-items: center;
  font-weight: var(--font-weight-medium);
}
```

## 18. Tabs

Horizontal tab strip with bottom-border indicator on active. Reka UI Tabs.

### Rules
- Active tab: ink `--ink-primary`, weight medium, 2px underline `--accent`
- Inactive: ink `--ink-secondary`, no underline
- Hover inactive: ink `--ink-primary`
- Body-md size on tab labels
- Underline transitions across via CSS `transform`, not redrawing
- Mobile: scrollable tabs row, never wrap

## 19. Accordion

Reka UI Accordion. Default: only one open at a time.

### Anatomy
```
┌─────────────────────┐
│ Title  ▾            │  trigger (radius-md, bg-transparent, hover bg-sunken)
└─────────────────────┘
   panel content (animated height)
```

Chevron rotates 180° on open via `transform`, `--duration-base`.

## 20. Breadcrumb

For deep entity navigation. Inline, body-sm.
```
Locations / Home / Bedroom
```
Separators: `/` in `--ink-muted`. Last item not a link, weight medium.

## 21. Pagination

For lists >50 items.

### Anatomy
```
[‹ Prev]  [1] [2] [3] … [12] [Next ›]
```

### Rules
- Show first, last, and ±1 around current; ellipsis for gaps
- Current page: `bg=accent  ink=accent-foreground  radius=md`
- Page size selector inline: "12 per page ▾"
- Total count: "Showing 1–12 of 142" (`body-sm` muted)

## 22. Search input

Specialized text input.

### Anatomy
```
┌────────────────────────────────┐
│ [magnifier]  Search…  [⌘K]     │
└────────────────────────────────┘
```

### Rules
- Leading magnifier icon (Phosphor MagnifyingGlass, regular)
- Placeholder "Search…" (em-dash ellipsis, not three dots)
- Trailing keyboard shortcut hint when global ("⌘K"), with subtle bg
- Debounce 250ms before firing query
- Clear (X) button when has value
- On focus, opens command palette if it's the global one

## 23. Filter bar

Horizontal row above lists/tables with filter chips and search.

### Anatomy
```
[Search…]  [Type ▾]  [Status ▾]  [Date range ▾]  [+ Add filter]   [Clear]
```

Mobile: collapses to single "Filters (3)" button that opens a Drawer.

## 24. Table / Data grid

For dense list views (Exports). When user selection is multi-row + sort + bulk-actions.

### Anatomy
- Sticky header with sortable columns (chevron icon when sorted, dimmed when not)
- Row hover: bg `--surface-sunken`
- Row selected: bg `--accent-soft` + checkbox checked
- Empty state replaces tbody when zero rows
- Bulk-action bar appears as a sticky overlay when ≥1 row selected: "3 selected • [Export] [Delete] [Cancel]"
- Mobile: cards instead of rows (responsive transform)

### Rules
- Avoid tables for <12 columns of data; use cards (Inventario commodities) instead
- Tabular numerics on all numeric cells
- Right-align numeric/currency columns; left-align text

## 25. List

Vertical stack of rows (locations, areas, files in folders).

### Anatomy
```
┌─────────────────────────────────────┐
│ [icon] Title      [meta]  [actions] │  row 1
├─────────────────────────────────────┤
│ [icon] Title      [meta]  [actions] │  row 2
└─────────────────────────────────────┘
```

Border between rows in `--border-subtle`. Hover bg `--surface-sunken`.

## 26. Empty state

Specced in `11-page-layouts-and-flows.md`. Variants:
- First-time empty
- Filtered empty
- Search empty
- Error empty

Component takes `icon | illustration`, `title`, `description`, `action` (optional CTA).

## 27. Skeleton

Per `08-interaction-states.md`. Component variants:
- `<Skeleton variant="text" />` — single line of text-shaped skeleton
- `<Skeleton variant="title" />` — heading-shaped
- `<Skeleton variant="card" />` — card-shaped block
- `<Skeleton variant="circle" :size="40" />` — for avatars

## 28. Loader / Spinner

Phosphor `CircleNotch` rotating. Sizes match icon scale.
```vue
<Spinner size="md" />
```

For page-level loading: full-page centered spinner is **forbidden** — use skeletons.

## 29. Progress bar

Linear bar. Determinate (with percentage label) preferred.

```
┌─────────────────────────────┐
│ ▓▓▓▓▓▓▓▓░░░░░░░░░░  43%      │
└─────────────────────────────┘
Uploading 3 of 7 files…
```

Indeterminate: shimmer animation, no percentage. Used when total unknown.

## 30. Notification (severity callout)

Inline alert box for non-toast warnings (e.g., on a form: "This action cannot be undone.").

### Variants
- info, success, warning, destructive

### Anatomy
```
┌─────────────────────────────────────┐
│ [icon] Title                        │
│        Optional description text.   │
│        [optional action]            │
└─────────────────────────────────────┘
```

### Rules
- bg: `--*-soft` (e.g., `--warning-soft`)
- border-left 3px solid in semantic color
- icon: Phosphor Info / Check / Warning / X-circle
- Used on dashboards (attention items), forms (irreversible warnings), file pages (storage low)

## 31. Form section

Logical grouping inside a form view.

### Anatomy
```
┌───────────────────────────────────────┐
│ Section title                         │  heading-md, --ink-primary
│ Optional subtitle / context.          │  body-sm, --ink-muted
│                                       │
│ [field grid]                          │
└───────────────────────────────────────┘
```

Vertical gap between sections: `--gap-stack-section`.

## 32. Form footer

Sticky bottom bar within form views. Contains primary action and cancel.

### Anatomy
```
┌────────────────────────────────────────────────┐
│ [unsaved-changes hint]  [Cancel] [Save changes] │
└────────────────────────────────────────────────┘
```

### Rules
- Sticky to bottom of form viewport (within scroll container)
- Saves are explicit, no auto-save unless `15-form-and-data-ux.md` flow specifies
- Disabled "Save" button when no changes vs. baseline
- Unsaved-changes hint: "Unsaved changes" in body-sm, `--ink-muted`, replaced by spinner/checkmark on save in flight/done

## What ships in sprint 0

Build/refresh primitives in this priority order (week 1):
1. Button (all variants, sizes, loading state)
2. Input + Textarea
3. Dialog (sm/md/lg sizes — fullscreen waits for FileViewer rebuild)
4. Toast (Sonner integration)
5. Skeleton
6. EmptyState
7. Card (refactor to spec)
8. Badge / Status pill / Tag

Then in week 2:
9. Select / Combobox
10. Tabs / Accordion
11. Tooltip / Popover
12. Form section / footer
13. Pagination
14. Search input
15. Notification

Remaining (Drawer, Avatar, Breadcrumb, full Table) ship as needed in sprint 1+.
