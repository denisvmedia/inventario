# Interaction States

Every interactive element must have a defined behavior in **eight states**. Missing any of these is what makes UI feel like a wireframe.

## The eight states

| State | When | Visual signal |
| --- | --- | --- |
| **Default** (resting) | At rest, idle | Base styling |
| **Hover** | Mouse over | Subtle elevation, color shift |
| **Focus-visible** | Keyboard focus | Focus ring (per `04-elevation-and-effects.md`) |
| **Active** (pressed) | Mouse down / Enter pressed | Slight inset / scale-down |
| **Selected** | User has chosen this | Sustained accent treatment |
| **Disabled** | Cannot interact | Reduced ink, no cursor pointer |
| **Loading** | Async work in progress | Skeleton, spinner inline, or progress |
| **Error** | Something failed | Border/text in destructive, message |

## State definitions per primitive

### Button (default variant)

```
Default:    bg=transparent  border=border-default  ink=ink-primary
Hover:      bg=surface-sunken               (+shadow-xs if raised variant)
Focus:      + focus-ring
Active:     bg=surface-sunken  transform: scale(0.98)
Disabled:   ink=ink-disabled  border=border-subtle  cursor: not-allowed
Loading:    spinner replaces icon, text becomes "..."  not-interactive
Error:      no error state for buttons (errors live on form fields)
```

### Button (primary variant)

```
Default:    bg=accent  ink=accent-foreground
Hover:      bg=accent-hover  shadow-xs
Focus:      + focus-ring
Active:     bg=accent-hover  transform: scale(0.98)
Disabled:   bg=accent  opacity-disabled  cursor: not-allowed
Loading:    inline spinner; bg unchanged  not-interactive
```

### Input (text)

```
Default:    bg=surface-base  border=border-default  ink=ink-primary
Hover:      border=border-strong
Focus:      border=accent  + focus-ring  ink=ink-primary
Active:     same as focus
Filled (has value):  border=border-strong  (subtle indication state ≠ empty)
Disabled:   bg=surface-sunken  ink=ink-disabled  border=border-subtle
Loading:    overlay subtle shimmer (rare — typically inputs aren't async)
Error:      border=destructive  ink=ink-primary  + below-message
```

### Card (clickable)

```
Default:    bg=surface-raised  border=border-subtle  shadow=none
Hover:      shadow=xs  transform=translateY(-1px)  border=border-default
Focus:      + focus-ring  (focus from keyboard only)
Active:     transform=translateY(0)  shadow=none
Selected:   border=accent  bg=accent-soft (rare, e.g. multi-select mode)
Disabled:   ink-disabled  opacity-faded  cursor: not-allowed
Loading:    skeleton replacement
```

### List row

```
Default:    bg=transparent  ink=ink-primary
Hover:      bg=surface-sunken
Focus:      + focus-ring (inset, since rows often span container width)
Active:     bg=surface-sunken (deeper)
Selected:   bg=accent-soft  ink=ink-primary  (multi-select)
Disabled:   ink-disabled
```

### Checkbox / radio

```
Default:    bg=surface-base  border=border-strong  ink=transparent
Hover:      border=accent
Focus:      + focus-ring
Checked:    bg=accent  border=accent  ink=accent-foreground (the check)
Disabled:   bg=surface-sunken  border=border-subtle
Indeterminate (checkbox): bg=accent  ink=accent-foreground (dash)
```

### Switch (toggle)

```
Default (off):  bg=border-default  thumb=surface-raised
Hover (off):    bg=border-strong
Default (on):   bg=accent  thumb=surface-base
Hover (on):     bg=accent-hover
Focus:          + focus-ring around track
Disabled:       opacity-disabled  cursor: not-allowed
```

## Loading states

Three forms, used in different contexts:

### Skeleton (preferred for layout-stable content)

A pulsing placeholder shaped like the eventual content. Use for:
- Lists, cards, tables loading data
- Profile/detail pages on first render
- Chart loading

Pulsing background gradient via CSS animation, no JS needed.

