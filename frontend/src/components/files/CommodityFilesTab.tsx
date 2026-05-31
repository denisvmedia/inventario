import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import {
  File as FileIcon,
  FileText,
  Image as ImageIcon,
  Paperclip,
  Receipt,
  Upload,
} from "lucide-react"

import { FileCollection } from "@/components/files/FileCollection"
import { FileDetailSheet } from "@/components/files/FileDetailSheet"
import { FileViewToggle } from "@/components/files/FileViewToggle"
import type { GalleryImage } from "@/components/files/ImageViewer"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"
import { isImageMime } from "@/features/files/constants"
import { useFiles } from "@/features/files/hooks"
import type { ListedFile } from "@/features/files/api"
import { useFilesViewMode } from "@/features/files/useFilesViewMode"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { cn } from "@/lib/utils"

// CommodityFilesTab is the Commodity-detail Files tab. It keeps the
// segmented chip-bar (All / Images / Invoices / Documents / Other) and
// the contextual upload zone from the design mock
// (`design-mocks/src/components/ItemDetail.tsx`), but renders the
// chip-filtered set through the shared `FileCollection` (FileCard grid /
// FileListRow list) with a grid/list toggle — so a file looks and
// behaves the same here as on the global Files page and the
// location/area panel (#1966). The previous bespoke split (a square
// PhotoGrid for images + a separate NonPhotoList for documents shown at
// once) is gone.
//
// Clicking a file opens the same right-side `FileDetailSheet` the global
// Files page and the location/area `EntityFilesPanel` use, but *in place*
// — driven by local state instead of navigating to `/files/:id` — so the
// user keeps the commodity-detail context (metadata + inline preview, with
// expand-to-fullscreen from there). Image siblings are passed through so
// the fullscreen viewer's gallery cycles this commodity's photos. Delete /
// download / edit all happen from inside the sheet.
//
// Chip IDs map 1:1 to the BE `FileCategory` enum, with one synthetic
// `invoices` chip retained for UX continuity (post-#1622 the `invoices`
// FileCategory is gone, but a per-commodity "show me invoices" affordance
// is still load-bearing — the chip now filters by the `invoice` tag
// instead of the dropped category value).
type FilesTabCategory = "all" | "images" | "invoices" | "documents" | "other"

interface ChipDef {
  id: FilesTabCategory
  // i18n key under `commodities:detail.filesTab.chip`; passed to t().
  labelKey: "all" | "images" | "invoices" | "documents" | "other"
  // Lucide icon component. Rendered inside the chip + the empty state.
  icon: typeof Paperclip
  // i18n key under `commodities:detail.filesTab` for the empty-state copy
  // when the chip's bucket is empty.
  emptyKey: "emptyAll" | "emptyImages" | "emptyInvoices" | "emptyDocuments" | "emptyOther"
  // i18n key under `commodities:detail.filesTab` for the upload-zone copy
  // when this chip is active.
  dropKey: "dropFiles" | "dropImages" | "dropInvoices" | "dropDocuments"
}

