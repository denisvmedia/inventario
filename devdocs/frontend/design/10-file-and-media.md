# File & Media Handling

Inventario stores files heavily — receipts, warranty PDFs, product photos, manuals. The current FileViewerDialog (`11-file-viewer-dialog` in screenshots) is mediocre. This document specifies the full media experience.

## File-type taxonomy

Inventario surfaces care about five media classes:

| Class | Examples | Preview support |
| --- | --- | --- |
| **Image** | jpg, png, webp, gif, heic, avif | Full preview, zoom, rotate |
| **Document** | pdf | Multi-page preview, navigation, zoom |
| **Video** | mp4, webm, mov | Inline player, fullscreen, scrubber |
| **Audio** | mp3, wav, m4a | Inline player with waveform |
| **Other** | xml, json, txt, archives, dwg, etc. | Type-specific icon, "Download to view" |

Internal classification is on MIME type, not extension.

## Thumbnail rules

Generated server-side, cached. Sizes:

| Use | Pixel size |
| --- | --- |
| List item icon | 24×24 |
| Card thumbnail | 320×240 (16:9 cover) |
| Gallery tile | 480×360 |
| Detail preview | 1024 longest edge |

For non-image files, thumbnail is a styled tile with file-type icon (Phosphor) on `--surface-sunken` background, file extension label below.

## File Card primitive (revised from `09-component-patterns.md` Card)

Specialized card for file representation.

### Anatomy
```
┌─────────────────────────────┐
│                             │
│      [thumbnail | icon]     │  240×180 viewport (16:9)
│                             │
├─────────────────────────────┤
│ filename-or-display.ext     │  body, weight medium, truncate
│ Document · PDF · 1.2 MB     │  body-xs, --ink-muted
│ [linked-entity badge]       │  optional
└─────────────────────────────┘
```

### Hover state
- Card gets shadow `xs` and translateY(-1px)
- Thumbnail darkens slightly (`bg-overlay 0.05`)
- Action overlay appears: View / Download / More-menu

### Multi-select state
- Checkbox appears top-left
- Selected: blue ring inside card, accent-soft tint

## File Gallery (replacement for current MediaGallery)

The pattern for displaying multiple files attached to an entity (commodity images, manuals, invoices).

### Layout
- CSS grid `auto-fill minmax(220px, 1fr)` with `gap: --space-4`
- Cards maintain aspect ratio
- "Add" tile at the start (or end on read-only views) with dashed border

### Density modes
| Mode | Min tile size | Per-row at 1200px |
| --- | --- | --- |
| Default | 220px | 5 |
| Compact | 160px | 7 |
| Cozy | 280px | 4 |

### Empty state
```
[illustration: empty drawer or paper stack]
No images yet.

Add your first image to keep a visual record —
helpful for insurance and just for memory.

[+ Add Image]
```

### Loading state
6 skeleton cards in a grid until data arrives.

## File Viewer (fullscreen lightbox)

The big rebuild. Current dialog is small and underfeatured. Replace with a fullscreen lightbox.

### Layout
```
┌─────────────────────────────────────────────────────────────┐
│ [×] [filename · type · size]    [↺] [↻] [Download] [Delete] │  top toolbar
│                                                             │
│                                                             │
│                                                             │
│   ‹                  [media canvas]                       › │  navigation arrows
│                                                             │
│                                                             │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  ▣ ▢ ▢ ▢ ▢ ▢ ▢ ▢ ▢                              [3 / 27]    │  thumbnail strip + counter
└─────────────────────────────────────────────────────────────┘
```

### Toolbar (top)
- Close (X) far left, escapable
- Filename, type, size — center-left, body-sm muted
- Rotate left / right (images only)
- Zoom controls (images, PDFs)
- Download button
- Overflow menu (...) — Edit metadata, Delete, Share, Set as cover

### Canvas
- Background: `--overlay-scrim-strong` (near-black) with backdrop-blur
- Image / PDF / video centered, max 90% viewport in each direction
- Pan & zoom enabled for images and PDFs (via panzoom library)
- Double-click to zoom 2x; double-click again to fit
- Pinch-to-zoom on touch
- Mousewheel zoom on desktop
- Drag-to-pan when zoomed

### Navigation
- Left/right arrow keys → prev/next file
- Edge arrow buttons (visible on hover only)
- Touch swipe horizontally → prev/next

### Thumbnail strip (bottom)
- Horizontal scroller, current file highlighted
- Click thumbnail → jump to that file
- Auto-scrolls to keep current visible
- Hides on viewport <640px (use swipe only)

### Counter
- Right-aligned: "3 / 27"
- `body-sm tabular`

### Per-type behavior

