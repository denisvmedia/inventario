# Form & Data UX

How forms, mutations, and async state behave from the user's perspective. The unglamorous work that separates "competent app" from "pleasant to use".

## Form save patterns

Three modes, used in different contexts:

### A. Explicit save (default for create / edit)

User edits → fields hold pending state → user clicks Save → save fires → confirmation.

**When to use:** create or edit forms with multiple fields; anything where the user thinks of the changes as a "draft" until committed.

**Behavior:**
- Save button disabled until any field has changed from baseline
- "Unsaved changes" indicator next to the Save button (subtle, body-sm muted)
- Exit-without-saving guard: if user navigates away with unsaved changes, show a confirmation dialog ("You have unsaved changes. Discard them?")
- After save: success toast, button returns to disabled, route may redirect to detail
- After save error: button re-enabled, error inline above form, fields keep user's input

### B. Auto-save (for incremental edits)

User edits a single field → blur or 1s debounce → save fires invisibly → subtle confirmation.

**When to use:** detail-view inline edits, settings, single-field updates where the field IS the form.

**Behavior:**
- After value changes, wait 800ms debounce or `blur`
- Save fires; field shows subtle pending indicator (small spinner adjacent or inset)
- On success: brief check icon (1s) then back to normal
- On error: red border + inline message, value reverts to last-saved on Esc, retry on Enter
- Last-saved time visible somewhere ("Saved 2 seconds ago")

### C. Optimistic mutation (for low-risk toggles)

User clicks → UI updates instantly → save fires → on error, revert + toast.

**When to use:** toggles (mark as draft, archive, favorite), reorder, delete-with-undo.

**Behavior:**
- UI changes immediately on click
- Network request fires in background
- On success: nothing visible (UI already shows the new state)
- On failure: revert UI + toast "Couldn't save. Try again." + Retry button
- Concurrent rapid clicks debounce — last click wins

## Validation timing

| Trigger | What validates | Why |
| --- | --- | --- |
| onBlur | Format checks (email, date, length) | User is "done" with this field |
| onSubmit | All fields | Final gate |
| onChange (live) | Only when **fixing** an existing error | Don't pester with errors while typing |

**Anti-pattern:** validating on every keystroke from the start. Causes "Required" to flash before user is finished.

## Delete patterns

### Single delete (low-risk: file, tag, single thing without children)

- Click Delete → toast appears with "Deleted [name]. · Undo"
- Optimistic — UI updates immediately
- Undo button restores within 5 seconds
- After 5s, deletion is committed
- If hard-delete only, treat as high-risk pattern instead

### Single delete (high-risk: thing with files, place with areas)

- Click Delete → confirmation dialog
- Dialog shows consequence count: "This will remove the item, its 5 images, and 3 manuals."
- Type-to-confirm only if cascade count > 10 children
- After confirm: optimistic UI update + commit

### Bulk delete

- Multi-select rows → bulk action bar appears at bottom
- Click "Delete N selected" → confirmation dialog with full count summary
- Always type-to-confirm for bulk delete (typo "delete X items")
- After: undoable for 10s if implementation allows; otherwise committed immediately

## Conflict resolution

When user edits a record that someone else has changed since they loaded it:

- On save, server returns 409 Conflict with current version
- UI shows: "Someone else updated this while you were editing. [See changes] [Save mine anyway] [Reload theirs]"
- "See changes" opens a side-by-side diff
- "Save mine anyway" force-overwrites (with another confirmation)
- "Reload theirs" discards local edits, shows fresh data

For single-user inventories (most cases), this is rare but the UI must handle it gracefully.

## Loading states (recap from `08-interaction-states.md`)

| Operation duration | UI behavior |
| --- | --- |
| <100ms | No state shown; result direct |
| 100–500ms | Subtle pulse on affected area |
| 500ms–3s | Skeleton + after 800ms a small spinner |
| 3–10s | Progress indicator (bar or count) |
| >10s | Progress + "Still working…" copy after 5s, cancel button |

## Empty states (recap from `11-page-layouts-and-flows.md`)

Use the EmptyState primitive. Three flavors per surface — first-time, filtered, error.

## Optimistic UI examples

| Action | Optimistic update |
| --- | --- |
| Toggle "draft" status on commodity | Pill changes immediately |
| Reorder file gallery | Files rearrange immediately |
| Add tag | Chip appears immediately |
| Remove tag | Chip disappears immediately |
| Mark as archived | Item fades + moves to archive section |
| Delete (low-risk) | Item disappears with undo toast |

## Pessimistic UI (no optimistic)

| Action | Reason |
| --- | --- |
| Save form fields | Validation might fail server-side; user expects "Saved" confirmation |
| Upload file | Long-running, progress matters |
| Run backup | Long-running, shows discrete progress |
| Delete with type-to-confirm | High-risk; user expects explicit confirmation feedback |

