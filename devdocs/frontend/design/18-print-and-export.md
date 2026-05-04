# Print & Export

The print stylesheet, the standalone print route, and the relationship
to backup/restore exports.

## Print

The product has one print-optimized route in production:

```
/g/:slug/commodities/:id/print
```

Component: `frontend/src/pages/commodities/CommodityPrintPage.tsx`.

Used by:

- The commodity detail page's "Print" action (kebab menu).
- Future: a "Print location summary" surface (out of scope today).

## Print layout

Single-column, max width 3xl, no sidebar, no top bar:

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

`@media print` rules in `frontend/src/index.css`:

- Hide non-essential chrome (`@media print { .print\:hidden { display: none; } }` etc.).
- Force `bg-card` to `bg-white` (printers don't render backgrounds by
  default; explicit white).
- Disable all hover / transition / animation utilities.
- Page break: `break-inside-avoid` on cards, `break-after-page` on
  major sections.

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

- A ZIP / XML envelope containing every commodity, location, area,
  file (binary or reference), tag.
- Created via `/g/:slug/exports`.
- Surfaces a server-side polling status — pending → running → ready
  → expired (or failed).
- When ready, the user downloads the file.

This is **not** the print path. Conceptually:

| Term | What it means | Where |
| --- | --- | --- |
| **Print** | Browser-rendered output for paper / PDF (via the browser's "Save as PDF") | `/g/:slug/commodities/:id/print` |
| **Export** / **Backup** | Server-generated file that can be **imported** back to recreate the data | `/g/:slug/exports` |
| **Restore** | The inverse of import — apply an export back to a group | `/g/:slug/exports/:id/restore` |

The voice contract (`12-tone-of-voice-and-copy.md`) calls export/backup
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

1. **One print route in production.** Adding a second is an issue +
   PR with the layout spec.
2. **`print:` utilities for chrome hiding.** No `@media print` blocks
   inside components — keep all print rules in `index.css`.
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
  bundle budget — see `../imports-and-bans.md`. Use the print route +
  browser PDF export.
- A `@media print` block inside a feature component. All print rules
  in `index.css`.
- Printing the sidebar. Use `print:hidden` on the Shell.
- Watermarks ("CONFIDENTIAL", company logos). Inventario doesn't
  watermark prints; the user controls the data.

## Cross-refs

- Tokens (print bg overrides): `01-palette.md`.
- Voice for "Export" vs. "Backup": `12-tone-of-voice-and-copy.md`.
- Backup feature slice: `frontend/src/features/export/`.
- Print component: `frontend/src/pages/commodities/CommodityPrintPage.tsx`.
- Print stylesheet: `frontend/src/index.css` (`@media print`).
