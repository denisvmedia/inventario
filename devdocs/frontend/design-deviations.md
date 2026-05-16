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
- **Reversion plan**: Resolve when #1540 lands the scanner backend + scanning/review phases. The card + dropzone markup stays; the disclosure line gets dropped, the file picker + Sparkles click handler get wired to the real scan endpoint.

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

#### 2026-05-10 — Commodity grid card purchase-date chip

- **Issue/PR**: #1547 / PR #1631
- **Mock**: [`design-mocks/src/components/ItemsPanel.tsx`](../../design-mocks/src/components/ItemsPanel.tsx) grid-card top-right cluster (lines ~438–450) groups item-level chips — Draft / status / `WarrantyBadge`. `CardContent` (lines ~459–475) is strictly `area | currentValue` + tags. No purchase-date anywhere.
- **Reality**: When `commodity.purchase_date` is present, the top-right chip cluster gains an extra neutral outline badge `📅 9/12/24` (lucide `Calendar` at `size-2.5` + locale-short date) at the end of the cluster. The visible chip carries only the date — full copy `Purchased {{date}}` (i18n key `commodities:card.purchasedOn`) is exposed via `title` / `aria-label`. `CardContent` is left at strict mock fidelity (`short_name | price`). The chip is gated purely on `!!row.purchase_date` — so it's hidden whenever the field is unset (the typical case for drafts that never filled it in, and for pre-#1367 entries that predate the column), and it still renders on drafts that *did* record a purchase date, matching the rest of the cluster (Status / Lent / In Service / WarrantyBadge don't gate on `row.draft` either, and the card's `opacity-70 border-dashed` already de-emphasises drafts).
- **Why**: Not present in mock. Issue #1547 (spun off from the closed #189) asks for a compact surfacing of `purchase_date` on the card, and explicitly says "Skip the line entirely when `purchase_date` is empty (drafts / pre-#1367 entries)" — so the gate is the value, not the draft flag. Temporal metadata is the same conceptual layer as `WarrantyBadge`, so the mock-spirited home is the top-right chip cluster (not a new line in `CardContent`, which crowds the price row and truncates at `lg:grid-cols-3`). Neutral `border-border text-muted-foreground` styling keeps it visually subordinate to status chips that carry semantic color. Tooltip + `aria-label` preserve the "Purchased {date}" copy without the chip needing a text prefix. Locale-aware via the existing `formatDate` helper, no new "date format" settings plumbing.
- **Approved by**: user (explicit) — issue #1547 asks for a compact surfacing of `purchase_date` on the grid card ("below `area` or alongside `current_price`"), names the i18n key shape, and names the `formatDate` helper. The chip-in-top-right-cluster placement is an agent-chosen interpretation of "compact" that the user confirmed visually after two earlier iterations (standalone line, inline dot-separator) were rejected.
- **Reversion plan**: Permanent until the upstream mock adopts the same line; reconcile when it does.

### Dashboard / Overview

#### 2026-05-10 — Stat-card grid: 6 inventory metrics in a single `lg:grid-cols-3` block

