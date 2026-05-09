import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import {
  ExternalLink,
  File as FileIcon,
  FileText,
  Image as ImageIcon,
  Paperclip,
  Receipt,
  Star,
  Trash2,
  Upload,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useDeleteFile, useFiles } from "@/features/files/hooks"
import type { FileCategory, ListedFile } from "@/features/files/api"
import { isImageMime, isPdfMime } from "@/features/files/constants"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatBytes } from "@/lib/intl"
import { cn } from "@/lib/utils"

// CommodityFilesTab is the Commodity-detail Files tab redesigned to
// match the design mock (`design-mocks/src/components/ItemDetail.tsx`,
// `<TabsContent value="files">`):
//
// 1. Segmented chip-bar — `flex items-center gap-1 rounded-lg
//    bg-muted/50 p-1`. All / Photos / Invoices / Documents (mock
//    deliberately omits "Other"). Each chip carries a count badge
//    when > 0, derived client-side from the loaded set so toggling
//    chips does NOT refetch.
// 2. Contextual upload zone — dashed-border CTA whose copy reflects
//    the active chip ("Drop photos…" / "Drop invoices…" / etc.).
//    Clicking opens the page-level upload dialog via `onAttachClick`;
//    the page already owns the drag-drop overlay so we don't need a
//    second drop catcher here.
// 3. 3-column photo grid — `grid grid-cols-3 gap-1.5`, aspect-square
//    thumbnails, hover-reveal cover star + delete button.
// 4. Non-photo list — vertical list with mime-aware icon, title /
//    size / tags + View/Open/Download CTA. Click anywhere on the
//    row opens the existing `FileDetailSheet` via the unified
//    `/files/:id` deep-link (same pattern as `EntityFilesPanel`).
// 5. Empty state — chip-aware copy.
//
// `EntityFilesPanel` is unchanged — `LocationDetailPage` still uses
// it. This component is commodity-specific because the chip-bar is
// the mock contract for that surface only.
type FilesTabCategory = "all" | "photos" | "invoices" | "documents"

interface ChipDef {
  id: FilesTabCategory
  // i18n key under `commodities:detail.filesTab.chip`; passed to t().
  labelKey: "all" | "photos" | "invoices" | "documents"
  // Lucide icon component. Rendered inside the chip + the empty
  // state.
  icon: typeof Paperclip
  // i18n key under `commodities:detail.filesTab` for the empty-state
  // copy when the chip's bucket is empty.
  emptyKey: "emptyAll" | "emptyPhotos" | "emptyInvoices" | "emptyDocuments"
  // i18n key under `commodities:detail.filesTab` for the upload-zone
  // copy when this chip is active.
  dropKey: "dropFiles" | "dropPhotos" | "dropInvoices" | "dropDocuments"
}

const CHIPS: ChipDef[] = [
  { id: "all", labelKey: "all", icon: Paperclip, emptyKey: "emptyAll", dropKey: "dropFiles" },
  {
    id: "photos",
    labelKey: "photos",
    icon: ImageIcon,
    emptyKey: "emptyPhotos",
    dropKey: "dropPhotos",
  },
  {
    id: "invoices",
    labelKey: "invoices",
    icon: Receipt,
    emptyKey: "emptyInvoices",
    dropKey: "dropInvoices",
  },
  {
    id: "documents",
    labelKey: "documents",
    icon: FileText,
    emptyKey: "emptyDocuments",
    dropKey: "dropDocuments",
  },
]

// Per-page cap for the underlying `useFiles({ linkedEntity… })`
// query. The chip-bar derives counts from the loaded set, so this
// also caps the count displayed; 100 is well above the realistic
// per-commodity attachment count and keeps a single round-trip
// covering all four chips. If a future commodity blows past this,
// the chip-bar count will be capped at 100 and only the loaded
// rows render — consistent with how the global Files page handles
// pagination today.
const PAGE_SIZE = 100

