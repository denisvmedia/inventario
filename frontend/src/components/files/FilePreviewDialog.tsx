import { useTranslation } from "react-i18next"
import { Download, File as FileIcon, Trash2 } from "lucide-react"

import { ImageViewer } from "@/components/files/ImageViewer"
import { PdfViewer } from "@/components/files/PdfViewer"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type { ListedFile } from "@/features/files/api"
import { isImageMime, isPdfMime } from "@/features/files/constants"
import { formatBytes } from "@/lib/intl"

// FilePreviewDialog mirrors `design-mocks/src/components/FilePreviewDialog.tsx`
// 1:1: it is the in-place preview that opens when a user clicks an
// attached file from a commodity / location detail surface. Three
// shapes by MIME:
//
//   - PDF  → fullscreen Dialog hosting our `PdfViewer` (paged canvas
//            + zoom + download).
//   - Image → fullscreen overlay hosting our `ImageViewer`
//            (mouse-wheel zoom + click-and-drag pan).
//   - Other → small dialog with title / size / tags + Download +
//            Delete affordances. The user can still grab the file
//            via the signed URL when no preview is possible.
//
// Delete confirmation is delegated to the parent (which wraps it with
// `useConfirm` so the project's destructive-confirm UX is reused);
// the dialog itself only fires `onDelete` and closes.
export interface FilePreviewDialogProps {
  // The row to preview, or null to close the dialog.
  file: ListedFile | null
  onClose: () => void
  // Optional. When supplied, the dialog renders the destructive Delete
  // affordance and forwards the click to the caller. Caller is
  // responsible for the confirm + mutation; the dialog closes itself
  // straight after `onDelete` is invoked.
  onDelete?: (fileId: string) => void
}

export function FilePreviewDialog({ file, onClose, onDelete }: FilePreviewDialogProps) {
  const { t } = useTranslation()
  if (!file) return null

  const mime = file.file.mime_type
  const title = file.file.title?.trim() || file.file.path?.trim() || file.file.id
  const downloadName = file.file.original_path || file.file.path || title
  const downloadUrl = file.signedUrl?.url

  function handleDelete() {
    if (!onDelete || !file) return
    onDelete(file.file.id)
    onClose()
  }

  if (isImageMime(mime) && downloadUrl) {
    // ImageViewer is already a self-contained fullscreen overlay; we
    // pass through `open` + `onOpenChange` and let it render the
    // toolbar (zoom / pan / close + an optional delete pill when
    // `onDelete` is wired). Skipping the viewer when `downloadUrl`
    // is missing avoids handing the viewer an empty `<img src="">`
    // (which loads the current page); we fall through to the
    // catch-all branch below in that case so the user still sees
    // metadata + (optional) delete UI.
    return (
      <ImageViewer
        open
        onOpenChange={(next) => {
          if (!next) onClose()
        }}
        url={downloadUrl}
        alt={title}
        onDelete={onDelete ? handleDelete : undefined}
      />
    )
  }

  if (isPdfMime(mime)) {
    // PDF gets a dedicated fullscreen Dialog: `PdfViewer` is a panel
    // (toolbar + canvas), so the wrapping Dialog does the chrome —
    // backdrop, focus trap, Esc handler. The Dialog's auto-rendered
    // close button is hidden via `[&>button]:hidden` to mirror the
    // mock; the toolbar inside `PdfViewer` already exposes navigation
    // and the user can press Esc to close.
    return (
      <Dialog
        open
        onOpenChange={(next) => {
          if (!next) onClose()
        }}
      >
        <DialogContent
          className="h-screen w-screen max-w-none gap-0 rounded-none border-0 p-0 [&>button]:hidden"
          data-testid="file-preview-dialog-pdf"
        >
          <DialogHeader className="sr-only">
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{t("files:detail.metadataTitle")}</DialogDescription>
          </DialogHeader>
          <div className="flex h-full flex-col overflow-hidden bg-background p-4">
            <div className="flex items-center gap-3 pb-3">
              <p className="line-clamp-1 flex-1 text-sm font-medium" title={title}>
                {title}
              </p>
              {onDelete ? (
                <Button
                  variant="ghost"
                  size="sm"
                  className="gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive"
                  onClick={handleDelete}
                  data-testid="file-preview-dialog-pdf-delete"
                >
                  <Trash2 className="size-3.5" aria-hidden="true" />
                  {t("files:detail.delete")}
                </Button>
              ) : null}
              <Button
                variant="outline"
                size="sm"
                onClick={onClose}
                data-testid="file-preview-dialog-close"
              >
                {t("files:viewer.close", { defaultValue: "Close" })}
              </Button>
            </div>
            <div className="flex-1 overflow-auto">
              {downloadUrl ? (
                <PdfViewer url={downloadUrl} />
              ) : (
                <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                  {t("files:detail.previewLoadError")}
                </div>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>
    )
  }

  // Catch-all: small dialog for non-previewable types. Mirrors the
  // `type === "other"` branch in the mock — file metadata + a single
  // Download CTA + optional Delete.
  const sizeLabel = file.file.size_bytes ? formatBytes(file.file.size_bytes) : null
  const tags = file.file.tags ?? []
  return (
    <Dialog
      open
      onOpenChange={(next) => {
        if (!next) onClose()
      }}
    >
      <DialogContent className="sm:max-w-md" data-testid="file-preview-dialog-other">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
            {sizeLabel ? <span>{sizeLabel}</span> : null}
            {sizeLabel && tags.length > 0 ? <span aria-hidden="true">·</span> : null}
            {tags.slice(0, 4).map((tag) => (
              <Badge key={tag} variant="secondary" className="h-4 px-1.5 text-[10px]">
                {tag}
              </Badge>
            ))}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col items-center justify-center gap-4 rounded-xl border border-border bg-muted/40 px-6 py-8 text-center">
          <div className="flex size-14 items-center justify-center rounded-xl border border-border bg-background">
            <FileIcon className="size-7 text-muted-foreground" aria-hidden="true" />
          </div>
          <p className="max-w-xs text-xs text-muted-foreground">{t("files:detail.noPreview")}</p>
          {downloadUrl ? (
            <Button
              asChild
              size="sm"
              className="gap-1.5"
              data-testid="file-preview-dialog-download"
            >
              <a href={downloadUrl} download={downloadName} rel="noopener noreferrer">
                <Download className="size-3.5" aria-hidden="true" />
                {t("files:detail.download")}
              </a>
            </Button>
          ) : null}
        </div>
        {onDelete ? (
          <DialogFooter className="sm:justify-end">
            <Button
              variant="ghost"
              size="sm"
              className="gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive"
              onClick={handleDelete}
              data-testid="file-preview-dialog-other-delete"
            >
              <Trash2 className="size-3.5" aria-hidden="true" />
              {t("files:detail.delete")}
            </Button>
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