const CHIPS: ChipDef[] = [
  { id: "all", labelKey: "all", icon: Paperclip, emptyKey: "emptyAll", dropKey: "dropFiles" },
  {
    id: "images",
    labelKey: "images",
    icon: ImageIcon,
    emptyKey: "emptyImages",
    dropKey: "dropImages",
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
  {
    // Inventario's `models.FileCategory` enum has an `other` bucket
    // alongside images / invoices / documents. The mock omits it but we
    // surface it here so files that don't fit the three primary chips
    // remain reachable from a dedicated tab — the upload zone falls back
    // to the generic dropFiles copy because there is no "drop
    // other-files" affordance worth distinguishing.
    id: "other",
    labelKey: "other",
    icon: FileIcon,
    emptyKey: "emptyOther",
    dropKey: "dropFiles",
  },
]

// Per-page cap for the underlying `useFiles({ linkedEntity… })` query.
// The chip-bar derives counts from the loaded set, so this also caps the
// count displayed; 100 is well above the realistic per-commodity
// attachment count and keeps a single round-trip covering all chips.
const PAGE_SIZE = 100

export interface CommodityFilesTabProps {
  commodityId: string
  // Opens the page-level UploadFilesDialog with the commodity
  // preselected. The page also exposes a drag-drop overlay that opens the
  // same dialog with files preloaded — see `CommodityDetailPage`'s
  // `useFileDropZone` wiring.
  onAttachClick: () => void
  // Cover-photo wiring (issue #1451 option B) — forwarded to FileCard's
  // per-photo star button (grid view, image cards only). When omitted,
  // the star is hidden.
  coverState?: { current?: string; auto?: string }
  onSetCover?: (fileId: string | null) => void
  // True while the cover mutation is inflight — disables the star to
  // prevent double-clicks from racing the optimistic cache update.
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

  const [activeChip, setActiveChip] = useState<FilesTabCategory>("all")
  // Grid/list toggle, shared with the location/area panel via the same
  // localStorage key (entity-detail surfaces default to grid).
  const [viewMode, setViewMode] = useFilesViewMode("files:entityViewMode", "grid")
  // The file whose detail sheet is open, kept locally so opening it never
  // leaves the commodity detail page (mirrors EntityFilesPanel, #1963).
  const [selectedId, setSelectedId] = useState<string | null>(null)

  // Single query fans all chips; client-side filtering keeps chip-toggle
  // latency at zero. The badge query in `CommodityDetailPage` uses
  // perPage=1 to fetch only meta.total and lives in a different cache
  // slot on purpose.
  const filesQuery = useFiles(
    { linkedEntityType: "commodity", linkedEntityId: commodityId, perPage: PAGE_SIZE },
    { enabled: !!commodityId && !!slug }
  )
  // Stable reference for downstream useMemo deps — a `?? []` fallback
  // would mint a fresh array each render and bust the memos every time.
  const files = useMemo(() => filesQuery.data?.files ?? [], [filesQuery.data?.files])

  const counts = useMemo(() => deriveCounts(files), [files])

  const visible = useMemo(() => {
    if (activeChip === "all") return files
    // Post-#1622: the "invoices" chip filters by the `invoice` tag — the
    // FileCategory enum dropped its `invoices` value. Every other chip
    // still matches BE FileCategory 1:1.
    if (activeChip === "invoices") {
      return files.filter(
        (row) => Array.isArray(row.file.tags) && row.file.tags.includes("invoice")
      )
    }
    return files.filter((row) => row.file.category === activeChip)
  }, [files, activeChip])

  // This commodity's photos, in grid order, for the fullscreen viewer's
  // gallery navigation inside the detail sheet. Memoized like `files` /
  // `counts` / `visible` so a re-render doesn't hand FileDetailSheet a
  // fresh array reference each time.
  const imageSiblings: GalleryImage[] = useMemo(
    () =>
      files
        .filter(({ file, signedUrl }) => isImageMime(file.mime_type) && !!signedUrl?.url)
        .map(({ file, signedUrl }) => ({
          id: file.id,
          url: signedUrl?.url ?? "",
          alt: file.title?.trim() || file.path?.trim() || file.id,
        })),
    [files]
  )

  const activeChipDef = CHIPS.find((c) => c.id === activeChip) ?? CHIPS[0]
  const ActiveEmptyIcon = activeChipDef.icon

  return (
    <>
      <div className="flex flex-col gap-3" data-testid="commodity-detail-files">
        {/* Chip bar — `flex items-center gap-1 rounded-lg bg-muted/50 p-1`
          segmented control lifted from the mock. Each chip renders an
          icon + (responsive) label + count badge. */}
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

        {/* Upload zone — dashed-border CTA. Click opens the upload dialog
          via the parent. Drop is owned by the page-level `<DropOverlay>`
          wired in CommodityDetailPage so a second drop catcher here would
          just fight that one. */}
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
            className={cn(
              viewMode === "grid"
                ? "grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4"
                : "space-y-1"
            )}
            data-testid="commodity-files-loading"
            aria-busy="true"
          >
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton
                key={i}
                className={viewMode === "grid" ? "aspect-[4/3] w-full" : "h-12 w-full"}
              />
            ))}
          </div>
        ) : visible.length === 0 ? (
          <div
            className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border py-8 text-center"
            data-testid="commodity-files-empty"
          >
            <ActiveEmptyIcon className="size-7 text-muted-foreground/30" aria-hidden="true" />
            <p className="max-w-xs text-sm leading-relaxed text-muted-foreground">
              {t(`commodities:detail.filesTab.${activeChipDef.emptyKey}`)}
            </p>
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            <div className="flex justify-end">
              <FileViewToggle
                value={viewMode}
                onChange={setViewMode}
                testIdPrefix="commodity-files-view"
              />
            </div>
            <FileCollection
              items={visible}
              viewMode={viewMode}
              onOpen={(id) => setSelectedId(id)}
              coverState={coverState}
              onSetCover={onSetCover}
              coverBusy={coverBusy}
              idPrefix="commodity-files"
            />
          </div>
        )}
      </div>
      <FileDetailSheet
        fileId={selectedId}
        open={!!selectedId}
        onOpenChange={(next) => {
          if (!next) setSelectedId(null)
        }}
        onEdit={(id) =>
          navigate(`/g/${encodeURIComponent(slug)}/files/${encodeURIComponent(id)}/edit`)
        }
        imageSiblings={imageSiblings}
        onSelectSibling={(id) => setSelectedId(id)}
      />
    </>
  )
}

// deriveCounts collapses a loaded file set into the five chip counts.
// `all` is the total; `images` / `documents` / `other` match the BE
// `models.FileCategory` enum 1:1. The `invoices` chip is a synthetic
// tag-based filter (post-#1622) — it counts files carrying the `invoice`
// tag regardless of category (they live in `documents` now). A file can
// be in both `documents` (its category bucket) and `invoices` (its
// tag-based view); the two counts overlap on purpose.
function deriveCounts(rows: ListedFile[]): Record<FilesTabCategory, number> {
  const counts: Record<FilesTabCategory, number> = {
    all: rows.length,
    images: 0,
    invoices: 0,
    documents: 0,
    other: 0,
  }
  for (const row of rows) {
    const cat = row.file.category
    if (cat === "images" || cat === "documents" || cat === "other") {
      counts[cat] += 1
    }
    if (Array.isArray(row.file.tags) && row.file.tags.includes("invoice")) {
      counts.invoices += 1
    }
  }
  return counts
}