- **Issue/PR**: #1544 / PR #1621
- **Mock**: [`design-mocks/src/views/DashboardView.tsx`](../../design-mocks/src/views/DashboardView.tsx) L53-L82 + L112 ships **4** stat cards (Total Items, Active Warranties, Expired Warranties, Est. Total Value) in a `grid-cols-2 gap-4 lg:grid-cols-4` block. The Active/Expired counters depend on `warrantyStatus(item)` rolled up across `MOCK_ITEMS`.
- **Reality**: `frontend/src/pages/Dashboard.tsx` ships **6** stat cards (Total Value, Avg Value, Total Items, Locations, Areas, Files) in a single `grid-cols-2 gap-4 lg:grid-cols-3` block. Mobile renders as a clean 3×2; desktop stays 3×2 (vs the mock's 1×4). Card composition uses the existing `<StatCard>` component, not the mock's inline `<Card>` + `<CardHeader>` shape.
- **Why**: Dual constraint. (1) The warranty status counts the mock's "Active Warranties" / "Expired Warranties" cards depend on require warranty rollups gated on #1367 / #1529 — neither has landed yet, so the cards literally have no data to show. (2) We already had the six Inventory metrics surfacing live data; the prior shipped layout was two grids of three (`lg:grid-cols-3`) which produced two awkward half-empty rows on mobile. Merging into a single 6-card grid is the smallest move that improves mobile without reaching for unimplemented BE rollups.
- **Approved by**: user (explicit) — "Доделай ещё свою главную задачу - дашборд."
- **Reversion plan**: When #1367 / #1529 land the warranty rollups, swap to the mock's 4-card layout (Active / Expiring / Expired / Total Value) by replacing the six `<StatCard>` calls with the four warranty-rooted ones and switching `lg:grid-cols-3` → `lg:grid-cols-4`. The Locations / Areas / Files counts then either move to a secondary surface (sidebar metric strip, or a "More stats" expander) or get folded into the relevant feature pages.

### Locations & Areas

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

#### 2026-05-10 — Per-area items panel ships v1 with two stats + simple list (no toolbar / files) — _resolved 2026-05-12_

- **Issue/PR**: #1531 (item 1) / PR _pending_
- **Resolution**: Resolved by the item-1 follow-up PR. `AreaDetailPage` now mounts a new `AreaItemsPanel` (`frontend/src/pages/areas/AreaItemsPanel.tsx`) that ports the mock's full Level-3 toolbar: search input, type / status / warranty filter dropdowns, sort, view-mode toggle (grid + list), hide-inactive switch, and pagination — all URL-state-backed so refresh + back / forward survive. The third "Active warranties" stat cell shipped alongside, sampled from the same page-level fetch that drives the list (so a single network round-trip powers both); partial counts past the page cap surface as `{N}+` with the same truncation cue the locations list uses. The Area Files panel landed under the items section once the BE's `linked_entity_type` enum was widened to include `"area"` (validators in `go/models/models.go:357` + `go/jsonapi/files.go` three sites); `EntityFilesPanel` + `UploadFilesDialog` now accept the `"area"` type union and a `files:entityPanel.dropOverlay_area` i18n key was added. Bulk select stays deliberately deferred — the area surface intentionally doesn't surface bulk actions (those belong to the global `/commodities` page); not tracked as a separate follow-up. The page wrapper stays at `max-w-4xl` (one bucket short of the mock's `max-w-5xl`) — a smaller drift than the embedded list would justify at `max-w-3xl`.

### Files & Attachments

#### 2026-05-16 — Files page keeps the multi-select BulkBar; surfaces it as a fixed bottom overlay

- **Issue/PR**: #1659 / PR (this branch)
- **Mock**: [`design-mocks/src/views/FileBrowserView.tsx`](../../design-mocks/src/views/FileBrowserView.tsx) has no bulk-select affordance at all — file rows have no per-row checkbox, no select-all, no bulk action toolbar.
- **Reality**: `frontend/src/pages/files/FilesListPage.tsx` ships per-file checkboxes (both on `FileCard` and `FileListRow`) and a context-mode `BulkBar` with select-all, "Move to…" reclassification, and bulk delete (`data-testid="files-bulk-bar"`). Under this PR the bar shifts from an in-flow `bg-muted/40` row to a `fixed bottom-6 left-1/2 -translate-x-1/2 z-40` overlay on the `popover` token, animating in via `tw-animate-css` (`animate-in slide-in-from-bottom-4 fade-in-0 duration-200`) so the first selection no longer reflows the page.
- **Why**: Bulk operations (move, delete) are real product affordances on a page that can hold tens of thousands of files — dropping them to match the mock would be an unambiguous regression. The fixed-overlay shape follows the "context-mode toolbar" pattern (GMail/Drive/shadcn) so the bar stays visible across scroll without ever pushing the grid down. Using `bg-popover` (a token that already implies a floating-surface elevation) keeps us off bespoke `shadow-*` decoration per the design language.
- **Approved by**: user (explicit) — issue #1659 §9 "PRESERVE: file multi-select — but animate the bulk bar entry" lists the bar as a real product feature and asks for the fixed-overlay treatment as the preferred option.
- **Reversion plan**: Permanent unless the upstream mock adopts a richer mass-action pattern. The unrelated hidden-by-default checkbox UX (separate UX concern) is tracked in #1484; if that lands, this entry stays as-is (the BulkBar shape doesn't change) and the checkbox visibility note moves to its own entry.

#### 2026-05-09 — Curated tag pills match by lowercase tag name, not opaque tag id

