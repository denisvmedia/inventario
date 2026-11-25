# Frontend design deviations

Tracks every place where Inventario's React frontend (`frontend/`) intentionally diverges from the canonical visual contract in [`design-mocks/`](../../design-mocks/).

`design-mocks/` is a read-only mirror of `github.com/denisvmedia/inventario-design` (see [AGENTS.md](../../AGENTS.md)). Default fidelity is 1:1; this file is the trail of every decision _not_ to be 1:1.

## When to add an entry

- Default is 1:1 fidelity. Any divergence is a deliberate decision.
- If the agent suggests a deviation, it must be **explicitly approved by the user** before merge.
- If the user requests a deviation, the agent must **explain the consequences** (visual drift, maintenance friction, future review cost) and confirm understanding before implementing.
- Backend/data realities that don't fit the mock count as deviations and must be logged.
- Pages or surfaces absent from `design-mocks/` should fall back to [`design-mocks/src/views/UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) and the gap should be logged here (`Why: not present in mock`).

A change that diverges without a corresponding entry here is **not finished**.

## Entry format

Append entries to the matching section. Use this template:

```markdown
### YYYY-MM-DD — Surface name

- **Issue/PR**: #NNNN / PR #NNNN
- **Mock**: <what the design mock shows — view file, key visual element>
- **Reality**: <what we ship, and why it differs visually or behaviourally>
- **Why**: <reason — backend constraint, missing data, UX decision, mock-omission, etc.>
- **Approved by**: user (explicit) | agent-suggested-then-user-confirmed
- **Reversion plan**: <how/when this might be reconciled, or "permanent">
```

Do not edit prior entries except to fix factual errors (typos, wrong issue number). When a deviation is reverted (the code is brought back in line with the mock), keep the entry but append a final line `- **Resolved**: YYYY-MM-DD, PR #NNNN — back to 1:1`.

## Sections

### Items / Commodities

#### 2026-11-25 — Server drafts in the Add item dialog: Save draft on every step, draft alerts, close-confirm copy

- **Issue/PR**: #2812 / #2861
- **Mock**: [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) has the **Save as draft** `Switch` row on the form steps (L554–L569); with it on, the last step's primary button reads "Save Draft" (L702–L711) and the earlier steps keep Continue. The mock has no notice about where a draft is saved, no conflict handling, and no list of files already saved with a draft.
- **Reality**: In create mode for a signed-in user (`serverDrafts`), turning the switch on adds a primary **Save draft** button on every form step and turns Continue into an outline button; it saves the form and the staged files as a server draft without checking the form, then closes. A draft opened with **Continue** skips the Fill with AI step, opens on Basics with the switch off, and hides the fields behind "Opening the draft…" until the draft has loaded. Below the steps, a destructive `Alert` "Draft not saved to the server" names the files that did not upload and has a **Retry** button; a destructive `Alert` "This draft changed elsewhere" has two outline buttons, **Load the newer version** and **Save as a new draft**. When the draft was finished elsewhere, **Save draft** or **Add item** shows a destructive `Alert` "This draft is already an item" with **Open item** and **Save as a new draft**; when it was discarded elsewhere, "This draft was discarded" with **Save as a new draft**. Under the switch, default `Alert`s say "Restored from this device" (input kept locally that the server lacks), "Draft not found" or "This draft is already an item". The Files step lists "Saved with this draft" rows with the pending-file row's anatomy, without the tags input. The close confirm says the draft is saved for any device, reads **Discard changes** when a draft is being continued, and for the anonymous landing-page form reads **Keep on this device** with copy about signing in.
- **Why**: The issue asks for quick capture without the full validation, explicit handling of a stale save from another tab or device, and copy that tells local recovery from a server save. The notices compose existing `Alert` and `Button` primitives from [`UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx). Save draft on every step (rather than on the last step only) and opening a continued draft on Basics with the switch off go beyond the issue text and are agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2812 by the 2026-11-24 goal to implement the open enhancements; Save draft on every step and the continued-draft opening agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock if `inventario-design` draws server drafts; otherwise permanent.

#### 2026-11-25 — Drafts card on the Items page

- **Issue/PR**: #2812 / #2861
- **Mock**: [`design-mocks/src/views/ItemsView.tsx`](../../design-mocks/src/views/ItemsView.tsx) has no list of drafts.
- **Reality**: `frontend/src/components/items/DraftsCard.tsx` renders a `Card` between the page header and the toolbar, only while the group has drafts. The title "Drafts" carries a secondary `Badge` with the count, and a `CardDescription` says drafts can be continued on any device. Each row shows the draft's name (an italic muted "Untitled draft" when it has none), "Saved {relative time}" and a paperclip with the file count, then an outline small **Continue** button and a ghost icon button (`Trash2`) that discards after the confirm dialog. The card lists five drafts; a ghost small "Show all N" in `CardAction` lists up to 100. Viewers see the rows without the buttons.
- **Why**: not present in mock. The issue asks that drafts can be found and resumed on another device, with the inbox of #2794 as the later home; the card follows the `Card` anatomy in [`UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx). The placement above the toolbar and the five-row cap are agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2812 by the 2026-11-24 goal to implement the open enhancements; placement and the row cap agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Move the list into the inbox when #2794 lands.

#### 2026-11-25 — Short name follows the name in the Add item dialog

- **Issue/PR**: #2812 / #2861
- **Mock**: [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) Basics step has an independent Short name input with a 20-character limit (L215–L216).
- **Reality**: In create mode the short name is filled from the name as the user types it, cut to whole words of up to 20 characters (counted as code points, so an emoji counts once), until the user types in the short name field or the AI scan supplies one. Edit mode leaves the short name alone. The field keeps its 40-character limit, the server's.
- **Why**: The issue asks for a short name derived from the title that keeps a manual choice. The 20-character cut follows the mock's limit and is agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2812 by the 2026-11-24 goal to implement the open enhancements; the 20-character cut agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Permanent.

#### 2026-11-25 — Batch intake on the Items page: drop target, Add from files, intake dialog, batches card

- **Issue/PR**: #2810 / PR (pending)
- **Mock**: [`design-mocks/src/views/ItemsView.tsx`](../../design-mocks/src/views/ItemsView.tsx) has one primary **Add Item** button in the header, no drop target and no list of scanned files. [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) scans photos for one item only.
- **Reality**: Files dragged from the desktop over the Items page show a fixed overlay (`bg-background/80`, a dashed `border-primary` card with a `FileUp` icon, "Drop files to add items"); dropping them opens `frontend/src/components/items/IntakeUploadDialog.tsx`. An outline **Add from files** button (`FileUp`) before **Add item** opens the same dialog with no files. Both show only to members who can edit items, outside a currency migration, when the server has an AI scan provider. The dialog (`sm:max-w-lg`) states the limits, lists each file with its size or the reason it will not be sent (type, size, batch count), has an outline small **Add files** button, a **All files show one item** `Switch` with a hint line, a destructive `Alert` when the scan queue has no room, and a destructive `Alert` when an upload or the queue request fails; the primary button reads "Scan N files", "Uploading N of M…" while it runs, then "Try again". `frontend/src/components/items/IntakeCard.tsx` renders a `Card` above the Drafts card while the group has batches, in the Drafts card's anatomy: a `ScanText` title "Added from files" with a count `Badge`, one row per batch with a state icon, "N files", a state `Badge` (Scanning / To review / Done), "Added {relative time} · summary", an outline small **Review** or **Open** button and, for a batch that is not scanning, a ghost `Trash2` button that removes it after the confirm dialog.
- **Why**: not present in mock. The issue asks for a drop target on the Items page, explicit limits and progress, and a summary such as "7 drafts created, 1 file unreadable". The **Add from files** button gives keyboard and touch users the same entry as the drop target, and the queued-batch navigation to the review page follows from the issue's progress requirement; both are agent-suggested. The card's placement above the Drafts card is agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2810 by the 2026-11-24 goal to implement the open enhancements; the Add from files button, the opening of the review after queuing and the card placement agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Move the batches into the inbox when #2794 lands; otherwise follow the mock if `inventario-design` draws batch intake.

#### 2026-11-25 — Batch review page (`/g/:slug/intake/:id`)

- **Issue/PR**: #2810 / PR (pending)
- **Mock**: No such page in [`design-mocks/src/views/`](../../design-mocks/src/views/). Fallback: [`design-mocks/src/views/UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) (Card, Checkbox, Badge, Alert, Button, Input).
- **Reality**: `frontend/src/pages/intake/IntakeSessionPage.tsx` is a narrow `Page` with a detail `PageHeader`: a back link to Items, "Review files" with the state `Badge`, "Added {date} · summary" as the subtitle, and for members who can edit an outline **Stop scanning** (while scanning), an outline **Retry unfinished** (when files failed or were stopped) and a ghost `Trash2` remove button. While files are scanned a default `Alert` with a spinning `Loader2` says the scan runs on the server. Each purchase is a `Card`: a 40px thumbnail or `FileText` tile, the file name (or "name and N more files"), a **One item** outline `Badge`, a status line (queued, waiting for the hourly scan limit, retried shortly, scanning, stopped, N products found, or why it failed in destructive text) and a ghost small **Rescan** in `CardAction` while none of its products has a draft. Each product is a row with a `Checkbox` (make a draft), the name (italic muted "Unnamed product" without one), brand, model and the line amount, a quantity `Input` (`h-8 w-20`), a ghost toggle "N fields used" that lists every field the scan read with a `Checkbox`, its value and the confidence, and an outcome `Badge` (Draft created / Left out / Draft failed) with an outline **Continue** button that opens the draft in the item form. A sticky footer with a top border says how many products become drafts and holds the primary **Create N drafts** (or **Leave all out**) button while something is undecided.
- **Why**: not present in mock. The issue asks for a durable review where the member keeps some detected products, accepts or rejects fields, and sees per-product and per-file outcomes on any device. The sticky footer and the per-field confidence are agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2810 by the 2026-11-24 goal to implement the open enhancements; the sticky footer and the confidence column agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock if `inventario-design` adds a batch review; permanent otherwise.

#### 2026-11-25 — Views control on the items list toolbar

- **Issue/PR**: #2793 / #2839
- **Mock**: [`design-mocks/src/views/ItemsView.tsx`](../../design-mocks/src/views/ItemsView.tsx) has no saved views: its toolbar ends with the grid/list toggle.
- **Reality**: `frontend/src/features/saved-views/SavedViewControl.tsx` sits in the toolbar before the grid/list buttons, an outline `Button size="sm"` with a `Bookmark` icon and **Views**, opening a `DropdownMenu` like the toolbar's filter menus and the menus catalogued in `UIShowcaseView`. It saves the current filter, sort and layout through `SaveViewDialog` (name, a 16-icon picker built on the Radix radio group, so it takes one tab stop and the arrow keys choose, Sidebar / Dashboard / Share checkboxes), lists the member's views and links to the manage page. While a view is open (`saved_view=<id>` in the page URL) the button turns `secondary` and shows the view's icon and name, a `bg-primary` dot when the list no longer matches the view, and a `text-status-expiring` warning icon when the view names a deleted area, tag or seller (sellers are checked only once the group's sellers have loaded); the menu then offers **Save changes**, **Revert to saved** and **Close view**, and a notice lists what no longer exists. A view whose stored query cannot run, or holds a parameter this client cannot read, is a disabled menu entry with a warning icon and is never opened, by the menu or by Revert; while the page URL names one, the menu explains it and offers only **Close view**, and the page shows a destructive `Alert` (title, the parameter, **Close view**) in place of the items, as it does for an invalid query. While the saved views fail to load and none are cached, a page URL that names a view shows a destructive `Alert` with **Try again** and **Close view** in place of the items. The page passes the control through one `savedViews` slot of its toolbar.
- **Why**: not present in mock. #2793 asks for a Save view control on the items list; the changed marker, revert and the deleted-reference notice follow from the issue's "opening, editing and returning preserves context" and "show what no longer matches" requirements.
- **Approved by**: user (explicit) — pre-approved for #2793 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` draws saved views.

#### 2026-11-25 — Saved views page (`/g/:slug/views`)

- **Issue/PR**: #2793 / #2839
- **Mock**: none; the mock has no saved views.
- **Reality**: `frontend/src/pages/saved-views/SavedViewsPage.tsx` is a `Page width="narrow"` with one `Card` of rows (the row pattern of the dashboard lists): icon tile, the view name as a link, a **Personal** / **Shared** badge, the item count, the conditions as outline badges (`features/saved-views/QueryConditions.tsx`, a condition naming something deleted in the `status-expiring` tone and saying so in its text: "Deleted area", "Deleted seller", or "(deleted)" after a tag or an item type the group no longer has; a seller reads "Seller <name>", an item type by the group's name for it), ghost icon toggles for Sidebar (`PanelLeft`) and Dashboard (`LayoutDashboard`) that set the member's own placement on every row, shared views of others included, and a `MoreHorizontal` menu with Edit, Move up, Move down and Delete for the views the member may change. Reached from the Views menu and the gear on the sidebar section; it is not in the sidebar's nav lists.
- **Why**: not present in mock. A member needs one place to rename, reorder, unpin and delete views, including views that are on neither the sidebar nor the dashboard; the issue does not name the surface, so the page is agent-suggested.
- **Approved by**: agent-suggested; listed in the PR for the owner (the feature itself is pre-approved for #2793 by the 2026-11-24 goal to implement the open enhancements).
- **Reversion plan**: Follow the mock once `inventario-design` draws view management, or fold the page into Settings if the owner prefers.

#### 2026-11-25 — Brand and model on the item: inputs, suggestions, cards, item page, search

- **Issue/PR**: #2804 / PR #2840
- **Mock**: [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) Basics step puts Brand and Model side by side under Short name (L1042–L1049). `BrandInput` (L916–L958) suggests up to six names from the fixed `ALL_BRANDS` list that start with the typed text, and `ModelInput` (L960–L1002) suggests models of the chosen brand from `BRAND_MODELS`. [`ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) shows `{brand} · {model}` as the sheet description (L658–L660) and plain Brand / Model inputs in `ItemEditForm` (L468–L473). [`ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) grid card: `{brand} · {model}` as `CardDescription`, then `area | value` in `CardContent` (L454–L466); list row subtitle `{brand} · {area}` (L507–L509); search placeholder "Search items, brands…" (L226).
- **Reality**: `frontend/src/components/items/BrandModelFields.tsx` renders the two inputs, with the mock's grid, under Short name in the step dialog and under Name in the edit form, so the edit form now carries Brand and Model as the mock does, next to the Short name and Quantity it already had. `BrandInput.tsx` keeps the mock's popover classes but takes its suggestions from `GET /commodities/brands`: the brands the group's items carry that contain the typed text anywhere, ignoring case, most used first, at most six; it is a `combobox` with a `listbox`, and the arrow keys and Enter pick a suggestion. The edit form gets the same suggestions. Model has no suggestions. Grid cards follow the mock (brand · model as the description, area in the content row, with the existing purchase-date chip beside it); the items table, the area page's grid and list, and the item page header show brand · model as the line under the name. An item with neither shows its short name in that place; the photo cards of #2797 show the brand · model line above the area only when the item has either, and the item page keeps the short name and the type line around it. The search match line (#2800) now sits under the brand · model line. The items search placeholder reads "Search by name, brand, model, serial or part number, tag, notes or asset ID…". The AI scan review lists Brand and Model rows after Short name.
- **Why**: The issue asks for the fields, for "brand · model" on cards, and for brand suggestions from the brands the group already uses (the mock's fixed list is placeholder data). Matching anywhere in the brand, the suggestions in the edit form, the short-name fallback and keeping the list row's subtitle as brand · model (the table has its own area column) are agent-suggested. Model suggestions are left out: the issue asks only for brand suggestions, and a per-brand model list would need a second endpoint.
- **Approved by**: user (explicit) — pre-approved for #2804 by the 2026-11-24 goal to implement the open enhancements; substring matching, edit-form suggestions, keyboard selection and the short-name fallback agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Add model suggestions if the mock's `ModelInput` is wanted with real data; otherwise permanent. The short-name fallback stays while brand and model are optional.

#### 2026-11-25 — Items bulk bar: Edit menu, select all matching, result dialog

- **Issue/PR**: #2798 / PR (pending)
- **Mock**: Not present in mock. [`design-mocks/src/components/ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) has no bulk selection at all; the shared `BulkActionBar` is itself a recorded deviation (the 2026-06-05 entry below), and this extends it. Fallback: [`design-mocks/src/views/UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) (DropdownMenu with a label and separator, Dialog, Select, link Button) and the mock's tag palette (`ItemEditForm` in [`design-mocks/src/components/ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx)).
- **Reality**: The Items bar moved out of the list page into `frontend/src/components/items/CommodityBulkActions.tsx` (Print labels with it) and is wider (`max-w-3xl` instead of the shared `max-w-xl`) to fit one more control; the shared bar's action row now wraps, so the four buttons fit a phone. Between Move and Print labels an outline **Edit** `DropdownMenu` trigger (chevron) offers **Tags…**, **Type…**, a separator, a **Set status** label and the statuses In Use, Lost, Disposed and Written Off. **Tags…** opens `BulkTagsDialog` (`frontend/src/components/tags/`, shared with the Files bar): two `TagPalette`s, **Add tags** (with the new-tag input) and **Remove tags** (existing tags only, through a new `allowCreate={false}` prop); picking a tag on one side takes it off the other. **Type…** opens `BulkTypeDialog` with the edit form's `ItemTypeField` (the group's item types and **New type**, #2802); a type the server refuses because another member deleted it is named on the field. Lost, Disposed and Written Off reuse `StatusTransitionDialog` with a new optional `title` / `description` pair that speaks of N items; In Use is a confirm dialog. Once the whole page is selected, a link button next to the count offers **Select all N matching items**; then the count reads "All N matching items selected", a **Clear selection** link button follows it, and Print labels is hidden. Above 500 matching items a muted line says to narrow the filters instead. When some items fail, `BulkResultDialog` shows the counts in its description, a bordered scrolling list of the failed items (the name links to the item) with the reason, and **Close** / **Retry failed items**.
- **Why**: not present in mock. The issue asks for bulk tags, type and status (with the date and note dialog of a single change), for "Select all N matching" as an action separate from "Select this page", and for a report of completed, skipped and failed items with a retry of the failed subset. The grouping under one Edit menu, the wider bar, the link to each failed item, keeping the failed items selected, and leaving Sold out of the bulk statuses go beyond the issue text and are agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2798 by the 2026-11-24 goal to implement the open enhancements; the Edit menu grouping, wider bar, failed-item links, kept selection and the Sold omission agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock once `inventario-design` adds a bulk-selection pattern to the items list; reconcile with the Files bar through the shared component.

#### 2026-11-25 — Seller on the item: picker with New seller, AI suggestion, items-list filter, Details row

- **Issue/PR**: #2804 / PR #2844
- **Mock**: No seller anywhere in the mock. The Purchase step of [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) has Purchase Date and Purchase Price only (L1158–L1180); `ItemEditForm` in [`ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) has Purchase Date (L516–L517) and the Details tab a `DetailRow` for it (L793). The [`ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) toolbar has Type and Status `DropdownMenu` filters (L234–L297).
- **Reality**: `frontend/src/components/sellers/SellerField.tsx` is a `Select` of the group's sellers under Purchase date, in the Purchase step and in the edit form's Purchase section, with **Clear seller** next to the label. The list ends with a `SelectSeparator` and **New seller** (a `Plus` icon), which opens `SellerFormDialog` over the form and selects the seller it created, the pattern of the Location picker (#2796). When the AI scan reads a seller the group does not have, a dashed box under the picker (`Sparkles` icon) names it with a **Create seller** button, or **Use {name}** when a seller with that name exists by then; nothing is created on its own. The scan review lists a Seller row after Purchase date. The items toolbar gets a **Seller** `DropdownMenu` of checkbox items, built like the Type filter, after Location; it shows once the group has a seller or the URL filters by one, and lists an unknown id as "Deleted seller" so it can be cleared. The Details tab shows a Seller row (`Store` icon) after Purchase date, linking to the items list filtered by the seller with inactive items shown. When the server refuses a new seller's name as taken, the dialog adds a small outline **Use {name}** button under the name error once the refreshed list has that seller, and picking it selects that seller. A picked seller that is no longer in the group's list (deleted by another member) shows a destructive line under the picker asking to pick another one.
- **Why**: not present in mock. #2804 asks for a seller per item, created on the fly from the form, a filter on the items list, and the AI scan filling the seller. The suggestion box instead of creating the scanned seller, the "Deleted seller" entry and the Details link are agent-suggested. The **Use {name}** offer and the deleted-seller line come from the #2844 review.
- **Approved by**: user (explicit) — pre-approved for #2804 by the 2026-11-24 goal to implement the open enhancements; suggestion box, "Deleted seller" entry and Details link agent-suggested, listed in the PR for the owner; the taken-name offer and the deleted-seller line asked for in the PR review.
- **Reversion plan**: Follow the mock once `inventario-design` adds a seller field.

#### 2026-11-25 — Item type picker with New type, the group's types in the Type filter

- **Issue/PR**: #2802 / PR #2863
- **Mock**: [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) Basics step has a **Category** `Select` over the fixed `CATEGORIES` list (L1051–L1057), and `ItemEditForm` in [`ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) the same `Select` (L475–L481). The [`ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) toolbar filter lists `CATEGORIES` as `DropdownMenuCheckboxItem`s with an icon each (L250–L262). There is no way to add a category.
- **Reality**: `frontend/src/components/itemTypes/ItemTypeField.tsx` keeps the mock's `Select`, filled from the group's item types (`GET /item-types`): each option shows the type's icon and its name, the built-in types by their translated default name until someone renames them. The list ends with a `SelectSeparator` and **New type** (a `Plus` icon), which opens `ItemTypeFormDialog` over the form and selects the type it created, the pattern of the Location and Seller pickers. When the name is taken, the dialog offers **Use {name}** to pick the existing type. A picked type that is missing from the cached list makes the field fetch the list once more, and a destructive line under the picker asks to pick another one only when a list fetched after that still lacks it (deleted by another member); a save the server refuses with 422 `item_type.unknown` shows "This type is not in the group." on the field and runs the same check. Without a group (the item form on the public landing page) the picker offers the six built-in types and no **New type**. The items toolbar's Type filter and the area page's filter list the group's types with their icons; a slug from the URL that the group does not have stays in the menu, by its slug, so it can be cleared. The AI scan review shows the type's name.
- **Why**: #2802 makes item types a per-group list with a name and an icon, created on the fly from the item form. The deleted-type line, the field error for an unknown type, the built-in types on the landing page and the unknown slug in the filter are agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2802 by the 2026-11-24 goal to implement the open enhancements; deleted-type line, unknown-type field error, landing-page fallback and unknown filter slug agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock once `inventario-design` adds a create affordance to its pickers and a managed category list.

#### 2026-11-25 — Photo cards as the default Items list layout, compact cards kept as a third layout

- **Issue/PR**: #2797 / PR (pending)
- **Mock**: [`design-mocks/src/components/ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) draws the Items grid as `grid gap-4 sm:grid-cols-2 lg:grid-cols-3` cards with a 36px (`size-9`) icon tile beside the name, and a grid / list toggle of two icon buttons. There is no photo-forward card.
- **Reality**: The toolbar toggle has three buttons: **Photo cards** (`LayoutGrid`, the default), **Compact cards** (`LayoutList`) and **List** (`List`). Compact cards and the list are the mock's grid and table unchanged. A photo card (`frontend/src/components/items/CommodityPhotoCard.tsx`) puts the cover on top at a fixed 4:3 ratio, under a muted box that shows the type icon when the item has no photo or the photo does not load, with the selection checkbox over its top-left corner; below it the name, the area, the status, warranty, loan and service pills, up to three tags, and the purchase date and value. The photo grid runs from two columns on a phone to six on a wide window, by container queries on the width the grid gets (`@xl:grid-cols-3 @3xl:grid-cols-4 @5xl:grid-cols-5 @6xl:grid-cols-6`), so the card stays about 180 to 280 px wide whether the sidebar is open or not. Photos load lazily with a `srcset` of the small (150 px), medium (300 px) and large (600 px) thumbnails. Settings → Appearance → Default view offers the three layouts. The layout buttons carry `aria-pressed` and a `title`.
- **Why**: not present in mock. The issue asks for a photo-forward card, 4 to 6 columns on wide screens, and the current card kept as a compact variant. Making the photo cards the default, the area line on the card, and container queries instead of window breakpoints go beyond the issue text and are agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2797 by the 2026-11-24 goal to implement the open enhancements; the default layout, the area line and the container queries agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock once `inventario-design` draws a photo card; the compact card stays the mock's card.

#### 2026-11-25 — Items per page choice (24 / 48 / 96) under the Items list

- **Issue/PR**: #2797 / PR (pending)
- **Mock**: [`design-mocks/src/views/ItemsView.tsx`](../../design-mocks/src/views/ItemsView.tsx) has no pager and no page size; the app's `Pagination` under the list follows [`UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx).
- **Reality**: When the list holds more than 24 items, a row under the list shows an **Items per page** `Select` (24, 48, 96; `size="sm"`, `w-20`) on the left and the pager on the right (stacked and centered on a phone). The size is the `per_page` URL parameter and is remembered on the device (`localStorage` key `commodities:pageSize`); changing it keeps the first item of the current page in view. An item opened from the list walks it with the same page breaks.
- **Why**: not present in mock. The issue asks for the 24 / 48 / 96 choice. Remembering it on the device and keeping the position go beyond the issue text and are agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2797 by the 2026-11-24 goal to implement the open enhancements; the remembered size and the kept position agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock if `inventario-design` adds a pager to the Items view.

#### 2026-11-25 — Cover photo beside the details on the item page

- **Issue/PR**: #2797 / PR (pending)
- **Mock**: [`design-mocks/src/components/ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) opens the header with a 48px (`size-12`) icon tile beside the title; photos appear only in the Files tab.
- **Reality**: On the item page, an item with a cover shows it as a 4:3 panel (`frontend/src/components/items/CommodityCoverPanel.tsx`) left of the title and the pills (192px wide from `sm`, 224px from `lg`), and above them at full width on a phone; the action row stays below. With more than one photo, a chip in the corner says how many (`Images` icon, "3 photos"). The panel is a button with a focus ring: a click, Enter or Space opens the existing fullscreen `ImageViewer` on the cover, with Previous / Next through up to 48 of the item's photos. An item without a photo keeps the 48px type tile. The detail sheet shows the cover at 96px, without the viewer, and the 48px tile without a photo. The Files tab is unchanged.
- **Why**: not present in mock. The issue asks for a cover image or a small gallery beside the details in addition to the Files tab. The photo count chip and the 96px cover in the sheet go beyond the issue text and are agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2797 by the 2026-11-24 goal to implement the open enhancements; the photo count chip and the sheet's 96px cover agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock once `inventario-design` draws a cover on the item page.

#### 2026-11-25 — New location / New area entries in the item's Location and Area pickers

- **Issue/PR**: #2796 / PR #2833
- **Mock**: The Location and Area `Select`s of [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) (L1074–L1091) and of `ItemEditForm` in [`design-mocks/src/components/ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) list only existing locations and areas; nothing in the mock creates one from an item. The create dialogs are the mock's [`LocationDialog.tsx`](../../design-mocks/src/components/LocationDialog.tsx) and [`AreaDialog.tsx`](../../design-mocks/src/components/AreaDialog.tsx).
- **Reality**: `frontend/src/components/locations/LocationAreaFields.tsx` is the one picker behind the item edit form, the Basics step of the edit-mode step dialog and the location quick edit on the Details tab. For admins and owners it ends the Location list with a `SelectSeparator` and a **New location** item, and the Area list with **New area**, each with a `Plus` icon muted like the icons of the Type select. Picking one opens the existing `LocationFormDialog` / `AreaFormDialog` over the form (stacked over the step dialog when that is open), selects what was created, and shows the Locations page's "Location created." / "Area created." toast. Users and viewers get the lists as before.
- **Why**: not present in mock. The issue asks for the entries in the pickers, for the existing create dialogs, and for the role gate. The toast and returning the focus to the select the dialog was opened from (a new optional `onCloseAutoFocus` prop on both dialogs) go beyond the issue text and are agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2796 by the 2026-11-24 goal to implement the open enhancements; toast and focus return agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock once `inventario-design` adds a create affordance to its pickers. Item types (#2802) and sellers (#2804) take the same pattern when their pickers land.

#### 2026-11-25 — Delete confirmations say items and files go to the trash; Undo on the delete toast; restored event in the history

- **Issue/PR**: #2799 / PR #2835
- **Mock**: [`design-mocks/src/components/ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx), [`FileDetail.tsx`](../../design-mocks/src/components/FileDetail.tsx), [`FilePreviewDialog.tsx`](../../design-mocks/src/components/FilePreviewDialog.tsx), [`FileBrowserView.tsx`](../../design-mocks/src/views/FileBrowserView.tsx) and [`LocationPickerView.tsx`](../../design-mocks/src/views/LocationPickerView.tsx) word every delete confirmation as "will be permanently deleted. This cannot be undone."
- **Reality**: The item, bulk item, file, bulk file and "Delete everything" (location and area) confirmations say the entries move to the trash and can be restored; the success toasts say "moved to the trash". The item and bulk item delete toasts carry an **Undo** action that restores the items (`features/trash/useUndoItemDelete.ts`). The item history timeline labels the `deleted` event "Moved this item to the trash" and renders the new `restored` event with a `RotateCcw` icon, noting when the item came back without a location.
- **Why**: Deleting no longer removes data at once, so the mock's wording would be false. The Undo action goes beyond the issue text and is agent-suggested: it answers the wrong-click case without a trip to the Trash page.
- **Approved by**: user (explicit) — pre-approved for #2799 by the 2026-11-24 goal to implement the open enhancements; the Undo action agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Permanent for the copy while deletes go to the trash. Drop the Undo action if the owner prefers the Trash page alone.

#### 2026-11-24 — Label print page for items and areas (`/g/:slug/labels`)

- **Issue/PR**: #2807 / PR (pending)
- **Mock**: Not present in mock. `design-mocks/` has no label or QR surface; the issue notes that it needs a mock in `inventario-design` or a recorded deviation. Fallback: [`design-mocks/src/views/UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) (Select, Button, Input, Alert, Skeleton) and the toolbar pattern of the insurance report ([`design-mocks/src/views/InsuranceReportView.tsx`](../../design-mocks/src/views/InsuranceReportView.tsx)).
- **Reality**: `frontend/src/pages/labels/LabelPrintPage.tsx` mounts inside the Shell like the other print pages. Toolbar: Back, a layout `Select` (single 63.5 × 38.1 mm label, A4 3 × 7 Avery L7160, Letter 3 × 10 Avery 5160), a **Skip positions** number input on sheet layouts, a "N labels · N pages" summary and Print. Below it, a real-size preview of every sheet (`features/labels/LabelSheets.tsx`): white paper with black ink in both themes, a dashed outline on each label position on screen only, and per label a QR code (`qrcode.react`), the asset ID in bold mono and the item name, or the area name and its location for an area label. Physical sizes reach Tailwind as CSS custom properties (`w-(--label-w)`) because they come from the chosen format at run time, and the paper colors are the CSS keywords `white` / `black` in the page's scoped style block, following the `index.css` print block, since no theme token is meant to stay white in dark mode. The page is registered in [`design/18-print-and-export.md`](design/18-print-and-export.md).
- **Why**: not present in mock. The label must print at real size on die-cut sheets, which no existing surface does. **Skip positions** (reusing a partly used sheet) goes beyond the issue text and is agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2807 by the 2026-11-24 goal to implement the open enhancements; Skip positions agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Reconcile when `inventario-design` adds a label view; the formats and `LabelSheets` stay, the toolbar follows the mock.

#### 2026-11-24 — Asset ID badge and Print label button on the item page; asset ID in search results

- **Issue/PR**: #2807 / PR (pending)
- **Mock**: [`design-mocks/src/components/ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) has no asset ID and no label action; its pill row holds only status and warranty, and its action row has Edit, Insurance Report and Delete.
- **Reality**: `frontend/src/pages/commodities/CommodityDetailPage.tsx` shows the asset ID (`A-0042`) as the first pill, an outline `Badge` in mono, in page and sheet mode. Page mode adds an icon-only **Print label** button (`QrCode` icon) next to the existing Print button, linking to the label page. The search page's commodity card subtitle reads `A-0042 · <short name>`, and the items search placeholder (list page and area items panel) reads "Search by name, short name, or asset ID…".
- **Why**: not present in mock. The issue asks for the asset ID on the item page and for search by it, and the label action belongs next to the existing Print action.
- **Approved by**: user (explicit) — pre-approved for #2807 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` places the asset ID and the label action.

#### 2026-11-24 — Items bulk bar: Print labels

- **Issue/PR**: #2807 / PR (pending)
- **Mock**: [`design-mocks/src/components/ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) has no bulk label action.
- **Reality**: `frontend/src/pages/commodities/CommoditiesListPage.tsx` adds an outline **Print labels** button (`QrCode` icon) between Move and Delete in the bulk-action bar. It links to the label page with the selected item ids.
- **Why**: not present in mock, and beyond the issue text, which names one label and a sheet but no entry point for an arbitrary set of items. Agent-suggested: selecting items is how the list already acts on several at once.
- **Approved by**: agent-suggested, listed in the PR for the owner to confirm.
- **Reversion plan**: Drop the button if the owner prefers the item and area entry points only.

#### 2026-06-04 — Commodity detail SHEET drops the in-sheet tab strip; full detail behind a "View full details" CTA

- **Issue/PR**: #1965 / PR #2011
- **Mock**: [`design-mocks/src/components/ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) (line ~768) renders the slide-over item detail with a tab strip inside the panel — the mock had **4** tabs (Details / Warranty / Files + one more) sitting comfortably in the overlay.
- **Reality**: In **sheet** mode only (`variant="sheet"` in `frontend/src/pages/commodities/CommodityDetailPage.tsx`), the tab strip and the six non-Details tab bodies are no longer rendered. The sheet now shows a slim quick-peek: the existing header + status pills + action row (Edit / Insurance / Delete), the `DetailsTab` fields rendered directly (no tab strip, and the `CommodityHistoryTimeline` is dropped to keep the peek short), and a prominent outline "View full details" button (`data-testid="commodity-detail-view-full"`, `Maximize2` icon). That button navigates via `useNavigate()` to the **bare** detail href `/g/<slug>/commodities/<id>` with **no** `state.background`, so the router (`app/router.tsx`) mounts the full-page `CommodityDetailPage` with all 7 tabs. **Page** mode (`variant="page"`) is untouched — all 7 tabs stay exactly as before.
- **Why**: The app grew from the mock's 4 tabs to **7** (Details / Warranty / Files / Lend / Service / Supplies / Maintenance). Seven triggers overflow the narrow `sm:max-w-lg` Sheet — the right-hand tabs clip past the panel edge and become unclickable (the #1965 symptom). Rather than horizontally scroll or shrink the strip, the owner's preferred direction is to make the sheet a fast quick-peek and route users to the full tabbed page for everything beyond the Details fields, mirroring `FileDetailSheet`'s "open in new tab" affordance.
- **Approved by**: user (explicit) — issue #1965, owner-preferred direction stated in the task.
- **Reversion plan**: Reconcile if the upstream mock adopts a 7-tab overlay (e.g. a scrollable/segmented tab strip that fits the narrow panel). At that point restore `{<Tabs … variant="sheet" />}` + the per-tab body chain in sheet mode and drop the "View full details" button + its i18n key.

#### 2026-05-16 — Commodity-type marker is a Lucide icon, not an emoji

- **Issue/PR**: #1392 / PR (pending)
- **Mock**: [`design-mocks/src/data/mock.ts`](../../design-mocks/src/data/mock.ts) defines `CATEGORY_ICONS` as an emoji map (`appliance: "🏠"`, `electronics: "💻"`, `tool: "🔧"`, `furniture: "🪑"`, `vehicle: "🚗"`, `other: "📦"`) and [`design-mocks/src/components/ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) / [`ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) render those emoji as the per-item visual marker on cards / rows / detail header.
- **Reality**: `frontend/src/features/commodities/constants.ts` now exports `COMMODITY_TYPE_ICONS` as a `Record<CommodityTypeValue, LucideIcon>` using `Refrigerator` (white_goods), `Laptop` (electronics), `Wrench` (equipment), `Armchair` (furniture), `Shirt` (clothes), `Package` (other / fallback). `CommodityThumb`, the commodities list filter, the Add/Edit item dialog's type Select, the Areas panel list+grid, and the Commodity print header all render the Lucide icon component instead of the emoji glyph.
- **Why**: User-approved in the issue body itself — emoji render inconsistently across OS font fallbacks (Windows vs. macOS vs. Linux), don't inherit theme `currentColor` for dark mode / muted-foreground, and rasterize differently at thumbnail vs. detail-hero sizes. Lucide is already a dependency, so this is zero new bundle weight; it also unifies the commodity surface with the rest of the app, which is Lucide-only per [`devdocs/frontend/icons.md`](icons.md) and [`devdocs/frontend/imports-and-bans.md`](imports-and-bans.md). The backend enum (`white_goods/electronics/equipment/furniture/clothes/other`) stays as-is — this is a pure presentation change.
- **Approved by**: user (explicit) — issue #1392 spec calls out "Use **Phosphor or Lucide icons**, not emoji" with the same rationale, and selects Lucide for v1.
- **Reversion plan**: Permanent for the commodity-type marker. Group / location / area icons are still emoji-based via the `IconPicker` (`features/group/icons.ts`) — that's a per-instance user choice, not a type-derived enum, and stays out of scope.

#### 2026-05-10 — Add Item dialog: Tags surface as a tinted CTA card with empty-state suggestion chips

- **Issue/PR**: #1544 / PR #1621
- **Mock**: [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) L1281-L1306 renders the Extras-step Tags as a flat `<Input>` with a "+ Add" button on the right of the label and the picked tags as `<Badge>`s below. No empty-state CTA, no leading icon, no surrounding card.
- **Reality**: `frontend/src/components/items/CommodityFormDialog.tsx` wraps the Extras-step Tags input in a tinted `bg-muted/20 rounded-xl border` card with a small leading `<Tag>` icon-tile. When no tags are selected yet, a row of up to 5 ghost-styled "+ {slug}" suggestion chips renders below — pulled from `useTagAutocomplete("")` (top-by-usage). One tap drops the slug into the form value; the chip row hides as soon as `selected.length > 0`, after which the standard popover-on-focus autocomplete (TagsInput's existing path) takes over. Files-step per-file tags keep the flat / compact look (no card, no chips).
- **Why**: User-driven UX request — flat input read as just-another-field rather than a primary affordance for tagging. Suggestion chips give a one-tap-to-act CTA that's especially useful on mobile and during onboarding (no typing required to feel productive). Bounding the prominence to the empty state keeps the field calm once it's serving its normal function.
- **Approved by**: user (explicit) — "tags не являются CTA. А должны … 1+2".
- **Reversion plan**: Keep until upstream design adopts a similar pattern, or until first-class Tags entity (#1400) reframes how the empty state should look. The component (`TagsSuggestionChips`) is local to CommodityFormDialog.tsx; revert is one block-removal + drop the tinted-card wrapper to fall back to the flat input.
- **Update 2026-05-15 (#1628)**: chip candidates are now sourced from `useTagAutocomplete("", 8, { scope: "commodity" })` — the empty-state CTA used to surface file-only tags (e.g. `invoice`) because the autocomplete pool was merged. After #1628 the pool is scoped to commodity-only usage, so the chips reflect "tags I use on items" and never surface file-only labels. No behaviour change when no scoped data exists yet — the chip row stays hidden via the same `candidates.length === 0` guard.

#### 2026-05-10 — Add Item dialog: per-file & per-item tag input is focus-triggered autocomplete (not datalist)

- **Issue/PR**: #1544 / PR #1621
- **Mock**: [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) — Extras step's tags are a chip input with no live suggestions; the file step in the mock has no tag entry on the row at all.
- **Reality**: `frontend/src/components/files/TagsInput.tsx` ships an `autocomplete` prop. When set, the input uses a `<AutocompleteSink>` sub-component to call `useTagAutocomplete(draft)` (BE endpoint `/g/:slug/tags/autocomplete?q=`) and renders the results in a focus-triggered absolutely-positioned `<ul>` dropdown below the input. Empty draft surfaces the BE's usage-ranked top tags so the user sees options on first focus. Outside-click / Escape / Tab close the dropdown. Used in two places: per-file row inside the Files step (`compact` mode) and the per-item Tags field on the Extras step (default size). Helper copy is shared via `commodities:fields.tagsHelp`.
- **Why**: User-driven UX request — "А мы можем ещё давать дропдаун для выбора из существующих?". The BE-side autocomplete API was already in place from #1400's groundwork; surfacing it here meets the user's expectation and keeps free-form entry as the fallback. Picking via mouseDown + preventDefault (not onClick) keeps focus on the input so the user can chain multiple picks without re-clicking.
- **Approved by**: user (explicit) — direct request and refinements ("Когда на экране появляются новые элементы — это должно быть как-то плавно"; "После первого тага надо убрать CTA текст"; "Поле под Первый урл должно быть сразу показано").
- **Reversion plan**: Permanent. If the BE autocomplete API changes shape, only `useTagAutocomplete` needs updating; the dropdown UI is component-local.
- **Update 2026-05-15 (#1628)**: `TagsInput` now accepts a `scope?: "commodity" | "file"` prop that flows through to the autocomplete hook + the GET `/g/:slug/tags/autocomplete?scope=` query. The two consumers pass it explicitly: the per-item Extras-step input uses `scope="commodity"`, the per-file row in the Files step uses `scope="file"`. Strict scoping (BE drops tags with zero usage in the requested bucket) means the dropdown on the commodity input no longer surfaces a file-only `invoice` tag (and vice versa) — the previous merged-pool behaviour was the known limitation called out in #1628.

#### 2026-05-10 — Add Item dialog: Product URLs without per-row Label sub-input

- **Issue/PR**: #1544 / PR #1621
- **Mock**: [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) L1309-L1339 renders each URL row as **two** inputs side-by-side: a fixed-width "Label" (`w-28`) and a flex-1 `https://…` URL input, plus an `X` to remove. The mock's local state is `{ id, label, url }[]`.
- **Reality**: `UrlList` inside `CommodityFormDialog.tsx` renders one full-width URL input per row + the `X`. No Label sub-input. Empty list shows a single un-removable phantom row; clicking "+ Add" promotes the phantom and appends a second row, after which both have `X` until a remove brings the list back to one.
- **Why**: BE-blocked. `go/models/url.go` has `type URL net/url.URL` — only the URL string persists. To match the mock we'd need a BE schema change (`URLEntry { Label, URL }`), a JSONB migration, regenerated typegen, and an FE schema update. Out of scope for #1544; tracked as a follow-up. Phantom-row UX (always one input visible) was added so the user never has to click "+ Add" to find an empty input.
- **Approved by**: user (explicit) — labels acknowledged as BE-bound ("Единственное, что у урлов продуктов нет лейблов") and confirmed deferred. Phantom + reveal-vs-add UX explicitly walked through and signed off.
- **Reversion plan**: Resolve when a BE issue lands per-URL Label support — restore `Label` (`w-28`) as the second input on each row, change schema to `urls: z.array(z.object({ label, url }))`, regenerate types.

#### 2026-05-10 — Add Item dialog: Product URLs on Basics, reveal toggles for Extra serials / Part numbers on Extras

- **Issue/PR**: #1544 / PR #1621
- **Mock**: In [`AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) the Basics step holds Serial Number followed by chevron-down toggles for "This item has multiple serial numbers" and "Add part numbers" (L1103-L1139). Product URLs and Supply Links live on the Extras step (L1309-L1371).
- **Reality**: Our 5-step layout puts Serial Number on the Purchase step (price + serial), so the reveal toggles for Extra serials + Part numbers ride on the Extras step instead. Product URLs were moved onto the Basics step right after Short name — per direct user direction that "это важно" / users should see / fill URLs without paging deep into the wizard. Both reveal toggles use the mock's chevron-down + muted-foreground style verbatim.
- **Why**: User-driven priority shift. Product URLs on Basics surface the most-shared piece of identity (manufacturer / store / docs link) up-front. The serial / part-number reveals match the mock's affordance shape but live where the dependent fields naturally fall in our step ordering — same UX treatment, different parent step.
- **Approved by**: user (explicit) — "Вообще, product urls должно быть на первом шаге сразу после short name, потому что это важно."
- **Reversion plan**: If we ever realign step ordering with the mock (Serial on Basics), the reveal toggles relocate alongside it; URLs would stay on the first user-visible step regardless.

#### 2026-05-10 — Add Item dialog Files step: single dropzone + auto-category + per-file tags

- **Issue/PR**: #1544 / PR #1621
- **Mock**: [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) L1378-L1444 renders three categorized dropzone buckets (Photos / Receipts & Invoices / Documents) with bucket-specific accept-globs, hint copy, and accent tints. The user picks a category up-front and drops the file into the matching bucket; files inherit the bucket's `category` server-side.
- **Reality**: `frontend/src/components/items/CommodityFormDialog.tsx`'s `FilesStep` renders one universal dropzone above a flat list. Each picked file gets a row with the file name, size, a remove button, a category badge inferred from MIME (`categoryFromMime` → `images` / `documents` / `other`), and an inline `ChipInput` for free-form tags. At submit time, `uploadPendingFiles` POSTs each file, then PUTs the derived category + tags via `updateFile`.
- **Why**: User-requested deliberate deviation. Up-front bucket-picking forces the user to classify before they've seen what they're attaching — most uploads are obvious from extension, and a flat list lets them stage tags right there without re-opening the file detail later. The mock's three-bucket layout also collides with our actual `FileCategory` enum on the BE (`images` / `invoices` / `documents` / `other`), where most "uncategorisable" files (e.g. text notes, archives) had no home; the MIME-derived classifier is the same one already used by the file detail page so the two surfaces stay consistent. Per-file tags are surfaced inline because there is no other write-tags surface during commodity creation today.
- **Approved by**: user (explicit) — direct request: "Давай переделаем форму загрузки в диалоге (сознательное отступление от мока)."
- **Reversion plan**: Permanent unless the upstream mock unifies the bucket layout. If the BE later collapses `invoices` into `documents`, the only required change is removing the badge's "documents" branch — all other code paths stay valid.

#### 2026-05-09 — Add Item dialog: inert AI step + tracker note

- **Issue/PR**: #1544 / PR #1621 — full implementation tracked in [#1540](https://github.com/denisvmedia/inventario/issues/1540).
- **Mock**: [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) L274-L856. Step `-1` (AI) has three live phases — `offer` (two photo-type cards + dropzone with file picker), `scanning` (animated progress), `review` (extracted fields preview) — and a footer with `Fill manually` / `Scan photos` actions feeding into a real scanner.
- **Reality**: `frontend/src/components/items/CommodityFormDialog.tsx` ports the `offer`-phase markup verbatim (same two cards, same `bg-amber-500/10` Sparkles tile, same dropzone copy + hint), but renders it inert — no file input, no `cursor-pointer`, no scanner wiring. A single `text-xs text-muted-foreground` line under the dropzone hint tags it "AI photo-scan is on the roadmap — issue #1540". The AI-step footer is its own thing (not the standard `Back` / `Next` wizard footer): `Cancel` (ghost, mr-auto) + `Fill manually` (outline, jumps to Basics) + `Scan photos` (primary, Sparkles icon, disabled until #1540 lands the scanner). Hidden when not on the AI step. The `scanning` and `review` phases are not implemented.
- **Why**: The full AI vision service + scanning state machine + review phase land in #1540. Surfacing the offer-phase visual now (vs. waiting on the entire backend) gives users an honest preview of where the affordance is going while keeping the wizard usable today. The single inline tracker line is the minimum disclosure needed; styling it as a banner / Alert would have introduced its own raw-color palette and competed with the mock's clean offer-phase composition.
- **Approved by**: user (explicit) — direct request to add a placeholder step pointing to the tracker issue.
- **Reversion plan**: Resolve when the scanner backend + scanning/review phases land. The card + dropzone markup stays; the disclosure line gets dropped, the file picker + Sparkles click handler get wired to the real scan endpoint. Tracker reference updated 2026-05-16: see the entry below — #1540 closed-as-deferred, AI vision BE work tracked in [#1720](https://github.com/denisvmedia/inventario/issues/1720).
- **Resolved**: 2026-05-23, PR #1835 — back to 1:1; AI photo-scan now wired through the new backend service (issue #1720). `AiScanStep` ports the full four-phase state machine (`offer` / `scanning` / `review` / `error`) from the mock; the inert tracker line and `scanDisabledTitle` / `comingSoon` i18n keys are removed.

#### 2026-05-16 — Add Item dialog: AI photo-scan step formally deferred; #1540 closed, #1527 closed

- **Issue/PR**: #1540 / PR #1718 (typed `ServerErrorBanner` work; the AI photo-scan checklist item is the only outstanding piece left after this PR). Successor BE tracker: [#1720](https://github.com/denisvmedia/inventario/issues/1720).
- **Mock**: Same as the 2026-05-09 entry above — `design-mocks/src/components/AddItemDialog.tsx` step `-1` AI phase (offer → scanning → review).
- **Reality**: After PR #1718 ships the typed server-error banner and confirms the CurrencyCombobox swap, two of the three items on #1540's checklist are complete. The remaining item — the live AI photo-scan flow — has no shipping path until a BE vision/AI service exists. Rather than keep the umbrella audit issue (#1527) open indefinitely on a multi-week BE dependency, the audit is being closed-as-completed today: the AI work moves to a dedicated BE tracker (#1720) with the same offer-phase stub continuing to disclose the deferral in-product.
- **Why**: #1527's sub-issue list is now 14/14 resolved (12 shipped, 2 closed-with-deferral logged in this file — the Data & Storage backup/sync deferral on 2026-05-16 12:18Z and this AI photo-scan deferral). Keeping #1527 open until the vision BE lands would conflate "design audit done" with "every audited-but-BE-blocked feature shipped" — those are separate ownership cycles. The dedicated BE tracker (#1720) carries the multi-week scoping work; the in-product disclosure line in `CommodityFormDialog.tsx`'s `AiStep` already points users at the right place.
- **Approved by**: user (explicit) — selected "Deferral в design-deviations.md, закрыть оба (Recommended)" when offered the closure path for #1540 + #1527.
- **Reversion plan**: When #1720 lands the BE vision service, append the `- **Resolved**: YYYY-MM-DD, PR #NNNN — back to 1:1` line to the 2026-05-09 entry above (the FE wiring is the actual deviation; this entry is the closure note for the audit umbrella).

#### 2026-05-08 — Commodity detail "Originally purchased for {price}" line

- **Issue/PR**: #1553 / PR #1604
- **Mock**: [`design-mocks/src/components/ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) shows OriginalPrice / ConvertedOriginalPrice / CurrentPrice as three flat rows. No "originally purchased" subline anywhere.
- **Reality**: When `acquisition_price` AND `acquisition_currency` are both set on a commodity (i.e. the BE froze the pre-migration purchase amount per epic #202 §2 Case A), the OriginalPrice row gains a subdued `text-xs text-muted-foreground` second line: "Originally purchased for {formatCurrency(price, acquisition_currency)}". When either field is null, no extra line renders, so unmigrated groups look identical to the mock.
- **Why**: The currency-migration feature did not exist when the mock was authored. The data point is required by issue #1553 §"Commodity detail / edit": users must see the original purchase amount in the original currency after a migration. Inlining as a subline keeps the existing 2-col grid intact rather than introducing a fourth price row.
- **Approved by**: user (explicit) — issue spec carries the exact copy.
- **Reversion plan**: Permanent until/unless the upstream mock adopts a richer price block. Reconcile if the design team adds an "acquisition history" pattern.

#### 2026-05-09 — Terminal-status info card without date / note / sale_price metadata

- **Issue/PR**: #1530 (item 1) / PR #1610 — follow-up tracked in [#1611](https://github.com/denisvmedia/inventario/issues/1611).
- **Mock**: [`ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) lines 736–762 render a tinted info card carrying the terminal status name **plus** the `statusDate`, `statusNote`, and (for `sold`) `salePrice` captured during the transition, then a "Revert to In Use" affordance. The same flow's `StatusTransitionDialog` (lines 113–185) collects those fields in the first place.
- **Reality**: The card surfaces only the status name + a `TriangleAlert` icon + the "Revert to In Use" ghost button. No metadata rows. Forward transitions remain a simple `useConfirm` instead of the mock's metadata-capture dialog.
- **Why**: BE-driven. `models.Commodity` carries no `status_date` / `status_note` / `sale_price` columns; the Ptah migrations would need to land on the BE before a richer FE can persist the user's input. Building the dialog FE-only would silently drop the captured metadata, which is worse UX than the current confirm flow. Issue #1611 carries the full BE + FE plan and gets the deviation "Resolved: ..." line on merge.
- **Approved by**: agent-suggested-then-user-confirmed — scoped FE-only by the existing `CommodityDetailPage.tsx` BE-comment ("Adding the metadata is a follow-up that needs BE work first").
- **Reversion plan**: Resolve when [#1611](https://github.com/denisvmedia/inventario/issues/1611) lands the BE schema columns + FE `StatusTransitionDialog` — the metadata block then surfaces on this card.
- **Resolved**: 2026-05-17, PR #1727 — back to 1:1.

#### 2026-05-10 — Commodity grid card purchase-date chip

- **Issue/PR**: #1547 / PR #1631
- **Mock**: [`design-mocks/src/components/ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) grid-card top-right cluster (lines ~438–450) groups item-level chips — Draft / status / `WarrantyBadge`. `CardContent` (lines ~459–475) is strictly `area | currentValue` + tags. No purchase-date anywhere.
- **Reality**: When `commodity.purchase_date` is present, the top-right chip cluster gains an extra neutral outline badge `📅 9/12/24` (lucide `Calendar` at `size-2.5` + locale-short date) at the end of the cluster. The visible chip carries only the date — full copy `Purchased {{date}}` (i18n key `commodities:card.purchasedOn`) is exposed via `title` / `aria-label`. `CardContent` is left at strict mock fidelity (`short_name | price`). The chip is gated purely on `!!row.purchase_date` — so it's hidden whenever the field is unset (the typical case for drafts that never filled it in, and for pre-#1367 entries that predate the column), and it still renders on drafts that *did* record a purchase date, matching the rest of the cluster (Status / Lent / In Service / WarrantyBadge don't gate on `row.draft` either, and the card's `opacity-70 border-dashed` already de-emphasises drafts).
- **Why**: Not present in mock. Issue #1547 (spun off from the closed #189) asks for a compact surfacing of `purchase_date` on the card, and explicitly says "Skip the line entirely when `purchase_date` is empty (drafts / pre-#1367 entries)" — so the gate is the value, not the draft flag. Temporal metadata is the same conceptual layer as `WarrantyBadge`, so the mock-spirited home is the top-right chip cluster (not a new line in `CardContent`, which crowds the price row and truncates at `lg:grid-cols-3`). Neutral `border-border text-muted-foreground` styling keeps it visually subordinate to status chips that carry semantic color. Tooltip + `aria-label` preserve the "Purchased {date}" copy without the chip needing a text prefix. Locale-aware via the existing `formatDate` helper, no new "date format" settings plumbing.
- **Approved by**: user (explicit) — issue #1547 asks for a compact surfacing of `purchase_date` on the grid card ("below `area` or alongside `current_price`"), names the i18n key shape, and names the `formatDate` helper. The chip-in-top-right-cluster placement is an agent-chosen interpretation of "compact" that the user confirmed visually after two earlier iterations (standalone line, inline dot-separator) were rejected.
- **Reversion plan**: Permanent until the upstream mock adopts the same line; reconcile when it does.

#### 2026-06-05 — Items bulk-action bar adopts the Files fixed-bottom-overlay shape (shared `BulkActionBar`)

- **Issue/PR**: PR #2016
- **Mock**: [`design-mocks/src/components/ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) has no bulk-select affordance, and the design language doc ([`design/15-form-and-data-ux.md`](design/15-form-and-data-ux.md)) describes the multi-select bar as an _in-flow_ element that "shifts the page header down — doesn't overlay it".
- **Reality**: The Items list bulk bar (`data-testid="commodities-bulk-bar"`) moved from an in-flow `bg-primary/5` block (which reflowed the list on first selection) to the same `fixed bottom-6 left-1/2 -translate-x-1/2 z-40` `bg-popover` overlay the Files page already uses. Both pages now render the one shared `BulkActionBar` component (`frontend/src/components/ui/bulk-action-bar.tsx`): identical floating shell, slide-in animation, select-all checkbox + "N selected" cluster, and a destructive Delete. The move affordance stays page-specific (Items opens an area-picker dialog; Files uses an inline category `<select>`) because the destination sets differ. The Items bar dropped its separate "Cancel" button (the select-all checkbox now toggles the whole page off, matching Files).
- **Why**: User-requested unification — the two surfaces shipped visibly different bulk bars (inline tinted block vs. floating pill) and the floating overlay is the already-approved canonical shape (see the Files entry of 2026-05-16). Extracting one component removes the drift risk and supersedes the in-flow description in `15-form-and-data-ux.md` for the overlay treatment.
- **Approved by**: user (explicit) — asked to make the Items selection panel match the Files one.
- **Reversion plan**: Permanent unless the upstream mock adopts a richer mass-action pattern; reconcile both surfaces together via the shared component if it does.

#### 2026-11-24 — In-place item edit form: real fields, sticky actions, All fields

- **Issue/PR**: #2795 / PR #2825
- **Mock**: [`design-mocks/src/components/ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) `ItemEditForm` replaces the detail panel's content with a sticky "Editing" header (back arrow, item name) and five sections: Identity (Item Name, Brand, Model, Category, Serial Number), Location (Location, Area), Purchase (Purchase Date, Purchase Price, Current Value), Warranty (Expiry Date, Warranty Notes), Notes & Tags (Notes, a tag palette of toggle buttons with removable pills under it). Cancel and Save Changes sit after a Separator at the end of the form.
- **Reality**: `frontend/src/components/items/CommodityEditForm.tsx` keeps the layout, the section order, the element ids and the palette classes, and maps the mock's fields onto the commodity model: Brand and Model become Short name and Quantity (the model has neither brand nor model), Category is the item Type, Purchase Price gets the compact `CurrencyCombobox` beside it, and a foreign purchase currency adds the converted-price field the server requires. A bundle (quantity above 1) disables empty warranty fields, as the dialog does. The form is URL-driven (`/commodities/:id/edit`) in the page and in the sheet; on the full page it is a centered `max-w-2xl` column, and in the sheet its header keeps `pr-8` clear of the sheet's close button. The Separator and the Cancel / Save Changes row stick to the bottom of the view. Agent-suggested additions: an **All fields** ghost button in the header that opens the step-by-step dialog with the values typed so far (links, extra serial numbers, part numbers, status and draft are not on the mock's form); an alert with the same button when the server rejects a field the form does not show; a **Clear location** text button over the Location select; and a new-tag input with an Add button under the palette (`frontend/src/components/tags/TagPalette.tsx`), since the palette alone can only toggle tags the group already has.
- **Why**: The issue asks for the mock's form 1:1; the field mapping is a backend data reality. The issue also asks for Save and Cancel to stay visible on a long form, hence the sticky row. The agent-suggested parts keep every field of the record reachable from the form and let a user unassign an item or add a first tag without leaving it.
- **Approved by**: user (explicit) — pre-approved for #2795 by the 2026-11-24 goal to implement the open enhancements; All fields, the hidden-field alert, Clear location and the new-tag input are agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock if `inventario-design` adds these fields or controls; the field mapping is permanent while the model has no brand or model.
- **Resolved**: 2026-11-25, PR #2840 — in part: Brand and Model are on the form, next to Short name and Quantity (#2804).

#### 2026-11-24 — Item page: Previous / Next and Save and next over the originating list

- **Issue/PR**: #2795 / PR #2825
- **Mock**: Not present in mock. [`ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) shows one item at a time with no way to move to the next one from the panel or from `ItemEditForm`. Fallback: [`design-mocks/src/views/UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) outline icon buttons.
- **Reality**: An item opened from the Items list carries the list in `?from=` (`frontend/src/lib/list-context.ts`, `frontend/src/features/commodities/listContext.ts`). The shared `frontend/src/components/ListPager.tsx` (outline `icon-sm` chevrons around an "N of M" position, also used by the file edit page) appears next to Back to items on the full page, in a strip above the header in the sheet (`pr-8`, clear of the close button), and at the right end of the edit form's header. The edit form's footer gains **Save and next** as the primary button, with Save Changes turned outline. Back to items returns to the list on the page holding the item, and an item that has left the list's results shows a one-line note. The walk is `frontend/src/hooks/useListNavigation.ts` from #2814.
- **Why**: not present in mock. The issue asks for previous / next and "Save and next" following the originating list's filters and order.
- **Approved by**: user (explicit) — pre-approved for #2795 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` places list navigation on the item surfaces.

#### 2026-11-24 — Quick edits on the item Details tab

- **Issue/PR**: #2795 / PR #2825
- **Mock**: Not present in mock. The Details rows of [`ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) are read-only; the only way to change a field is the Edit button. Tags and notes rows are left out when empty. Fallback: [`design-mocks/src/views/UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) ghost `icon-xs` buttons and small Save / Cancel buttons.
- **Reality**: `frontend/src/pages/commodities/CommodityDetailPage.tsx` puts a ghost `icon-xs` pencil next to the Location (Area on the full page), Tags and Notes labels, in the page and in the sheet. A pencil opens an editor in place of the value: the Location / Area selects from the edit form, the tag palette, or a textarea, with Save (enabled once the value changes) and Cancel. Save writes only that field. The Tags and Notes rows are always shown, with a dash when empty, so their pencils are there to click. The "Place in location" banner opens the location editor instead of the dialog. The pencils are hidden while a currency migration locks the group.
- **Why**: not present in mock. The issue asks for quick edits of tags, location / area and notes on the item page.
- **Approved by**: user (explicit) — pre-approved for #2795 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` adds inline editing to the item details.

#### 2026-11-24 — Item edit dialog: every step open, Save on every step

- **Issue/PR**: #2795 / PR #2825
- **Mock**: The mock edits an item in place (`ItemEditForm`); its step-by-step dialog ([`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx)) is for adding an item, with Continue on every step and the save button on the last one only.
- **Reality**: `frontend/src/components/items/CommodityFormDialog.tsx` in edit mode (opened from the edit form's **All fields**) marks every step visited on open, so any stepper segment can be clicked straight away, turns Continue into an outline button and shows **Save changes** on every step. A failed save opens the first step with an invalid field. The stepper tooltips no longer catch the pointer, so a quick click on a thin segment is not lost while the tooltip slides in.
- **Why**: The issue asks for every step to be reachable in edit mode and for Save on every step.
- **Approved by**: user (explicit) — pre-approved for #2795 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Permanent while the dialog stays the full editor behind All fields.

#### 2026-11-25 — Search match line on item cards, list rows and search results

- **Issue/PR**: #2800 / PR #2826
- **Mock**: [`design-mocks/src/components/ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) grid card header holds the title and the area line only; the table name cell holds name and short name. The search page is not in the mock (its result rows follow the [`CommandPalette`](../../design-mocks/src/components/CommandPalette.tsx) title-and-subtitle row). None of them says why an item matched a search.
- **Reality**: With a search active, `frontend/src/components/items/SearchMatchHint.tsx` adds one muted `text-[11px]` truncated line under the area line of an items grid card, under the short name in the items table name cell, and under the short name in the area page's items grid and list (`AreaItemsPanel`). It reads `Serial number: SN-1234`, `Tag: Power tools` (the tag's label, not its slug) or a note excerpt. The search page's commodity subtitle shows the same text in place of `A-0042 · <short name>` when there is a match. Nothing is added when the name matched, since the title already shows it. The items search placeholder (items list and area items panel) reads "Search by name, serial or part number, tag, notes or asset ID…".
- **Why**: not present in mock. The issue asks that a result say which field matched, "so a hit on a serial number is not a mystery".
- **Approved by**: user (explicit) — pre-approved for #2800 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` shows match context on results.

#### 2026-11-25 — Items list: alert for a filter the server refuses

- **Issue/PR**: #2800 / PR #2826
- **Mock**: [`design-mocks/src/components/ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) has no error state for the list; the app shows a generic destructive `Alert` ("Could not load items") for any failed load.
- **Reality**: When `GET /commodities` answers 400 with `code: "query.invalid_parameter"` (a hand-edited or outdated link), `CommoditiesListPage` shows a destructive `Alert` titled "This filter is not supported" that names the parameter and carries an outline **Clear filters** button, instead of the generic load error. Fallback pattern: the destructive `Alert` in [`design-mocks/src/views/UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx).
- **Why**: not present in mock. The query contract of epic #2811, which #2800 implements, requires that unsupported filters fail clearly instead of being ignored; the page has to tell the user which part of the link it cannot apply and how to get back to a working list.
- **Approved by**: user (explicit) — pre-approved for #2800 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Permanent until the mock defines list error states.

#### 2026-11-25 — Items list: Tags filter menu with counts

- **Issue/PR**: #2792 / PR (pending)
- **Mock**: [`design-mocks/src/components/ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) toolbar has search, type and status filters and the view toggle; there is no tag filter.
- **Reality**: `frontend/src/components/tags/TagFilterMenu.tsx` adds a **Tags** toolbar button (same `Button size="sm"` outline/default shape and count `Badge` as the other filter dropdowns, with a `Tag` icon) that opens a `Popover` holding a `Command` list: a search box, an **All tags** / **Any tag** switch, a **No tags** row, then one row per item tag with a check/exclude mark, the tag's color dot, its label and its item count. A click requires a tag, a second click excludes it (crossed out, `Ban` mark), a third removes it; a footer says so and carries **Clear**. Counts load from `GET /commodities/tag-counts` when the menu opens. Fallback pattern: the `Popover` + `Command` combobox in [`design-mocks/src/components/CurrencyCombobox.tsx`](../../design-mocks/src/components/CurrencyCombobox.tsx).
- **Why**: not present in mock. #2792 asks for a Tags dropdown with a search box, color and count per tag, all/any, "No tags" and click-again-to-exclude, and says the dropdown needs a mock or a recorded deviation.
- **Approved by**: user (explicit) — pre-approved for #2792 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` draws a tag filter.

#### 2026-11-25 — Tags, area and type link to the filtered items list

- **Issue/PR**: #2792 / PR (pending)
- **Mock**: [`design-mocks/src/components/ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) grid cards and [`design-mocks/src/components/ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) draw item tags with `TagPill size="xs"` as plain spans; the area and the type on the item page are plain text.
- **Reality**: `frontend/src/components/tags/TagPill.tsx` follows the mock's pill (sizes, `#` glyph, `bg-*/15` and `border-*/30` tones, using the `tag-*` color tokens that `TagBadge` already uses for real tag colors) and renders a `Link` when given an `href`, with `hover:opacity-80` and a focus outline. A tag on an item card adds itself to the list's current query (other filters kept, back to page 1); the pill sits above the card-wide link (`relative z-10 pointer-events-auto`), like the selection checkbox. The item page's Details tab links each tag to `?inactive=1&tags=<slug>`, the area value to `?area=<id>&inactive=1` and the type under the item name to `?type=<type>&inactive=1`, the last two with `hover:underline`.
- **Why**: not present in mock. #2792 asks that a tag anywhere, and the type and area values on the item page, link to the items list filtered by them.
- **Approved by**: user (explicit) — pre-approved for #2792 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` shows interactive tags.

#### 2026-11-25 — Item page: Reminders card on the Details tab

- **Issue/PR**: #2805
- **Mock**: [`design-mocks/src/components/ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) has no reminders surface; its Details tab is the facts card only.
- **Reality**: The Details tab renders a **Reminders** card between the details card and the history timeline (`CommodityRemindersSection`, `data-testid="commodity-reminders"`). The card header carries the title, a one-line description and an outline **Add reminder** button; each row shows a `BellRing` icon, the title, a muted line with the repeat rule and lead times, the next date with "In N days" on the right, and ghost icon buttons for edit and delete. A one-off reminder whose date has passed keeps its row with an outline **Passed** badge. Create and edit use a dialog (`ReminderDialog`) with title, date, a comma-separated "Remind me (days before)" input and a Radix **Repeat** select that reveals an **Every (unit)** number input.
- **Why**: Not present in mock. Built from the [`UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) Card and the row anatomy of [`WarrantiesView.tsx`](../../design-mocks/src/views/WarrantiesView.tsx) (icon, two-line text block, date column on the right). The card lives on the Details tab instead of a new tab so the item page stays at seven tabs and the change is one self-contained component.
- **Approved by**: user (explicit) — pre-approved for #2805 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Reconcile when the upstream mock adds a reminders surface to the item page.

### Dashboard / Overview

#### 2026-11-25 — Saved view cards and suggested views on the dashboard

- **Issue/PR**: #2793 / #2839
- **Mock**: [`design-mocks/src/views/DashboardView.tsx`](../../design-mocks/src/views/DashboardView.tsx) goes from the stat cards straight to Expiring Warranties and Recently Added; it has no user-defined cards.
- **Reality**: `frontend/src/components/dashboard/SavedViewsSection.tsx` renders between the stat cards and the two-up grid. Each view the member put on their own dashboard gets a `Card` in a `lg:grid-cols-2` grid, built like Recently Added: icon and name as the title, the item count as the description (the rows' own total once they load, the counts endpoint's until then), an **Open** ghost button as `CardAction`, and the first five items as rows that open the item sheet. While the member keeps no view of their own other than these, a **Start with a saved view** card lists ready-made views (Expiring warranties, Lent out, Without a location, Untagged), each with an **Add** button that saves it to the sidebar and the dashboard, and an `X` that dismisses the card for the group (localStorage).
- **Why**: not present in mock. #2793 asks for a dashboard widget whose count matches the sidebar and the opened list, and for a small useful initial set without dashboard building.
- **Approved by**: user (explicit) — pre-approved for #2793 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` draws dashboard widgets.

#### 2026-05-10 — Stat-card grid: 6 inventory metrics in a single `lg:grid-cols-3` block

- **Issue/PR**: #1544 / PR #1621
- **Mock**: [`design-mocks/src/views/DashboardView.tsx`](../../design-mocks/src/views/DashboardView.tsx) L53-L82 + L112 ships **4** stat cards (Total Items, Active Warranties, Expired Warranties, Est. Total Value) in a `grid-cols-2 gap-4 lg:grid-cols-4` block. The Active/Expired counters depend on `warrantyStatus(item)` rolled up across `MOCK_ITEMS`.
- **Reality**: `frontend/src/pages/Dashboard.tsx` ships **6** stat cards (Total Value, Avg Value, Total Items, Locations, Areas, Files) in a single `grid-cols-2 gap-4 lg:grid-cols-3` block. Mobile renders as a clean 3×2; desktop stays 3×2 (vs the mock's 1×4). Card composition uses the existing `<StatCard>` component, not the mock's inline `<Card>` + `<CardHeader>` shape.
- **Why**: Dual constraint. (1) The warranty status counts the mock's "Active Warranties" / "Expired Warranties" cards depend on require warranty rollups gated on #1367 / #1529 — neither has landed yet, so the cards literally have no data to show. (2) We already had the six Inventory metrics surfacing live data; the prior shipped layout was two grids of three (`lg:grid-cols-3`) which produced two awkward half-empty rows on mobile. Merging into a single 6-card grid is the smallest move that improves mobile without reaching for unimplemented BE rollups.
- **Approved by**: user (explicit) — "Доделай ещё свою главную задачу - дашборд."
- **Reversion plan**: When #1367 / #1529 land the warranty rollups, swap to the mock's 4-card layout (Active / Expiring / Expired / Total Value) by replacing the six `<StatCard>` calls with the four warranty-rooted ones and switching `lg:grid-cols-3` → `lg:grid-cols-4`. The Locations / Areas / Files counts then either move to a secondary surface (sidebar metric strip, or a "More stats" expander) or get folded into the relevant feature pages.

#### 2026-11-25 — Est. Total Value card says how many items it leaves out

- **Issue/PR**: #2813 / #2836
- **Mock**: [`design-mocks/src/views/DashboardView.tsx`](../../design-mocks/src/views/DashboardView.tsx) shows the Est. Total Value card with the fixed caption "current resale value"; every mock item has a value.
- **Reality**: `frontend/src/pages/Dashboard.tsx` keeps the card. While every in-use item has a value, the caption reads "Sum of current values or purchase prices" (`dashboard:stats.totalValueSub`), because an item without a current value counts at its purchase price. When some have neither, the caption reads "Excludes N items without a value" (`dashboard:stats.totalValueExcludes`), from the `unvalued_count` the values endpoint returns.
- **Why**: A purchase price or value can be unknown, and the total leaves such items out rather than counting them as zero. The issue asks every total to disclose how many items it excludes.
- **Approved by**: user (explicit) — pre-approved for #2813 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` shows valuation coverage on the card.

### Locations & Areas

#### 2026-11-24 — Area header: Print labels menu

- **Issue/PR**: #2807 / PR (pending)
- **Mock**: The area view in [`design-mocks/src/views/LocationPickerView.tsx`](../../design-mocks/src/views/LocationPickerView.tsx) has no label action in its header.
- **Reality**: `frontend/src/pages/areas/AreaDetailPage.tsx` adds an outline **Print labels** `DropdownMenu` trigger (`QrCode` icon, chevron) before Edit and Delete, with two items: "Label for this area" and "Labels for this area and its items". The header's button row now wraps on narrow screens.
- **Why**: not present in mock. The issue asks for an area QR code that opens the area with its contents; the area page is where a box gets its label.
- **Approved by**: user (explicit) — pre-approved for #2807 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` places the action.

#### 2026-05-11 — Location & area cards use neutral icon tiles (no per-row emoji) — _resolved 2026-05-12_

- **Issue/PR**: #1531 (items 2 + 3, follow-up resolved by item 4) / PR _pending_
- **Resolution**: Resolved by item 4 of #1531. `models.Location` now carries `icon` (TEXT) + `description` (TEXT) and `models.Area` carries `icon` (TEXT) — see migration `1779500000_add_location_area_icon_description`. `LocationCard` / `AreaTile` swap their `MapPin` / `Package` glyph for the user-picked emoji whenever the field is non-empty, and `LocationCard` prefers `location.description` over `location.address` in the muted-subtitle slot per the Level 1 mock. The address field stays on `LocationDetailPage` (smaller, secondary line) since it's still a real BE concept distinct from the mock's `description`. Form dialogs gained an `IconPicker` field plus a description textarea on the location side.

#### 2026-05-11 — Locations list dropped the inline-areas accordion

- **Issue/PR**: #1531 (item 2) / PR _pending_
- **Mock**: [`design-mocks/src/views/LocationPickerView.tsx`](../../design-mocks/src/views/LocationPickerView.tsx) Level 1 (lines 546–600) shows each location as a single click-through card with stat chips — no areas listed inline. Areas appear only on Level 2 (the location detail).
- **Reality**: `frontend/src/pages/locations/LocationsListPage.tsx` previously rendered each `LocationCard` with a flat `<ul>` of area `<Link>`s inside `CardContent` and an inline "+ Add area" button. That block is removed; the card is now strictly the Level 1 tile (avatar + name + address + Areas / Items stat chips + dropdown menu + chevron). Add area moved into the card's dropdown menu (`data-testid="location-card-add-area"`) so the e2e flow stays single-click without surfacing the action visually outside hover.
- **Why**: Mock-fidelity. The inline accordion was a real-frontend invention from the Vue era. Dropping it makes the list scannable and matches the drill-in flow the mock prescribes (list → location detail → area detail). Add-area lives in the dropdown rather than the location detail page only because the e2e helper (`e2e/tests/includes/areas.ts`) had a single fast entry point that callers across the suite already use; rerouting every test was higher friction than keeping the action one menu-click away.
- **Approved by**: agent-suggested — fits the umbrella issue's "deeper drill-in pages" goal; user reviews this PR.
- **Reversion plan**: Permanent. If the upstream mock ever surfaces areas-inside-list again, restore the `<ul>` block beneath the card body and drop the "Add area" menu item.

#### 2026-05-11 — Multi-segment breadcrumb is inline, not a sticky top strip

- **Issue/PR**: #1531 (item 5) / PR _pending_
- **Mock**: [`design-mocks/src/views/LocationPickerView.tsx`](../../design-mocks/src/views/LocationPickerView.tsx) lines 459–498 render the breadcrumb as a `sticky top-0 px-6 py-4 border-b border-border bg-background z-10` strip — edge-to-edge background, sticks under the (mock's) top of the viewport while the list scrolls.
- **Reality**: `frontend/src/components/locations/LocationsBreadcrumb.tsx` ships the same content (optional ArrowLeft button + chevron-separated segments, current segment bold) inline at the top of the page content area, no sticky behaviour, no edge bleed.
- **Why**: The real app shell already owns the sticky top edge — `<TopBar>` lives there. Stacking a second sticky strip directly beneath it (a) competes with the TopBar for the viewport's top edge, (b) doubles the visual chrome on short pages, and (c) requires bleeding past the page's `p-6` padding with negative margins, which couples the breadcrumb component to the page wrapper's spacing token. Inline placement keeps the breadcrumb readable on first paint without colliding with existing chrome.
- **Approved by**: agent-suggested — the sticky strip is a self-contained mock pattern; the real shell makes it redundant.
- **Reversion plan**: Permanent unless the app ever loses its persistent TopBar. Reconciliation would mean adding a `sticky` variant to `LocationsBreadcrumb` and using it from `LocationDetailPage` / `AreaDetailPage` once the shell decision changes.

#### 2026-05-17 — LocationCard "more actions" trigger keeps the 3-dot `MoreHorizontal` glyph instead of the mock's `MoveHorizontal` alias

- **Issue/PR**: #1654 / PR (pending)
- **Mock**: [`design-mocks/src/views/LocationPickerView.tsx`](../../design-mocks/src/views/LocationPickerView.tsx) imports `MoveHorizontal as MoreHorizontal` (line 3) and renders the renamed component as the per-row "more actions" trigger (lines 577, 645). `MoveHorizontal` is the left-right arrow glyph (`↔`), not the 3-dot glyph the local name implies.
- **Reality**: `frontend/src/pages/locations/LocationsListPage.tsx` imports the real `MoreHorizontal` (3-dot `• • •`) from `lucide-react` and uses it on the per-row dropdown trigger.
- **Why**: The mock's alias is a typo — the canonical "more actions" affordance across the wider codebase, including [`design-mocks/CLAUDE.md`](../../design-mocks/CLAUDE.md)'s Dropdown Menu example, is `MoreHorizontal` (3 dots). Following the typo would diverge from every other "more actions" trigger in the app (`MembersPage.tsx`, `LocationDetailPage.tsx`'s `AreaTile`, the mock's own Dropdown Menu reference snippet) and substitute an arrow glyph that does not communicate "menu trigger". Per the issue body the user explicitly framed this as a typo to be ignored.
- **Approved by**: user (explicit) — issue body: "likely the three-dots `MoreHorizontal` is correct for a 'more actions' trigger and the mock alias is a typo".
- **Reversion plan**: Permanent unless the upstream `inventario-design` repo deliberately rewrites the affordance to use `MoveHorizontal`. If that happens, drop this entry and switch the import to `MoveHorizontal as MoreHorizontal` here and on every other "more actions" call-site.

#### 2026-05-10 — Per-area items panel ships v1 with two stats + simple list (no toolbar / files) — _resolved 2026-05-12_

- **Issue/PR**: #1531 (item 1) / PR _pending_
- **Resolution**: Resolved by the item-1 follow-up PR. `AreaDetailPage` now mounts a new `AreaItemsPanel` (`frontend/src/pages/areas/AreaItemsPanel.tsx`) that ports the mock's full Level-3 toolbar: search input, type / status / warranty filter dropdowns, sort, view-mode toggle (grid + list), hide-inactive switch, and pagination — all URL-state-backed so refresh + back / forward survive. The third "Active warranties" stat cell shipped alongside, sampled from the same page-level fetch that drives the list (so a single network round-trip powers both); partial counts past the page cap surface as `{N}+` with the same truncation cue the locations list uses. The Area Files panel landed under the items section once the BE's `linked_entity_type` enum was widened to include `"area"` (validators in `go/models/models.go:357` + `go/jsonapi/files.go` three sites); `EntityFilesPanel` + `UploadFilesDialog` now accept the `"area"` type union and a `files:entityPanel.dropOverlay_area` i18n key was added. Bulk select stays deliberately deferred — the area surface intentionally doesn't surface bulk actions (those belong to the global `/commodities` page); not tracked as a separate follow-up. The page wrapper stays at `max-w-4xl` (one bucket short of the mock's `max-w-5xl`) — a smaller drift than the embedded list would justify at `max-w-3xl`.

#### 2026-06-21 — Non-empty area/location delete uses a dedicated cascade/unlink choice dialog (#2137)

- **Issue/PR**: #2137 / PR _pending_
- **Mock**: No upstream mock exists. `design-mocks/src/views/LocationPickerView.tsx` shows only a single destructive "Delete" item in the per-row dropdown; there is no design for a delete that must offer the user a choice about the items inside.
- **Reality**: When the user deletes a **non-empty** area or location, instead of the binary `useConfirm()` we show a new `DeleteWithItemsDialog` (`frontend/src/components/locations/DeleteWithItemsDialog.tsx`) built from the shadcn `Dialog` + `RadioGroup` primitives. It offers two strategies: **Keep items, just remove the {area\|location}** (`unlink` — items survive but become un-located; for a location its areas are also removed) and **Delete everything** (`cascade` — items + their files are permanently removed). The choice maps to the BE's `?strategy=cascade\|unlink` query param. An **empty** container keeps the original binary `useConfirm()` dialog + copy unchanged. The dialog defaults to the safe `unlink` option on every open; the confirm button is `destructive`-styled only when `cascade` is selected.
- **Why**: Backend lifecycle feature (#2137) — the delete endpoints now require an explicit strategy for non-empty containers (absent ⇒ 422). `useConfirm()` is binary and cannot host a three-way choice, so a bespoke dialog was the only way to surface the cascade/unlink decision. No mock omission to reconcile because the underlying behaviour (strategy choice) didn't exist when the mock was authored.
- **Approved by**: agent-suggested (per the issue's explicit instruction to "build a NEW dialog component … and log any design deviation").
- **Reversion plan**: Permanent unless `inventario-design` adds a canonical pattern for choice-based destructive deletes, at which point reconcile `DeleteWithItemsDialog` to match.

#### 2026-11-25 — Area items panel: Est. value says how many items it leaves out

- **Issue/PR**: #2813 / #2836
- **Mock**: [`design-mocks/src/components/ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) shows the Est. value stat as a value and a label only.
- **Reality**: `frontend/src/pages/areas/AreaItemsPanel.tsx` adds a second muted line under the label, "Excludes N items without a value", when the area total leaves items out (`locations:areaDetail.items.statsValueExcludes`). Item rows show "—" for an unknown current value and a formatted zero for a known zero, as the mock does for a missing value.
- **Why**: Same reason as the dashboard card: an unknown value is left out of the total, and the total says so.
- **Approved by**: user (explicit) — pre-approved for #2813 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` shows valuation coverage in the stats strip.

### Files & Attachments

#### 2026-11-25 — "Added from files" link on the file detail

- **Issue/PR**: #2810 / PR (pending)
- **Mock**: [`design-mocks/src/views/FileBrowserView.tsx`](../../design-mocks/src/views/FileBrowserView.tsx) has no "Linked to" list; the list itself is an earlier deviation.
- **Reality**: `frontend/src/components/files/FileLinksList.tsx` shows a file held by a batch of dropped files as "Added from files" with a `ScanText` icon, without a link and without the remove button: the batch holds the file until the batch is removed.
- **Why**: not present in mock. Files dropped for batch intake are ordinary files of the group, so the list has to name what holds them.
- **Approved by**: user (explicit) — pre-approved for #2810 by the 2026-11-24 goal to implement the open enhancements.
- **Reversion plan**: Permanent.

#### 2026-11-25 — Files bulk bar: Tags… button and dialog

- **Issue/PR**: #2798 / PR (pending)
- **Mock**: Not present in mock. [`design-mocks/src/views/FileBrowserView.tsx`](../../design-mocks/src/views/FileBrowserView.tsx) has no bulk selection; the bar itself is the recorded deviation of 2026-05-16 below. Fallback: [`design-mocks/src/views/UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) (outline Button with an icon, Dialog) and the mock's tag palette (`ItemEditForm` in [`design-mocks/src/components/ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx)).
- **Reality**: `frontend/src/pages/files/FilesListPage.tsx` adds an outline **Tags…** button (`Tag` icon, `data-testid="files-bulk-tags"`) between **Move to…** and **Delete selected**. It opens the items' bulk tag dialog, now `frontend/src/components/tags/BulkTagsDialog.tsx` with `kind="file"`: **Add tags** (with the new-tag input) and **Remove tags** (existing file tags only). The outcome is a toast; files that failed stay selected instead of opening a result dialog, so **Tags…** retries them. The shared `BulkActionBar` lets its action row wrap, which this third control needs on a phone.
- **Why**: not present in mock. The issue asks for bulk add and remove of file tags. Reusing the items dialog keeps the two surfaces alike; keeping failed files selected with a toast rather than the items' result dialog is agent-suggested, since a file failure carries no reason the user can act on beyond retrying.
- **Approved by**: user (explicit) — pre-approved for #2798 by the 2026-11-24 goal to implement the open enhancements; the toast-and-keep-selected outcome agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock once `inventario-design` adds a bulk-selection pattern to the file browser.

#### 2026-11-25 — Files page: "More tags" picker for any file tag

- **Issue/PR**: #2792 / PR (pending)
- **Mock**: [`design-mocks/src/views/FileBrowserView.tsx`](../../design-mocks/src/views/FileBrowserView.tsx) offers only the six curated `FILE_TAGS` pills in the toolbar.
- **Reality**: `FilesListPage` keeps the curated pills and adds, after them, one pressed pill (with an `X`) per selected tag that has no curated pill, and a **More tags** button that opens `TagFilterMenu` over the group's file tags (search box, color dot, label; no counts, exclusion, all/any or "No tags", since the files endpoint has none of them). Only file tags are listed; item tags stay in their own namespace (#1928).
- **Why**: not present in mock. #2792 asks that file browsing keep the curated tag shortcuts and also offer a searchable picker for any file tag, and that every condition in the URL stay visible.
- **Approved by**: user (explicit) — pre-approved for #2792 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` draws arbitrary file tag selection.

#### 2026-11-25 — File detail sheet: "Linked to" lists every attachment, each with Remove attachment

- **Issue/PR**: #2806 / PR #2827
- **Mock**: [`design-mocks/src/components/FileDetail.tsx`](../../design-mocks/src/components/FileDetail.tsx) shows one "Attached to" `DetailRow` with `file.attachedTo.name`; a file in the mock belongs to exactly one entity. The delete dialog says only that the file will be removed.
- **Reality**: `frontend/src/components/files/FileLinksList.tsx` renders the "Linked to" row of `FileDetailSheet` as a list: one line per item, location or area, with a type icon, the entity name linking to its page, and an icon-only **Remove attachment** button (`X`) that removes that link and nothing else. A file attached to nothing reads "Not attached to anything". The **Delete** confirmation names every place the file is attached to and points at Remove attachment for taking it off one place only. File cards and list rows show the first entity's name plus "+N" for the others.
- **Why**: The issue asks for "Linked to" with several entries in the file panel and for "remove attachment" and "delete file" as separate actions, with the delete action explaining the affected associations.
- **Approved by**: user (explicit) — pre-approved for #2806 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` models a file with several attachments.

#### 2026-11-25 — "Attach an existing file" on the item Files tab and the location/area files panel

- **Issue/PR**: #2806 / PR #2827
- **Mock**: [`design-mocks/src/components/ItemDetail.tsx`](../../design-mocks/src/components/ItemDetail.tsx) `ItemFilesTab` offers only the dashed upload zone; the mock has no way to attach a file that is already stored. Not present in mock; fallback [`design-mocks/src/views/UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) (Dialog, Input with a search icon, Badge, outline Button).
- **Reality**: The commodity Files tab (`frontend/src/components/files/CommodityFilesTab.tsx`) puts an outline **Attach an existing file** button (`Link2` icon) beside the upload zone; below `sm` it stacks under the zone. `EntityFilesPanel` (location and area pages) adds an outline **Attach existing** button before **Attach files**, icon-only below `sm`. Both open `AttachExistingFileDialog`: a search box over the group's files and a scrolling list where each row shows the title, where the file is attached already, and an **Attach** button, or an "Attached" badge when the file is attached to this entity. The dialog stays open so several files can be attached in a row.
- **Why**: not present in mock. The issue asks for "Attach an existing file" in an item's Files tab; the location and area panels get the same entry because files attach to them through the same links.
- **Approved by**: user (explicit) — pre-approved for #2806 by the 2026-11-24 goal to implement the open enhancements; the location/area entry point goes beyond the issue text and is agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock once `inventario-design` adds an attach-existing affordance.

#### 2026-11-25 — Upload dialog offers the file the group already holds

- **Issue/PR**: #2806 / PR #2827
- **Mock**: The upload flow in [`design-mocks/src/views/FileBrowserView.tsx`](../../design-mocks/src/views/FileBrowserView.tsx) has no duplicate check. Not present in mock; fallback [`design-mocks/src/views/UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) (muted inline notice, small outline and ghost Buttons).
- **Reality**: After each upload `UploadFilesDialog` asks the server for files with the same SHA-256 in the group. When there is one, the progress row grows a muted notice "Already in your files as …" with **Attach the existing file instead** (attaches the existing file to the dialog's item, location or area and deletes the new copy; a cover chosen on the new copy moves to the existing file) and **Keep both**. Without a linked entity the first button reads **Remove this copy** and only deletes the new upload.
- **Why**: The issue asks that a match within the group makes the upload dialog offer "attach the existing file instead". Remove this copy (uploads from the Files page, where there is nothing to attach to) and moving the cover are agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2806 by the 2026-11-24 goal to implement the open enhancements; Remove this copy and the cover move agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock once `inventario-design` designs a duplicate prompt.

#### 2026-11-24 — File edit page: preview beside the form, list walk with "Save and next"

- **Issue/PR**: #2814 / PR #2821
- **Mock**: [`design-mocks/src/components/FileDetail.tsx`](../../design-mocks/src/components/FileDetail.tsx) `EditMetadataDialog` edits a file in a small modal over the detail sheet: name, category and a tag palette, with Cancel and Save. It shows no preview, and the mock has no way to move from one file to the next while editing.
- **Reality**: `/g/<slug>/files/<id>/edit` (`frontend/src/pages/files/FileEditPage.tsx`) is a wide page. On `lg` and up it is a five-column grid: the preview (`FilePreview` with `variant="pane"`, the same image, PDF and no-preview rendering as the sheet, plus Open and Download links) takes three columns, and the form (title, filename, category, description, tags) with the action bar takes two. Below `lg` the two halves are `Tabs` (Preview, Details) whose panels stay mounted, so typed values, PDF page and zoom survive a tab switch; the action bar sticks to the bottom of the screen. When the page is opened from the Files list it carries that list in a `from` URL parameter (`frontend/src/lib/list-context.ts`). The header then shows the position ("3 of 12") with previous and next icon buttons, and the action bar gets "Save and next" as the primary button (Save becomes outline). Back, Save and Cancel return to the same list query, on the page the walk reached, with the file's sheet open. Leaving with unsaved edits opens a Save / Discard / Keep editing dialog (`frontend/src/hooks/useUnsavedChangesGuard.tsx`). The walk itself is `frontend/src/hooks/useListNavigation.ts`, written to be reused by the item page (#2795) and the inbox (#2794).
- **Why**: The issue asks for exactly this. Correcting a batch of uploads means reading each file while editing it, and a modal over the sheet hides the preview and returns the user to the list after every save.
- **Approved by**: user (explicit) — pre-approved for #2814 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Permanent unless the upstream mock adds its own edit surface with a preview; the list walk then keeps working on whatever surface replaces this page.

#### 2026-05-31 — File detail panel keeps the real-frontend composition (title-first + metadata `<dl>`), not the mock's banner-first order

- **Issue/PR**: #1962 / PR (this branch)
- **Mock**: [`design-mocks/src/components/FileDetail.tsx`](../../design-mocks/src/components/FileDetail.tsx) `FileDetailSheet` composes the panel as: full-bleed preview banner at the very top → `SheetHeader` whose title is the **filename** (icon-tile + MIME·size) → an inline action-button row (View/Open/Download/Edit/Delete) → `Separator` → `DetailRow` metadata. There is no separate "File details" heading and no loading/error/`<dl>` machinery.
- **Reality**: `frontend/src/components/files/FileDetailSheet.tsx` keeps its existing structure: a `SheetHeader` whose title is the static `t("files:detail.metadataTitle")` ("File details") with the filename as the description → the preview → a `<dl>` metadata grid (filename / category / type / MIME / linked entity / uploaded / tags / description) → a `SheetFooter` action row. #1962 brought it into line with the mock on the two **layout** defects it enumerated — the body is now horizontally inset (`px-5`, content no longer flush to the edges) and the image preview deliberately breaks out to full-bleed (`-mx-5 w-[calc(100%+2.5rem)]`) — but did **not** re-order the panel into the mock's banner-first composition.
- **Why**: Scope + real-data constraints. The issue scopes the layout fix to "no horizontal padding / image edge-to-edge" and the three behavioural defects; it does not ask for a full recomposition. The real panel also carries machinery the mock has no concept of — a TanStack-Query loading skeleton + error `Alert`, the richer real metadata `<dl>` (linked-entity, description, tag-filter chip links), and a required Radix `SheetTitle` for dialog a11y — so a 1:1 banner-first port would be a much larger, higher-regression change than the defects warrant. The padding + full-bleed reconciliation captures the visible defect from the issue's screenshots; the heading/metadata composition stays as-is.
- **Approved by**: agent-suggested-then-user-confirmed — the user directed (via the session goal) that #1962's enumerated defects be fixed; the issue (authored by the maintainer) frames the layout reconciliation as the padding/full-bleed fix, not a recomposition, which scopes this decision.
- **Reversion plan**: A later pass can adopt the mock's banner-first order (filename as `SheetTitle`, action row above the metadata) once the loading/error/`<dl>` concerns are folded into that shape; tracked against #1962 follow-up. The padding + full-bleed change is permanent.

#### 2026-05-16 — Files page keeps the multi-select BulkBar; surfaces it as a fixed bottom overlay

- **Issue/PR**: #1659 / PR (this branch)
- **Mock**: [`design-mocks/src/views/FileBrowserView.tsx`](../../design-mocks/src/views/FileBrowserView.tsx) has no bulk-select affordance at all — file rows have no per-row checkbox, no select-all, no bulk action toolbar.
- **Reality**: `frontend/src/pages/files/FilesListPage.tsx` ships per-file checkboxes (both on `FileCard` and `FileListRow`) and a context-mode `BulkBar` with select-all, "Move to…" reclassification, and bulk delete (`data-testid="files-bulk-bar"`). Under this PR the bar shifts from an in-flow `bg-muted/40` row to a `fixed bottom-6 left-1/2 -translate-x-1/2 z-40` overlay on the `popover` token, animating in via `tw-animate-css` (`animate-in slide-in-from-bottom-4 fade-in-0 duration-200`) so the first selection no longer reflows the page.
- **Why**: Bulk operations (move, delete) are real product affordances on a page that can hold tens of thousands of files — dropping them to match the mock would be an unambiguous regression. The fixed-overlay shape follows the "context-mode toolbar" pattern (GMail/Drive/shadcn) so the bar stays visible across scroll without ever pushing the grid down. Using `bg-popover` (a token that already implies a floating-surface elevation) keeps us off bespoke `shadow-*` decoration per the design language.
- **Approved by**: user (explicit) — issue #1659 §9 "PRESERVE: file multi-select — but animate the bulk bar entry" lists the bar as a real product feature and asks for the fixed-overlay treatment as the preferred option.
- **Reversion plan**: Permanent unless the upstream mock adopts a richer mass-action pattern. The unrelated hidden-by-default checkbox UX (separate UX concern) is tracked in #1484; if that lands, this entry stays as-is (the BulkBar shape doesn't change) and the checkbox visibility note moves to its own entry.

#### 2026-05-16 — Files page shows three category tiles (no Invoices), invoice semantic lives on a tag (#1622)

- **Issue/PR**: #1622 / PR _pending_
- **Mock**: [`design-mocks/src/views/FileBrowserView.tsx`](../../design-mocks/src/views/FileBrowserView.tsx) and [`design-mocks/src/components/CategoryTiles.tsx`](../../design-mocks/src/components/CategoryTiles.tsx) render four category tiles — Images / Invoices / Documents / Other — each backed by a `FileCategory` enum value. Storage pie + the per-commodity Files tab chip-bar carry the same four buckets.
- **Reality**: We collapsed `FileCategoryInvoices` into `documents` on both sides; only three tiles ship (`images` / `documents` / `other`). The "invoice" semantic survives as a conventional tag (`FileTagInvoice` = `"invoice"`), surfaced via the existing toolbar tag pill on the Files page and via the still-present "Invoices" chip on the commodity Files tab (whose filter switched from `category=invoices` to `tags @> "invoice"`). The migration `1779553060_collapse_invoice_category` reclassifies legacy rows and auto-provisions the tag. (`getFileVisualMeta` in `frontend/src/features/files/constants.ts` from #1659 grew an `invoice`-tag short-circuit so the Receipt glyph + chart-1 palette still surfaces on invoice-tagged PDFs.)
- **Why**: The four-axis model didn't scale — every new document kind (warranty, manual, contract, receipt, certificate) wanted its own top-level bucket. Tags are the right axis for "what kind of document is this"; collapsing leaves three buckets that map to actual *handling* differences (thumbnailing, viewers, default sort). The receipt-icon glyph on rows now tracks the tag rather than the category, so users still spot invoices visually.
- **Approved by**: agent-suggested-then-user-confirmed (issue #1622 §"Suggested implementation" explicitly directs the collapse + tag conversion).
- **Reversion plan**: Permanent — the down migration restores `invoices` category for any row that still carries the `invoice` tag, but the FE wouldn't render a fourth tile without re-introducing the dropped enum value (intentional one-way ratchet).

#### 2026-05-09 — Curated tag pills match by lowercase tag name, not opaque tag id

- **Issue/PR**: #1538 (item 3) / PR _pending_
- **Mock**: [`design-mocks/src/views/FileBrowserView.tsx`](../../design-mocks/src/views/FileBrowserView.tsx) (lines ~645–673) renders six curated tag pills sourced from `FILE_TAGS` in [`design-mocks/src/data/mock.ts`](../../design-mocks/src/data/mock.ts) — each pill is `{ id: "t1", label: "Invoice", color: "text-chart-1" }` and matches `file.tags.includes("t1")`. Files in the mock dataset are tagged with the same opaque ids (`t1`, `t2`, …).
- **Reality**: The real BE stores `tags` as a free-form `string[]` (no Tags entity yet — that's #1400). The FE's curated pills mirror the mock's six labels (Invoice / Warranty / Manual / Photo / Certificate / Backup) but match against the lowercase tag name (`invoice`, `warranty`, …) so the toolbar pill toggles a recognisable string into `?tags=`. Custom user-supplied tags still render on the file cards/rows but don't appear as toolbar pills, and the freeform `TagsInput` is removed from the toolbar (it stays on the upload/edit forms only, per the issue spec).
- **Why**: The mock's opaque-id taxonomy doesn't exist on the BE — there's no Tags table to assign ids from. Using the lowercase label as the literal tag string keeps the pill flow round-trippable through `?tags=` and the BE's `tags @> $` filter without inventing an id space the BE doesn't enforce. The discoverability gap for custom tags is a deliberate trade — the issue explicitly notes "Likely coordinates with #1400" — and the curated taxonomy is the canonical surface until that lands.
- **Approved by**: agent-suggested-then-user-confirmed — issue #1538 §3 specifies replacing the freeform input with curated pills and notes the #1400 coordination.
- **Reversion plan**: Resolve when #1400 lands a proper Tags entity — pills then match by id again, and the i18n keys become tag-record labels.

### Tags

#### 2026-11-25 — Tags page: tag names link to the filtered list

- **Issue/PR**: #2792 / PR (pending)
- **Mock**: [`design-mocks/src/views/TagsView.tsx`](../../design-mocks/src/views/TagsView.tsx) row label is plain text.
- **Reality**: `TagRow` renders the `# label` as a `Link` with `hover:underline`: an item tag opens the items list with `?inactive=1&tags=<slug>`, so the list matches the row's item count, and a file tag opens the Files page with `?tags=<slug>`.
- **Why**: not present in mock. #2792 asks that a tag in a row of the Tags page link to the list filtered by it.
- **Approved by**: user (explicit) — pre-approved for #2792 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` shows interactive tag rows.

#### 2026-05-15 — Tags page: full-shape TagFormDialog preserved alongside inline create

- **Issue/PR**: #1539 / PR _pending_
- **Mock**: [`design-mocks/src/views/TagsView.tsx`](../../design-mocks/src/views/TagsView.tsx) captures only a `label` string in its inline-create row (lines 150–199) and edit row (lines 109–146); the rendered tag pill displays the same string with a `#` glyph. There is no separate slug concept.
- **Reality**: `frontend/src/components/tags/TagFormDialog.tsx` exposes both a human-readable `label` (e.g. "Kitchen Supplies") and a kebab-cased `slug` (e.g. `kitchen-supplies`) as independent fields. The slug is the stable identifier referenced from `commodities.tags` / `files.tags` JSONB arrays on the BE — decoupling label from slug lets a user rename a tag without rewriting every reference (just the label column). The new inline "+ New tag" row on the Tags list page (port of the mock fast-path) only captures `label` and derives `slug` via `normaliseSlug()`; clicking the page-header "Add tag" button or any row's edit affordance still opens the full slug+label dialog.
- **Why**: BE-driven. Tag identity is the slug (`models.Tag.slug`, unique per group, immutable from the user's perspective once references exist); the label is purely display. The mock's single-field flow would force label-to-slug normalisation on every edit and silently break any commodity that referenced the prior slug. Keeping the dialog as the canonical edit surface preserves the BE contract; the inline create row is an additive fast path that auto-derives the slug for the common case where label and slug should match.
- **Approved by**: user (explicit) — issue #1539 §"PRESERVE" calls out the dialog and instructs the inline create to land as an additional fast path, not a replacement.
- **Reversion plan**: Permanent. If the BE ever exposes slug renames with reference fix-up (or drops slug entirely in favour of UUIDs), the dialog can collapse to label-only and the inline row would be the only path.

#### 2026-05-15 — Tags stats bar shows five tiles (tags + items + files split) where the mock shows three

- **Issue/PR**: #1539 / PR _pending_
- **Mock**: [`design-mocks/src/views/TagsView.tsx`](../../design-mocks/src/views/TagsView.tsx) lines 245–261 render three icon-headed stat tiles in a `grid-cols-3` row: "Total tags", "Tagged items", "Untagged items".
- **Reality**: `frontend/src/components/tags/TagsStatsBar.tsx` keeps the icon-headed mini-card pattern from the mock but renders five tiles in a `grid-cols-2 sm:grid-cols-3 lg:grid-cols-5` row — adding "Tagged files" and "Untagged files" alongside the items split. Values come from the BE `/tags/stats` endpoint (#1412), which already aggregates both surfaces.
- **Why**: The BE indexes tag adoption on both commodities *and* files (BE PR #1412); surfacing only the items half would hide half the picture on what is the canonical "how are tags being used?" page. The Files page exposes a tagged/untagged count in its own toolbar, but the Tags page is the natural home for the cross-surface roll-up. Mock styling (icon tile + label + value) is preserved per-tile.
- **Approved by**: agent-suggested — judgment call inside the issue's "broader visual representation drift (catch-all)" §4 latitude.
- **Reversion plan**: Drop the two file tiles to match the mock 1:1 if the file-tag rollout turns out to be noise on this page; the BE `/tags/stats` payload remains additive so back-and-forth doesn't require schema changes.

#### 2026-05-15 — Tags row item-preview chips aggregate client-side from a single commodities pull

- **Issue/PR**: #1539 / PR _pending_
- **Mock**: [`design-mocks/src/views/TagsView.tsx`](../../design-mocks/src/views/TagsView.tsx) lines 327–343 surface up to two item-preview chips per row plus a "+N" overflow, computed from `MOCK_ITEMS.filter(i => i.tags.includes(tag.id))`.
- **Reality**: `frontend/src/pages/tags/TagsListPage.tsx` calls `useCommodities({ perPage: 500, includeInactive: true })` once on mount and builds a `Map<slug, [{id, name}]>` (capped at two per tag) in a `useMemo`. Each `<TagRow>` reads its entry from the map and renders the same `≤2 + overflow` shape. Overflow count comes from `tag.usage.commodities` (the authoritative count from `/tags?include=usage`) minus the resolved-chip count, so the figure stays correct even if a tagged commodity sits past the 500-row window.
- **Why**: The BE does not expose a tag-filtered commodities index — same trade-off `#1531` made for area-counts on the Locations page. Client-side aggregation is acceptable for the page's expected scale (a group rarely exceeds a few hundred items); the chip data is a UX enrichment, not a primary read path, so the heavy fetch happens lazily and re-uses the existing `commodityKeys.list` cache.
- **Approved by**: agent-suggested — issue #1539 §1 calls out chips as "Effort M (chips need item-tag join)", signalling the maintainer expects a non-trivial path; client-side aggregation is the lowest-cost option.
- **Reversion plan**: If usage grows past the 500-row window often enough to be visibly wrong (chips empty even though `usage.commodities > 0`), expose a `/commodities?tags=<slug>` BE filter and switch the page to one query per visible tag (or batch via `/tags/{slug}/sample`). The map shape on the FE stays.

### Sellers

#### 2026-11-25 — Sellers page

- **Issue/PR**: #2804 / PR #2844
- **Mock**: none. The closest view is [`design-mocks/src/views/TagsView.tsx`](../../design-mocks/src/views/TagsView.tsx): a page header, a search box and a card of rows with an icon, a label, a usage line and edit / delete actions on hover.
- **Reality**: `frontend/src/pages/sellers/SellersListPage.tsx` at `/g/:slug/sellers` follows that view without its stats bar, inline create row and item chips: a `PageHeader` with **New seller**, a search box that filters by name on the client (`?q=`), and a card of `SellerRow`s with a `Store` icon, the name (a link to the items list filtered by the seller, inactive items shown), the website host as an external link, "N items · contact", the note, and edit / delete buttons on hover. `SellerFormDialog` takes name, website, contact and note. Deleting asks for confirmation and names the number of items that lose the seller.
- **Why**: not present in mock. #2804 asks for a page listing the sellers with their item counts, and for a confirmation naming the count before a seller is deleted.
- **Approved by**: user (explicit) — pre-approved for #2804 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` adds a sellers view.

### Item types

#### 2026-11-25 — Item types page

- **Issue/PR**: #2802 / PR #2863
- **Mock**: none. The closest view is [`design-mocks/src/views/TagsView.tsx`](../../design-mocks/src/views/TagsView.tsx): a page header, a search box and a card of rows with an icon, a label, a usage line and edit / delete actions on hover.
- **Reality**: `frontend/src/pages/itemTypes/ItemTypesListPage.tsx` at `/g/:slug/item-types` follows the Sellers page (#2804), which follows that view without its stats bar, inline create row and item chips: a `PageHeader` with **New type**, a search box that filters by the shown name on the client (`?q=`), and a card of `ItemTypeRow`s with the type's icon, its name (a link to the items list filtered by the type, inactive items shown), a secondary **Built-in** badge on the six built-in types, "N items · N in Trash", and edit / delete buttons on hover; the delete button is disabled on the group's last type. `ItemTypeFormDialog` takes a name and an icon from `ItemTypeIconPicker`, a grid of 38 Lucide icons as a radio group (`grid-cols-6 sm:grid-cols-8`, each a bordered icon tile, the selected one with a primary border and a `bg-primary/10` fill). A built-in type's name field starts empty with its translated name as the placeholder and a hint that an empty name keeps it. `ItemTypeDeleteDialog` replaces the confirm dialog: for a type no item has it confirms the delete; for a type items have, Trash included, it names the counts and asks, with a `Select` of the other types, where the items move, and its button reads **Move items and delete**. That is also how two types are merged.
- **Why**: not present in mock. #2802 asks for types managed like tags (rename, merge, delete with an in-use check), each with a name and an icon. The icon set and its picker, the Built-in badge, the separate Trash count and the empty-name rule for built-in types are agent-suggested.
- **Approved by**: user (explicit) — pre-approved for #2802 by the 2026-11-24 goal to implement the open enhancements; icon picker, Built-in badge, Trash count and empty-name rule agent-suggested, listed in the PR for the owner.
- **Reversion plan**: Follow the mock once `inventario-design` adds an item types view.

### Forms & Validation

#### 2026-05-29 — Design-system "start": blessed FieldError + dropdown policy with annotated native-`<select>` exceptions

- **Issue/PR**: #1264 / PR (this branch)
- **Mock**: The mock renders field errors as inline `<p className="text-xs text-destructive">` per field and uses the shadcn `<Select>` (`design-mocks/src/views/SettingsView.tsx`) — plus a vendored `components/ui/native-select.tsx` primitive — for dropdowns. It has no single shared field-error component.
- **Reality**: A shared `<FieldError>` (`components/FieldError.tsx`) replaces the ad-hoc `<p>` across the migrated forms (EditProfile, CreateGroup, AreaForm, SetPassword), baking in the `field-error` class + i18n resolution. Form & settings dropdowns use the shadcn `<Select>`; an ESLint `no-restricted-syntax` guard bans raw native `<select>`. Six **utility** native `<select>`s (bulk-move bars on Files/Commodities lists, the mobile category-tile collapse in `CategoryTiles`, the Tags list sort, the file-edit + per-file-upload category pickers) are kept native — they have `selectOption`-based test coverage and native semantics are intentional there — each annotated with an inline `eslint-disable-next-line` + reason. We did **not** port the mock's `native-select.tsx` primitive (those raw selects stay ad-hoc pending a later pass).
- **Why**: #1264 is largely satisfied by the React rewrite (#1397); this is the roadmap's "#1264 start" slice (per #1651). Extracting `<FieldError>` is invisible visually but removes n copies of the same markup. The dropdown guard makes every remaining native `<select>` a conscious, documented choice ("pick one and stick with it") without a risky sweep of tested utility selectors. Converting those six is a deferred follow-up.
- **Approved by**: user (explicit) — chose scope "Roadmap «start»" + approach "primitives-direct + FieldError" when offered the options.
- **Reversion plan**: Permanent for `<FieldError>` + the lint guard. The six annotated native selectors are the explicit backlog: migrate them to `<Select>` (or a ported `native-select` primitive) and drop the disable comments in the post-launch design-system "finish" pass.

#### 2026-05-29 — Two intentional IconPicker components (popover vs. inline grid) kept separate

- **Issue/PR**: #1264 / PR (this branch)
- **Mock**: Location/area dialogs (`design-mocks/src/components/LocationDialog.tsx`, `AreaDialog.tsx`) render an inline emoji grid. The group surface is a different composition.
- **Reality**: Two components share the name `IconPicker` — `components/groups/IconPicker.tsx` (Popover + category tabs, sourced from `features/group/icons.ts`) and `components/locations/IconPicker.tsx` (inline grid, palette via prop). They are **not** merged: the audit for #1264 flagged a "name collision", but the two are genuinely distinct UX with different data sources and a11y trade-offs. Resolved by documenting (cross-ref comments in both files + a forms.md table) which to use, rather than collapsing them into one `mode`-switching component.
- **Why**: Merging would either bloat one component with a popover/inline `mode` switch (poor cohesion) or break mock fidelity (the dialogs want the inline grid; the group form wants the space-saving popover). The folder paths already disambiguate the imports.
- **Approved by**: agent-suggested — judgment call within the #1264 "dedup IconPicker" scope item; surfaced to the user as "not duplicates → document, don't merge".
- **Reversion plan**: Permanent unless the upstream mock unifies the two surfaces.

#### 2026-11-25 — Item purchase step: purchase date and prices are never required

- **Issue/PR**: #2813 / #2836
- **Mock**: [`design-mocks/src/components/AddItemDialog.tsx`](../../design-mocks/src/components/AddItemDialog.tsx) L1158-L1161 marks Purchase Date required for a non-draft item, with the hint "Required for non-draft items"; the Purchase Price hint reads "Original purchase currency".
- **Reality**: `frontend/src/components/items/CommodityFormDialog.tsx` drops the asterisk and `aria-required` from Purchase date and Original price on every item. Purchase date carries the hint "Leave blank if you don't know it." and Original price "Leave blank if you don't know it; enter 0 if it was free." The in-place edit form (`CommodityEditForm.tsx`) shows the same two hints. A blank field is saved as unknown and "0" as a known zero. The foreign-currency banner and its "converted price or current value" rule apply only when the purchase price is known.
- **Why**: A reviewed item may have an unknown purchase date or price (a gift, an inherited object), and unknown is distinct from zero. The hints tell the user how to record each case.
- **Approved by**: user (explicit) — pre-approved for #2813 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` updates the purchase step.

### Auth & Profile

#### 2026-11-24 — A third sign-in button for the operator's OpenID Connect provider

- **Issue/PR**: #2786 / PR (pending)
- **Mock**: [`design-mocks/src/views/AuthView.tsx`](../../design-mocks/src/views/AuthView.tsx) (the "or continue with" block) shows exactly two outline buttons, Google and GitHub, in a two-column grid. Settings has no Connected accounts mock.
- **Reality**: `frontend/src/components/auth/OAuthRow.tsx` also renders an `oidc` button when the server lists that provider. It is the same outline `Button` with the label the operator configured (for example "Authelia") and the Lucide `KeyRound` icon, since the brand is unknown. The grid rule is unchanged (one column for one provider, two for more), so with all three providers the OpenID Connect button sits alone on a second row. `ConnectedAccountsCard` uses the same icon for the `oidc` row.
- **Why**: not present in mock — the mock predates sign-in with a self-hosted OpenID Connect provider (#2786). Closest pattern: the existing provider button, with a Lucide icon in place of the brand glyph (`devdocs/frontend/icons.md`).
- **Approved by**: user (explicit) — asked for the OpenID Connect provider in #2786.
- **Reversion plan**: Reconcile if the upstream mock adds a generic single sign-on button or a three-provider layout.

#### 2026-11-24 — Edit profile "Photo" section gains a "Remove photo" link and lists WebP

- **Issue/PR**: #1382 / PR (pending)
- **Mock**: [`design-mocks/src/views/EditProfileView.tsx`](../../design-mocks/src/views/EditProfileView.tsx) (the "Photo" `Section`) draws the 64px tile with a hover camera overlay, one outline "Upload photo" button and the hint "JPG or PNG, max 2 MB". There is no way to remove a photo.
- **Reality**: `ProfilePhotoSection` in `frontend/src/pages/EditProfilePage.tsx` keeps that anatomy (tile, camera overlay, outline button, hint) and adds a small destructive text link, "Remove photo", under the hint once a photo is set; it asks for confirmation through `useConfirm`. The hint reads "JPG, PNG or WebP, max 2 MB".
- **Why**: #1382 accepts WebP uploads and requires that removing the photo falls back to the initials, which needs a control the mock does not draw.
- **Approved by**: user (explicit) — pre-approved for #1382 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Reconcile if the upstream mock adds a remove control or changes the hint.

#### 2026-11-24 — Edit profile "Photo" section sits in a card, and its button follows the upload state

- **Issue/PR**: #1382 / PR (pending)
- **Mock**: [`design-mocks/src/views/EditProfileView.tsx`](../../design-mocks/src/views/EditProfileView.tsx) lays the sections out flat, title column on the left, separated by `<Separator />`. The button always reads "Upload photo".
- **Reality**: The section uses the same title-column layout inside a `rounded-xl border bg-card p-5` card, like the profile and password forms already on `/profile/edit`. The button reads "Change photo" once a photo is set and "Uploading…" while an upload runs, and the tile shows the chosen file until the server answers. The camera overlay also appears on keyboard focus (`focus-visible:opacity-100`), not only on hover.
- **Why**: Agent-suggested. The page already draws its forms as cards, and a flat section above them would look like a different page. The button labels and the preview tell the user what the button will do and that something is happening; the focus rule makes the overlay usable without a mouse.
- **Approved by**: agent-suggested (listed in the #1382 PR for review)
- **Reversion plan**: Drop the card if `/profile/edit` moves to the mock's flat layout.

#### 2026-11-24 — Avatars show the profile photo where the mock draws initials

- **Issue/PR**: #1382 / PR (pending)
- **Mock**: Every avatar in the mock is an initials tile: the sidebar user menu ([`AppSidebar.tsx`](../../design-mocks/src/components/AppSidebar.tsx)), the account card in [`SettingsView.tsx`](../../design-mocks/src/views/SettingsView.tsx), the identity card in [`UserProfileView.tsx`](../../design-mocks/src/views/UserProfileView.tsx) and the member rows in [`MembersView.tsx`](../../design-mocks/src/views/MembersView.tsx).
- **Reality**: All of them render `frontend/src/components/UserAvatar.tsx` (the vendored shadcn `Avatar`). It shows the user's photo, cropped to the tile's shape, and the initials tile when there is no photo or it fails to load. The admin user detail and admin group membership editor use it too, but always show initials, because the admin API does not hand out photo URLs. The initials follow one rule everywhere: the first letters of the first two words of the name, else the first letter of the email.
- **Why**: #1382 adds profile photos and asks for them wherever a user is drawn. The mock's initials are the no-photo state.
- **Approved by**: user (explicit) — pre-approved for #1382 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Permanent.

### Layout & Page chrome

#### 2026-05-25 — Canonical page widths + PageHeader: two named tokens (narrow / wide) instead of the mock's case-by-case max-w-\*

- **Issue/PR**: #1889 / PR #1895
- **Mock**: Every view in `design-mocks/src/views/` rolls its own outer wrapper — `max-w-2xl` (`TagsView`, `GroupSettingsView`), `max-w-3xl` (`UserProfileView`, `MembersView`, admin/`UserDetailView`, admin/`GroupDetailView`), `max-w-4xl` (`SettingsView`, `WarrantiesView`, `BackupView`, `PlansView`, admin/`TenantsView`, admin/`GroupsView`, admin/`TenantDetailView`), `max-w-5xl` (`DashboardView`, `ItemsView`, `FileBrowserView`), with each page's H1 picking its own size + tracking too (`text-3xl scroll-m-20 tracking-tight` for list pages, `text-2xl tracking-tight` for detail pages, plus a few `text-2xl font-semibold` no-tracking outliers). There is no shared layout primitive in the mock — every view is self-contained.
- **Reality**: `frontend/src/components/ui/page.tsx` introduces `<Page width="narrow" | "wide" | "full">` + `<PageHeader size="page" | "detail">`. `narrow` = 48rem / 768px (`--container-page-narrow`), `wide` = 80rem / 1280px (`--container-page-wide`), `full` = no max-width. Width tokens live in `frontend/src/index.css` (`@theme { --container-page-narrow / --container-page-wide; }`). Every top-level route now composes with one of the three; ad-hoc `max-w-*` literals on top-level wrappers are forbidden by the convention guard at `frontend/src/pages/__tests__/page-layout-tokens.test.ts`. The UIShowcasePage gains a "Page" tab enumerating the canonical variants for design + dev review.
- **Why**: User-driven UX request — issue #1889 calls out that walking through routes produces an inconsistent "envelope" rhythm (Warranties vs. Lent Out is the cited example) and asks for "a small, named set of layout widths and apply them consistently". Standardising on 2+1 named widths (and a single PageHeader shape) eliminates the per-page-author drift the mock didn't lock down, while preserving the narrow-vs-wide distinction the mock implicitly used. The tokens sit slightly wider than the most-common mock widths (`wide` = 80rem vs. mock's frequent 5xl=64rem, `narrow` = 48rem vs. mock's variable 2xl/3xl) — chosen from the issue's recommended ranges of "~720–800px narrow" / "~1200–1280px wide" to give data-heavy pages more room before the next mock sync. List pages stick with the mock's `scroll-m-20 text-3xl font-semibold tracking-tight`; detail pages stick with `text-2xl font-semibold tracking-tight`.
- **Approved by**: user (explicit) — issue body specifies the named-tokens approach and the canonical PageHeader requirement, plus a lint guard.
- **Reversion plan**: Permanent. If the upstream mock ever ships its own layout primitive, swap the token values to match and drop this entry. Pages requesting an exemption (centered error/auth/empty-state cards, print pages) are listed in the convention guard's `SPECIAL_PURPOSE_PAGES` allow list — those stay bespoke by design.

### Settings & Preferences

#### 2026-11-25 — Storage card: Trash bucket and note

- **Issue/PR**: #2799 / PR #2835
- **Mock**: [`design-mocks/src/views/SettingsView.tsx`](../../design-mocks/src/views/SettingsView.tsx) (Data & Storage) shows used storage without a Trash share.
- **Reality**: On the storage card (Group settings → Data & storage), `frontend/src/features/storage/StoragePie.tsx` adds a **Trash** slice and legend row (warm gray `#8c8279`). When the group has bytes in Trash, `StorageCard.tsx` adds a line "N of this is in the trash. It is freed when the trash is emptied or the entries expire." with an **Open trash** link.
- **Why**: The issue asks to show the trash portion of storage so cleanup has a predictable effect. Trashed files still count toward the quota until they are purged.
- **Approved by**: user (explicit) — pre-approved for #2799 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock if `inventario-design` adds a storage breakdown with Trash.

#### 2026-11-24 — Personal API tokens: Privacy & security row, `/profile/api-tokens` page, create dialog with one-time reveal

- **Issue/PR**: #2808 / PR (pending)
- **Mock**: [`design-mocks/src/views/SettingsView.tsx`](../../design-mocks/src/views/SettingsView.tsx) `PrivacySection` has three rows (Two-factor authentication, Active sessions, Login history) and the danger zone. The mock has no API tokens surface at all: no row, no list page, no create dialog.
- **Reality**: `frontend/src/pages/SettingsPage.tsx` adds a fourth `PrivacyRow` ("API tokens", `privacy-row-apiTokens`) under Login history, with an "N active" badge that counts unexpired tokens. It links to `frontend/src/pages/ApiTokensPage.tsx` at `/profile/api-tokens`, which copies the `SessionsPage` shape: `Page width="narrow"`, `PageHeader size="detail"` with the back link to Privacy & security, an action card with a `bg-primary/10` icon tile and a "New token" button, then one `rounded-xl border bg-card p-4` card per token (name, scope badge, destructive "Expired" badge, display prefix, group or "All your groups", last used or "Never used", expiry or "No expiry", created time, Revoke) and a confirm `Dialog` for revoke. `frontend/src/components/settings/CreateAPITokenDialog.tsx` is a shadcn `Dialog` with a name `Input`, a scope `RadioGroup` (bordered option cards, as in the showcase), a group `Select` and an expiry `Select`. When the account has a password it asks for it, and when MFA is active it asks for a TOTP or backup code with the same toggle as `MFADisableDialog`. On success the dialog switches to the reveal step: a warning `Alert`, a read-only monospace `Input` holding the token, an icon copy button and Done. While the token is shown, a click outside the dialog does not close it.
- **Why**: not present in mock. Composition falls back to the `SessionsPage` / `LoginHistoryPage` convention for `/profile/*` pages and to [`UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) for Dialog, RadioGroup, Select, Badge and Alert. The issue asks for name, optional expiry, scope, optional group, a one-time reveal, a list with last-used time, and revoke. These parts go beyond the issue text and are agent-suggested: expiry is a preset `Select` (30 days, 90 days, 1 year, no expiry; default 90 days) rather than a date picker; creating a token asks for the current password and the second factor; the Settings row shows an active-token badge; the reveal step cannot be dismissed by an outside click; the create and revoke buttons are disabled with an explanation while impersonating, matching the server's 403.
- **Approved by**: user (explicit) — pre-approved for #2808 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Reconcile when the upstream mock adds an API tokens surface: match its row, page and dialog anatomy and drop whatever this entry lists that the mock does not keep.

#### 2026-05-26 — Personal Preferences lands on Account, not Appearance

- **Issue/PR**: #1888 / PR (pending)
- **Mock**: [`design-mocks/src/views/SettingsView.tsx`](../../design-mocks/src/views/SettingsView.tsx) L41 initialises `useState<SettingsSection>("appearance")` — opening the Preferences view lands on the Appearance tab.
- **Reality**: `frontend/src/pages/SettingsPage.tsx` initialises `useState<SectionId>("account")` so `/settings` lands on Account. All other sub-tabs (Appearance, Notifications, Privacy, Help) remain siblings reachable via the nav rail.
- **Why**: User-driven UX correction. Appearance is a low-traffic settings area (theme / density / locale). The high-traffic surfaces under Preferences — email, password, MFA, default group, profile — all live in Account, which is what most users open Preferences to find or change.
- **Approved by**: user (explicit) — issue #1888 ("Personal Preferences: default landing tab should be Account, not Appearance").
- **Reversion plan**: Permanent unless the upstream mock flips its own default. The change is a single-line `useState` initialiser — trivial to reconcile if the mock catches up.

#### 2026-05-08 — "Migrate currency…" CTA + 4-step wizard dialog

- **Issue/PR**: #1553 / PR #1604
- **Mock**: [`design-mocks/src/views/GroupSettingsView.tsx`](../../design-mocks/src/views/GroupSettingsView.tsx) shows "Default currency" as a single `<CurrencyCombobox>` row with a "Save changes" button. No reprice/migrate button, no wizard dialog.
- **Reality**: The currency input is read-only (immutable per BE contract since #1550) and gains an outlined "Migrate currency…" button to its right (admins only, disabled while a migration is in flight). Clicking opens `MigrateCurrencyDialog` — a 4-step wizard (target → rate → preview → confirm) built on shadcn/ui `Dialog` + the existing `CurrencyCombobox`. Step indicator follows the `WizardSteps` pattern from `ExportNewPage`, primitives all live in [`UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx).
- **Why**: Not present in mock. The currency-migration feature is the entire point of issue #1553; the mock predates epic #202. Reused the export wizard's step layout for visual coherence inside the app rather than inventing a new wizard chrome.
- **Approved by**: user (explicit) — issue #1553 §"MigrateCurrencyDialog wizard" §5.2 spells out the four steps and the components to use.
- **Reversion plan**: Permanent. Reconcile if the upstream mock gains a `MigrateCurrencyView` or similar.

#### 2026-05-12 — Group settings keeps submenu-split while adopting mock's chevron-row pattern within sections

- **Issue/PR**: #1537 / PR #1649
- **Mock**: [`design-mocks/src/views/GroupSettingsView.tsx`](../../design-mocks/src/views/GroupSettingsView.tsx) ships a single flat page. The "Data" card (lines 134–156) wraps a `divide-y` block where each row is a `<button>` with `<Icon> <label> <ChevronRight>` — used for Members + Backup links side by side under one heading. The mock also surfaces Plan + Notifications cards as siblings, plus a Group identity card with `name + description` (no submenu, no leave-group, no storage usage).
- **Reality**: `frontend/src/pages/groups/GroupSettingsPage.tsx` keeps the four-section submenu split shipped in PR #1637 (Info / Members / Data & Storage / Management), but adopts the mock's chevron-right divide-y row pattern **inside** the existing sections rather than consolidating: the Members section renders the members link as one row in a divide-y card (leave-group panel stays as its own card below); the Data section renders the Export-data link as one row in a divide-y card with `StorageCard` retained below. Plan / Notifications cards are not rendered yet — only `// TODO(#1389)` / `// TODO(#1648)` comment-stubs sit at the top of `GroupSettingsBody`. Description field is similarly stubbed in InfoSection (`// TODO(#1647)`).
- **Why**: The submenu split is a deliberate design decision from PR #1637 (mirrors the user Preferences sub-navigation pattern + relocates the per-group Storage + Export surfaces here). Consolidating back to the mock's flat layout would walk back that decision; the better trade is to keep the section shell and adopt the mock's row-level visual language inside each section. Plan + Notifications + group description are BE-blocked (per-group plan / quota in #1389, per-group notification prefs in #1648, `description` column on LocationGroup in #1647), so the in-source `TODO`s mark the slots where those cards land once the BE arrives without inventing inert UI today.
- **Approved by**: user (explicit) — direct request to keep the submenu split (#1637) and adopt mock's chevron-row pattern within sections, with BE-blocked surfaces parked behind tracker TODOs.
- **Reversion plan**: As #1389 / #1648 / #1647 land, fill in the stubbed cards in-place (Plan card before InfoSection's form, Notifications card as its own section or alongside Info, description Textarea below the icon picker). The chevron-row pattern stays.

#### 2026-05-09 — Group settings split into Info / Members / Data & Storage / Management sub-sections

- **Mock**: [`design-mocks/src/views/GroupSettingsView.tsx`](../../design-mocks/src/views/GroupSettingsView.tsx) is a single flat page: header, then "Plan" / "Group" / "Notifications" / "Data" (members + backup links) / "Danger zone" cards stacked vertically, no sub-navigation.
- **Reality**: `frontend/src/pages/groups/GroupSettingsPage.tsx` now uses the same two-pane shell as the user Preferences page (`SettingsPage`): a left rail (Info / Members / Data & Storage / Management) + a right content pane that swaps in one section at a time. Each section owns its own card stack — Info has the identity form + currency migration, Members has the members shortcut + leave-group, Data & Storage has `<StorageCard />` + the Export-data CTA (relocated from user Preferences, where they were group-scoped surfaces masquerading as personal ones), Management has the delete-group danger zone.
- **Why**: The mock predates the storage + exports surfaces and predates the Preferences sub-navigation pattern that ships in `SettingsPage`. Keeping the group page flat while the personal page is sectioned would diverge the two settings hubs from each other for no design payoff. User explicitly asked for the same pattern ("таким же образом, как это сделано в Preferences"). Storage usage and Export data are per-group, so they belong here, not on `/settings`.
- **Approved by**: user (explicit) — directly requested the four-section layout (Info / Members / Data & Storage / Management) and the relocation of Storage + Export.
- **Reversion plan**: Permanent until/unless the upstream mock adopts the sub-navigation pattern across both settings surfaces. Reconcile if a future mock revision aligns the two.

#### 2026-05-12 — Notification preferences live on the `settings` table, not a dedicated `notification_preferences` table

- **Issue/PR**: #1373 (rolled up under #1536) / PR-A on the way (sub-issue #1643)
- **Spec**: [#1373](https://github.com/denisvmedia/inventario/issues/1373) proposes a structured `notification_preferences (user_id, category, channel, enabled)` table with a unique index per row.
- **Reality**: The new toggles persist as individual rows on the existing `settings` table (one row per `(tenant_id, user_id, name)`), using new `SettingName` constants under the `notifications.*` namespace. The structured-table approach was abandoned in favour of reusing the already-shipped, RLS-protected, JSONB-backed settings store. Defaults stay in code (`go/services/notifications/preferences.go`), so adding a new category never needs a backfill.
- **Why**: A separate table would duplicate RLS policies, registries, migrations, and indexing for a payload that is structurally `(string, bool)`. The `settings` table already supports exactly that shape (`name` → JSONB `value`). The only thing we trade is the ability to query "all users that disabled X" without a full scan — a future need we'll address with a GIN index if it materialises.
- **Approved by**: user (explicit) — confirmed during PR-A planning when picking "full BE+FE implementation".
- **Reversion plan**: If the cross-user query pattern shows up, move just the notification rows into a structured table via a one-time data migration; the SettingsObject pointer fields can stay (mark them deprecated) until the FE catches up.

#### 2026-05-16 — `Data & Storage` section is on Group Settings, not user Preferences

- **Issue/PR**: #1536 item 3 / closure PR (relocation shipped earlier in PR #1637 + PR-A sub-issue #1643)
- **Mock**: [`design-mocks/src/views/SettingsView.tsx`](../../design-mocks/src/views/SettingsView.tsx) L26-L33 lists `data` as the fifth nav entry inside the per-user Preferences left rail; the `DataSection` component (L402-L451) renders storage progress + automatic-backup / cross-device-sync toggles + Export-data (JSON / CSV) buttons as a personal-settings card.
- **Reality**: `frontend/src/pages/SettingsPage.tsx` `SECTIONS` ships five entries — `account / appearance / notifications / privacy / help` — with no `data` entry. The Storage card (`<StorageCard />`) and the Export-data CTA live on `GroupSettingsPage` instead, behind the group-scoped `/g/{slug}/settings#data` route. Backup automation and cross-device sync are not implemented (no BE).
- **Why**: Storage usage and Export are strictly per-group (RLS-scoped); surfacing them on a per-user page meant rendering against an implicit "active group" fallback. Relocating them to `GroupSettingsPage` (PR #1637) keeps the data-scope contract honest and consolidates per-group surfaces in one place — matches the 2026-05-09 + 2026-05-12 Group Settings deviation entries above. Automatic backup + cross-device sync stay deferred until the plan / quota model lands (#1389) and a real backup automation feature is scoped — the mock's standalone toggles would be inert today.
- **Approved by**: user (explicit) — selected the Data-section relocation in the #1643 PR-A scope question and confirmed during the #1637 group-settings refactor.
- **Reversion plan**: Permanent for the relocation. If automatic backup / cross-device sync ship in the future, they belong on `GroupSettingsPage` for the same RLS / per-group-scope reason, not in the user Preferences left rail.

#### 2026-05-12 — User-level "Preferred currency" selector is display-only (not yet wired to formatting)

- **Issue/PR**: #1536 item 4 / PR-A (sub-issue #1643)
- **Mock**: [`design-mocks/src/views/SettingsView.tsx`](../../design-mocks/src/views/SettingsView.tsx) `AppearanceSection` renders a "Currency" row with `<CurrencyCombobox>` and the description "Used for item values throughout the app".
- **Reality**: `SettingsPage` exposes the row + persists the choice to `appearance.preferred_display_currency` on the user's settings, but the value is NOT consumed by commodity / total / export formatting yet — those continue to use the per-group currency. The help text in the row reflects this: "Stored values on items keep their per-group currency — this only affects display formatting once wired (follow-up)."
- **Why**: The product's commodity-currency model is per-group and immutable post-creation (#1550 / #202). A per-user display override needs (a) a conversion path or (b) a "display-as" override that is purely cosmetic; both are non-trivial and out of PR-A scope. The selector ships now so the design-audit row exists and the storage shape is decided before any UI wires up to it.
- **Approved by**: user (explicit) — selected "include Currency selector" in the PR-A scope question, with the explicit understanding that wiring is deferred.
- **Reversion plan**: Either remove the row (if the per-user override semantics aren't worth the maintenance) or write the wiring follow-up (track as its own issue when the BE/FE team picks it up).

#### 2026-05-12 — Help & Support "Contact support" row goes to a `mailto:` while a real ticketing surface is scoped

- **Issue/PR**: #1536 item 5 / PR-A (sub-issue #1643)
- **Mock**: [`design-mocks/src/views/SettingsView.tsx`](../../design-mocks/src/views/SettingsView.tsx) `HelpSection` ships a "Contact support" row with chevron — destination not specified by the mock.
- **Reality**: Renders the row as a `mailto:support@inventario.app` external link. The other rows stay in-app routes; the support row is the only `<a>` in the section.
- **Why**: A real ticketing system / contact form is a larger surface. `mailto:` is the lowest-friction stand-in that doesn't depend on an in-app destination that doesn't exist yet.
- **Approved by**: agent-suggested.
- **Reversion plan**: Swap the href for a real `/support` route or a dialog when the support surface lands.
- **Resolution**: Resolved 2026-06-06 by #1387 / PR #2014 — the static `mailto:` row was folded into the in-app feedback entry point. The "Send feedback" row was relabelled "Contact support / share feedback" and now opens the existing `FeedbackDialog` (whose "Question" type doubles as the support channel); the `mailto:` row, its i18n keys, and the `HelpSection` external-link (`<a>`) render arm were removed. No `<a>`/`mailto` remains in the section.

#### 2026-05-08 — "Currency migrations" history list inside Danger Zone

- **Issue/PR**: #1553 / PR #1604
- **Mock**: [`GroupSettingsView.tsx`](../../design-mocks/src/views/GroupSettingsView.tsx) Danger Zone contains a single "Delete group" button.
- **Reality**: Danger Zone gains a second sub-section under a thin top divider: "Currency migrations" — a paginated list (server-capped at latest 10) showing per-row `from → to` + rate + timestamps + status pill. Empty state, loading skeleton, and the row layout follow the existing `RestoreHistoryList` (`frontend/src/components/exports/RestoreHistoryList.tsx`) — same shadcn/ui `Card`-less border + `divide-y` rhythm; same `Skeleton` + empty-state copy pattern.
- **Why**: Not present in mock. Issue #1553 §"Group settings" requires the history surface; we picked the existing restores list as the closest mock-aligned pattern (since `RestoreHistoryList` itself ships in production today against `BackupView`). No undo affordance per spec.
- **Approved by**: user (explicit) — issue #1553 §"Group settings" §5.1 names the placement and the row content.
- **Reversion plan**: Permanent until the design team explicitly adds a history pattern; if it lands, this list adopts the new chrome.

#### 2026-11-24 — Help & Support "What's new" badge is hidden on builds without a release number

- **Issue/PR**: PR #2790
- **Mock**: [`design-mocks/src/views/SettingsView.tsx`](../../design-mocks/src/views/SettingsView.tsx) `HelpSection` always shows a version badge (`v1.2`) on the "What's new" row.
- **Reality**: The badge shows `v<Major.Minor>` of the version the build was given (`v0.2.2` shows `v0.2`). For a build with no release number, such as `dev` (a local build), `edge` (the master image) or `pr-123` (a PR image), the row renders without the badge.
- **Why**: The version now comes from the build (PR #2790), and those builds have no release number to show. Before, every build showed the `version` from `frontend/package.json`, which no release bumped, so every release showed `v0.1`. A made-up number or `vpr` would mislead the same way.
- **Approved by**: agent-suggested-then-user-confirmed — suggested in PR #2790, confirmed by the user on 2026-11-24.
- **Reversion plan**: Permanent. If the mock gains a state for non-release builds, follow it.

#### 2026-11-24 — Help & Support gains a "Restart product tour" row

- **Issue/PR**: #2787 / PR #2816
- **Mock**: [`design-mocks/src/views/SettingsView.tsx`](../../design-mocks/src/views/SettingsView.tsx) `HelpSection` lists Documentation, Keyboard shortcuts, What's new, Send feedback and Contact support. It has no tour entry; the mock's only restart control is the `RestartTourButton` in the top bar of [`design-mocks/src/App.tsx`](../../design-mocks/src/App.tsx).
- **Reality**: `HelpSection` in `frontend/src/pages/SettingsPage.tsx` has a "Restart product tour" row after Documentation, with the chrome of the other rows (label, description, arrow). It calls the same restart as the user-menu entry, through `useOnboardingTourControls()`. While the tour cannot start, the row is a disabled button and its description says why: no active group, or a window below the mobile breakpoint.
- **Why**: Not present in mock. Before #2787 the only way back into the tour was the user menu, and nothing pointed to it. The row reuses the existing Help row pattern instead of a new primitive.
- **Approved by**: user (explicit) — the Help entry is one of the decisions for #2787.
- **Reversion plan**: Permanent unless the mock adds its own tour entry; follow the mock then.

#### 2026-11-25 — Settings → Notifications gains an "Item reminders" row

- **Issue/PR**: #2805
- **Mock**: [`SettingsView.tsx`](../../design-mocks/src/views/SettingsView.tsx) `NotificationsSection` lists two rows under **Reminders**: Warranty expiry and Maintenance reminders.
- **Reality**: The **Reminders** subgroup has a fourth row, **Item reminders** (`data-testid="notification-row-item-reminder"`), after Loan reminders. Same row anatomy and Switch as its neighbors; default on.
- **Why**: Item reminders (#2805) send their own emails, and every reminder category has an opt-out toggle in this section.
- **Approved by**: user (explicit) — pre-approved for #2805 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Permanent; reconcile if the upstream mock adds the row.

#### 2026-11-25 — Settings → Notifications: "Days before" field under three reminder rows

- **Issue/PR**: #2805
- **Mock**: [`SettingsView.tsx`](../../design-mocks/src/views/SettingsView.tsx) `NotificationsSection` rows are a label, a description and a Switch. The Warranty expiry description states fixed lead times ("60, 30, and 7 days before").
- **Reality**: The **Warranty expiry**, **Maintenance reminders** and **Loan reminders** rows show a **Days before** field under the description (`LeadTimeField` in `frontend/src/components/settings/LeadTimeField.tsx`, `data-testid="lead-times-{warranty,maintenance,loan}"`): a small label, an `h-8` Input holding comma-separated days, a link-style **Use default (…)** button while a custom value is stored, and a hint line that turns into the validation message. It saves on blur or Enter and is disabled while the row's Switch is off. The Warranty expiry description no longer lists fixed days, and the Maintenance description mentions the overdue email.
- **Why**: Not present in mock. Lead times became per-user settings in #2805. The field reuses the Input, Label and link Button from the UIShowcase forms section and keeps the row's existing anatomy, so the Switch stays in place.
- **Approved by**: user (explicit) — pre-approved for #2805 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Permanent; reconcile if the upstream mock adds lead-time controls.

### Navigation & App shell

#### 2026-11-25 — Saved views section in the sidebar

- **Issue/PR**: #2793 / #2839
- **Mock**: [`design-mocks/src/components/AppSidebar.tsx`](../../design-mocks/src/components/AppSidebar.tsx) has fixed groups (Inventory, Group, Reports, Personal) and no per-row counts.
- **Reality**: `frontend/src/features/saved-views/SidebarSavedViews.tsx` adds a **Saved views** `SidebarGroup` between Inventory and Group, listing the views the member pinned to their own sidebar (placement is per member; sharing pins a view for nobody else) in the group's order, each with its icon and a `SidebarMenuBadge` count (the shadcn primitive the mock vendors in [`design-mocks/src/components/ui/sidebar.tsx`](../../design-mocks/src/components/ui/sidebar.tsx) but does not use). A `SidebarGroupAction` gear opens the saved views page. The row is active while the items list shows that view. A view whose stored query cannot run is an `aria-disabled` row with a warning icon instead of a link. The group is left out while no view is pinned.
- **Why**: not present in mock. #2793 asks for views in the sidebar with a count that matches the opened list.
- **Approved by**: user (explicit) — pre-approved for #2793 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` draws saved views in its sidebar.

#### 2026-11-25 — Item types entry in the sidebar and the command palette

- **Issue/PR**: #2802 / PR #2863
- **Mock**: [`design-mocks/src/components/AppSidebar.tsx`](../../design-mocks/src/components/AppSidebar.tsx) lists Tags then Files (L66–L67), and [`design-mocks/src/components/CommandPalette.tsx`](../../design-mocks/src/components/CommandPalette.tsx) has no Item types entry.
- **Reality**: `frontend/src/components/AppSidebar.tsx` adds **Item types** (`Shapes` icon) after Sellers in the Group section, linking to `/g/:slug/item-types`. `CommandPalette.tsx` lists the same entry after Sellers. Both resolve the label through `lib/nav-labels.ts` (`common:nav.itemTypes`).
- **Why**: not present in mock. #2802 asks for a page that manages the group's item types.
- **Approved by**: user (explicit) — pre-approved for #2802 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` adds an item types view.

#### 2026-11-25 — Sellers entry in the sidebar and the command palette

- **Issue/PR**: #2804 / PR #2844
- **Mock**: [`design-mocks/src/components/AppSidebar.tsx`](../../design-mocks/src/components/AppSidebar.tsx) lists Tags then Files (L66–L67), and [`design-mocks/src/components/CommandPalette.tsx`](../../design-mocks/src/components/CommandPalette.tsx) has no Sellers entry.
- **Reality**: `frontend/src/components/AppSidebar.tsx` adds **Sellers** (`Store` icon) after Tags in the Group section, linking to `/g/:slug/sellers`. `CommandPalette.tsx` lists the same entry after Tags. Both resolve the label through `lib/nav-labels.ts` (`common:nav.sellers`).
- **Why**: not present in mock. #2804 asks for a page listing the sellers.
- **Approved by**: user (explicit) — pre-approved for #2804 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Move the row if `inventario-design` places sellers elsewhere.

#### 2026-11-25 — Trash entry in the sidebar and the command palette

- **Issue/PR**: #2799 / PR #2835
- **Mock**: [`design-mocks/src/components/AppSidebar.tsx`](../../design-mocks/src/components/AppSidebar.tsx) and [`design-mocks/src/components/CommandPalette.tsx`](../../design-mocks/src/components/CommandPalette.tsx) have no Trash entry.
- **Reality**: `frontend/src/components/AppSidebar.tsx` adds **Trash** (`Trash2` icon) as the last row of the Group section, after Settings, linking to `/g/:slug/trash`. `CommandPalette.tsx` lists the same entry after Settings. Both resolve the label through `lib/nav-labels.ts` (`common:nav.trash`).
- **Why**: not present in mock. The issue asks for a Trash page in the sidebar.
- **Approved by**: user (explicit) — pre-approved for #2799 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Move the row if `inventario-design` places Trash elsewhere.

#### 2026-05-08 — Persistent "currency migration in progress" banner in Shell

- **Issue/PR**: #1553 / PR #1604
- **Mock**: The mock has no app-shell banner pattern beyond the existing pending-invites banner (rendered as `InviteBanner` in `frontend/src/components/InviteBanner.tsx`).
- **Reality**: A new `CurrencyMigrationBanner` mounts directly under `TopBar` in `frontend/src/app/Shell.tsx`. It reads the active group's `currency_migration_id` from `GroupContext`; when set, an amber strip surfaces "Currency migration in progress for {group}." with a small spinning loader. No dismiss affordance — the banner is the lock indicator and must stay until the worker terminates the migration.
- **Why**: Not present in mock. The lock UX (issue #1553 §5.4) requires a persistent surface so the user understands why commodity / restore CTAs across the app are disabled. Patterned on `InviteBanner` (same `flex items-center gap-3 border-b px-4 py-2.5` chrome, same role="status") so it slots into the shell rhythm without inventing a new banner system.
- **Approved by**: user (explicit) — issue #1553 §5.4 calls for "persistent banner at top of layout".
- **Reversion plan**: Permanent until the upstream mock adopts a richer banner taxonomy; if it does, this banner adopts the new chrome.

#### 2026-05-08 — Lock-state disabled CTAs across commodity + restore surfaces

- **Issue/PR**: #1553 / PR #1604
- **Mock**: The mock has no concept of a per-group lock; all commodity write CTAs (Add/Edit/Delete/Bulk-move/Bulk-delete/Status-transition) and the export-restore Start CTA always render enabled.
- **Reality**: When the active group has `currency_migration_id` set, those CTAs render disabled with a `title={t("errors:lockedDuringMigration")}` tooltip and `aria-disabled` set. The disabling reads from a single `useGroupMigrationLock()` selector that wraps `useOptionalCurrentGroup()`; the disabled state itself uses each component's existing `disabled` prop (button/link Button asChild) — no new visual treatment is introduced.
- **Why**: Not present in mock. Required by issue #1553 §5.4 to keep the BE 423 from surfacing as an unexplained failure. We chose disabling over hiding to preserve the user's mental model of the page (everything's still there, just paused).
- **Approved by**: user (explicit) — issue #1553 §5.4.
- **Reversion plan**: Permanent. The lock is BE-driven and non-negotiable while a migration runs.

#### 2026-05-20 — Admin layout shell: breadcrumb + secondary-nav strip

- **Issue/PR**: #1752
- **Mock**: The admin surface mocks ([`design-mocks/src/views/admin/`](../../design-mocks/src/views/admin/) — `TenantsView`, `TenantDetailView`, `UserDetailView`, `GroupsView`, `GroupDetailView`, `admin-shared.tsx`) have **no layout shell**: each admin view is self-contained, navigates via a state string in the mock's `App.tsx`, and uses an `AdminBackButton` (ArrowLeft ghost button) for back-navigation. There is no breadcrumb and no secondary nav anywhere in the admin mock.
- **Reality**: `frontend/src/pages/admin/AdminLayout.tsx` is a real layout route that supplies a breadcrumb (`Admin → <section>`, reusing the shared `LocationsBreadcrumb` primitive) and a secondary-nav strip (Tenants / Groups underline-tab pills) with the section pages rendered through `<Outlet />`. The strip carries no `Users` pill — there is no cross-tenant user list; users are reached through a tenant (the tenant-detail Users tab) and the per-user page lives at `/admin/users/:id`.
- **Why**: Not present in mock. The frontend uses `react-router-dom`, not the mock's `view`-state machine — multiple admin pages need a shared chrome and a way to move between sub-sections, which issue #1752 explicitly mandates ("layout shell — breadcrumbs, secondary nav"). The layout reuses existing design-language tokens (overline breadcrumb, `border-b-2` active-tab treatment) and the existing `LocationsBreadcrumb` component rather than inventing new chrome.
- **Approved by**: agent-suggested-then-user-confirmed — issue #1752 spec calls for the layout shell directly.
- **Reversion plan**: Permanent until/unless the upstream design adds an admin layout; if it does, this layout adopts the mock's chrome.

#### 2026-05-20 — Admin sidebar section is a single top-level link

- **Issue/PR**: #1752 / PR #1773
- **Mock**: [`design-mocks/src/components/AppSidebar.tsx`](../../design-mocks/src/components/AppSidebar.tsx) renders an `Admin` `SidebarGroup` with **two** entries — `Tenants` and `Groups` — both always visible (the mock has no auth/permission model).
- **Reality**: `frontend/src/components/AppSidebar.tsx` renders the `Admin` section with a **single** top-level entry → `/admin/tenants`, and only when `useIsSystemAdmin()` is true. Sub-section navigation (Groups) lives in the `AdminLayout` secondary nav instead of the sidebar.
- **Why**: Issue #1752 specifies "a new 'ADMIN' section … Top-level link → `/admin/tenants`" — one sidebar entry, with the AdminLayout owning sub-navigation. The conditional render is required because the mock has no permission gating; non-admins must not see the section.
- **Approved by**: agent-suggested-then-user-confirmed — issue #1752 spec dictates both the single-link shape and the conditional render.
- **Reversion plan**: Permanent — the single-link + secondary-nav split is the intended IA for the React app.

#### 2026-11-24 — Onboarding tour skips steps without a target and waits for a group

- **Issue/PR**: #2787 / PR #2816
- **Mock**: [`design-mocks/src/components/OnboardingTour.tsx`](../../design-mocks/src/components/OnboardingTour.tsx) always walks all seven steps. A step whose `data-tour` target is missing falls back to a centered card over a full-screen dim. The mock has no notion of a user without a group or of a breakpoint.
- **Reality**: `frontend/src/components/OnboardingTour.tsx` shows only the steps whose target is in the DOM (the centered welcome card needs none) and counts only those in its dots and progress bar. When no step with a target is left, it renders nothing and closes without marking the tour as seen. `frontend/src/app/Shell.tsx` does not auto-launch the tour without an active group or below the mobile breakpoint, and leaves "Restart product tour" out of the user menu in both cases.
- **Why**: Without a group the sidebar has no Add item button and no group sections, so every step fell back to a centered card about an element the user could not see, and finishing that tour marked it as seen before the user had a group (#2787). Below the breakpoint the sidebar is a closed sheet, which is the same situation.
- **Approved by**: user (explicit) for skipping steps and for launching once a group is active (#2787 decisions), and for the narrow-screen gate with the user-menu entry left out (2026-11-24, after #2816 merged: a guided tour of a collapsed sidebar on a phone has no good shape).
- **Reversion plan**: Permanent for the no-group behavior. If the tour gains a mobile variant (for example one that opens the sidebar sheet first), drop the narrow-screen gate.

#### 2026-11-25 — TopBar search entry on narrow screens

- **Issue/PR**: #2801 / PR (pending)
- **Mock**: The header in [`design-mocks/src/App.tsx`](../../design-mocks/src/App.tsx) has a "Search…" button with a `⌘ K` `KbdGroup` and an "Open command palette" tooltip. It is `hidden sm:flex`, so below the `sm` breakpoint the header has no way into the palette.
- **Reality**: `frontend/src/components/HeaderSearch.tsx` renders the mock's button from `sm` up. Below `sm` it renders an outline icon `Button` (`size="icon"`, `Search` icon, accessible name "Search") in the same place, so phones and touch devices can open the palette.
- **Why**: The issue asks for a search entry that works by click and touch, on narrow screens too. The mock's button does not fit next to the group switcher and the toggles at phone width.
- **Approved by**: user (explicit) — pre-approved for #2801 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` gives the header a narrow-screen search entry.

#### 2026-11-25 — TopBar search entry: shortcut label and focus

- **Issue/PR**: #2801 / PR (pending)
- **Mock**: The header search button in [`design-mocks/src/App.tsx`](../../design-mocks/src/App.tsx) always shows `⌘` and has no focus style.
- **Reality**: `HeaderSearch` shows `⌘ K` on Apple platforms and `Ctrl K` elsewhere (the same detection the shortcuts dialog uses), sets `aria-keyshortcuts`, and has the `focus-visible` ring of the other outline controls. Closing the palette returns focus to the entry that opened it.
- **Why**: The shortcut is Ctrl+K on Windows and Linux, so `⌘` there names a key the keyboard does not have. The issue asks for a keyboard-accessible entry, and a control without a focus style is not.
- **Approved by**: agent-suggested — listed in the PR for confirmation
- **Reversion plan**: Permanent unless the mock adopts its own platform label and focus style.

#### 2026-11-25 — Command palette lists search results by kind

- **Issue/PR**: #2801 / PR (pending)
- **Mock**: [`design-mocks/src/components/CommandPalette.tsx`](../../design-mocks/src/components/CommandPalette.tsx) shows fixed groups (Navigate, Recent Items, Recent Files, Locations) filtered on the client by cmdk; item and file rows end in a category `Badge`.
- **Reality**: `frontend/src/components/CommandPalette.tsx` searches the active group through `GET /search` from two characters on. Under the "Search …" row that opens the search page it lists Items, Locations, Areas, Files and Tags, each only when it has matches, then the Navigation group. Rows keep the mock's anatomy (`size-7` muted icon box, medium title, muted `text-xs` subtitle); the subtitle carries the matched field, the asset ID, the address, the path or the tag kind instead of a category badge. The Items group ends in an "All matching items (N)" row that opens the items list with the same query. Area and tag rows have a second option, "Show items" (or "Show files" for a file tag), next to the row. With an empty query a Recent group lists the results opened from the palette or the search page. A status line under the input says to type more, that the search runs, or that nothing matched.
- **Why**: The issue asks for results grouped by kind, a "show its items" action for tags and areas, recent results, and a way to all results. Locations get no "show items" action because the item list has no location filter yet.
- **Approved by**: user (explicit) — pre-approved for #2801 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock if `inventario-design` draws a palette with live results.

#### 2026-11-25 — Search page: tags group, item links and recent results

- **Issue/PR**: #2801 / PR (pending)
- **Mock**: The search page is not in the mock. Its result rows follow the [`CommandPalette`](../../design-mocks/src/components/CommandPalette.tsx) title-and-subtitle row.
- **Reality**: `frontend/src/pages/SearchPage.tsx` lists matching tags (label, and "Item tag" or "File tag") instead of the "not available yet" card. Area and tag cards carry a ghost "Show items" (or "Show files") button at their right edge that opens the item or file list filtered by that area or tag; those cards drop the chevron. Opening a result records it, so the recent list on the empty page fills, and each recent row names its kind (Item, Location, Area, File, Tag).
- **Why**: not present in mock. The issue asks for the same grouping and "show its items" action as the palette, and the page's recent list stayed empty because nothing recorded a visit.
- **Approved by**: user (explicit) — pre-approved for #2801 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` adds a search page.

#### 2026-11-25 — "Upcoming" sidebar entry

- **Issue/PR**: #2805
- **Mock**: [`design-mocks/src/components/AppSidebar.tsx`](../../design-mocks/src/components/AppSidebar.tsx) has no Upcoming entry.
- **Reality**: The **Inventory** group gains **Upcoming** (lucide `BellRing`) between **All Items** and **Warranties**, linking to `/g/:slug/upcoming`.
- **Why**: Not present in mock. The Upcoming page is the group-wide list the issue asks for; it sits next to the per-kind pages it merges.
- **Approved by**: user (explicit) — pre-approved for #2805 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Reconcile when the upstream mock adds the entry.

### i18n & Formatting

#### 2026-05-16 — Dashboard hero: K/M/B compact-notation fallback at ≥1e7

- **Issue/PR**: #1684 / PR (this change)
- **Mock**: [`design-mocks/src/views/DashboardView.tsx`](../../design-mocks/src/views/DashboardView.tsx) `formatCurrency()` uses `Intl.NumberFormat(..., { maximumFractionDigits: 0 })` only — full digit groups regardless of magnitude. Hard-coded to `en-US` + USD in the mock; doesn't model low-denomination currencies.
- **Reality**: `frontend/src/lib/intl.ts`'s `formatCurrency` now accepts `notation?: "standard" | "compact"`. `frontend/src/pages/Dashboard.tsx`'s hero call site switches to `notation: "compact"` once `data.totalValue >= 1e7`, otherwise keeps `compact: true` (no cents). So `$329,849` reads the same as it did post-#1678 for a typical group, while a HUF 100,000,000 inventory renders as "HUF 100M" instead of clipping. Per-item / detail / list surfaces never see compact-notation — they keep full precision via the default path.
- **Why**: PR #1678 dropped cents (`compact: true`), which handles six-figure totals fine but still clips at 8–9 digits — common in HUF / IDR / VND / KRW / IRR. The threshold-based hybrid (#1684 "Option 2") keeps the current production reading for the vast majority of groups and only kicks in at the edges where width pressure is otherwise unsolvable without truncation or a separate font-size hack.
- **Approved by**: user (explicit) — issue spec walked through the two options and noted Option 2 preserves the current "$329,849" reading; pre-authorized via the goal-message instruction to implement #1684.
- **Reversion plan**: If a future hero design reflows to give the totals card more horizontal room (full-width hero strip), or if Intl ever ships a "narrow currency" mode that's locale-aware and width-aware, drop the threshold and revert to `compact: true` everywhere on the hero. The `notation` option on `formatCurrency` stays — its existence is orthogonal to whether the dashboard uses it.

### Tables & Lists

#### 2026-11-25 — Trash page (`/g/:slug/trash`)

- **Issue/PR**: #2799 / PR #2835
- **Mock**: Not present in mock. `design-mocks/` has no Trash or recycle-bin view; the issue notes that it needs a mock in `inventario-design` or a recorded deviation. Fallback: [`design-mocks/src/views/UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) (Table in a bordered rounded container, line Tabs, Button, the empty-state block) and the shared `BulkActionBar`.
- **Reality**: `frontend/src/pages/trash/TrashPage.tsx` uses the standard `PageHeader` (title, a subtitle that states the retention in days from the list response) with a destructive **Empty trash** button for admins and owners. Line tabs split **Items (n)** and **Files (n)**. Each tab (`components/trash/TrashedItemsTab.tsx`, `TrashedFilesTab.tsx`) is a table with a selection checkbox, the name, the files that went to Trash with the item (count and size) or the places a file was attached to, the deletion date with the member who deleted it, the date it is deleted for good, and per-row **Restore** and **Delete now** buttons (both outline, Delete now in the destructive color; an outline button keeps its own background, so the red text keeps its contrast on a hovered row). Selecting rows opens the `BulkActionBar` with the same two actions. Viewers see the lists without actions; users restore; admins and owners also delete for good. Restoring an item whose area is gone shows a warning toast with an **Open** action. An empty tab shows the empty-state block with a `Trash2` icon.
- **Why**: not present in mock. The issue asks for restore, delete now and empty trash.
- **Approved by**: user (explicit) — pre-approved for #2799 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Reconcile when `inventario-design` adds a Trash view; the data layer in `features/trash/` stays.

#### 2026-05-20 — Admin Tenants list is a divide-y card list, not a `<Table>`

- **Issue/PR**: #1752 / PR #1773
- **Mock**: [`design-mocks/src/views/admin/TenantsView.tsx`](../../design-mocks/src/views/admin/TenantsView.tsx) renders the tenant list with the shadcn `<Table>` primitive (TableHeader/TableBody/TableRow/TableCell) and a shadcn `<Pagination>` control below it.
- **Reality**: `frontend/src/pages/admin/AdminTenantsPage.tsx` renders the tenant list as a `divide-y divide-border` card list of rows (the established frontend convention — see `LoginHistoryPage`, `SessionsPage`). The header card + stat tiles + search input + status-badge palette all match the mock. The search box is wired to the server: the debounced input feeds `?q` (the BE matches it against name/slug/domain) via `features/admin/api.ts` + `keys.ts`, so search stays correct across pages. The remaining pagination params (`page`/`per_page`/`sort`) are plumbed through the data layer but no `<Pagination>` UI ships in this foundation issue.
- **Why**: The frontend has **no `table.tsx` or `pagination.tsx` shadcn primitive** yet. Pulling two new primitives in a foundation issue widens the blast radius beyond the issue's scope; the `divide-y` card list is the pattern every other list page in `frontend/` already uses, so it's the lower-drift choice. The pagination data layer is ready for a later sub-issue to add the UI.
- **Approved by**: agent-suggested — foundation-issue scope decision; the visual rhythm (cards, tokens, badges) still matches the mock.
- **Reversion plan**: When a later admin sub-issue needs the full table/pagination treatment, add `table.tsx` + `pagination.tsx` via the shadcn CLI and port `TenantsView` 1:1. Resolve this entry then.
- **Resolved**: 2026-05-21, PR #1774 — back to 1:1. `#1753` added `frontend/src/components/ui/table.tsx` + `pagination.tsx` (ported from the mock's shadcn primitives) and rebuilt `AdminTenantsPage` as the mock's `<Table>` + `<Pagination>` layout. The list is now a sortable, server-paginated table matching `TenantsView.tsx`.

#### 2026-05-21 — Admin Tenants stat row shows one tile (Tenants), not four

- **Issue/PR**: #1753 / PR #1774
- **Mock**: [`design-mocks/src/views/admin/TenantsView.tsx`](../../design-mocks/src/views/admin/TenantsView.tsx) renders a 4-up stat grid above the table — Tenants, Active, Total users, Total groups — each computed by reducing over the full `MOCK_TENANTS` array.
- **Reality**: `frontend/src/pages/admin/AdminTenantsPage.tsx` renders a single "Tenants" tile, fed by the server-provided `meta.total`.
- **Why**: Backend constraint. The list endpoint is paginated — Active / Total users / Total groups cannot be computed correctly from one page, and the BE exposes no platform-wide aggregate endpoint. Showing a per-page sum would be misleading. The total-tenants count is the one figure the pagination envelope (`meta.total`) makes correct. (This carries forward the same decision the #1752 foundation took.)
- **Approved by**: agent-suggested — backend-shape constraint, same rationale as the #1752 foundation.
- **Reversion plan**: Restore the 4-up grid when a BE aggregate endpoint (platform-wide active-tenant / user / group totals) lands.

#### 2026-05-21 — Tenant detail header has no Plan-name lookup; row navigation replaces mock callbacks

- **Issue/PR**: #1753 / PR #1774
- **Mock**: [`design-mocks/src/views/admin/TenantDetailView.tsx`](../../design-mocks/src/views/admin/TenantDetailView.tsx) shows the Plan stat as a human label via `TENANT_PLAN_CONFIG[tenant.plan].label` ("Business", "Enterprise", …) and the Users tab carries a `Role` column + per-user avatar initials; navigation is via `onSelectUser` / `onSelectGroup` prop callbacks.
- **Reality**: `frontend/src/pages/admin/AdminTenantDetailPage.tsx` renders the raw `plan_id` string in the Plan tile, drops the Users-tab `Role` column and the avatar-initials chip, and navigates with `react-router-dom` (`/admin/users/{id}`, `/admin/groups/{id}`).
- **Why**: Backend shape. `jsonapi.AdminTenantListItem` carries `plan_id` (an opaque identifier), not a resolved plan name, and there is no plan-catalog endpoint to map it — so the raw id is shown until one exists. `jsonapi.AdminUserListItem` has **no `role` field** (role is per-group-membership, not tenant-wide) and **no avatar field**, so the Role column and initials chip are omitted rather than faked. Router navigation replacing the mock's prop callbacks is the standard mock→frontend translation, not a visual deviation.
- **Approved by**: agent-suggested — backend-shape constraints; the column omissions track missing data, not a design choice.
- **Reversion plan**: Restore the Plan label when a plan-catalog lookup ships; the Users `Role` column stays out unless the BE adds a tenant-scoped role to the user list item.

#### 2026-05-21 — Admin Tenants list / detail page naming follows the `Admin*Page` convention

- **Issue/PR**: #1753 / PR #1774
- **Mock**: The mock view files are `TenantsView.tsx` / `TenantDetailView.tsx`; issue #1753's text proposed `TenantsListPage.tsx` / `TenantDetailPage.tsx`.
- **Reality**: The list upgrades `AdminTenantsPage.tsx` in place and the detail page is `AdminTenantDetailPage.tsx`.
- **Why**: Code-organisation choice, not a visual deviation. The #1752 foundation established the `Admin*Page` convention (`AdminTenantsPage`, `AdminUsersPage`, `AdminGroupsPage`, `AdminLayout`, `AdminForbiddenPage`); the issue text predates that foundation. Logged here only for traceability — there is no visual drift.
- **Approved by**: agent-suggested — follows the established codebase convention.
- **Reversion plan**: Permanent.

#### 2026-05-21 — Admin Tenants list drops the mock's `Plan` column

- **Issue/PR**: #1753 / PR #1774
- **Mock**: [`design-mocks/src/views/admin/TenantsView.tsx`](../../design-mocks/src/views/admin/TenantsView.tsx) renders a `Plan` `TableHead` + per-row cell, showing the plan as a human label via `TENANT_PLAN_CONFIG[tenant.plan].label` ("Business", "Enterprise", …).
- **Reality**: `frontend/src/pages/admin/AdminTenantsPage.tsx` omits the `Plan` column entirely — the table is Name / Domain / Status / Users / Groups / Created.
- **Why**: Backend shape. `jsonapi.AdminTenantListItem` carries only `plan_id` — an opaque identifier — and there is no plan-catalog endpoint to resolve it to a name. The tenant **detail** tile has the same constraint (logged in "Tenant detail header has no Plan-name lookup" above), where it falls back to the raw id; the list column is a separate surface, so dropping it outright (rather than showing a column of opaque ids) is the lower-noise choice for a wide table. Logged separately from the detail-tile entry because it is its own surface.
- **Approved by**: agent-suggested — backend-shape constraint; same `plan_id`-is-opaque rationale as the detail tile.
- **Reversion plan**: Restore the `Plan` column when a plan-catalog lookup ships and `plan_id` can be resolved to a label.

#### 2026-05-21 — Admin Tenants list + detail add controls (search, filters, pagination, sortable headers) the mock lacks

- **Issue/PR**: #1753 / PR #1774
- **Mock**: [`design-mocks/src/views/admin/TenantDetailView.tsx`](../../design-mocks/src/views/admin/TenantDetailView.tsx) renders the tenant-detail Users / Groups tabs as plain static `<Table>`s — no search box, no filter dropdown, no pagination footer. [`design-mocks/src/views/admin/TenantsView.tsx`](../../design-mocks/src/views/admin/TenantsView.tsx) renders the tenant-list table with plain (non-sortable) column headers.
- **Reality**: `frontend/src/pages/admin/AdminTenantDetailPage.tsx` adds a debounced search box + tri-state `isActive` filter + pagination footer to the Users tab, a status filter + pagination footer to the Groups tab; `frontend/src/pages/admin/AdminTenantsPage.tsx` adds sortable column headers (Name / Status / Created) with `aria-sort`.
- **Why**: Issue #1753 mandates these controls — the issue text predates the mock, which was vendored before the admin surface grew its server-side list semantics. The lists are server-paginated/-searched/-sorted (the BE endpoints accept `?q`, `?sort`, `?order`, `?page`, `?per_page`, `?is_active`, `?status`), so the UI must surface the controls that drive those params. This entry exists so the doc is honest that the shipped admin tables are not 1:1 with the mock's static tables — the divergence is issue-mandated, not a taste call.
- **Approved by**: user (explicit) — issue #1753 acceptance criteria mandate the search / filter / sort / pagination controls.
- **Reversion plan**: Permanent — the controls are load-bearing for the server-side list endpoints. If the upstream mock adds equivalent controls, this entry is resolved by reconciling visual details.

#### 2026-05-21 — Tenant-detail Users / Groups rows link to `/admin/users/:id` and `/admin/groups/:id` (forward reference)

- **Issue/PR**: #1753 / PR #1774
- **Mock**: [`design-mocks/src/views/admin/TenantDetailView.tsx`](../../design-mocks/src/views/admin/TenantDetailView.tsx) drills into a user / group via `onSelectUser` / `onSelectGroup` prop callbacks (the mock is a `view`-state machine with no router).
- **Reality**: `frontend/src/pages/admin/AdminTenantDetailPage.tsx` navigates row clicks to `/admin/users/{userID}` and `/admin/groups/{groupID}` with `react-router-dom`. Those routes have **no element in `src/app/router.tsx` yet** — they are delivered by sibling sub-issues #1754 (admin user detail) and #1755 (admin group detail) of the same umbrella #1744, landing next in order. Until #1754 / #1755 merge, clicking such a row falls through the `/admin/*` subtree to the top-level `*` catch-all route and renders the standard `NotFoundPage` (`src/pages/NotFound.tsx`) — no crash, no blank screen.
- **Why**: Issue #1753 explicitly mandates these link targets; the route components are a deliberate forward reference to #1754 / #1755. The link targets are committed now so the detail page is feature-complete per its spec and the sibling issues only need to add the route entries.
- **Approved by**: user (explicit) — issue #1753 spec mandates the `/admin/users/:id` and `/admin/groups/:id` link targets.
- **Reversion plan**: Resolved when #1754 / #1755 add the `users/:userId` and `groups/:groupId` route entries under `/admin` in `router.tsx`; the links then resolve to real detail pages.

#### 2026-05-21 — Admin user detail Sessions section is a count summary, not a per-session table

- **Issue/PR**: #1754 / PR #1776
- **Mock**: [`design-mocks/src/views/admin/UserDetailView.tsx`](../../design-mocks/src/views/admin/UserDetailView.tsx) renders a Sessions section as a shadcn `<Table>` with one row per active session — columns Device, IP address, Location, Last active — driven by a per-session `user.sessions[]` array in `mock.ts`.
- **Reality**: `frontend/src/pages/admin/AdminUserDetailPage.tsx` renders the Sessions section as a single count summary — an icon-headed block reading "N active sessions" — with the centred icon + muted-text empty state when the count is 0. No per-session table.
- **Why**: Backend shape. `GET /api/v1/admin/users/{id}` (`jsonapi.AdminUserDetail`) returns only `active_session_count` — an integer derived from unrevoked refresh tokens — and exposes **no per-session detail** (no device / IP / location / last-active list). There is no admin endpoint that lists another user's sessions, so the table cannot be populated; rendering a count is the honest representation of the data the API provides.
- **Approved by**: agent-suggested-then-user-confirmed — the issue #1754 brief explicitly calls out this BE limitation and instructs the count-summary substitution plus this log entry.
- **Reversion plan**: If a future admin endpoint returns per-session detail for an arbitrary user, replace the count summary with the mock's `<Table>` layout (Device / IP / Location / Last active) and resolve this entry.

#### 2026-05-21 — Admin user detail block/unblock confirm uses `Dialog`, not `AlertDialog`

- **Issue/PR**: #1754 / PR #1776
- **Mock**: [`design-mocks/src/views/admin/UserDetailView.tsx`](../../design-mocks/src/views/admin/UserDetailView.tsx) wraps the block confirmation in the shadcn `<AlertDialog>` primitive (AlertDialogContent / AlertDialogAction / AlertDialogCancel).
- **Reality**: `frontend/src/pages/admin/AdminUserDetailPage.tsx` hosts the block/unblock confirmation in the generic shadcn `<Dialog>` primitive with a `<Textarea>` for the required `reason` and inline typed-error banner.
- **Why**: Two constraints. (1) The frontend has no `alert-dialog.tsx` primitive — only `dialog.tsx` is vendored; the established codebase confirmation pattern (`hooks/useConfirm.tsx`) deliberately uses `Dialog`, not `AlertDialog`. (2) The issue brief points at the `ConfirmProvider`/`useConfirm` pattern, but that provider only renders a title + description and cannot host a `<textarea>`; the required free-form `reason` field forces a bespoke dialog. Using `Dialog` keeps it consistent with the in-repo confirmation convention. Visual anatomy (header, description, footer, destructive confirm button) still matches the mock's confirmation shape.
- **Approved by**: agent-suggested-then-user-confirmed — the issue #1754 brief instructs the agent to build a dedicated dialog if `ConfirmProvider` cannot host a textarea and to explain the choice.
- **Reversion plan**: If `alert-dialog.tsx` is later vendored via the shadcn CLI, the confirmation can be ported to `<AlertDialog>` for closer 1:1 with the mock. The textarea-hosting requirement remains regardless.

#### 2026-05-21 — Admin user detail group memberships render as a link list, not a `<Table>`

- **Issue/PR**: #1754 / PR #1776
- **Mock**: [`design-mocks/src/views/admin/UserDetailView.tsx`](../../design-mocks/src/views/admin/UserDetailView.tsx) renders the user's group memberships as a shadcn `<Table>` — one row per group with column headers (Group, Role, Joined).
- **Reality**: `frontend/src/pages/admin/AdminUserDetailPage.tsx` renders group memberships as a vertical link list — each row is an icon-headed card (`<Link>` to `/admin/groups/{id}`, or a non-interactive `<div>` when `group_id` is absent) with the group name, relative join time, and a `RoleBadge`. No tabular header row.
- **Why**: Component-composition choice. The membership row's primary affordance is navigation to the group-detail page; a clickable link-list row communicates that affordance more directly than a `<Table>` row, and the section is a short, scannable list (a user typically belongs to a handful of groups) rather than a sortable/paginated dataset. The link-list also degrades gracefully when `group_id` is missing — the row simply becomes non-interactive — whereas a `<Table>` row would still look like tabular data. Visual anatomy (group name, role badge, join time) still matches the mock's information set.
- **Approved by**: agent-suggested-then-user-confirmed — flagged in the #1754 code review (N5) with an explicit instruction to log the deviation.
- **Reversion plan**: If the memberships section grows sorting / pagination / multi-column needs, port it to the shared admin `<Table>` primitives used by `AdminTenantsPage` / `AdminTenantDetailPage`.

#### 2026-05-21 — Admin user detail page naming follows the `Admin*Page` convention

- **Issue/PR**: #1754 / PR #1776
- **Mock**: The issue text proposes `UserDetailPage.tsx`; the mock view file is `design-mocks/src/views/admin/UserDetailView.tsx`.
- **Reality**: The page ships as `frontend/src/pages/admin/AdminUserDetailPage.tsx`.
- **Why**: Code-organisation choice, not a visual deviation. The #1752 foundation established the `Admin*Page` convention (`AdminTenantsPage`, `AdminTenantDetailPage`, `AdminUsersPage`, `AdminGroupsPage`, `AdminLayout`, `AdminForbiddenPage`); the issue text predates that foundation. Logged for traceability — there is no visual drift.
- **Approved by**: agent-suggested-then-user-confirmed — same rationale as the #1753 `AdminTenantDetailPage` naming entry above.
- **Reversion plan**: Permanent — the `Admin*Page` convention is the established pattern for this subtree.

#### 2026-05-21 — Admin Groups list / detail page naming follows the `Admin*Page` convention

- **Issue/PR**: #1755 / PR #1775
- **Mock**: [`design-mocks/src/views/admin/GroupsView.tsx`](../../design-mocks/src/views/admin/GroupsView.tsx) and [`GroupDetailView.tsx`](../../design-mocks/src/views/admin/GroupDetailView.tsx) are named `*View` per the mock's `view`-state-machine convention.
- **Reality**: `frontend/src/pages/admin/AdminGroupsPage.tsx` and `AdminGroupDetailPage.tsx` follow the `Admin*Page` naming the #1752 foundation established for this subtree (AdminTenantsPage / AdminTenantDetailPage / AdminUsersPage).
- **Why**: Codebase convention — `pages/admin/*` are routed React Router pages, not mock `view` states. Consistent with the prior admin entries in this log.
- **Approved by**: agent-suggested — same rationale as the #1753 AdminTenants naming entry.
- **Reversion plan**: Permanent; this is a frontend↔mock structural difference, not a visual one.

#### 2026-05-21 — Admin Groups list + detail add controls (search, filters, sortable headers, pagination) the mock lacks

- **Issue/PR**: #1755 / PR #1775
- **Mock**: `GroupsView.tsx` filters/paginates a client-side `MOCK_ADMIN_GROUPS` array (search + tenant Select + status Select + numbered pager, no sortable headers). `GroupDetailView.tsx` has no URL state.
- **Reality**: `AdminGroupsPage.tsx` drives every control server-side via `?q`, `?tenantID`, `?status`, `?sort`, `?order`, `?page` on the URL — debounced search, sortable column headers (name/slug/created_at/status, the BE-supported `?sort` set), windowed `AdminPagination`, and out-of-range page recovery. The mock's table has no sortable headers; the frontend adds the asc/desc affordance to match the rest of the admin surface (AdminTenantsPage).
- **Why**: Backend reality — `GET /admin/groups` is a real paginated/sortable/searchable endpoint, not a static array. URL-persisted state is an issue #1755 acceptance criterion. The sortable-header mechanism mirrors the canonical AdminTenantsPage pattern.
- **Approved by**: user (explicit) — issue #1755 spec mandates URL-persisted filters + sortable columns.
- **Reversion plan**: Permanent; the controls reflect the live BE contract.

#### 2026-05-21 — Admin Groups list adds a dedicated sortable Slug column

- **Issue/PR**: #1755 / PR #1775
- **Mock**: `GroupsView.tsx` shows six columns — Group, Tenant, Status, Currency, Members, Created — with the slug not surfaced as its own column (the mock row shows only the group name).
- **Reality**: `AdminGroupsPage.tsx` shows only the group name in the Group cell (matching the mock) AND adds a separate, sortable `Slug` column. Seven columns total — one more than the mock.
- **Why**: Issue #1755 explicitly requires sortable headers for the full BE-supported `?sort` set, which includes `slug`. A sortable header needs a column to attach to; the dedicated Slug column is the affordance for the `slug` sort.
- **Approved by**: agent-suggested — the issue mandates `slug` sortability without specifying the column; a dedicated column is the minimal, consistent way to expose it.
- **Reversion plan**: If `slug` sort is dropped from the BE or deemed redundant, remove the Slug column and the `slug` entry from the page's `SORTABLE` set; the Group cell stays name-only as in the mock.

#### 2026-05-21 — Admin Groups tenant filter is fetched as one large page (per_page=100)

- **Issue/PR**: #1755 / PR #1775
- **Mock**: `GroupsView.tsx` maps the in-memory `MOCK_TENANTS` array directly into the tenant-filter `<Select>` — no pagination, every tenant is always available.
- **Reality**: `AdminGroupsPage.tsx` populates the tenant-filter `<Select>` from `useAdminTenants({ page: 1, perPage: 100, sort: "name" })` — a single first page at the BE's `per_page` cap. A deployment with more than 100 tenants would not list tenants 101+ in the dropdown.
- **Why**: Backend reality — tenants come from a paginated endpoint. A searchable combobox would be the fully-correct fix but is heavier than #1755's scope; one 100-row page covers every realistic deployment today. A deep-linked `?tenantID` still filters correctly even when that tenant is absent from the dropdown options (the filter value round-trips through the URL independently of the option list).
- **Approved by**: agent-suggested — pragmatic scoping for #1755; flagged here for a future combobox upgrade.
- **Reversion plan**: Swap the `<Select>` for a `cmdk`-backed searchable combobox with server-side tenant search when a >100-tenant deployment appears, or when umbrella #1744 adds a shared admin tenant-picker.

#### 2026-05-21 — Admin Group detail: Members panel is a placeholder; soft-delete confirm uses `useConfirm`, not `AlertDialog`

- **Issue/PR**: #1755 / PR #1775
- **Mock**: `GroupDetailView.tsx` renders a full Members panel — add-member dialog, per-row role `<Select>`, remove-member `AlertDialog` — and the Danger-zone soft-delete confirm is a shadcn `AlertDialog`.
- **Reality**: `AdminGroupDetailPage.tsx` renders the Members panel as a clearly-labelled **placeholder** card (dashed border, `Users` glyph, "Member management ships in a follow-up update.") — the real add/remove/role editor is a later sub-issue of umbrella #1744, out of #1755's scope. The Danger-zone soft-delete confirm uses the codebase's `useConfirm()` primitive (a root-mounted `Dialog` exposing a promise-based `confirm()`), not a per-call `AlertDialog`.
- **Why**: (1) Members editor — scoped out of #1755 by the issue text; shipping a placeholder keeps the page coherent without pre-empting the follow-up. (2) `AlertDialog` — the frontend has **no `AlertDialog` primitive**; `frontend/src/hooks/useConfirm.tsx` is the established destructive-confirm pattern (see its own header comment) and is already mounted by Shell's `ConfirmProvider`. The confirm body still explains the two-phase async purge as the issue requires.
- **Approved by**: user (explicit) — issue #1755 spec mandates the Members placeholder and a soft-delete confirm dialog; the `useConfirm` substitution follows the documented codebase convention.
- **Reversion plan**: Members placeholder is replaced by the real editor in the follow-up sub-issue. The `useConfirm` choice is permanent unless the codebase adopts a shadcn `AlertDialog` primitive repo-wide.
- **Resolved**: 2026-05-21, PR #1783 — the Members placeholder is replaced by the real `<MembershipEditor>` in #1756 (see the entry below). The `useConfirm`-not-`AlertDialog` decision still stands.

#### 2026-05-21 — Admin Group detail: membership editor uses `useConfirm` for the remove confirm; add-member dialog resolves email → userID instead of free-text name+email

- **Issue/PR**: #1756 / PR #1783
- **Mock**: `GroupDetailView.tsx`'s Members section is a `<Table>` (Member / Role / actions) with an inline role `<Select>` per row, an "Add member" button, a per-row dropdown carrying "Remove from group", a remove-confirmation `AlertDialog`, and an Add-member `Dialog` with a **free-text Full name + Email** pair plus a role picker — the mock fabricates a member from whatever strings are typed.
- **Reality**: `frontend/src/components/admin/MembershipEditor.tsx` replicates the `<Table>` markup/classNames 1:1 (the inline role `<Select>`, the `Ellipsis` per-row dropdown, the "Add member" button, the icon-headed Add dialog). Two behavioural divergences: (1) the remove confirmation uses the codebase's `useConfirm()` primitive (a root-mounted `Dialog`), not a per-call shadcn `AlertDialog` — same rationale as the #1755 Danger-zone confirm. (2) The Add-member dialog has **no free-text name field**: the BE's `POST .../members` takes a resolved `userID`, so the dialog debounces a tenant-scoped `?q=<email>` lookup (`listAdminTenantUsers`) as the operator types, confirms the resolved user's name/email inline, and only enables Add once exactly one exact email match is found. A no-match shows an inline "no user with that email in this tenant" notice.
- **Why**: (1) `useConfirm` — the frontend has no `AlertDialog` primitive; `useConfirm` is the established destructive-confirm pattern and is already mounted app-wide. (2) Email-lookup add — the mock has no real backend and can mint a member from arbitrary strings; the real BE requires a `userID` for an *existing* account. Scoping the lookup to the group's owning tenant also structurally prevents cross-tenant adds from the UI (a `admin.member.tenant_mismatch` 422 is still mapped defensively per the issue AC). A free-text name field would let the operator type a name that isn't backed by any real user.
- **Approved by**: user (explicit) — issue #1756 spec mandates the membership editor, an email-lookup-based Add dialog ("Do NOT add a free-text name field that isn't backed by a real user"), and typed inline error banners; the `useConfirm` substitution follows the documented codebase convention.
- **Reversion plan**: The email-lookup Add UX is permanent — it is dictated by the BE contract. The `useConfirm`-not-`AlertDialog` choice is permanent unless the codebase adopts a shadcn `AlertDialog` primitive repo-wide.

#### 2026-11-25 — Upcoming page

- **Issue/PR**: #2805
- **Mock**: No Upcoming view exists in `design-mocks/src/views/`.
- **Reality**: `/g/:slug/upcoming` (`data-testid="page-upcoming"`) follows [`WarrantiesView.tsx`](../../design-mocks/src/views/WarrantiesView.tsx): a page header, a line tab strip (**All / Reminders / Warranties / Loans / Maintenance**) with outline count badges, and one `Card p-0` list whose rows carry a kind icon, the item name, a muted "kind · title" line, and the date with days left on the right. Overdue rows use `text-destructive`, rows due within seven days `text-primary`. The header's actions slot holds a Radix **Period** select (30 days / 90 days / 12 months). The summary cards of the warranties view are left out.
- **Why**: Not present in mock. The warranties view is the closest dated-list pattern; the period select replaces its summary cards because the list mixes four kinds whose counts already sit on the tabs.
- **Approved by**: user (explicit) — pre-approved for #2805 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Reconcile when the upstream mock adds an Upcoming view.

### Empty / Error / Loading states

#### 2026-05-20 — Admin 403 page reuses the 404 empty-state pattern

- **Issue/PR**: #1752 / PR #1773
- **Mock**: [`design-mocks/src/views/EmptyStatesView.tsx`](../../design-mocks/src/views/EmptyStatesView.tsx) ships `NotFoundView` (404), `NoLocationGroupView`, `NoGroupOnboardingView`, `NoLocationView`, `NoAreaView`, `MaintenanceView` — but **no 403 / "access denied" surface**.
- **Reality**: `frontend/src/pages/admin/AdminForbiddenPage.tsx` (shown in-place by the `RequireSystemAdmin` guard for a signed-in non-admin) replicates the `NotFoundView` anatomy exactly — concentric muted circles + centred Lucide glyph (`ShieldAlert` instead of `SearchX`), `403` overline, bold heading, muted lede, single primary action — swapping only the glyph and copy.
- **Why**: Not present in mock. Issue #1752 requires a "403-style page (NOT a crash, NOT a silent redirect)". The closest analogue in the mock is the 404 empty state, so the page ports that pattern verbatim per the design-mock-fidelity fallback rule.
- **Approved by**: agent-suggested-then-user-confirmed — issue #1752 spec mandates the 403 page; the showcase-fallback pattern is the documented recipe for mock-silent surfaces.
- **Reversion plan**: If the upstream design adds a dedicated 403/forbidden empty state, port it and resolve this entry.

### Cross-cutting (theme, density, a11y, performance)

_None yet._

### Backup & Restore

#### 2026-05-12 — Standalone `/exports/:id/restore` page preserved alongside the in-context Restore dialog

- **Issue/PR**: #1534 / PR #1641
- **Mock**: [`design-mocks/src/views/BackupView.tsx`](../../design-mocks/src/views/BackupView.tsx) L538-L614 surfaces restore exclusively as an in-context `Dialog` launched from the Restore CTA on each completed export row. There is no standalone page.
- **Reality**: `frontend/src/pages/exports/ExportRestorePage.tsx` is preserved as a standalone route (`/exports/:id/restore`). Both surfaces share `frontend/src/components/exports/RestoreOptionsForm.tsx` so the strategy cards, risk pills, dry-run switch, and destructive warning render identically.
- **Why**: The standalone URL is shareable (e.g. for support contexts or post-incident playbooks) and already shipped before #1534; ripping it out in favour of a dialog-only flow would break inbound links. The shared form keeps drift cost at zero.
- **Approved by**: user (explicit) — selected "Full mock-style + share form" when asked how the standalone page should adopt the dialog visuals.
- **Reversion plan**: Permanent unless the upstream mock adds an equivalent shareable surface, at which point the page becomes redundant.

#### 2026-05-12 — Restore dialog exposes an "Include attached files" switch that the mock omits

- **Issue/PR**: #1534 / PR #1641
- **Mock**: The mock's `RestoreDialog` (L538-L614) only surfaces strategy radios + a dry-run switch + a description-less footer. There is no toggle for whether to restore attachments.
- **Reality**: Both `RestoreOptionsForm` and `RestoreDialog` render an `Include attached files` switch (mock-style, `bg-muted/40` row), bound to `RestoreOptions.include_file_data`.
- **Why**: The BE has shipped `include_file_data` on `RestoreOptions` for some time, and the existing standalone page already exposed it as a checkbox. Dropping it from the dialog-only flow would silently change behaviour (attachments would always be restored), so the option is surfaced as a Switch instead.
- **Approved by**: user (explicit) — confirmed the standalone page and dialog should stay visually aligned via the shared form, which keeps the existing option in place.
- **Reversion plan**: Remove the switch (and pass `include_file_data: true` unconditionally) if the upstream mock adds a different mechanism or the BE field is deprecated.

#### 2026-05-12 — Destructive warning only renders when `full_replace` is paired with a non-dry-run

- **Issue/PR**: #1534 / PR #1641
- **Mock**: L584-L591 of `BackupView.tsx` shows the destructive warning whenever `restoreStrategy === "full_replace"`, regardless of the dry-run state.
- **Reality**: `RestoreOptionsForm` only shows the destructive warning when `strategy === "full_replace" && !dry_run` (preserving the pre-#1534 behaviour from `ExportRestorePage`).
- **Why**: Dry-run with `full_replace` does not actually delete anything, so flashing a "this will permanently delete all current data" warning is misleading. Existing tests (`ExportRestorePage.test.tsx`) also encode the gated behaviour.
- **Approved by**: agent-suggested — keeps behaviour-truth over mock-fidelity on a strictly-better-UX call; user invited to revert if visual parity matters.
- **Reversion plan**: Drop the `&& !dry_run` guard and update the test if upstream insists on always-on warning.

#### 2026-05-16 — "New export" stays a dedicated page (`/exports/new`), not a Dialog

- **Issue/PR**: #1661 / PR (this branch)
- **Mock**: [`design-mocks/src/views/BackupView.tsx`](../../design-mocks/src/views/BackupView.tsx) L498-L536 opens the "Create Export" CTA as a single-step `Dialog` (RadioGroup of types → submit footer).
- **Reality**: `frontend/src/pages/exports/ExportNewPage.tsx` is preserved as a standalone two-step wizard route (`/g/{slug}/exports/new`): step 1 picks scope + the optional `selected_items` set + `include_file_data`, step 2 confirms with an optional description and the synthesised-default hint.
- **Why**: The wizard's step 1 carries the `SelectedItemsPicker` tree (locations → areas → commodities) which is too tall for a dialog on mobile and benefits from a dedicated URL that survives accidental nav-aways and is shareable in support contexts. Step 2's description field + summary pane also wants room. The page surface predates the mock and the wizard's two-step shape was deliberately chosen on the previous polish pass.
- **Approved by**: user (explicit) — issue #1661 acceptance criterion 5 ("PRESERVE — no Dialog (intentional)").
- **Reversion plan**: Permanent unless the upstream mock adopts a wizard-shaped Dialog. The component is the only consumer of `/exports/new`; ripping it out requires a Dialog scaffold plus moving `SelectedItemsPicker` into the dialog body.

#### 2026-05-30 — Import file input accepts `.inb` (signed archive), not the mock's `.xml`

- **Issue/PR**: #534 / PR (this branch)
- **Mock**: [`design-mocks/src/views/BackupView.tsx`](../../design-mocks/src/views/BackupView.tsx) L488 wires the restore/import file `<input>` with `accept=".xml"` — the mock predates the backup-format migration and still assumes an XML envelope.
- **Reality**: `frontend/src/pages/exports/ExportImportPage.tsx` sets `accept=".inb,application/x-inventario-backup,application/octet-stream"` and adds a pre-upload client-side guard that rejects any chosen file whose name does not end (case-insensitive) in `.inb`, surfacing the new `exports:errors.invalidFileType` copy ("Choose a backup (.inb) file."). The import copy (`importView.dropHint` / `importView.intro`) and the export-wizard summary (`wizard.summary.includeFileDataYes` → "Included in archive") were rewritten from XML / "Inline base64" wording to the `.inb` signed-archive model. The download path needs no FE change — the browser uses the BE's `Content-Disposition` filename, which the format-agnostic backend now emits as `.inb`.
- **Why**: Backend migration (#534) — backups moved from a hand-rolled XML envelope to a signed `.inb` archive (a tar of `payload.tar.gz` + `payload.tar.gz.sig`, MIME `application/x-inventario-backup`). The extension is the real gate because browsers rarely set the custom MIME on a picked file. The signature-verification badge + public-key UI are **deferred** as a follow-up — the `Export` model carries no signature/verification fields yet, so there is nothing to surface; this entry covers only the upload/accept + copy slice.
- **Approved by**: user (explicit) — issue #534 scope brief instructs the `.inb` accept change, the guard, and the copy rewrite, and to log this deviation.
- **Reversion plan**: Permanent — the XML format is gone. If `inventario-design` re-syncs `BackupView.tsx` to `accept=".inb"`, drop this entry. When the signature/verification badge + public-key surface lands (follow-up), add its own entry rather than amending this one.

### Other

#### 2026-05-17 — Maintenance reminders surface (#1368) has no design mock

- **Issue/PR**: #1368 / PR (this branch)
- **Mock**: There is no `design-mocks/src/views/MaintenanceView.tsx` — the maintenance reminders feature was filed after the design mock was vendored, and the closest analog is `WarrantiesView.tsx` (group-wide list of things expiring soon). The Settings notification toggle for "Maintenance reminders" exists in `SettingsView.tsx`, but no per-commodity tab or list page exists in the mock.
- **Reality**: `frontend/src/components/maintenance/MaintenanceTab.tsx` mounts as a new tab on the commodity detail page (alongside Details / Warranty / Files / Lend / Service); `frontend/src/pages/maintenance/MaintenanceListPage.tsx` is the dedicated group-wide list at `/g/:slug/maintenance`, sidebar entry next to Warranties (Lucide `CalendarClock` icon). Both surfaces follow the existing Card-based table layout used by `LoansListPage` / `WarrantiesListPage` for visual parity.
- **Why**: Mock-omission. The feature is a natural sibling of warranty tracking (one-shot → recurring), and the simplest path that ships a usable surface is to reuse the warranty/loan patterns the mock already established.
- **Approved by**: agent-suggested as part of the #1368 implementation; reviewable here for follow-up alignment with a future maintenance mock.
- **Reversion plan**: When `inventario-design` ships a maintenance mock, replace the per-tab card layout + the list page with the mock-matched components and remove this entry.

#### 2026-05-15 — Tags settings page: All / Item / File scope tabs above the flat list

- **Issue/PR**: #1628 / PR (this branch)
- **Mock**: [`design-mocks/src/views/TagsView.tsx`](../../design-mocks/src/views/TagsView.tsx) renders a single flat list with a search input and seed-from-items CTA. No tabs, no per-scope filtering.
- **Reality**: `frontend/src/pages/tags/TagsListPage.tsx` renders three Radix Tabs (`All` / `Item tags` / `File tags`) directly under the stats bar. The active tab maps to the BE `?scope=` query via `useTags({ scope })` — `commodity` / `file` strictly filter to tags with usage in that bucket; `All` omits the param and returns every tag, matching the legacy ranking. Tab state survives back/forward via `?tab=` on the URL.
- **Why**: Required by #1628 acceptance criteria — the existing flat list mixed commodity and file tags in one namespace, which was confusing once the autocomplete pool was scoped. Tabs were chosen over a sidebar filter because Tabs is the dominant scope-affordance pattern in this app (Files page, Warranties page, Loans page) and the existing Tabs primitive is well-tested.
- **Approved by**: user (explicit) — issue request.
- **Reversion plan**: Remove the `Tabs` block + the `urlTab` / `scopeForTab` derivations + the `scope` option from `listOpts`. Revert i18n keys under `tags:tabs`. Page falls back to the merged flat list.

#### 2026-05-21 — Impersonation FE keeps only the impersonated user's id, not an `admin_return_token`

- **Issue/PR**: #1757 / PR (this branch)
- **Mock**: Not a visual deviation. The issue text for #1757 proposed an `admin_return_token` localStorage slot — the frontend would cache the admin's access token before starting impersonation and restore it when the session ended.
- **Reality**: `frontend/src/lib/auth-storage.ts` persists only `inventario_impersonation` = `{ targetUserId }` — the impersonated user's id. It does NOT store the admin's token. `POST /admin/impersonation/end` returns the admin's freshly minted `access_token` / `csrf_token` in the response body, so the frontend simply adopts those. The stored target id exists solely so the End flow (and the auto-expiry recovery) can route back to `/admin/users/{thatId}`.
- **Why**: Backend design changed after the issue was written. The final BE keeps the admin's "return slot" server-side (jti-keyed) plus an httpOnly marker refresh cookie; `end` self-validates and hands the admin tokens back. A frontend-cached admin token would be redundant and a security liability (a long-lived admin credential sitting in localStorage during an impersonation session). Relatedly, auto-expiry recovery (`recoverFromImpersonationExpiry` in `lib/http.ts`) does NOT restore a cached admin token — it calls `POST /admin/impersonation/end` with the now-expired impersonation token, which the BE deliberately tolerates, and adopts the admin tokens from that response.
- **Approved by**: agent-suggested-then-user-confirmed — the issue brief explicitly instructed this deviation and required it logged here.
- **Reversion plan**: Permanent. The `admin_return_token` approach is incompatible with the shipped backend contract; it would only return if the BE dropped the server-side return slot.

### Reports

#### 2026-05-30 — Insurance report (#1370)

- **Issue/PR**: #1370 / PR #1950
- **Mock**: [`design-mocks/src/views/InsuranceReportView.tsx`](../../design-mocks/src/views/InsuranceReportView.tsx) ships an insurance report with two modes (Item + Location) toggled in a toolbar via a `ToggleGroup`, a subject `Select`, a thumbnail/full-size photo `ToggleGroup`, a Print button, and a footer (group name + generated timestamp). The mock renders as a fullscreen takeover, hardcodes USD, titles each item `{brand} {name}` with a `{model}` subtitle, classifies via a `category` field, and renders a full per-item photo gallery in both modes.
- **Reality**: `frontend/src/pages/reports/InsuranceReportPage.tsx` (+ `features/reports/components/{ItemReport,LocationReport,PhotoSection}.tsx`) follows the mock's two-mode layout, toolbar, subject selector, photo-size toggle, Print button, and footer, but adapts to the real app's data model and chrome via the following divergences: (1) **Scope** — only the mock's Item + Location modes ship; the GitHub issue's broader full-inventory / filtered-report scope was NOT built. (2) **Title** — `name` + `short_name` replace the mock's `{brand} {name}` / `{model}`; the brand/model rows are omitted (no such BE columns). (3) **Classification** — the mock's `category` becomes the real `type` enum, rendered via the existing `commodities:type.*` catalogue (no report-side category map). (4) **Currency** — uses the group currency + converted prices, not hardcoded USD: item purchase = `original_price` in `original_price_currency`, estimated value = `current_price` (group currency), location purchase total sums `converted_original_price` (group currency). (5) **No color / B&W toggle** — neither the mock nor `CommodityPrintPage` has one; the thumb/full photo toggle is kept. (6) **Mounted inside the Shell** — chrome is hidden via a global `@media print` block in `frontend/src/index.css` (see `devdocs/frontend/design/18-print-and-export.md`), not the mock's fullscreen takeover — matches the `CommodityPrintPage` precedent. (7) **Location-mode photos** use the per-item cover thumbnail (from the list endpoint's `meta.covers`, surfaced as `useAllCommodities().covers`) instead of a full per-item gallery — the list endpoint carries covers, not full galleries; item mode still renders the full gallery from the unified `/files` surface. (8) **No `ToggleGroup` primitive** in the app, so the mode + photo toggles are a small segmented-button group (`role="tablist"` / `role="group"`) rather than the mock's `ToggleGroup`.
- **Why**: Backend data-model constraints (no brand/model columns, `type` not `category`, per-group immutable currency from #202/#1550, covers-not-galleries on the list endpoint), missing shadcn primitives (`ToggleGroup`), and the established `CommodityPrintPage` precedent (Shell-mounted print route, chrome hidden via `@media print`). The issue's broader full-inventory / filtered scope was deliberately deferred to match the design mock's two-mode shape.
- **Approved by**: agent-suggested-then-user-confirmed.
- **Reversion plan**: Permanent for the data-model-bound divergences (brand/model, category→type, currency, covers). The `ToggleGroup` segmented-button substitution resolves to 1:1 if/when a `toggle-group.tsx` primitive is vendored. The issue's full-inventory / filtered report scope is a future follow-up if requested.
- **Resolved**: 2026-11-25, PR #2840 — divergence (2) back to 1:1: the title is `{brand} {name}` (the name alone when it already starts with the brand) and the subtitle the model, falling back to the short name for an item without a model (#2804).

#### 2026-11-25 — Seller row in the insurance reports

- **Issue/PR**: #2804 / PR #2844
- **Mock**: [`design-mocks/src/views/InsuranceReportView.tsx`](../../design-mocks/src/views/InsuranceReportView.tsx) item details and the location report's item sections have no seller.
- **Reality**: `features/reports/components/ItemReport.tsx` adds a Seller detail (`Store` icon) after Purchase Date, and `LocationReport.tsx` a Seller row after Purchase Date in each item section; an item without a seller shows the same dash as the other empty values. When the sellers cannot be loaded, the seller of an item that has one stays empty with a small muted "Could not load the seller." line (`UnavailableNote`, hidden in print), and the rest of the report renders.
- **Why**: not present in mock. #2804 asks for the seller in the insurance report. A failed sellers request should not take the whole report down (review of #2844).
- **Approved by**: user (explicit) — pre-approved for #2804 by the 2026-11-24 goal to implement the open enhancements; the empty cell with a note asked for in the #2844 review
- **Reversion plan**: Follow the mock once `inventario-design` adds a seller to the report.

#### 2026-11-25 — Location report totals leave unknown prices out and say so

- **Issue/PR**: #2813 / #2836
- **Mock**: [`design-mocks/src/views/InsuranceReportView.tsx`](../../design-mocks/src/views/InsuranceReportView.tsx) L224-L277 sums `purchasePrice ?? 0` and `currentValue ?? 0` into the Total Purchase and Est. Total Value cards, so a missing price counts as zero.
- **Reality**: `frontend/src/features/reports/components/LocationReport.tsx` sums only the known amounts (`features/reports/aggregate.ts`). Under each total card a muted line reads "Excludes N items without a purchase price" or "… without a value" when items are left out. The purchase total and each item's purchase row use the purchase price in the group currency: the price itself when it was paid in the group currency, otherwise its converted amount. The Est. Total Value card, each item's value row and the item report's Estimated Value use the dashboard's rule (the current value, else that purchase price), so one item has one estimated value everywhere. Item rows and the item report show "—" for an unknown price and a formatted zero for a known zero.
- **Why**: An unknown price is not a zero, and the issue asks report totals to disclose what they exclude. A price paid in the group currency has no converted amount, so summing the converted column alone dropped it.
- **Approved by**: user (explicit) — pre-approved for #2813 by the 2026-11-24 goal to implement the open enhancements
- **Reversion plan**: Follow the mock once `inventario-design` shows coverage on the report cards.

---

## #1966 — Commodity Files tab: unified grid/list, diverging from the mock

**Date:** 2026-05-31

**Mock:** `design-mocks/src/components/ItemDetail.tsx` (`<TabsContent value="files">`) renders the commodity Files tab as a chip-bar + a square-thumbnail **photo grid** (`grid-cols-3`) **plus** a separate **non-photo row list**, shown simultaneously, with per-row View/Open/Download CTAs and inline delete. No grid/list toggle.

**Reality (owner-requested, #1966):** the commodity Files tab now renders the chip-filtered set through the **shared `FileCard` (grid) / `FileListRow` (list)** components with a **grid/list toggle**, identical to the global Files page (`FilesListPage`) and the location/area `EntityFilesPanel`. The bespoke `PhotoGrid` + `NonPhotoList` split is gone; a file looks and behaves the same on every surface. The per-category chip filter is retained; clicking a file still opens the in-place `FilePreviewDialog` (the user stays on the commodity detail page).

**Why the divergence:** the mock produced three different file-list presentations across the app (Files page toggle, 3-col entity grid, commodity chip + photo-grid + row-list). #1966 converges them for consistency. `design-mocks/` is a read-only mirror of upstream `inventario-design`, so it is intentionally **not** edited; this entry records the deliberate drift.

**Behaviour notes:**
- The per-photo **cover star** is available in **grid view only** (`FileCard`); `FileListRow` has no star. Photos were only ever a grid concept, so this is not a regression — set a cover from grid view.
- Per-card inline **delete** and the **View/Open/Download** CTAs are removed (now consistent with the main Files page); delete/download happen from inside `FilePreviewDialog`.
- The grid/list choice persists in `localStorage` under `files:entityViewMode` (shared with `EntityFilesPanel`; default **grid** — entity-detail surfaces are photo-first), distinct from the global Files page's `files:viewMode` (default list).

**Approved by:** @denisvmedia (repo owner) — requested both the commodity-tab unification and the EntityFilesPanel convergence during the #1966 / PR #1984 review.

**Reversion plan:** Revert PR #1984. Concretely: restore `CommodityFilesTab`'s bespoke `PhotoGrid` + `NonPhotoList` from git history, delete `FileCollection` / `FileViewToggle` / `useFilesViewMode`, and re-point `EntityFilesPanel` and `FilesListPage` back at their inline `FileCard`/`FileListRow` renderers. That returns every surface to the per-surface layouts defined in `design-mocks/src/components/ItemDetail.tsx` and `design-mocks/src/views/FileBrowserView.tsx`.
