import { useTranslation } from "react-i18next"
import { File as FileIcon, Maximize2 } from "lucide-react"

import { PdfViewer } from "@/components/files/PdfViewer"
import { FadeInImage } from "@/components/ui/fade-in-image"
import { isImageMime, isPdfMime } from "@/features/files/constants"
import { cn } from "@/lib/utils"

export interface FilePreviewProps {
  mime?: string
  url?: string
  alt: string
  onExpandImage?: () => void
  onExpandPdf?: () => void
  // "sheet" bleeds an image to the edges of the detail panel, whose body is
  // inset by px-5. "pane" keeps it inside a bordered box, for a column on a
  // page.
  variant?: "sheet" | "pane"
}

// Inline preview of one file: an image (click to expand), a PDF reader, or a
// "cannot preview, download to view" card for every other type and for a
// previewable file whose signed URL is missing. Callers offer Open and
// Download next to it.
export function FilePreview({
  mime,
  url,
  alt,
  onExpandImage,
  onExpandPdf,
  variant = "sheet",
}: FilePreviewProps) {
  const { t } = useTranslation()

  if (isImageMime(mime) && url) {
    return (
      <button
        type="button"
        onClick={onExpandImage}
        // `min-h` reserves a box for the fade-in shimmer and centers a short
        // `object-contain` image so the preview doesn't grow from a
        // zero-height placeholder.
        className={cn(
          "group relative flex min-h-[12rem] items-center justify-center overflow-hidden bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          variant === "sheet"
            ? "-mx-5 w-[calc(100%+2.5rem)] focus-visible:ring-inset"
            : "w-full rounded-xl border border-border"
        )}
        data-testid="file-preview-image-trigger"
      >
        <FadeInImage
          src={url}
          alt={alt}
          className={cn(
            "w-full object-contain",
            variant === "sheet" ? "max-h-[60vh]" : "max-h-[75vh]"
          )}
          data-testid="file-preview-image"
        />
        <span className="pointer-events-none absolute inset-x-0 bottom-0 flex items-center justify-end gap-1 bg-gradient-to-t from-black/60 to-transparent p-2 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100">
          <Maximize2 className="size-3.5" aria-hidden="true" />
        </span>
      </button>
    )
  }

  if (isPdfMime(mime) && url) {
    return <PdfViewer url={url} onRequestFullscreen={onExpandPdf} />
  }

  return (
    <div
      className="flex flex-col items-center justify-center gap-3 rounded-xl border border-border bg-muted/40 px-6 py-10 text-center"
      data-testid="file-detail-no-preview"
    >
      <div className="flex size-14 items-center justify-center rounded-xl border border-border bg-background">
        <FileIcon className="size-7 text-muted-foreground" aria-hidden="true" />
      </div>
      <p className="max-w-xs text-xs text-muted-foreground">{t("files:detail.noPreview")}</p>
    </div>
  )
}
