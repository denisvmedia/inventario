# Print & Export

The print stylesheet, the standalone print route, and the relationship
to backup/restore exports.

## Print

The product has three print-optimized routes in production:

```
/g/:slug/commodities/:id/print     — single commodity
/g/:slug/reports/insurance         — insurance report (#1370)
/g/:slug/labels                    — QR labels for items and areas (#2807)
```

Components:

- `frontend/src/pages/commodities/CommodityPrintPage.tsx`
- `frontend/src/pages/reports/InsuranceReportPage.tsx`
- `frontend/src/pages/labels/LabelPrintPage.tsx` (+
  `features/labels/{formats.ts,LabelSheets.tsx,links.ts}`)

Used by:

- The commodity detail page's "Print" action (kebab menu) →
  commodity print.
- The commodity detail page's "Insurance report" action and the
  location detail page's "Insurance report" action → the insurance
  report (item / location mode, respectively).
- The "Reports" sidebar section → the reports landing
  (`/g/:slug/reports`), which links to the insurance report.
- The commodity detail page's "Print label" action, the items list's
  bulk "Print labels" action, and the area detail page's "Print labels"
  menu → the label page.

Both routes are mounted **inside the protected `<Shell>`** (same as
every other group-scoped page), so on screen the user sees the sidebar
and top bar. A single global `@media print` block in
`frontend/src/index.css` is what hides the Shell chrome (sidebar, top
bar, banners) and neutralizes the inset/container padding when the user
actually prints — the page itself doesn't skip the shell. A toolbar at
the top of each rendered page exposes Back + Print actions (the
insurance report adds mode / subject / photo-size controls); these
**page-local** controls are gated behind `print:hidden` / `.print-hide`
so they don't land on paper.

### Insurance report (#1370)

`/g/:slug/reports/insurance` is a single print-capable view with two
modes selected via the query string:

- `?mode=item&item=<id>` — one commodity (name, type, purchase price,
  estimated value, warranty, location, photo gallery, notes).
- `?mode=location&location=<id>` — every commodity in a location, with
  per-location totals (count, purchase, estimated value) and a per-item
  cover thumbnail.

It mirrors the design mock's `InsuranceReportView` (Item + Location
modes). Currency follows the same contract as the commodity print page
(`original_price`/`original_price_currency` for purchase;
`current_price`/`converted_original_price` in the group currency). See
`devdocs/frontend/design-deviations.md` (#1370) for the adaptations from
the mock.

### Labels (#2807)

`/g/:slug/labels` prints labels with a QR code that encodes the absolute
URL of an item or area page, plus the item's asset ID and name (or the
area name and its location). What to print comes from the query string:
`item=<id>` (repeatable), `area=<id>` (repeatable), `area_items=<id>`
(every active item in that area), `format=single|a4-3x7|letter-3x10`,
and `skip=<n>` (empty positions at the start of the first sheet).

Unlike the report pages, the labels must land on die-cut positions, so
the layout is physical, not responsive:

- Each format in `features/labels/formats.ts` carries the page size, the
  label size, the grid, the offset of the first label and the gaps, in
  millimeters (A4, Avery L7160: 3 × 7 labels of 63.5 × 38.1 mm) or
  inches (US Letter, Avery 5160: 3 × 10 labels of 2.625 × 1 in). The
  single format is one 63.5 × 38.1 mm label per page.
- `LabelSheets` passes those sizes to Tailwind utilities as CSS custom
  properties (`w-(--label-w)` and so on); they are chosen at run time, so
  no static class can carry them.
- The page sets `@page { size: <format>; margin: 0 }` in its scoped
  block and drops its own padding with `print:` utilities. Together
  with the global block in `index.css`, the first sheet starts at the
  corner of the paper. The e2e spec (`asset-id-labels.spec.ts`)
  measures the first label under print emulation, so padding added to
  the Shell later shows up as a test failure, not as misprinted sheets.
- Labels print black on white (`.label-paper`) in either theme. The
  dashed outline of every label position is screen-only.
- QR codes are rendered client-side with `qrcode.react` (`QRCodeSVG`),
  already a dependency for the MFA setup dialog.

## Print layout

Single-column, max width 3xl, on-screen with the protected shell, but
the printed sheet renders only the report sections:

```tsx
<div className="mx-auto max-w-3xl p-6 print:p-0">
  <div className="space-y-6">
    {/* hero */}
    <div className="space-y-1">
      <h1 className="text-2xl font-semibold tracking-tight">{commodity.title}</h1>
      <p className="text-sm text-muted-foreground">{location} · {area}</p>
    </div>
    {/* details table */}
    <table className="w-full text-sm">
      <tbody className="divide-y divide-border">
        <tr><th>Type</th><td>{commodity.type}</td></tr>
        <tr><th>Status</th><td>{commodity.status}</td></tr>
        {/* … */}
      </tbody>
    </table>
    {/* file thumbnails */}
    <div className="grid grid-cols-3 gap-2">
      {commodity.files.map((file) => …)}
    </div>
  </div>
</div>
```

## Print stylesheet

The single global `@media print` block in `frontend/src/index.css`
hides the app-shell chrome for every print route:

- Hide the Shell sidebar (`[data-slot="sidebar"]`), the SidebarInset's
  TopBar (`[data-slot="sidebar-inset"] > header`) and any active
  banners (`[data-slot="sidebar-inset"] > [role="status"]`).
