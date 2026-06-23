# Form & Data UX

The behavior contract for every form and every data surface in the
app. Engineering rules in [../forms.md](../forms.md) and [../data.md](../data.md); this doc is
the **UX** decisions — when to validate, when to confirm, when to
optimistically update, when to surface progress.

## Validation timing

| Mode | When validation fires | When to use |
| --- | --- | --- |
| `onSubmit` (default) | After the user clicks Submit | Almost every form |
| `onChange` | On every keystroke | Forms where the user benefits from live feedback (password complexity, search-as-you-type) |
| `onBlur` | When the field loses focus | Specific fields with deferrable rules (email format) |

The default is `onSubmit`. Live validation is friction unless the
user has a reason to want it; "your password isn't long enough" on
keystroke 4 is patronizing.

For password fields specifically, the strength meter updates
on-keystroke but doesn't *block* submit until the user has stopped
typing — feedback without paternalism.

## Server errors

`parseServerError(err, fallback)` collapses every backend envelope
into a single string. See [../forms.md](../forms.md). UX-side:

- **Form-level** (auth, single-section edit): banner above the form
  with `<Alert variant="destructive">`. The banner clears when the
  user starts typing.
- **Field-level** (rare — server validates a specific field): show
  inline below the field, `text-destructive text-xs`. Mark the field
  `aria-invalid="true"`.
- **Network blip during background mutation** (auto-save, settings
  toggle): sonner toast with retry + rollback. Don't change the page
  layout for a transient blip.

## Confirmation

Confirm before:

- **Delete** anything the user can't undo within 30 seconds.
  Use `useConfirm()`.
- **Leaving an unsaved form**. Use the same dialog.
- **Bulk action ≥ 5 items**. The user might have miss-selected.

Don't confirm:

- A reversible toggle (e.g. "Mark as sold" — the user can switch back).
- A view change.
- A search.

## Optimistic updates

Use when the server's answer is predictable AND the user expects
instant feedback:

| Action | Optimistic? |
| --- | --- |
| Toggle a tag on/off | Yes |
| Mark warranty seen | Yes |
| Settings toggle | Yes |
| Logout | Yes |
| Add commodity | No — show pending state, replace on success |
| Delete commodity | Yes (with rollback on error) |
| Create / move location | No — these have validation rules the server owns |
| Bulk delete | Yes (5+ items: confirm first) |

The pattern is the `useLogout` reference in [data.md](data.md). Always:

- `cancelQueries` before writing optimistically.
- Snapshot previous data in `onMutate`.
- Restore in `onError`.
- `invalidateQueries` in `onSettled`.

## Draft persistence

For any form > 4 fields or > 2 minutes' commitment, persist the draft
to `localStorage` keyed `<feature>-draft:{slug}:{op}` (e.g.
`commodity-draft:household:create`):

- Hydrate on mount.
- Write through every change (debounced).
- Cancel clears.
- Save clears on success.

The Add Item wizard (`features/commodities/`) is the canonical
implementation. Adopt the helper rather than re-inventing.

## Multi-step (wizard) flow

Default to single-step for ≤ 4 fields. Wizards for everything else.
See [11-page-layouts-and-flows.md](11-page-layouts-and-flows.md) ("Multi-step wizard").

UX rules:

