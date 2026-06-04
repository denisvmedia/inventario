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

#### 2026-06-04 — Commodity detail SHEET drops the in-sheet tab strip; full detail behind a "View full details" CTA

- **Issue/PR**: #1965 / PR (pending)
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

### Files & Attachments

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

### Auth & Profile

_None yet._

### Layout & Page chrome

#### 2026-05-25 — Canonical page widths + PageHeader: two named tokens (narrow / wide) instead of the mock's case-by-case max-w-\*

- **Issue/PR**: #1889 / PR #1895
- **Mock**: Every view in `design-mocks/src/views/` rolls its own outer wrapper — `max-w-2xl` (`TagsView`, `GroupSettingsView`), `max-w-3xl` (`UserProfileView`, `MembersView`, admin/`UserDetailView`, admin/`GroupDetailView`), `max-w-4xl` (`SettingsView`, `WarrantiesView`, `BackupView`, `PlansView`, admin/`TenantsView`, admin/`GroupsView`, admin/`TenantDetailView`), `max-w-5xl` (`DashboardView`, `ItemsView`, `FileBrowserView`), with each page's H1 picking its own size + tracking too (`text-3xl scroll-m-20 tracking-tight` for list pages, `text-2xl tracking-tight` for detail pages, plus a few `text-2xl font-semibold` no-tracking outliers). There is no shared layout primitive in the mock — every view is self-contained.
- **Reality**: `frontend/src/components/ui/page.tsx` introduces `<Page width="narrow" | "wide" | "full">` + `<PageHeader size="page" | "detail">`. `narrow` = 48rem / 768px (`--container-page-narrow`), `wide` = 80rem / 1280px (`--container-page-wide`), `full` = no max-width. Width tokens live in `frontend/src/index.css` (`@theme { --container-page-narrow / --container-page-wide; }`). Every top-level route now composes with one of the three; ad-hoc `max-w-*` literals on top-level wrappers are forbidden by the convention guard at `frontend/src/pages/__tests__/page-layout-tokens.test.ts`. The UIShowcasePage gains a "Page" tab enumerating the canonical variants for design + dev review.
- **Why**: User-driven UX request — issue #1889 calls out that walking through routes produces an inconsistent "envelope" rhythm (Warranties vs. Lent Out is the cited example) and asks for "a small, named set of layout widths and apply them consistently". Standardising on 2+1 named widths (and a single PageHeader shape) eliminates the per-page-author drift the mock didn't lock down, while preserving the narrow-vs-wide distinction the mock implicitly used. The tokens sit slightly wider than the most-common mock widths (`wide` = 80rem vs. mock's frequent 5xl=64rem, `narrow` = 48rem vs. mock's variable 2xl/3xl) — chosen from the issue's recommended ranges of "~720–800px narrow" / "~1200–1280px wide" to give data-heavy pages more room before the next mock sync. List pages stick with the mock's `scroll-m-20 text-3xl font-semibold tracking-tight`; detail pages stick with `text-2xl font-semibold tracking-tight`.
- **Approved by**: user (explicit) — issue body specifies the named-tokens approach and the canonical PageHeader requirement, plus a lint guard.
- **Reversion plan**: Permanent. If the upstream mock ever ships its own layout primitive, swap the token values to match and drop this entry. Pages requesting an exemption (centered error/auth/empty-state cards, print pages) are listed in the convention guard's `SPECIAL_PURPOSE_PAGES` allow list — those stay bespoke by design.

### Settings & Preferences

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

### i18n & Formatting

#### 2026-05-16 — Dashboard hero: K/M/B compact-notation fallback at ≥1e7

- **Issue/PR**: #1684 / PR (this change)
- **Mock**: [`design-mocks/src/views/DashboardView.tsx`](../../design-mocks/src/views/DashboardView.tsx) `formatCurrency()` uses `Intl.NumberFormat(..., { maximumFractionDigits: 0 })` only — full digit groups regardless of magnitude. Hard-coded to `en-US` + USD in the mock; doesn't model low-denomination currencies.
- **Reality**: `frontend/src/lib/intl.ts`'s `formatCurrency` now accepts `notation?: "standard" | "compact"`. `frontend/src/pages/Dashboard.tsx`'s hero call site switches to `notation: "compact"` once `data.totalValue >= 1e7`, otherwise keeps `compact: true` (no cents). So `$329,849` reads the same as it did post-#1678 for a typical group, while a HUF 100,000,000 inventory renders as "HUF 100M" instead of clipping. Per-item / detail / list surfaces never see compact-notation — they keep full precision via the default path.
- **Why**: PR #1678 dropped cents (`compact: true`), which handles six-figure totals fine but still clips at 8–9 digits — common in HUF / IDR / VND / KRW / IRR. The threshold-based hybrid (#1684 "Option 2") keeps the current production reading for the vast majority of groups and only kicks in at the edges where width pressure is otherwise unsolvable without truncation or a separate font-size hack.
- **Approved by**: user (explicit) — issue spec walked through the two options and noted Option 2 preserves the current "$329,849" reading; pre-authorized via the goal-message instruction to implement #1684.
- **Reversion plan**: If a future hero design reflows to give the totals card more horizontal room (full-width hero strip), or if Intl ever ships a "narrow currency" mode that's locale-aware and width-aware, drop the threshold and revert to `compact: true` everywhere on the hero. The `notation` option on `formatCurrency` stays — its existence is orthogonal to whether the dashboard uses it.

### Tables & Lists

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

**Approved by:** denis@5x5.cz (repo owner) — requested both the commodity-tab unification and the EntityFilesPanel convergence during the #1966 / PR #1984 review.

**Reversion plan:** Revert PR #1984. Concretely: restore `CommodityFilesTab`'s bespoke `PhotoGrid` + `NonPhotoList` from git history, delete `FileCollection` / `FileViewToggle` / `useFilesViewMode`, and re-point `EntityFilesPanel` and `FilesListPage` back at their inline `FileCard`/`FileListRow` renderers. That returns every surface to the per-surface layouts defined in `design-mocks/src/components/ItemDetail.tsx` and `design-mocks/src/views/FileBrowserView.tsx`.
