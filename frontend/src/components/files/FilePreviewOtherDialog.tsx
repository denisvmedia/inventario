import { useTranslation } from "react-i18next"
import { Download, File as FileIcon, Trash2 } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog"
import type { FileEntity } from "@/features/files/api"
import { formatBytes, formatDateTime } from "@/lib/intl"

export interface FilePreviewOtherDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  file: FileEntity & { id: string }
  signedUrl?: string
  onDelete: () => void
  deletePending: boolean
}

// Small focused Dialog for files whose MIME type can't be previewed in
// the browser (everything outside image/* and application/pdf). Mirrors
// `design-mocks/src/components/FilePreviewDialog.tsx` "other" branch:
// filename header + meta strip · centred "cannot preview" card with a
// Download CTA · destructive ghost button. The fullscreen Sheet path
// stays the home for image/PDF — see FileDetailSheet.
export function FilePreviewOtherDialog({
  open,
  onOpenChange,
  file,
  signedUrl,
  onDelete,
  deletePending,
}: FilePreviewOtherDialogProps) {
  const { t } = useTranslation()

  const filename = file.path ? `${file.path}${file.ext ?? ""}` : ""
  const headerLabel = file.title?.trim() || filename || file.id
  const size =
    typeof file.size_bytes === "number" && file.size_bytes > 0 ? formatBytes(file.size_bytes) : null
  const uploaded = file.created_at ? formatDateTime(file.created_at) : null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md" data-testid="file-preview-other-dialog">
        <DialogTitle className="sr-only">{t("files:detail.metadataTitle")}</DialogTitle>
        <DialogDescription className="sr-only">{t("files:detail.noPreview")}</DialogDescription>

        <div className="flex items-start gap-3 pr-6">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-muted">
            <FileIcon className="size-5 text-muted-foreground" aria-hidden="true" />
          </div>
          <div className="min-w-0 flex-1">
            <p
              className="truncate text-base font-semibold leading-snug"
              data-testid="file-preview-other-filename"
            >
              {headerLabel}
            </p>
            <p className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
              {size ? <span data-testid="file-preview-other-size">{size}</span> : null}
              {size && uploaded ? <span aria-hidden="true">·</span> : null}
              {uploaded ? (
                <span data-testid="file-preview-other-uploaded">
                  {t("files:detail.uploadedAt")} {uploaded}
                </span>
              ) : null}
              {file.tags && file.tags.length > 0
                ? file.tags.map((tag) => (
                    <Badge key={tag} variant="secondary" className="h-4 px-1.5 text-[10px]">
                      {tag}
                    </Badge>
                  ))
                : null}
            </p>
          </div>
        </div>

        <div className="mt-1 flex flex-col items-center justify-center gap-4 rounded-xl border border-border bg-muted/40 px-6 py-8 text-center">
          <div className="flex size-14 items-center justify-center rounded-xl border border-border bg-background">
            <FileIcon className="size-7 text-muted-foreground" aria-hidden="true" />
          </div>
          <p className="max-w-xs text-xs text-muted-foreground">{t("files:detail.noPreview")}</p>
          {signedUrl ? (
            <Button asChild size="sm" className="gap-1.5">
              <a href={signedUrl} download data-testid="file-preview-other-download">
                <Download className="size-3.5" aria-hidden="true" />
                {t("files:detail.download")}
              </a>
            </Button>
          ) : null}
        </div>

        <div className="flex justify-end pt-1">
          <Button
            variant="ghost"
            size="sm"
            className="gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive"
            onClick={onDelete}
            disabled={deletePending}
            data-testid="file-preview-other-delete"
          >
            <Trash2 className="size-3.5" aria-hidden="true" />
            {t("files:detail.delete")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