- Per-step validation before "Continue".
- "Back" preserves entered values (RHF's default).
- Step dots show progress + completion. Click a previous step to jump
  back; forward dots are disabled until validated.
- Cancel asks for confirmation if any step has been edited.

## Inline edit vs. separate form

| When | UX |
| --- | --- |
| Single field, low risk (rename, change tag) | Inline — click the value, becomes editable, blur to save |
| Multiple fields | Sheet or full page |
| Destructive consequence | Always a separate form, never inline |
| Sortable / reorderable list | Drag-and-drop with keyboard fallback |

Inline edit anatomy:

```tsx
{editing ? (
  <Input
    autoFocus
    value={value}
    onChange={(e) => setValue(e.target.value)}
    onBlur={save}
    onKeyDown={(e) => e.key === "Enter" && save()}
    className="h-8 -my-0.5"
  />
) : (
  <button
    onClick={() => setEditing(true)}
    className="-mx-1 px-1 py-0.5 text-left rounded hover:bg-muted/40"
  >
    {value || <span className="text-muted-foreground italic">{placeholder}</span>}
  </button>
)}
```

Escape cancels (revert + blur). Enter saves (blur fires save handler).

## Loading data

| Surface | Behavior |
| --- | --- |
| Page first load | Skeleton matching the page's shape. Never a centered spinner. |
| Page navigation between cached views | Show stale data, fetch in background, no skeleton |
| List paging | `keepPreviousData: false` by default — show skeleton for the next page (pagination is fast) |
| List sorting / filtering | Replace data with skeleton on chip click; React Query refetches with the new key |
| Mutation in flight | Spinner inside the button, button label preserved |
| Long-running server task (export, restore) | Server-side state polled or streamed; UI shows actual progress, not a spinner |

## Empty / not found / error

See [08-interaction-states.md](08-interaction-states.md) and [20-edge-cases.md](20-edge-cases.md) for the visual
specs. UX-side:

- **Empty** — give the user the next action (CTA inside the empty
  state).
- **Not found** — link back to where they probably came from.
- **Error** — give a Retry. The retry calls
  `queryClient.invalidateQueries(...)`.

## Search

- The search input lives **inside** the list page, not above the page
  title.
- Keyboard shortcut: `/` focuses search.
- Debounce 300ms before firing the query.
- Empty input + recent searches = recent items, not "Type to search".
- Empty results = empty-state pattern with the search term echoed back
  ("No items match \"foo\".").

## Sorting and filtering

- Sort and filter are query-string params (`?sort=-created_at&type=appliance`)
  so deep links round-trip.
- Active filter chips visible above the list. Click X on a chip to
  clear that filter; "Clear all" resets to default.
- Don't open a multi-select filter inside a side rail. Use a Popover.

## Bulk actions

When a list supports multi-select (commodities, files, areas, tags),
the toolbar:

- Slides in from the top of the list when the first row is selected.
- Says "N selected" first, then the actions.
- "Clear" / "Cancel" deselects all.
- Shifts the page header down — doesn't overlay it.
- Sticks to the top during scroll.

Available actions per surface:

| Surface | Bulk |
| --- | --- |
| Items | Delete, Move, Tag |
| Files | Delete, Move to commodity |
| Areas | Delete, Move to location |
| Tags | Delete |

5+ items → confirm before destruction.

## Drag-and-drop

Used for:

- Reordering areas inside a location.
- Reordering files inside a commodity.
- Drop-zone upload ([10-file-and-media.md](10-file-and-media.md)).

Always with a keyboard fallback: arrow keys to move within a focused
list, Space to grab/drop. Use Radix's `<DragHandle>` primitive when
copied in.

Multi-select drag (drag many items at once) is out of scope for now.

## Auto-save

For settings / draft persistence. UX-side:

- Save is debounced 500ms after last change.
- A subtle "Saved" indicator appears next to the field for 1.5s after
  save.
- Failure → toast with retry + rollback. The field's value reverts.

## Hard rules

1. **Default validation to `onSubmit`.** Live validation is opt-in.
2. **Confirm destructive.** Always.
3. **Server errors via `parseServerError`.** Don't render raw errors.
4. **Drafts persist** for forms > 4 fields.
5. **Skeletons over spinners** for in-flight queries.
6. **One filled-primary** per page surface.
7. **Optimistic updates roll back** on error.

## Anti-patterns

- A form that validates on every keystroke for no good reason.
- "Are you sure?" with no body text. Body text answers the implicit
  follow-up.
- A bulk-delete button without a confirmation.
- Auto-saving a settings field without showing the user it saved.
- A wizard that loses entered data when the user clicks Back.
- A search that fires on every keystroke without debounce.
- A filter that opens as a fullscreen Sheet on desktop. Use a Popover.

## Cross-refs

- Form engineering: [../forms.md](../forms.md).
- Data engineering (mutations, optimistic, keys): [../data.md](../data.md).
- Toasts and notifications: [16-notifications-and-trust.md](16-notifications-and-trust.md).
- States (loading, empty, error): [08-interaction-states.md](08-interaction-states.md).
- Wizard pattern: [11-page-layouts-and-flows.md](11-page-layouts-and-flows.md).
