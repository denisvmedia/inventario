import {
  File as FileIcon,
  FileArchive,
  FileImage,
  FileText,
  Files as FilesIcon,
  Receipt,
} from "lucide-react"
import type { ComponentType, SVGProps } from "react"

import type { FileCategory, FileEntity } from "./api"

type LucideIcon = ComponentType<SVGProps<SVGSVGElement>>

// The three user-meaningful buckets surfaced as tiles on the Files page.
// Order matches the design mock's "All / Photos / Documents / Other"
// pill row; the `all` synthetic tile is rendered alongside but isn't
// part of the FileCategory enum (the BE only filters by the three real
// values).
//
// #1622 collapsed the legacy `invoices` category into `documents` — the
// "this is an invoice" semantic is now carried by the conventional
// `invoice` tag (see FILE_TAG_PILLS below), reachable from the toolbar
// as a one-click filter.
export const FILE_CATEGORIES = ["images", "documents", "other"] as const

export type FileCategoryTile = "all" | FileCategory

export interface CategoryTile {
  key: FileCategoryTile
  icon: ComponentType<SVGProps<SVGSVGElement>>
  // Tailwind utility class for the icon foreground when this tile is
  // active. The whole point is a per-category accent — All=muted, the
  // four real buckets each get a distinct hue so the active tile reads
  // as "this category" at a glance.
  activeColor: string
  // Matching tinted background for the icon chip when active (uses the
  // same hue as `activeColor` at /10 opacity).
  activeBg: string
}

// Label + description for each tile go through the static helpers in
// ./labels.ts so the i18next-cli extractor can see literal `t(...)`
// keys. Keep the labels.ts switch in sync with this list.
export const FILE_CATEGORY_TILES: CategoryTile[] = [
  {
    key: "all",
    icon: FilesIcon,
    activeColor: "text-muted-foreground",
    activeBg: "bg-muted",
  },
  {
    key: "images",
    icon: FileImage,
    activeColor: "text-status-active",
    activeBg: "bg-status-active/10",
  },
  {
    key: "documents",
    icon: FileText,
    activeColor: "text-chart-3",
    activeBg: "bg-chart-3/10",
  },
  {
    key: "other",
    icon: FileIcon,
    activeColor: "text-chart-4",
    activeBg: "bg-chart-4/10",
  },
]

// Curated tag pills shown in the Files toolbar as quick filters. The
// real BE filters by exact tag string (`tags @> $`), so each pill's
// `id` is the literal tag value used to match. Until the proper Tags
// entity (#1400) exists, this list is the canonical taxonomy surfaced
// in the toolbar; arbitrary user-supplied tags still show on cards
// but aren't reachable via the toolbar — see #1538 design deviation.
// Label per tag goes through useTagPillLabel in ./labels.ts so the
// i18next-cli extractor can see literal `t(...)` keys. Keep the
// labels.ts switch in sync with this list.
export interface FileTagPill {
  id: "invoice" | "warranty" | "manual" | "photo" | "certificate" | "backup"
  // Tailwind text utility for the pill label and the inline tag chip
  // rendered next to a file's title in the list/grid views. Mirrors
  // the mock's `FILE_TAGS[].color`.
  colorClass: string
}

export const FILE_TAG_PILLS: FileTagPill[] = [
  { id: "invoice", colorClass: "text-chart-1" },
  { id: "warranty", colorClass: "text-status-active" },
  { id: "manual", colorClass: "text-chart-3" },
  { id: "photo", colorClass: "text-status-expiring" },
  { id: "certificate", colorClass: "text-chart-2" },
  { id: "backup", colorClass: "text-muted-foreground" },
]