```css
@keyframes skeleton-pulse {
  0%, 100% { background-position: 0 0; }
  50%      { background-position: -200% 0; }
}

.skeleton {
  background: linear-gradient(
    90deg,
    var(--surface-sunken) 0%,
    var(--surface-base) 50%,
    var(--surface-sunken) 100%
  );
  background-size: 200% 100%;
  animation: skeleton-pulse 1500ms linear infinite;
  border-radius: var(--radius-md);
  color: transparent;
  user-select: none;
  pointer-events: none;
}
```

Skeleton must match eventual content shape (same heights, widths, spacing) — otherwise content "jumps" in.

### Inline spinner

For button-internal loading and small async ops. ~14px Phosphor `CircleNotch` with `animate-spin`. Replaces the leading icon, never appears alongside.

### Progress bar

For long-running operations (file upload, large export). Linear bar at top of dialog or inline. Always determinate when possible — show percentage.

### Pulsing dot

For "live" indicators (connection state, recent change). Used sparingly.

## Empty states

A separate state from loading. **Documented in `11-page-layouts-and-flows.md`** as a major design pattern. Three flavors:

1. **First-time empty** — "You haven't added anything yet. Let's start."
2. **Filtered empty** — "No items match this filter." + Clear filters button
3. **Search empty** — "No results for 'foo'." + Try other terms hint

Different copy per context. Same component primitive (`EmptyState`).

## Error states

Three layers, by scope:

| Scope | Pattern |
| --- | --- |
| Single field | Below the input, red text + icon, border on field |
| Form | Top of form, summary list with anchor links to broken fields |
| Page | Replaces page content with error illustration + retry CTA |
| Toast (transient) | Bottom-right, dismissible, auto-clear after 8s |
| Critical (destructive blocked) | Inline alert in dialog, prevents submission |

**Error copy rules** (per `12-tone-of-voice.md`):
- Plain language ("Couldn't save that.")
- What to do next ("Try again, or check your connection.")
- Never "Error: 500" — translate at boundary

## Selected vs active

These two states confuse implementers regularly:

- **Selected** is **persistent** — the user picked this item and it remains chosen until they pick another or close the screen. Examples: tab selected, table row in multi-select, sidebar item for current page.
- **Active** is **transient** — the user is currently pressing this. Lasts the duration of the interaction.

Use `aria-selected="true"` for selected; `:active` pseudo-class for active. They have different visual treatments above.

## Drag-and-drop states

For draggable thumbnails (file gallery reorder), draggable list items, or drop targets:

| State | Treatment |
| --- | --- |
| Drag-source (during drag) | opacity 0.4, cursor: grabbing, no shadow |
| Drag-over (drop target valid) | border 2px accent dashed, bg accent-soft |
| Drag-over (drop target invalid) | border 2px destructive dashed, bg destructive-soft |
| Drop (release) | brief flash bg accent-soft, then back to default |

Implement via Vue Draggable or HTML5 native drag. Always show drop-zone affordance — never a silent drop.

## Hover-tooltip pattern

Every icon-only button gets a tooltip on hover (and on focus, for keyboard). Tooltip:
- Appears after **300ms** delay (avoids flicker)
- Disappears immediately on mouseleave
- Maximum 3 words ("Edit location", "Delete", "Sort by date")
- Uses Reka UI Tooltip primitive, not browser-native title attribute (which is unstyled)

Title attribute kept as fallback for screen readers.

## Sticky / pinned states

For sticky table headers, sticky section headers in long pages:
- At rest, transparent background
- When stuck (scrolled into view): backdrop-filter blur(4px) + bg=surface-base/0.85
- Subtle shadow appears beneath when content scrolls under

Use `IntersectionObserver` to toggle a `.is-stuck` class — pure CSS via `position: sticky` won't trigger the styling change.

## Locked / read-only states

For fields that cannot be edited (e.g., file upload date):
- Same visual as default but no border (or border-subtle)
- Cursor: default (not text)
- No focus ring on tab — skipped from tab order
- Distinct from disabled — disabled means "not allowed right now", read-only means "this can never be changed"

## Anti-patterns

- ❌ Hover effect that causes layout shift (e.g., margin change) — use transform
- ❌ Loading states that disable the whole page when only one widget is loading
- ❌ Click handlers without `:hover` style — invisible interactivity
- ❌ Disabled state that shows pointer cursor (lying)
- ❌ Error message inside the field placeholder (disappears when typing)
- ❌ Loading spinner with no max-time — must transition to error after 30s
