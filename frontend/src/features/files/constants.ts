import { File as FileIcon, FileImage, FileText, Files as FilesIcon, Receipt } from "lucide-react"
import type { ComponentType, SVGProps } from "react"

import type { FileCategory } from "./api"

// The four user-meaningful buckets surfaced as tiles on the Files page.
// Order matches the design mock's "All / Photos / Invoices / Documents
// / Other" pill row; the `all` synthetic tile is rendered alongside but
// isn't part of the FileCategory enum (the BE only filters by the four
// real values).
export const FILE_CATEGORIES = ["images", "invoices", "documents", "other"] as const

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
    key: "invoices",
    icon: Receipt,
    activeColor: "text-chart-1",
    activeBg: "bg-chart-1/10",
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