export interface CommodityFilesTabProps {
  commodityId: string
  // Opens the page-level UploadFilesDialog with the commodity
  // preselected. The page also exposes a drag-drop overlay that
  // opens the same dialog with files preloaded — see
  // `CommodityDetailPage`'s `useFileDropZone` wiring.
  onAttachClick: () => void
  // Cover-photo wiring (issue #1451 option B) — propagates through
  // to the per-photo star button. When omitted, the star is hidden.
  coverState?: { current?: string; auto?: string }
  onSetCover?: (fileId: string | null) => void
  // True while the cover mutation is inflight — disables the star
  // to prevent double-clicks from racing the optimistic cache update.
  coverBusy?: boolean
}

export function CommodityFilesTab({
  commodityId,
  onAttachClick,
  coverState,
  onSetCover,
  coverBusy = false,
}: CommodityFilesTabProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const toast = useAppToast()
  const confirm = useConfirm()
  const deleteMutation = useDeleteFile()

  const [activeChip, setActiveChip] = useState<FilesTabCategory>("all")

  // Single query fans the four chips; client-side filtering keeps
  // chip-toggle latency at zero. The cache key matches the existing
  // file-count query in CommodityDetailPage so the same data is
  // shared.
  const filesQuery = useFiles(
    { linkedEntityType: "commodity", linkedEntityId: commodityId, perPage: PAGE_SIZE },
    { enabled: !!commodityId && !!slug }
  )
  // Stable reference for downstream useMemo deps — a `?? []` fallback
  // would mint a fresh array each render and bust the count + visible
  // memos every time.
  const files = useMemo(
    () => filesQuery.data?.files ?? [],
    [filesQuery.data?.files]
  )

  const counts = useMemo(() => deriveCounts(files), [files])

  const visible = useMemo(() => {
    if (activeChip === "all") return files
    return files.filter((row) => row.file.category === activeChip)
  }, [files, activeChip])

  // Photos: rendered as the photo grid in "all" or "photos". Other
  // chips skip the grid because their bucket can't carry a photo.
  const photos = useMemo(
    () => visible.filter((row) => row.file.category === "photos"),
    [visible]
  )
  // Non-photos: rendered as the list in every chip except "photos"
  // (photo-only view doesn't show anything else).
  const nonPhotos = useMemo(
    () => visible.filter((row) => row.file.category !== "photos"),
    [visible]
  )
  const showGallery = (activeChip === "all" || activeChip === "photos") && photos.length > 0
  const showList = activeChip !== "photos" && nonPhotos.length > 0

  function handleOpen(fileId: string) {
    if (!slug) return
    navigate(`/g/${encodeURIComponent(slug)}/files/${encodeURIComponent(fileId)}`)
  }

  async function handleDelete(file: ListedFile["file"]) {
    const title = file.title?.trim() || file.path?.trim() || file.id
    const ok = await confirm({
      title: t("files:detail.deleteConfirm.title"),
      description: t("files:detail.deleteConfirm.description", { title }),
      confirmLabel: t("files:detail.deleteConfirm.confirm"),
      destructive: true,
    })
    if (!ok) return
    try {
      await deleteMutation.mutateAsync(file.id)
      toast.success(t("files:detail.deleteSuccess"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  const activeChipDef = CHIPS.find((c) => c.id === activeChip) ?? CHIPS[0]
  const ActiveEmptyIcon = activeChipDef.icon

  return (
    <div className="flex flex-col gap-3" data-testid="commodity-detail-files">
      {/* Chip bar — `flex items-center gap-1 rounded-lg bg-muted/50
          p-1` segmented control lifted from the mock. Each chip
          renders an icon + (responsive) label + count badge. */}
      <div
        role="tablist"
        aria-label={t("commodities:detail.tabs.files")}
        className="flex items-center gap-1 rounded-lg bg-muted/50 p-1"
        data-testid="commodity-files-chip-bar"
      >
        {CHIPS.map((chip) => {
          const Icon = chip.icon
          const count = counts[chip.id] ?? 0
          const selected = activeChip === chip.id
          return (
            <button
              key={chip.id}
              type="button"
              role="tab"
              aria-selected={selected}
              tabIndex={selected ? 0 : -1}
              onClick={() => setActiveChip(chip.id)}
              className={cn(
                "flex flex-1 items-center justify-center gap-1.5 rounded-md px-2 py-1.5 text-xs font-medium transition-all",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                selected
                  ? "bg-background text-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground"
              )}
              data-testid={`commodity-files-chip-${chip.id}`}
              data-state={selected ? "active" : "inactive"}
            >
              <Icon className="size-3" aria-hidden="true" />
              <span className="hidden sm:inline">
                {t(`commodities:detail.filesTab.chip.${chip.labelKey}`)}
              </span>
              {count > 0 ? (
                <span
                  className={cn(
                    "flex size-4 items-center justify-center rounded-full text-[10px] font-semibold",
                    selected ? "bg-muted text-foreground" : "bg-muted/60 text-muted-foreground"
                  )}
                  data-testid={`commodity-files-chip-${chip.id}-count`}
                >
                  {count}
                </span>
              ) : null}
            </button>
          )
        })}
      </div>

      {/* Upload zone — dashed-border CTA. Click opens the upload
          dialog via the parent. Drop is owned by the page-level
          `<DropOverlay>` wired in CommodityDetailPage so a second
          drop catcher here would just fight that one. */}
      <button
        type="button"
        onClick={onAttachClick}
        className={cn(
          "flex w-full items-center gap-3 rounded-lg border-2 border-dashed border-border bg-transparent px-4 py-3 text-left transition-colors",
          "hover:border-primary/40 hover:bg-muted/30",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        )}
        data-testid="commodity-files-upload-zone"
      >
        <Upload className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <span className="flex-1 text-sm text-muted-foreground">
          {t(`commodities:detail.filesTab.${activeChipDef.dropKey}`)}{" "}
          <span className="font-medium text-foreground">
            {t("commodities:detail.filesTab.browse")}
          </span>
        </span>
      </button>

      {filesQuery.isError ? (
        <Alert variant="destructive" data-testid="commodity-files-error">
          <AlertTitle>{t("files:entityPanel.errorTitle")}</AlertTitle>
          <AlertDescription>{t("files:entityPanel.errorDescription")}</AlertDescription>
        </Alert>
      ) : filesQuery.isLoading ? (
        <div
          className="grid grid-cols-3 gap-1.5"
          data-testid="commodity-files-loading"
          aria-busy="true"
        >
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="aspect-square w-full rounded-lg" />
          ))}
        </div>
      ) : visible.length === 0 ? (
        <div
          className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border py-8 text-center"
          data-testid="commodity-files-empty"
        >
          <ActiveEmptyIcon
            className="size-7 text-muted-foreground/30"
            aria-hidden="true"
          />
          <p className="max-w-xs text-sm leading-relaxed text-muted-foreground">
            {t(`commodities:detail.filesTab.${activeChipDef.emptyKey}`)}
          </p>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          {showGallery ? (
            <PhotoGrid
              photos={photos}
              onOpen={handleOpen}
              onDelete={handleDelete}
              coverState={coverState}
              onSetCover={onSetCover}
              coverBusy={coverBusy}
            />
          ) : null}
          {showList ? (
            <NonPhotoList
              rows={nonPhotos}
              showCategoryPill={activeChip === "all"}
              onOpen={handleOpen}
              onDelete={handleDelete}
            />
          ) : null}
        </div>
      )}
    </div>
  )
}

interface PhotoGridProps {
  photos: ListedFile[]
  onOpen: (id: string) => void
  onDelete: (file: ListedFile["file"]) => void
  coverState?: { current?: string; auto?: string }
  onSetCover?: (fileId: string | null) => void
  coverBusy?: boolean
}

function PhotoGrid({
  photos,
  onOpen,
  onDelete,
  coverState,
  onSetCover,
  coverBusy = false,
}: PhotoGridProps) {
  const { t } = useTranslation()
  return (
    <ul
      className="grid grid-cols-3 gap-1.5"
      data-testid="commodity-files-photo-grid"
    >
      {photos.map(({ file, signedUrl }) => {
        const title = file.title?.trim() || file.path?.trim() || file.id
        const thumbUrl =
          signedUrl?.thumbnails?.small ??
          signedUrl?.thumbnails?.medium ??
          signedUrl?.thumbnails?.large ??
          signedUrl?.url
        const isExplicit = onSetCover && coverState?.current === file.id
        const isAutoPick =
          onSetCover && !coverState?.current && coverState?.auto === file.id
        const showStar = !!onSetCover
        const starLabel = isExplicit
          ? t("files:cover.clearLabel", { defaultValue: "Clear cover" })
          : isAutoPick
            ? t("files:cover.pinLabel", { defaultValue: "Pin as cover" })
            : t("files:cover.setLabel", { defaultValue: "Set as cover" })
        return (
          <li key={file.id} className="relative">
            <button
              type="button"
              onClick={() => onOpen(file.id)}
              aria-label={t("files:list.openDetail", {
                title,
                defaultValue: `Open ${title}`,
              })}
              className={cn(
                "group relative aspect-square w-full overflow-hidden rounded-lg border border-border bg-muted",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              )}
              data-testid={`commodity-files-photo-${file.id}`}
            >
              {thumbUrl ? (
                <img
                  src={thumbUrl}
                  alt={title}
                  loading="lazy"
                  className="absolute inset-0 size-full object-cover"
                />
              ) : (
                <div className="absolute inset-0 flex items-center justify-center">
                  <ImageIcon
                    className="size-8 text-muted-foreground/30"
                    aria-hidden="true"
                  />
                </div>
              )}
              {/* Hover overlay — surfaces the title only when the
                  user hovers; keeps the grid clean otherwise. */}
              <div className="absolute inset-0 flex items-end bg-black/0 p-1.5 opacity-0 transition-colors group-hover:bg-black/30 group-hover:opacity-100">
                <p className="truncate text-[10px] font-medium leading-tight text-white">
                  {title}
                </p>
              </div>
            </button>
            {showStar ? (
              <Button
                type="button"
                size="icon"
                variant={isExplicit ? "default" : "outline"}
                onClick={(e) => {
                  e.preventDefault()
                  e.stopPropagation()
                  if (!onSetCover || coverBusy) return
                  onSetCover(isExplicit ? null : file.id)
                }}
                disabled={coverBusy}
                aria-label={starLabel}
                aria-pressed={!!isExplicit}
                title={starLabel}
                className={cn(
                  "absolute left-1 top-1 z-10 size-6 rounded-full bg-background/90 backdrop-blur",
                  !isExplicit &&
                    !isAutoPick &&
                    "opacity-0 transition-opacity hover:opacity-100 focus:opacity-100"
                )}
                data-testid={`commodity-files-photo-cover-${file.id}`}
              >
                <Star
                  className={cn("size-3", isExplicit ? "fill-current" : "")}
                  aria-hidden="true"
                />
              </Button>
            ) : null}
            <button
              type="button"
              onClick={(e) => {
                e.preventDefault()
                e.stopPropagation()
                onDelete(file)
              }}
              aria-label={t("files:detail.delete")}
              title={t("files:detail.delete")}
              className={cn(
                "absolute right-1 top-1 z-10 flex size-5 items-center justify-center rounded-full bg-black/60 text-white transition-all",
                "opacity-0 hover:bg-destructive group-hover:opacity-100 focus-visible:opacity-100",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              )}
              data-testid={`commodity-files-photo-delete-${file.id}`}
            >
              <Trash2 className="size-2.5" aria-hidden="true" />
            </button>
          </li>
        )
      })}
    </ul>
  )
}

interface NonPhotoListProps {
  rows: ListedFile[]
  showCategoryPill: boolean
  onOpen: (id: string) => void
  onDelete: (file: ListedFile["file"]) => void
}

function NonPhotoList({ rows, showCategoryPill, onOpen, onDelete }: NonPhotoListProps) {
  const { t } = useTranslation()
  return (
    <ul className="flex flex-col gap-1.5" data-testid="commodity-files-list">
      {rows.map(({ file }) => {
        const title = file.title?.trim() || file.path?.trim() || file.id
        const ctaKey = previewLabelKey(file.mime_type)
        const Icon = mimeIconFor(file.mime_type, file.category)
        return (
          <li
            key={file.id}
            className="flex items-center gap-3 rounded-lg border border-border bg-card px-3 py-2.5"
            data-testid={`commodity-files-row-${file.id}`}
          >
            <Icon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-1.5">
                <p className="truncate text-sm font-medium" title={title}>
                  {title}
                </p>
                {showCategoryPill && file.category ? (
                  <span
                    className={cn(
                      "shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-medium",
                      categoryPillTone(file.category)
                    )}
                  >
                    {t(`files:category${capitalize(file.category)}`)}
                  </span>
                ) : null}
              </div>
              <div className="mt-0.5 flex items-center gap-2">
                {file.size_bytes !== undefined ? (
                  <span className="text-xs text-muted-foreground">
                    {formatBytes(file.size_bytes)}
                  </span>
                ) : null}
                {file.tags?.slice(0, 3).map((tag) => (
                  <Badge key={tag} variant="secondary" className="h-4 px-1.5 text-[10px]">
                    {tag}
                  </Badge>
                ))}
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-1">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-7 gap-1 px-2 text-xs"
                onClick={() => onOpen(file.id)}
                data-testid={`commodity-files-row-open-${file.id}`}
              >
                {t(`commodities:detail.filesTab.${ctaKey}`)}
                <ExternalLink className="size-3" aria-hidden="true" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-7 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                onClick={() => onDelete(file)}
                aria-label={t("files:detail.delete")}
                title={t("files:detail.delete")}
                data-testid={`commodity-files-row-delete-${file.id}`}
              >
                <Trash2 className="size-3.5" aria-hidden="true" />
              </Button>
            </div>
          </li>
        )
      })}
    </ul>
  )
}

// deriveCounts collapses a loaded file set into the four chip
// counts. `all` is the total; `photos` / `invoices` / `documents`
// match the BE category enum 1:1. Files in the BE's "other"
// category are still counted into `all` (mock parity — no chip
// for "other" so they show up in the All view).
function deriveCounts(rows: ListedFile[]): Record<FilesTabCategory, number> {
  const counts: Record<FilesTabCategory, number> = {
    all: rows.length,
    photos: 0,
    invoices: 0,
    documents: 0,
  }
  for (const row of rows) {
    const cat = row.file.category
    if (cat === "photos" || cat === "invoices" || cat === "documents") {
      counts[cat] += 1
    }
  }
  return counts
}

// previewLabelKey picks the View / Open / Download CTA copy based
// on the file's MIME — image gets "View" (opens the inline image
// viewer), PDF gets "Open" (browser native viewer), everything
// else gets "Download" (the only meaningful action). Keys map to
// `commodities:detail.filesTab.{ctaView,ctaOpen,ctaDownload}`.
function previewLabelKey(mime: string | undefined): "ctaView" | "ctaOpen" | "ctaDownload" {
  if (isImageMime(mime)) return "ctaView"
  if (isPdfMime(mime)) return "ctaOpen"
  return "ctaDownload"
}

function mimeIconFor(
  mime: string | undefined,
  category: FileCategory | undefined
): typeof Paperclip {
  if (category === "invoices") return Receipt
  if (category === "documents" || isPdfMime(mime)) return FileText
  if (isImageMime(mime)) return ImageIcon
  return FileIcon
}

function categoryPillTone(category: FileCategory): string {
  switch (category) {
    case "invoices":
      return "bg-chart-1/10 text-chart-1"
    case "documents":
      return "bg-chart-3/10 text-chart-3"
    case "photos":
      return "bg-chart-2/10 text-chart-2"
    default:
      return "bg-muted text-muted-foreground"
  }
}

function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1)
}
