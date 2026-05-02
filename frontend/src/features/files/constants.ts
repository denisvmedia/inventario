import { File as FileIcon, FileImage, FileText, Files as FilesIcon, Receipt } from "lucide-react"
import type { ComponentType, SVGProps } from "react"

import type { FileCategory } from "./api"

// The four user-meaningful buckets surfaced as tiles on the Files page.
// Order matches the design mock's "All / Photos / Invoices / Documents
// / Other" pill row; the `all` synthetic tile is rendered alongside but
// isn't part of the FileCategory enum (the BE only filters by the four
// real values).
export const FILE_CATEGORIES = ["photos", "invoices", "documents", "other"] as const

export type FileCategoryTile = "all" | FileCategory

export interface CategoryTile {
  key: FileCategoryTile
  i18nKey: string
  icon: ComponentType<SVGProps<SVGSVGElement>>
}

export const FILE_CATEGORY_TILES: CategoryTile[] = [
  { key: "all", i18nKey: "categoryAll", icon: FilesIcon },
  { key: "photos", i18nKey: "categoryPhotos", icon: FileImage },
  { key: "invoices", i18nKey: "categoryInvoices", icon: Receipt },
  { key: "documents", i18nKey: "categoryDocuments", icon: FileText },
  { key: "other", i18nKey: "categoryOther", icon: FileIcon },
]

// Mirrors models.FileCategoryFromMIME on the BE — used to suggest a
// category after the user drops a file into the upload form so the
// dropdown defaults to something sensible (the BE then decides
// authoritatively from MIME at write time, but a matching default keeps
// the metadata step from showing an obviously-wrong selection).
export function categoryFromMime(mime: string | undefined): FileCategory {
  if (!mime) return "other"
  if (mime.startsWith("image/")) return "photos"
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