## Form layout patterns

### Single-column (default)

Most forms. Max-width `--text-measure-form` (~540px). Fields stacked, vertical rhythm `--gap-stack-default` between them.

### Two-column (when fields are short and naturally paired)

For metadata forms (commodity: Type / Count / Original Price / Current Price / etc.), allow 2-column on desktop:

```
┌──────────────────────┐ ┌──────────────────────┐
│ Type                 │ │ Count                │
└──────────────────────┘ └──────────────────────┘
┌──────────────────────┐ ┌──────────────────────┐
│ Original Price       │ │ Current Price        │
└──────────────────────┘ └──────────────────────┘
```

Mobile: collapses to single column.

**Rule:** Each row's two fields should logically belong together (both about price, both about identity). Don't put unrelated fields side-by-side just because they fit.

### Section pattern

Group related fields into named sections (Basic, Where it lives, Documentation). Each section gets a heading and optional subtitle.

```
Basic
─────────────────────────────
[name field]
[description field]
[type field]    [count field]

Where it lives
─────────────────────────────
[place field]    [area field]
[notes field]

Documentation
─────────────────────────────
[receipts uploader]
[manuals uploader]
```

### Long form pagination

For multi-step flows (onboarding, complex creation): use a wizard pattern with progress bar at top, "Back" / "Next" buttons at bottom. Maximum 5 steps; if you need more, the form is too complex.

## Field requirements communication

| Type | Display |
| --- | --- |
| Required | `*` after label, in `--destructive` color |
| Optional | "(optional)" suffix in `--ink-muted` (only when ambiguity matters) |
| Conditional | Show/hide field based on other field's value, with smooth height transition |

Avoid the "required" label on every field — assume required is default and mark optional ones, *or* mark required ones consistently. Pick one convention per app. **Inventario picks: mark required, since most data is optional in this domain.**

## Smart defaults

- New thing: pre-fill location with last-used location
- New file upload: pre-select file type from drop-zone context (Images uploader → image type)
- Currency: default to user's primary currency
- Date: default to today for purchase date; default to today + 2 years for warranty (configurable)
- Counter (e.g., "count of items"): default to 1
- Tags: suggest existing tags as user types (autocomplete from corpus)

## Field interactions

### Conditional fields

When field A's value reveals field B (e.g., "Has warranty?" yes → show "Warranty until" field):
- Field B animates in (height 0 → auto, opacity 0 → 1, `--duration-base`)
- When hidden, field B is excluded from validation and form data

### Linked fields

When field A's value affects field B's options (e.g., Place selected → Area dropdown filters to that place's areas):
- Field B's value clears if previously-selected option no longer applies
- Field B placeholder updates to reflect the change

### Computed fields

Some fields are computed from others (e.g., "Per-unit price" = Total price / Count). Display as read-only, with a tooltip explaining the formula on hover.

## File upload UX

Per `10-file-and-media.md`. Recap:
- Drop zone with idle / drag-over / uploading / done / error states
- Per-file progress
- Multi-file parallel
- Cancel per file
- Validation upfront

## Bulk operations

When user selects multiple things:
- Bulk action bar appears at bottom: "5 selected · [Tag] [Move] [Export] [Delete] [Cancel]"
- Bar has shadow `lg`, sticky bottom, full width
- Click an action → applies to all selected
- Long-running bulk ops show inline progress

## Search behavior

Across all surfaces:
- Debounce 250ms
- Show recent searches as suggestions when search input focused with empty query
- Search results highlight matched terms (subtle bold, not shouty highlight)
- Keyboard `↓` from search input enters results list
- "No results" empty state with suggestion to try different terms

## Network resilience

- Request timeout: 30s default, 5min for upload/backup
- Retry with exponential backoff for transient errors (502, 503, network-down)
- Indicate network state in UI: small dot in user menu showing "Online" / "Offline" / "Reconnecting"
- Queue mutations while offline (where feasible) and replay on reconnect — v2 feature

## Print and export

When user prints (Cmd+P), apply print stylesheet (`18-print-and-export.md`). Inline print button on relevant detail pages.

## Drafts

For long-form entries (commodity with many fields), a "Save as draft" option is available alongside "Save". Drafts:
- Appear in `Things` filtered to "Drafts only"
- Have a Draft pill on cards
- Are excluded from dashboard counts unless user opts in
- Auto-promote to published when user clicks Save (without "as draft")

## What ships in sprint 0

1. Form validation pattern (onBlur format check, onSubmit comprehensive)
2. Unsaved-changes guard on form views
3. Toast undo for low-risk deletes
4. Optimistic toggle for status changes (draft, archive)
5. Smart defaults for new entity creation (last-used location, today's date, etc.)
6. EmptyState integration on all list views

Sprint 1+:
- Auto-save for inline detail edits
- Conflict resolution UI
- Bulk operations bar
- Drafts pattern