// Mirrors models.FileCategoryFromMIME on the BE — used to suggest a
// category after the user drops a file into the upload form so the
// dropdown defaults to something sensible (the BE then decides
// authoritatively from MIME at write time, but a matching default keeps
// the metadata step from showing an obviously-wrong selection).
export function categoryFromMime(mime: string | undefined): FileCategory {
  if (!mime) return "other"
  if (mime.startsWith("image/")) return "images"
  if (
    mime === "application/pdf" ||
    mime.startsWith("text/") ||
    mime === "application/msword" ||
    mime === "application/json" ||
    mime.startsWith("application/vnd.ms-") ||
    mime.startsWith("application/vnd.openxmlformats-")
  ) {
    return "documents"
  }
  return "other"
}

// Visual descriptor for a file's leading icon — drives both the
// FileCard fallback (when there is no thumbnail) and the leading icon on
// the FileListRow. Mirrors design-mocks/src/views/FileBrowserView.tsx
// `mimeIconAndColor` so the two surfaces stay in lock-step with the
// design contract: per-MIME tokens for the four hot paths (image / pdf /
// archive) and a per-category fallback for everything else.
//
// Tokens-only, per the design language: `text-status-active`,
// `text-status-expired`, `text-chart-*`, `text-muted-foreground` paired
// with `bg-*/10` (or `bg-muted` for the neutral fallback).
export interface FileVisualMeta {
  icon: LucideIcon
  colorClass: string
  bgClass: string
  // Stable identifier for the bucket — useful in tests and as a
  // `data-mime-group` attribute so we can assert "this card uses the
  // PDF palette" without coupling to the exact Tailwind utility.
  group: "image" | "pdf" | "archive" | "invoice" | "document" | "other"
}

export function getFileVisualMeta(
  file: Pick<FileEntity, "mime_type" | "category" | "tags">
): FileVisualMeta {
  const mime = file.mime_type ?? ""
  if (mime.startsWith("image/")) {
    return {
      icon: FileImage,
      colorClass: "text-status-active",
      bgClass: "bg-status-active/10",
      group: "image",
    }
  }
  // Post-#1622 the "invoice" semantic lives on a tag, not on the
  // FileCategory enum. Tag-based detection slides in here so the
  // Receipt glyph + chart-1 palette still surfaces on invoice-tagged
  // PDFs / docs — keeps the per-MIME palette contract from #1659.
  const tags = Array.isArray(file.tags) ? file.tags : []
  if (tags.includes("invoice")) {
    return {
      icon: Receipt,
      colorClass: "text-chart-1",
      bgClass: "bg-chart-1/10",
      group: "invoice",
    }
  }
  if (mime === "application/pdf") {
    return {
      icon: FileText,
      colorClass: "text-status-expired",
      bgClass: "bg-status-expired/10",
      group: "pdf",
    }
  }
  if (mime.includes("zip") || mime.includes("archive")) {
    return {
      icon: FileArchive,
      colorClass: "text-chart-4",
      bgClass: "bg-chart-4/10",
      group: "archive",
    }
  }
  switch (file.category) {
    case "images":
      return {
        icon: FileImage,
        colorClass: "text-status-active",
        bgClass: "bg-status-active/10",
        group: "image",
      }
    case "documents":
      return {
        icon: FileText,
        colorClass: "text-chart-3",
        bgClass: "bg-chart-3/10",
        group: "document",
      }
    default:
      return {
        icon: FileIcon,
        colorClass: "text-muted-foreground",
        bgClass: "bg-muted",
        group: "other",
      }
  }
}

// Whether a file MIME is renderable inline by a plain <img> tag. Used by
// the detail view to decide between the image preview block and the
// generic "download to view" placeholder.
export function isImageMime(mime: string | undefined): boolean {
  return !!mime && mime.startsWith("image/")
}

// PDFs render via the browser's native <embed> in the detail view. A
// follow-up PR will swap this for a pdfjs-dist canvas viewer (port of
// the legacy frontend/src/components/PDFViewerCanvas.vue) so we get
// page nav + zoom + custom controls.
export function isPdfMime(mime: string | undefined): boolean {
  return mime === "application/pdf"
}
