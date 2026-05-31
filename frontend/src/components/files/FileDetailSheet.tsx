import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { Download, ExternalLink, File as FileIcon, Maximize2, Pencil, Trash2 } from "lucide-react"

import { ImageViewer, type GalleryImage } from "@/components/files/ImageViewer"
import { PdfFullViewer } from "@/components/files/PdfFullViewer"
import { PdfViewer } from "@/components/files/PdfViewer"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useCurrentGroup } from "@/features/group/GroupContext"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { FadeInImage } from "@/components/ui/fade-in-image"
import { useDeleteFile, useFile } from "@/features/files/hooks"
import { isImageMime, isPdfMime } from "@/features/files/constants"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatDateTime } from "@/lib/intl"

// Entry point for the file detail surface. EVERY file — image, PDF, and
// any other MIME — opens the same right-side Sheet (#1962): image + PDF
// render an inline preview, everything else shows a "cannot preview,
// download to view" card in the same body. The metadata block + Open /
// Download / Edit / Delete action row are identical regardless of type.
// The fullscreen ImageViewer (gallery + zoom) portals above the Sheet so
// it's never trapped behind the panel.
export interface FileDetailSheetProps {
  fileId: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onEdit: (id: string) => void
  // Optional sibling list for the fullscreen image viewer's gallery
  // navigation. The list page populates this with the image rows in
  // the current filter view; deep-linked detail (where there's no
  // surrounding list) leaves it undefined and the viewer falls back
  // to single-image mode.
  imageSiblings?: GalleryImage[]
  onSelectSibling?: (id: string) => void
}

export function FileDetailSheet({
  fileId,
  open,
  onOpenChange,
  onEdit,
  imageSiblings,
  onSelectSibling,
}: FileDetailSheetProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const confirm = useConfirm()
  const query = useFile(fileId ?? undefined, { enabled: open && !!fileId })
  const deleteMutation = useDeleteFile()
  // Group slug for tag-chip filter links — clicking a tag in the
  // detail sheet routes to `/g/<slug>/files?tags=<tag>`, mirroring
  // the toolbar tag pills on FilesListPage. Falls back to a relative
  // link if the group context isn't available (e.g. the sheet
  // rendering inside a feature panel that doesn't hydrate it).
  const { currentGroup } = useCurrentGroup()
  const filesBase = currentGroup?.slug
    ? `/g/${encodeURIComponent(currentGroup.slug)}/files`
    : "/files"
  const [imageViewerOpen, setImageViewerOpen] = useState(false)
  // PDF gets the same expand-to-fullscreen affordance as images (#1963):
  // the inline viewer's toolbar button flips this, and a fullscreen Dialog
  // (a stacked layer, so it doesn't dismiss this Sheet) hosts a second viewer.
  const [pdfViewerOpen, setPdfViewerOpen] = useState(false)
  // The Sheet is always mounted by its callers (FilesListPage, EntityFilesPanel,
  // CommodityFilesTab) and just swaps `fileId`, so a fullscreen viewer left open
  // on one file would otherwise carry over to the next. Reset on file change.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setImageViewerOpen(false)
    setPdfViewerOpen(false)
  }, [fileId])

  const file = query.data?.file
  const signedUrl = query.data?.signedUrl?.url
  // Inline URL for "Open in new tab" — the BE serves it with
  // Content-Disposition: inline for preview-safe types (image / PDF /
  // text) and downloads the rest, so the browser views instead of
  // downloading (#1962). Falls back to the attachment URL when the BE
  // didn't mint an inline variant.
  const inlineUrl = query.data?.signedUrl?.inline_url ?? signedUrl
  const title = file?.title?.trim() || file?.path?.trim() || file?.id || ""
  // Backend may return path="" for files attached via the unified upload+linkage
  // flow (#1448), so gate on path being truthy — an empty path means we have no
  // displayable filename even if `ext` is set (otherwise we'd render a stray ".pdf").
  const filename = file?.path ? `${file.path}${file.ext ?? ""}` : ""

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
      toast.success(t("files:detail.deleteSuccess", { defaultValue: "File deleted" }))
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        {/* p-0 on the content: the header / body / footer each supply
            their own px-5 inset so the metadata isn't flush to the edges
            (#1962), while the image preview deliberately breaks back out
            to full-bleed. */}
        <SheetContent
          side="right"
          className="flex w-full max-w-xl flex-col gap-4 overflow-y-auto p-0 sm:max-w-2xl"
          data-testid="file-detail-sheet"
        >
          <SheetHeader className="px-5 pt-5 pb-0">
            <SheetTitle>{t("files:detail.metadataTitle")}</SheetTitle>
            <SheetDescription className="line-clamp-2">{title || "—"}</SheetDescription>
          </SheetHeader>

          <div className="flex flex-col gap-4 px-5">
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
                  onExpandImage={() => setImageViewerOpen(true)}
                  onExpandPdf={() => setPdfViewerOpen(true)}
                />
                <dl className="grid grid-cols-1 gap-x-4 gap-y-2 text-sm sm:grid-cols-[120px_1fr]">
                  <dt className="text-muted-foreground">{t("files:detail.filename")}</dt>
                  <dd className="break-all" data-testid="file-detail-filename">
                    {filename || "—"}
                  </dd>

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
                          // Tag chip routes back to the files list with the
                          // tag pre-applied — issue #1622 acceptance:
                          // "clicking a chip filters the list". Uses the
                          // toolbar's `?tags=` shape (FilesListPage reads
                          // splitTags) for back-compat.
                          <Badge key={tag} variant="outline" className="px-0 text-xs" asChild>
                            <Link
                              to={{
                                pathname: filesBase,
                                search: `?tags=${encodeURIComponent(tag)}`,
                              }}
                              onClick={() => onOpenChange(false)}
                              data-testid={`file-detail-tag-chip-${tag.toLowerCase()}`}
                              className="px-2 py-0.5"
                            >
                              {tag}
                            </Link>
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
          </div>

          <SheetFooter className="mt-auto flex-row flex-wrap gap-2 px-5 pb-5 sm:justify-end">
            {signedUrl ? (
              <>
                <Button asChild variant="outline" size="sm">
                  <a
                    href={inlineUrl}
                    target="_blank"
                    rel="noreferrer"
                    data-testid="file-detail-open"
                  >
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

      {/* Viewer lives OUTSIDE the Sheet and portals to document.body so it
          paints above the open panel (#1962) rather than behind it. */}
      {file && signedUrl && isImageMime(file.mime_type)
        ? renderViewer({
            file,
            signedUrl,
            title,
            imageViewerOpen,
            setImageViewerOpen,
            siblings: imageSiblings,
            onSelectSibling,
          })
        : null}
      {file && signedUrl && isPdfMime(file.mime_type) ? (
        // Fullscreen PDF reader — a stacked Dialog (peer of the Sheet) so
        // closing it returns to the panel instead of dismissing the Sheet.
        // PdfFullViewer brings its own toolbar (incl. close), so the Dialog is
        // just a bare fullscreen host.
        <Dialog open={pdfViewerOpen} onOpenChange={setPdfViewerOpen}>
          <DialogContent
            className="flex h-screen w-screen max-w-none flex-col gap-0 rounded-none border-0 p-0 [&>button]:hidden sm:max-w-none"
            data-testid="file-detail-pdf-fullscreen"
          >
            <DialogHeader className="sr-only">
              <DialogTitle>{title}</DialogTitle>
              <DialogDescription>{t("files:detail.metadataTitle")}</DialogDescription>
            </DialogHeader>
            <PdfFullViewer url={signedUrl} title={title} onClose={() => setPdfViewerOpen(false)} />
          </DialogContent>
        </Dialog>
      ) : null}
    </>
  )
}