- **Issue/PR**: #1538 (item 3) / PR _pending_
- **Mock**: [`design-mocks/src/views/FileBrowserView.tsx`](../../design-mocks/src/views/FileBrowserView.tsx) (lines ~645–673) renders six curated tag pills sourced from `FILE_TAGS` in [`design-mocks/src/data/mock.ts`](../../design-mocks/src/data/mock.ts) — each pill is `{ id: "t1", label: "Invoice", color: "text-chart-1" }` and matches `file.tags.includes("t1")`. Files in the mock dataset are tagged with the same opaque ids (`t1`, `t2`, …).
- **Reality**: The real BE stores `tags` as a free-form `string[]` (no Tags entity yet — that's #1400). The FE's curated pills mirror the mock's six labels (Invoice / Warranty / Manual / Photo / Certificate / Backup) but match against the lowercase tag name (`invoice`, `warranty`, …) so the toolbar pill toggles a recognisable string into `?tags=`. Custom user-supplied tags still render on the file cards/rows but don't appear as toolbar pills, and the freeform `TagsInput` is removed from the toolbar (it stays on the upload/edit forms only, per the issue spec).
- **Why**: The mock's opaque-id taxonomy doesn't exist on the BE — there's no Tags table to assign ids from. Using the lowercase label as the literal tag string keeps the pill flow round-trippable through `?tags=` and the BE's `tags @> $` filter without inventing an id space the BE doesn't enforce. The discoverability gap for custom tags is a deliberate trade — the issue explicitly notes "Likely coordinates with #1400" — and the curated taxonomy is the canonical surface until that lands.
- **Approved by**: agent-suggested-then-user-confirmed — issue #1538 §3 specifies replacing the freeform input with curated pills and notes the #1400 coordination.
- **Reversion plan**: Resolve when #1400 lands a proper Tags entity — pills then match by id again, and the i18n keys become tag-record labels.

### Tags

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

### Forms & Validation

_None yet._

### Auth & Profile

_None yet._

### Settings & Preferences

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

#### 2026-05-08 — "Currency migrations" history list inside Danger Zone

- **Issue/PR**: #1553 / PR #1604
- **Mock**: [`GroupSettingsView.tsx`](../../design-mocks/src/views/GroupSettingsView.tsx) Danger Zone contains a single "Delete group" button.
- **Reality**: Danger Zone gains a second sub-section under a thin top divider: "Currency migrations" — a paginated list (server-capped at latest 10) showing per-row `from → to` + rate + timestamps + status pill. Empty state, loading skeleton, and the row layout follow the existing `RestoreHistoryList` (`frontend/src/components/exports/RestoreHistoryList.tsx`) — same shadcn/ui `Card`-less border + `divide-y` rhythm; same `Skeleton` + empty-state copy pattern.
- **Why**: Not present in mock. Issue #1553 §"Group settings" requires the history surface; we picked the existing restores list as the closest mock-aligned pattern (since `RestoreHistoryList` itself ships in production today against `BackupView`). No undo affordance per spec.
- **Approved by**: user (explicit) — issue #1553 §"Group settings" §5.1 names the placement and the row content.
- **Reversion plan**: Permanent until the design team explicitly adds a history pattern; if it lands, this list adopts the new chrome.

### Navigation & App shell

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

### i18n & Formatting

_None yet._

### Tables & Lists

_None yet._

### Empty / Error / Loading states

_None yet._

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

### Other

#### 2026-05-15 — Tags settings page: All / Item / File scope tabs above the flat list

- **Issue/PR**: #1628 / PR (this branch)
- **Mock**: [`design-mocks/src/views/TagsView.tsx`](../../design-mocks/src/views/TagsView.tsx) renders a single flat list with a search input and seed-from-items CTA. No tabs, no per-scope filtering.
- **Reality**: `frontend/src/pages/tags/TagsListPage.tsx` renders three Radix Tabs (`All` / `Item tags` / `File tags`) directly under the stats bar. The active tab maps to the BE `?scope=` query via `useTags({ scope })` — `commodity` / `file` strictly filter to tags with usage in that bucket; `All` omits the param and returns every tag, matching the legacy ranking. Tab state survives back/forward via `?tab=` on the URL.
- **Why**: Required by #1628 acceptance criteria — the existing flat list mixed commodity and file tags in one namespace, which was confusing once the autocomplete pool was scoped. Tabs were chosen over a sidebar filter because Tabs is the dominant scope-affordance pattern in this app (Files page, Warranties page, Loans page) and the existing Tabs primitive is well-tested.
- **Approved by**: user (explicit) — issue request.
- **Reversion plan**: Remove the `Tabs` block + the `urlTab` / `scopeForTab` derivations + the `scope` option from `listOpts`. Revert i18n keys under `tags:tabs`. Page falls back to the merged flat list.
