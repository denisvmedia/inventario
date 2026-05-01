import { useTranslation } from "react-i18next"
import { Download, ExternalLink, Pencil, Trash2 } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { useDeleteFile, useFile } from "@/features/files/hooks"
import { isImageMime, isPdfMime } from "@/features/files/constants"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatDateTime } from "@/lib/intl"

// Right-side drawer surfaced from the list page when the user opens a
// file. Renders an inline preview (image via <img>, PDF via <embed>,
// other types fall back to a "Download to view" placeholder), the
// metadata block, and the action row (Open in new tab / Download /
// Edit / Delete). Edit deep-links to the standalone edit page.
export interface FileDetailSheetProps {
  fileId: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onEdit: (id: string) => void
}

export function FileDetailSheet({ fileId, open, onOpenChange, onEdit }: FileDetailSheetProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const confirm = useConfirm()
  const query = useFile(fileId ?? undefined, { enabled: open && !!fileId })
  const deleteMutation = useDeleteFile()

  const file = query.data?.file
  const signedUrl = query.data?.signedUrl?.url
  const title = file?.title?.trim() || file?.path?.trim() || file?.id || ""
  const filename = file && file.path && file.ext ? `${file.path}${file.ext}` : file?.path

  async function onDelete() {
    if (!file) return
    const ok = await confirm({
      title: t("files:detail.deleteConfirm.title"),
      description: t("files:detail.deleteConfirm.description", { title }),
      confirmLabel: t("files:detail.deleteConfirm.confirm"),
      destructive: true,
    })
    if (!ok) return
    try {
      await deleteMutation.mutateAsync(file.id)
      toast.success(t("files:detail.deleteConfirm.confirm"))
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="flex w-full max-w-xl flex-col gap-4 overflow-y-auto sm:max-w-2xl"
        data-testid="file-detail-sheet"
      >
        <SheetHeader>
          <SheetTitle>{t("files:detail.metadataTitle")}</SheetTitle>
          <SheetDescription className="line-clamp-2">{title || "—"}</SheetDescription>
        </SheetHeader>

        {query.isLoading ? (
          <div className="flex flex-col gap-3">
            <Skeleton className="aspect-[4/3] w-full" />
            <Skeleton className="h-6 w-2/3" />
            <Skeleton className="h-4 w-1/2" />
          </div>
        ) : query.error ? (
          <Alert variant="destructive">
            <AlertTitle>
              {t("common:errors.generic", { defaultValue: "Something went wrong" })}
            </AlertTitle>
            <AlertDescription>{(query.error as Error).message}</AlertDescription>
          </Alert>
        ) : file ? (
          <>
            <FilePreview
              mime={file.mime_type}
              url={signedUrl}
              alt={title}
            />
            <dl className="grid grid-cols-1 gap-x-4 gap-y-2 text-sm sm:grid-cols-[120px_1fr]">
              <dt className="text-muted-foreground">{t("files:detail.filename")}</dt>
              <dd className="break-all" data-testid="file-detail-filename">{filename ?? "—"}</dd>

              <dt className="text-muted-foreground">{t("files:detail.category")}</dt>
              <dd>
                <Badge variant="secondary" data-testid="file-detail-category">
                  {file.category ?? "—"}
                </Badge>
              </dd>

              <dt className="text-muted-foreground">{t("files:detail.type")}</dt>
              <dd>{file.type ?? "—"}</dd>

              <dt className="text-muted-foreground">{t("files:detail.mimeType")}</dt>
              <dd className="break-all">{file.mime_type ?? "—"}</dd>

              {file.linked_entity_type ? (
                <>
                  <dt className="text-muted-foreground">{t("files:detail.linkedEntity")}</dt>
                  <dd className="break-all">
                    {file.linked_entity_type}
                    {file.linked_entity_meta ? ` / ${file.linked_entity_meta}` : ""}
                  </dd>
                </>
              ) : null}

              {file.created_at ? (
                <>
                  <dt className="text-muted-foreground">{t("files:detail.uploadedAt")}</dt>
                  <dd>{formatDateTime(file.created_at)}</dd>
                </>
              ) : null}

              {file.tags && file.tags.length > 0 ? (
                <>
                  <dt className="text-muted-foreground">{t("files:detail.tags")}</dt>
                  <dd className="flex flex-wrap gap-1">
                    {file.tags.map((tag) => (
                      <Badge key={tag} variant="outline" className="text-xs">
                        {tag}
                      </Badge>
                    ))}
                  </dd>
                </>
              ) : null}

              {file.description ? (
                <>
                  <dt className="text-muted-foreground">
                    {t("files:edit.fields.description")}
                  </dt>
                  <dd className="whitespace-pre-line">{file.description}</dd>
                </>
              ) : null}
            </dl>
          </>
        ) : null}

        <SheetFooter className="mt-auto flex-row flex-wrap gap-2 sm:justify-end">
          {signedUrl ? (
            <>
              <Button asChild variant="outline" size="sm">
                <a href={signedUrl} target="_blank" rel="noreferrer" data-testid="file-detail-open">
                  <ExternalLink className="mr-2 size-4" aria-hidden="true" />
                  {t("files:detail.openInNewTab")}
                </a>
              </Button>
              <Button asChild variant="outline" size="sm">
                <a href={signedUrl} download data-testid="file-detail-download">
                  <Download className="mr-2 size-4" aria-hidden="true" />
                  {t("files:detail.download")}
                </a>
              </Button>
            </>
          ) : null}
          {file ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => onEdit(file.id)}
              data-testid="file-detail-edit"
            >
              <Pencil className="mr-2 size-4" aria-hidden="true" />
              {t("files:detail.edit")}
            </Button>
          ) : null}
          {file ? (
            <Button
              variant="destructive"
              size="sm"
              onClick={onDelete}
              disabled={deleteMutation.isPending}
              data-testid="file-detail-delete"
            >
              <Trash2 className="mr-2 size-4" aria-hidden="true" />
              {t("files:detail.delete")}
            </Button>
          ) : null}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

interface PreviewProps {
  mime?: string
  url?: string
  alt: string
}

function FilePreview({ mime, url, alt }: PreviewProps) {
  if (!url) return <div className="aspect-[4/3] w-full rounded-md bg-muted" aria-hidden="true" />

  if (isImageMime(mime)) {
    return (
      <img
        src={url}
        alt={alt}
        className="max-h-[60vh] w-full rounded-md object-contain"
        data-testid="file-preview-image"
      />
    )
  }

  if (isPdfMime(mime)) {
    return (
      <embed
        src={url}
        type="application/pdf"
        className="aspect-[4/5] w-full rounded-md border"
        data-testid="file-preview-pdf"
      />
    )
  }

  return (
    <div
      className="flex aspect-[4/3] w-full items-center justify-center rounded-md border bg-muted text-center text-sm text-muted-foreground"
      data-testid="file-preview-fallback"
    >
      <p className="max-w-xs px-4">
        {/* i18n via parent t() is fine; this component is internal */}
        Preview not available. Download to view.
      </p>
    </div>
  )
}