interface RenderViewerArgs {
  file: { id: string; mime_type?: string }
  signedUrl: string
  title: string
  imageViewerOpen: boolean
  setImageViewerOpen: (open: boolean) => void
  siblings: GalleryImage[] | undefined
  onSelectSibling: ((id: string) => void) | undefined
}

function renderViewer({
  file,
  signedUrl,
  title,
  imageViewerOpen,
  setImageViewerOpen,
  siblings,
  onSelectSibling,
}: RenderViewerArgs) {
  // Gallery mode if the parent supplied siblings. Index is computed
  // here so a parent reorder (e.g. after a sort change) is reflected
  // without the parent having to track the active index.
  if (siblings && siblings.length > 0) {
    const idx = siblings.findIndex((s) => s.id === file.id)
    return (
      <ImageViewer
        open={imageViewerOpen}
        onOpenChange={setImageViewerOpen}
        siblings={siblings}
        index={idx === -1 ? 0 : idx}
        onIndexChange={(next) => onSelectSibling?.(siblings[next].id)}
      />
    )
  }
  // Single-image fallback for deep-link detail (no surrounding list).
  return (
    <ImageViewer
      open={imageViewerOpen}
      onOpenChange={setImageViewerOpen}
      url={signedUrl}
      alt={title}
    />
  )
}

interface PreviewProps {
  mime?: string
  url?: string
  alt: string
  onExpandImage?: () => void
  onExpandPdf?: () => void
}

function FilePreview({ mime, url, alt, onExpandImage, onExpandPdf }: PreviewProps) {
  const { t } = useTranslation()

  if (isImageMime(mime) && url) {
    return (
      <button
        type="button"
        onClick={onExpandImage}
        // Full-bleed banner: breaks out of the body's px-5 inset (#1962)
        // so the image spans the panel edge-to-edge while the metadata
        // below stays inset. `min-h` reserves a box for the fade-in
        // shimmer (#1961) and centers a short `object-contain` image so
        // the panel doesn't grow from a zero-height placeholder.
        className="group relative -mx-5 flex min-h-[12rem] w-[calc(100%+2.5rem)] items-center justify-center overflow-hidden bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
        data-testid="file-preview-image-trigger"
      >
        <FadeInImage
          src={url}
          alt={alt}
          className="max-h-[60vh] w-full object-contain"
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

  // Non-previewable MIME (or a previewable one whose signed URL is
  // missing): the "cannot preview, download to view" card, folded in
  // from the retired FilePreviewOtherDialog (#1962). The footer's Open /
  // Download row provides the actual fetch affordance.
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