| Type | Special features |
| --- | --- |
| Image | rotate, zoom, exif overlay (icon: Info → tooltip showing camera/date) |
| PDF | page nav (◂ N/M ▸), page thumbnails in expandable side panel, search within PDF (deferred) |
| Video | full HTML5 controls, play/pause/scrub/volume/captions, picture-in-picture |
| Audio | waveform render, play/pause, time display, no thumbnail strip |
| Other | replace canvas with "Preview not available" + Download CTA |

### Accessibility
- Focus trap inside viewer
- Tab order: close → toolbar buttons → canvas (focusable for keyboard pan/zoom) → thumbnail strip
- Esc closes
- Arrow keys: prev/next
- Plus/minus: zoom in/out
- 0: reset zoom
- R: rotate (images)
- Space: play/pause (audio/video)
- Announce file change to screen reader (`aria-live="polite"`)

### Implementation
- Reka UI Dialog with `inset-4` (full screen with subtle margin)
- Zoom: panzoom (~6KB)
- Touch gestures: `@vueuse/gesture`
- PDF: pdf.js (already in project)

### Open-from
- Click on FileCard thumbnail → opens viewer at that file
- Click on file in FileListView → opens viewer (currently navigates to detail page; change to lightbox)
- Cmd/Ctrl+click → still goes to detail page (preserves drill-down)

### File detail page (separate from viewer)

The file detail page (`/files/<id>`) remains for **metadata editing** and as a permalink. The viewer is the **viewing** experience. Keep them separate but linked.

## File Uploader

Specced separately because upload is a distinct flow.

### States
1. Idle / drop-zone
2. Drag-over
3. Uploading (progress per file)
4. Done
5. Error

### Idle anatomy
```
┌─────────────────────────────────────┐
│                                     │
│        [cloud-up icon, 48px]        │
│                                     │
│       Drag and drop a file here     │
│                  or                 │
│           click to browse           │
│                                     │
│  Supports images, PDFs, and more.   │
│                                     │
└─────────────────────────────────────┘
```

- Dashed border `--border-default`, radius `lg`, padding `padding-card-lg`
- Center-aligned
- Click anywhere triggers file picker
- "click to browse" link styled as `text-accent underline hover:no-underline`

### Drag-over anatomy
- Dashed border becomes solid `--accent`, 2px
- bg `--accent-soft`
- Cloud icon scales 1.05, color `--accent`
- Copy changes to "Drop to upload"

### Uploading anatomy

When files dropped, the drop-zone collapses and file rows appear:
```
┌─────────────────────────────────────────────────┐
│ [icon] receipt-2026.pdf                         │
│        ▓▓▓▓▓▓▓▓░░░░░░  43% · 1.2 MB / 2.8 MB   │
│                                          [×]    │
└─────────────────────────────────────────────────┘
```

Multiple files stack vertically. Progress per file. Cancel (X) per file.

### Error
- Failed file rows turn red border + error message ("Couldn't upload — try again." with [Retry] button)

### Validation
- Max file size per upload (configurable, server-side enforced; UI hints at limits — "up to 50 MB")
- Allowed types per context (commodity images: image/*; manuals: pdf + images; etc.)
- Show validation errors inline before send if client-detectable

### Multi-file upload
- Allow drop or pick of many files
- Process in parallel (concurrent 3 by default; queue rest)
- Progress aggregate at the top: "Uploading 3 of 7…"

## Linked-entity badge (file → owner)

Files have an "owner" entity (commodity, location, area). On the file card and file detail page, the owner is shown as a clickable badge.

### Anatomy
```
[entity-icon] Camping Equipment ↗
```

- bg `--accent-soft`, ink `--accent`, radius `md`, padding `space-1 space-2`
- Trailing external-link icon (Phosphor ArrowSquareOut)
- Hover: bg `--accent-hover-soft`

## Where file flows live in the app

| Surface | Behavior |
| --- | --- |
| `/files` list | All files; click → viewer; ⌘+click → detail page |
| Commodity detail Images section | FileGallery, click → viewer scoped to commodity images |
| Commodity detail Manuals/Invoices | Same, scoped per file-type bucket |
| Location detail Images/Files | Same |
| File detail `/files/<id>` | Metadata + preview-in-place (smaller, not lightbox) |

## What ships in sprint 0

1. Replace MediaGallery internals with new grid spec (auto-fill minmax)
2. Refactor FileCard to new anatomy with hover overlay
3. Build new FileViewer fullscreen component (replaces current dialog) — 4–6 days
4. Migrate file uploader to new states (idle/drag/uploading/done/error)
5. Add linked-entity badge to file cards everywhere

## Decision needed

None. All spec calls are made.
