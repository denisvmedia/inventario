# File & Media Handling

The four-category file model and the visual rules for thumbnails,
viewers, drop zones, and uploads. **Committed.** The four categories
are mirrored on the backend (`models.FileCategory`) and on the
canonical mock.

## The four categories

| Category | What it is | Default icon | Thumbnail |
| --- | --- | --- | --- |
| `image` | Photos, scans, screenshots — anything renderable as an `<img>`. | `Image` | Render the image at 1:1, contained in the tile. |
| `invoice` | Receipts, purchase invoices, warranty cards as PDFs. | `Receipt` | First-page PDF preview if available; otherwise the icon. |
| `document` | Manuals, certificates, anything PDF-shaped that isn't an invoice. | `FileText` | First-page PDF preview if available; otherwise the icon. |
| `other` | Everything else — `.zip`, `.csv`, `.txt`, video files. | `Paperclip` | Always the icon. |

Closed enum. Don't add a fifth — the backend won't accept it. If a
genuinely new bucket emerges (e.g. "video" as its own category), it's
a coordinated FE+BE issue.

## File card

The standard tile, used in galleries, item-detail file rows, and the
file picker:

```tsx
<button
  onClick={() => onOpen(file.id)}
  className="group relative aspect-square w-full rounded-lg border border-border bg-card overflow-hidden hover:border-foreground/15 transition-colors"
>
  {file.thumbnail ? (
    <img
      src={file.thumbnail}
      alt={file.title}
      className="h-full w-full object-cover"
    />
  ) : (
    <div className="flex h-full w-full items-center justify-center bg-muted">
      <Icon className="size-8 text-muted-foreground" />
    </div>
  )}
  <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-foreground/60 to-transparent p-2 opacity-0 group-hover:opacity-100 transition-opacity">
    <p className="text-xs text-background truncate">{file.title}</p>
  </div>
</button>
```

The gradient overlay is one of the **few** places we depart from
"borders, not shadows" (`04-elevation-and-effects.md`) — it's a
linear gradient, not a shadow, but the visual effect is similar. The
exception exists because thumbnails need readable captions on top of
arbitrary user content; a border can't carry that.

## File row (list view)

For the file detail panel inside a commodity, or the standalone Files
page:

```tsx
<button
  onClick={…}
  className="flex w-full items-center gap-3 px-4 py-3 hover:bg-muted/40 transition-colors"
>
  <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
    <CategoryIcon className="size-4 text-muted-foreground" />
  </div>
  <div className="flex-1 min-w-0">
    <p className="text-sm font-medium truncate">{file.title}</p>
    <p className="text-xs text-muted-foreground">
      {formatBytes(file.size)} · {formatDate(file.uploaded_at)}
    </p>
  </div>
  <CategoryBadge category={file.category} />
  <ChevronRight className="size-4 text-muted-foreground" />
</button>
```

`tabular-nums` on the size column when shown in a column-aligned
layout. See `02-typography.md`.

## Viewer

Two viewer routes (and matching components):

- `/g/:slug/files/:id` for full-page edit (rename, change category,
  attach to commodities).
- A `Sheet`-based preview for in-context viewing (tap a thumbnail in
  the commodity detail).

Both use the same internal viewer component (`ImageViewer`,
`PdfViewer`). Image viewer:

- Pinch-zoom on touch.
- `Cmd+0` resets zoom.
- Arrow keys navigate sibling files in the same commodity.
- Escape closes (Sheet) or returns to commodity detail (full page).

PDF viewer uses `pdfjs-dist` lazy-loaded — see
`frontend/src/lib/pdfjs.ts` for the worker shimming. PDF viewer:

- Page selector.
- Search within document.
- Download (always available).
- Print (browser print, no custom).

## Upload

The drop zone is the one place dashed borders are allowed:

```tsx
<div
  onDrop={…}
  onDragOver={…}
  className={cn(
    "rounded-xl border-2 border-dashed border-border p-10 text-center transition-colors",
    isDragOver && "border-primary bg-primary/5",
    isError && "border-destructive bg-destructive/5"
  )}
>
  <Upload className="mx-auto size-8 text-muted-foreground" />
  <p className="mt-3 text-sm font-medium">{t("files:upload.title")}</p>
  <p className="mt-1 text-xs text-muted-foreground">
    {t("files:upload.hint")}
  </p>
  <Button size="sm" variant="outline" className="mt-4">
    {t("files:upload.browse")}
  </Button>
</div>
```

Upload behavior:

- Multi-file drop allowed.
- Per-file progress indicator (a thin progress bar inside the file row).
- On success, the file appears in the list with a brief
  `bg-status-active/10` flash; the toast is a single grouped success
  ("3 files added"), not one per file.
- On failure, the file row stays with a destructive-tinted background
  and a retry button. No toast spam.

## Format and size limits

The backend enforces (and surfaces through error messages):

- Max single-file size: 50 MB.
- Allowed MIME types: image/* (jpeg, png, webp, heic), application/pdf,
  text/plain, application/zip.
- Per-tenant quota.

The UI surfaces these limits in the upload zone's hint text. Don't
hard-code the size in the component — read from the seed config.

## Image processing

Thumbnails are generated on the backend (a worker resizes to 256×256
WEBP). The frontend always reads `file.thumbnail_url` and falls back
to the category icon when missing. Don't compose thumbnails on the
client — the image bytes might be 8 MB.

## EXIF / orientation

Backend strips EXIF on upload (privacy + orientation correctness).
The displayed image is always upright; no rotation logic in the
frontend.

## Empty / loading / error

| State | UI |
| --- | --- |
| No files yet | Empty-state pattern with the `Image` icon and "Drop files here, or click Browse." |
| Loading first page | Skeleton grid of 6 `aspect-square` `bg-muted` tiles |
| Page query failed | Inline retry banner inside the gallery container |
| Upload in flight (per-file) | The file row shows a thin progress bar; the count appears in the upload zone |
| Upload failed (per-file) | The file row gets a destructive tint + a retry button |
| File deleted (in another tab / by another user) | Quietly remove from the list; no error |

## Multi-select

Long-press on a thumbnail (or a click on the row checkbox) enters
selection mode. Bulk actions toolbar slides in below the page header
("3 files selected" + "Delete", "Move to commodity"). Same pattern
as commodities multi-select — see `08-interaction-states.md`.

## Hard rules

1. **Four categories, no more.** `image | invoice | document | other`.
2. **Thumbnails from the backend.** No client-side resizing.
3. **EXIF stripped server-side.** Don't replicate the rotation logic
   on the client.
4. **Drop zone is the only dashed border in the app.** The dashed
   `border-2` is a visual cue specifically for "drop targets here";
   don't reach for it elsewhere.
5. **Upload toast is grouped.** "3 files added", not three separate
   toasts.
6. **Multi-file errors are inline.** No toast spam on partial-failure.

## Anti-patterns

- A "video" category. Files that are videos go in `other`.
- A toast per uploaded file. Group by batch.
- A custom "modern thumbnail" with rounded corners *and* a shadow.
  Use the canonical thumbnail style.
- Reading `file.thumbnail_data` (base64) — the backend doesn't return
  base64 thumbnails. URL only.
- A "download all" zip-button generated client-side. Use the
  backup/restore export path instead.

## Cross-refs

- File-related routes: `routing.md` (parent folder).
- Backend file model: `go/models/file.go` (`models.FileCategory`).
- Mock canonical: `denisvmedia/inventario-design/CLAUDE.md` (no
  dedicated section; the four-category model is encoded in the
  design's data shape).
- Backup / restore (the related-but-distinct domain): the exports
  feature slice.