- Neutralize the inset margins/rounding/shadow and the page
  `.container` padding so the report sheet isn't inset on paper.
- Force `body` background to white (printers don't render backgrounds
  by default; explicit white).

Page-local controls (each print page's own toolbar) hide themselves via
`print:hidden` / a small page-scoped `.print-hide` rule — see Hard Rule
2. Forcing `bg-card` → white on individual surfaces, disabling
hover/transition utilities, and `break-inside-avoid` / `break-after-page`
page-break rules ride on the page's own scoped block or `print:` utilities.

Print-specific Tailwind utilities (built-in in v4):

```tsx
<div className="hidden print:block">…</div>      {/* show only when printing */}
<div className="block print:hidden">…</div>      {/* hide when printing */}
<table className="text-sm print:text-xs">…</table>  {/* tighten in print */}
```

## What gets printed

The commodity print page includes:

- Title, location/area breadcrumb.
- Vendor, purchase date, current value, currency.
- Status, status history (last 3).
- Warranty status + dates.
- Tag list.
- All attached files as 3-column thumbnail grid.
- A small footer with print date + the route URL (so the printed copy
  references back to the source).

What it doesn't include:

- The sidebar.
- The top bar / search.
- Any action buttons.
- The status-update controls.
- Cross-tenant data.

## Export (backup) — different from print

"Export" in the UI means **backup the inventory to a file**:

- A signed `.inb` archive (tar of payload.tar.gz + payload.tar.gz.sig)
  containing every commodity, location, area, file (binary or
  reference), tag.
- Created via `/g/:slug/exports`.
- Surfaces a server-side polling status — pending → running → ready
  → expired (or failed).
- When ready, the user downloads the file.

This is **not** the print path. Conceptually:

| Term | What it means | Where |
| --- | --- | --- |
| **Print** | Browser-rendered output for paper / PDF (via the browser's "Save as PDF") | `/g/:slug/commodities/:id/print`, `/g/:slug/reports/insurance`, `/g/:slug/labels` |
| **Export** / **Backup** | Server-generated file that can be **imported** back to recreate the data | `/g/:slug/exports` |
| **Restore** | The inverse of import — apply an export back to a group | `/g/:slug/exports/:id/restore` |

The voice contract ([12-tone-of-voice-and-copy.md](12-tone-of-voice-and-copy.md)) calls export/backup
"Export" in the UI; the artifact itself can be referred to as a
"backup" in narrative copy ("Your latest backup was created 2h ago").

## Print to PDF

The browser handles "Save as PDF" natively from the print dialog. No
custom PDF generation library on the client. If the user wants a
machine-friendly export, they want a backup, not a print.

## Email digest (future)

Same voice + layout as the print page, but rendered server-side. Out
of scope for this brief; the email templates live BE-side.

## Hard rules

1. **Print routes are deliberate.** Three exist today
   (`commodities/:id/print`, `reports/insurance`, `labels`); adding
   another is an issue + PR with the layout spec, registered in this doc.
2. **App-shell chrome hiding lives in `index.css`.** The single global
   `@media print` block in `frontend/src/index.css` hides the
   sidebar / top bar / banners and neutralizes the inset + container
   padding for every print route. A print page may add **one** small
   scoped `@media print` block for its own **page-local** concerns
   (hiding its toolbar via `.print-hide`, dropping its sheet's shadow,
   `break-inside-avoid` on sections) — the precedent set by
   `CommodityPrintPage.tsx` and followed by `InsuranceReportPage.tsx`.
   The anti-pattern is *scattering* `@media print` blocks across many
   feature components, or duplicating the shell-chrome hiding outside
   `index.css`.
3. **No backgrounds in print.** Force `bg-white` (or omit) on
   surfaces; printers skip backgrounds by default.
4. **Print is the route, export is the file.** Don't conflate the two
   in copy or in code.
5. **`break-inside-avoid` on cards** so a card doesn't split across a
   page break.

## Anti-patterns

- A "Save as PDF" button that triggers `window.print()`. Use the
  browser's native UI; the button is redundant.
- A custom client-side PDF library (`pdfmake`, `jsPDF`). Bans the
  bundle budget — see [../imports-and-bans.md](../imports-and-bans.md). Use the print route +
  browser PDF export.
- *Scattering* `@media print` blocks across many feature components.
  Shell-chrome hiding lives once in `index.css`; a print page may keep a
  single scoped block for its own page-local controls (Hard Rule 2).
- Printing the sidebar / top bar / banners. The global `@media print`
  block in `index.css` hides them via the Shell's `data-slot` selectors;
  don't re-implement chrome-hiding per page.
- Watermarks ("CONFIDENTIAL", company logos). Inventario doesn't
  watermark prints; the user controls the data.

## Cross-refs

- Tokens (print bg overrides): [01-palette.md](01-palette.md).
- Voice for "Export" vs. "Backup": [12-tone-of-voice-and-copy.md](12-tone-of-voice-and-copy.md).
- Backup feature slice: `frontend/src/features/export/`.
- Print component: `frontend/src/pages/commodities/CommodityPrintPage.tsx`.
- Print stylesheet: `frontend/src/index.css` (`@media print`).
