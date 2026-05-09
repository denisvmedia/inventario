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

### Locations & Areas

_None yet._

### Files & Attachments

#### 2026-05-09 — Curated tag pills match by lowercase tag name, not opaque tag id

- **Issue/PR**: #1538 (item 3) / PR _pending_
- **Mock**: [`design-mocks/src/views/FileBrowserView.tsx`](../../design-mocks/src/views/FileBrowserView.tsx) (lines ~645–673) renders six curated tag pills sourced from `FILE_TAGS` in [`design-mocks/src/data/mock.ts`](../../design-mocks/src/data/mock.ts) — each pill is `{ id: "t1", label: "Invoice", color: "text-chart-1" }` and matches `file.tags.includes("t1")`. Files in the mock dataset are tagged with the same opaque ids (`t1`, `t2`, …).
- **Reality**: The real BE stores `tags` as a free-form `string[]` (no Tags entity yet — that's #1400). The FE's curated pills mirror the mock's six labels (Invoice / Warranty / Manual / Photo / Certificate / Backup) but match against the lowercase tag name (`invoice`, `warranty`, …) so the toolbar pill toggles a recognisable string into `?tags=`. Custom user-supplied tags still render on the file cards/rows but don't appear as toolbar pills, and the freeform `TagsInput` is removed from the toolbar (it stays on the upload/edit forms only, per the issue spec).
- **Why**: The mock's opaque-id taxonomy doesn't exist on the BE — there's no Tags table to assign ids from. Using the lowercase label as the literal tag string keeps the pill flow round-trippable through `?tags=` and the BE's `tags @> $` filter without inventing an id space the BE doesn't enforce. The discoverability gap for custom tags is a deliberate trade — the issue explicitly notes "Likely coordinates with #1400" — and the curated taxonomy is the canonical surface until that lands.
- **Approved by**: agent-suggested-then-user-confirmed — issue #1538 §3 specifies replacing the freeform input with curated pills and notes the #1400 coordination.
- **Reversion plan**: Resolve when #1400 lands a proper Tags entity — pills then match by id again, and the i18n keys become tag-record labels.

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

### Other

_None yet._
