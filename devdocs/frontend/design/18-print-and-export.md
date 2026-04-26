# Print & Export

Inventory documentation often leaves the app — for insurance claims, for selling, for sharing with family. This document specifies how.

## Why this matters

The most common Inventario "exit" use case is producing a PDF of the inventory for an insurance company. If that PDF looks unprofessional or is hard to read, the product fails its primary job. Print-quality output is a feature, not an afterthought.

## Print stylesheet

A `@media print` block applies globally and resets the chrome:

```css
@media print {
  /* Hide app shell */
  nav, header, aside, footer.app-footer,
  .sidebar, .topbar, .bottom-nav,
  .button, .toast, .tooltip,
  [data-print-hidden] {
    display: none !important;
  }

  /* Reset background to white, ink to near-black */
  body {
    background: white;
    color: #111;
  }

  /* Remove all shadows, borders become hairlines */
  * {
    box-shadow: none !important;
    border-color: #000 !important;
    border-width: 0.5pt !important;
  }

  /* Type sized for print */
  body { font-size: 10pt; line-height: 1.4; }
  h1   { font-size: 18pt; }
  h2   { font-size: 14pt; }
  h3   { font-size: 11pt; }

  /* Ensure colors are color-managed for print */
  * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }

  /* Page margins */
  @page {
    margin: 18mm 14mm;
    size: A4;
  }

  /* Page break controls */
  h1, h2, h3 { page-break-after: avoid; break-after: avoid; }
  .commodity-row { page-break-inside: avoid; break-inside: avoid; }
}
```

### What's preserved

- Page title block with date and workspace name (header, repeats per page via `position: running` if browser supports)
- The actual content — entity title, metadata, photos, descriptions
- Page numbers (footer, "Page X of Y")
- Workspace branding (small) at top

### What's hidden

- Navigation
- Action buttons (edit, delete, etc.)
- Loading skeletons (replaced with empty state)
- Backdrops, scrims
- Tooltips (their content is irrelevant in print)
- Search inputs
- Toast notifications
- Hover/focus states

## Export formats

Inventario supports four export formats:

### A. PDF (printable)

Browser-driven via `window.print()` or server-rendered via headless Chrome / wkhtmltopdf.

**When user clicks "Print" or "Export to PDF":**
- Server-rendered for high-fidelity (consistent fonts, predictable layout)
- Two layouts: **summary** (one row per thing) and **detailed** (each thing on its own page with photos)

### B. Spreadsheet (CSV / XLSX)

For users importing into Excel, Numbers, accounting software.

**Columns by entity:**
- Things: name, type, place, area, count, original_price, current_price, currency, purchase_date, warranty_until, serial_number, tags, notes, created_at, updated_at, file_count
- Places: name, address, total_value, area_count, thing_count
- Files: name, mime_type, size, owner_type, owner_name, uploaded_at

XLSX is preferred (preserves data types). CSV always available.

### C. JSON (full backup)

For users wanting a complete snapshot. Includes all entities, all relationships, all metadata. The "Backups" feature in nav already does this — keep the term.

### D. Single-thing snapshot (PDF)

For sharing one item (sending to a service tech, listing for sale). Single-page PDF with photo, key metadata, QR code linking back to the live record (if user opts in).

## Print template — summary

```
┌─────────────────────────────────────────────────┐
│ [logo]                          Apr 26, 2026   │  header
│                                                 │
│ Home Inventory                                  │  h1
│ Denis V — Inventario                            │  subtitle
│ 93 things across 4 places · approx. 167K CZK    │  meta
│ ─────────────────────────────────────────────── │
│                                                 │
│ Place: Home                                     │  section
│  ┌─────────────────────────────────────────┐   │
│  │ Bedroom                                 │   │  area
│  │  ─ Camping Equipment    Outdoor    21K │   │  thing rows
│  │  ─ Wool blanket         Bedding     2K │   │
│  │  ...                                    │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│ ...                                             │
│                                                 │
│ ─────────────────────────────────────────────── │
│ Page 1 of 12                  Inventario        │  footer
└─────────────────────────────────────────────────┘
```

## Print template — detailed (per thing)

One page per thing:

```
┌─────────────────────────────────────────────────┐
│ [thumbnail]   Camping Equipment                 │
│               Outdoor / Bedroom / Home          │
│ ────────────────────────────────────────────── │
│ Type:               Outdoor equipment           │
│ Count:              1                           │
│ Purchase date:      Jul 10, 2021                │
│ Original price:     17,500 CZK                  │
│ Current value:      21,250 CZK                  │
│ Warranty:           Until Jul 2024 (expired)    │
│ Serial:             —                           │
│ Tags:               outdoor, seasonal           │
│                                                 │
│ Notes                                           │
│ Family camping kit, used 3-4 times per year.    │
│                                                 │
│ Photos                                          │
│ [photo 1]  [photo 2]  [photo 3]                │
│ [photo 4]  [photo 5]                           │
│                                                 │
│ Documents                                       │
│  ─ Receipt (.pdf, 1.2 MB)                       │
│  ─ Warranty card (.pdf, 240 KB)                 │
│  ─ User manual (.pdf, 5.1 MB)                   │
└─────────────────────────────────────────────────┘
```

## Insurance summary template

A specialized export for insurance purposes. PDF with:
- Cover page: workspace name, date, total declared value, currency
- Per-place breakdown
- Per-thing rows with photo, name, value, serial, purchase date
- Each photo URL or embedded image (configurable)
- Footer: "Generated by Inventario on Apr 26, 2026 — values reflect user input, not appraised market value."

The legal disclaimer matters: insurance companies need to know this is a record, not an appraisal.

## Export UX

### Triggering export

- From any list view, top-right "Export ▾" dropdown:
  - Print (browser native)
  - PDF (download)
  - Spreadsheet (XLSX / CSV)
  - JSON (full backup) — only on "Backups" page
- From entity detail: "Print" / "Export" button in the top-right menu

### Configuring export

Modal for spreadsheet/PDF exports:

```
Export to PDF
─────────────────────────
What to include:
[•] Photos (largest 1)
[•] Documents (just file list, not file content)
[ ] Warranty / serial details
[ ] Internal notes

Layout:
( ) Summary (one line per thing)
(•) Detailed (one page per thing)

Filter:
Include only: [All things ▾]

[Cancel]   [Export]
```

Defaults: photos on, summary layout for >50 things, detailed for <50.

### Progress

For large exports:
- Modal closes
- Toast: "Generating your export…"
- Notifications-center entry appears with a progress bar
- When ready, toast: "Your export is ready. [Download]"
- Notification persists; user can re-download for 7 days

## Sharing exports

For users who want to share a single thing with a contractor / family member:

- Per-thing "Share as PDF" generates a one-page PDF with optional QR
- Copy a link to clipboard (signed URL, expires in 7 days by default, configurable)
- Email the export directly (if email service configured)

No public-by-default sharing. Every share is explicit.

## Data export under GDPR / privacy laws

User can request a full data export from Profile / Privacy:
- All entities
- All files (zipped)
- All metadata
- Audit log
- Email content sent to them

Generated server-side, downloadable for 30 days. Confirms user's right to data portability.

## What ships in sprint 0

Print stylesheet and basic PDF export are foundational; they need to exist for the product's primary use case (insurance documentation).

1. Implement `@media print` global stylesheet
2. Build server-side PDF export for summary template
3. Add "Print" and "Export to PDF" options in list views
4. Wire spreadsheet (XLSX) export — server-side via existing libraries

Sprint 1+:
- Detailed PDF template
- Per-thing share-as-PDF
- Insurance summary template
- Configurable export modal
- Email-the-export option
