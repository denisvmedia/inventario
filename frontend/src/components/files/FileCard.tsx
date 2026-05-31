import { useTranslation } from "react-i18next"
import { Star } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { FadeInImage } from "@/components/ui/fade-in-image"
import type { FileEntity, URLData } from "@/features/files/api"
import {
  FILE_CATEGORY_TILES,
  FILE_TAG_PILLS,
  getFileVisualMeta,
  isImageMime,
} from "@/features/files/constants"
import { useCategoryLabel, useTagPillLabel } from "@/features/files/labels"
import { formatBytes, formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

// `coverState` mirrors the resolved cover for the parent commodity
// (issue #1451 option B). When the parent supplies it, photo files
// gain a star button overlay that sets / clears `cover_file_id`.
//
//   - `current === file.id` → this file is the explicit cover (filled
//     star).
//   - `auto === file.id`    → this file is the auto-pick (first-photo)
//     cover (outline star, click to pin it explicitly).
//   - otherwise the card shows a hover-only outline star inviting the
//     user to set this file as the cover.
//
// When `onSetCover` is omitted the star is hidden — keeps the component
// usable on the global Files page where there's no commodity to pin
// against.
export interface FileCardCoverState {
  current?: string
  auto?: string
}

// One row in the Files grid. Renders a thumbnail (image MIME) or a
// per-MIME / per-category icon, the title (falls back to the path), a
// linked-entity line when the file is attached, and a meta row with
// tags + size. Click anywhere outside the checkbox opens the detail
// sheet. The checkbox toggles bulk selection.
// onToggleSelect + selected are optional: when onToggleSelect is omitted
// the card renders without the checkbox, making it usable as a
// read-only tile inside entity-detail Files panels (#1411 AC #4).
export interface FileCardProps {
  file: FileEntity & { id: string }
  signedUrl?: URLData
  selected?: boolean
  onToggleSelect?: (id: string) => void
  onOpen: (id: string) => void
  // Cover-toggle props (issue #1451 option B). Both must be supplied
  // together to enable the star button; absent props keep the card
  // read-only on global pages.
  coverState?: FileCardCoverState
  onSetCover?: (fileId: string | null) => void
  // True while a cover mutation is inflight — disables the star to
  // prevent double-clicks from racing the optimistic cache update.
  coverBusy?: boolean
}

export function FileCard({
  file,
  signedUrl,
  selected = false,
  onToggleSelect,
  onOpen,
  coverState,
  onSetCover,
  coverBusy = false,
}: FileCardProps) {
  const { t } = useTranslation()
  const categoryLabelOf = useCategoryLabel()
  const tagLabelOf = useTagPillLabel()
  const title = file.title?.trim() || file.path?.trim() || file.id
  const visual = getFileVisualMeta(file)
  const FallbackIcon = visual.icon
  // Thumbnail keys come from services/file_signing_service.go — the BE
  // emits `small` (150px) / `medium` (300px) / `large`. The grid card
  // renders the cover in a full-width `aspect-[4/3]` box (~250–300px wide
  // on a typical layout), so `small` gets upscaled ~2× and looks blurry —
  // prefer `medium`, then fall back to `small`/`large`, and finally to
  // the full-resolution signed URL if no thumbnails were generated yet.
  const thumbUrl =
    signedUrl?.thumbnails?.medium ?? signedUrl?.thumbnails?.small ?? signedUrl?.thumbnails?.large
  const renderImage = isImageMime(file.mime_type) && (thumbUrl || signedUrl?.url)
  const tags = file.tags ?? []
  const tile = FILE_CATEGORY_TILES.find((c) => c.key === file.category) ?? FILE_CATEGORY_TILES[0]
  const CategoryBadgeIcon = tile.icon
  const categoryLabel = categoryLabelOf(tile.key)
  // Mirror FileListRow: known curated tags pick up their accent colour
  // class; user-supplied tags fall back to muted-foreground.
  const matchedTags = tags.map((tag) => {
    const pill = FILE_TAG_PILLS.find((p) => p.id === tag.toLowerCase())
    return {
      id: tag,
      label: pill ? tagLabelOf(pill.id) : tag,
      colorClass: pill?.colorClass ?? "text-muted-foreground",
    }
  })
  const sizeStr = file.size_bytes ? formatBytes(file.size_bytes) : ""
  const linkedLabel = file.linked_entity_type?.trim() || ""
  // Star is only shown for photos and only when the parent wired up a
  // mutation handler. The auto-pick branch surfaces the same star with
  // an outline + tooltip "Pin as cover", so the user can promote the
  // current implicit cover to an explicit one without re-uploading.
  const canPin = !!onSetCover && file.category === "images"
  const isExplicit = canPin && coverState?.current === file.id
  const isAutoPick = canPin && !coverState?.current && coverState?.auto === file.id
  const showStar = canPin
  function handleStar(e: React.MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    if (!onSetCover || coverBusy) return
    // Filled star: clicking clears the override (resolver falls back).
    // Outline star: clicking sets this file as the explicit cover.
    onSetCover(isExplicit ? null : file.id)
  }
  const starLabel = isExplicit
    ? t("files:cover.clearLabel", { defaultValue: "Clear cover" })
    : isAutoPick
      ? t("files:cover.pinLabel", { defaultValue: "Pin as cover" })
      : t("files:cover.setLabel", { defaultValue: "Set as cover" })

  return (
    <Card
      data-testid={`file-card-${file.id}`}
      data-category={file.category}
      data-mime-group={visual.group}
      className={cn(
        "group relative flex h-full flex-col overflow-hidden focus-within:ring-2 focus-within:ring-ring",
        selected && "ring-2 ring-primary"
      )}
    >
      {onToggleSelect ? (
        <div className="absolute left-2 top-2 z-10">
          <Checkbox
            checked={selected}
            onCheckedChange={() => onToggleSelect(file.id)}
            aria-label={t("files:list.selectFile", { title, defaultValue: `Select ${title}` })}
            data-testid={`file-card-checkbox-${file.id}`}
            className="bg-background"
          />
        </div>
      ) : null}
      {showStar ? (
        <Button
          type="button"
          size="icon"
          variant={isExplicit ? "default" : "outline"}
          onClick={handleStar}
          disabled={coverBusy}
          aria-label={starLabel}
          aria-pressed={isExplicit}
          title={starLabel}
          className={cn(
            "absolute right-2 top-2 z-10 size-7 rounded-full bg-background/90 backdrop-blur",
            !isExplicit &&
              !isAutoPick &&
              "opacity-0 transition-opacity group-hover:opacity-100 focus:opacity-100"
          )}
          data-testid={`file-card-cover-${file.id}`}
          data-cover-state={isExplicit ? "explicit" : isAutoPick ? "auto" : "off"}
        >
          <Star className={cn("size-3.5", isExplicit ? "fill-current" : "")} aria-hidden="true" />
        </Button>
      ) : null}
      <button
        type="button"
        onClick={() => onOpen(file.id)}
        aria-label={t("files:list.openDetail", { title, defaultValue: `Open ${title}` })}
        data-testid={`file-card-open-${file.id}`}
        className="flex flex-1 flex-col text-left focus-visible:outline-none"
      >
        <div className="relative aspect-[4/3] w-full overflow-hidden bg-muted">
          {renderImage ? (
            <FadeInImage
              src={thumbUrl ?? signedUrl?.url}
              alt={title}
              loading="lazy"
              className="size-full object-cover"
            />
          ) : (
            <div
              className={cn("flex size-full items-center justify-center", visual.bgClass)}
              data-testid={`file-card-fallback-${file.id}`}
            >
              <FallbackIcon
                className={cn("size-12 opacity-80", visual.colorClass)}
                strokeWidth={1.5}
                aria-hidden="true"
              />
            </div>
          )}
          {/* Category badge overlay — mirrors design-mocks/src/views/FileBrowserView.tsx
              grid card (lines 801-810). Always visible so users can scan by
              category at a glance even when thumbnails dominate the tile. */}
          <span
            className={cn(
              "absolute left-2 bottom-2 flex items-center gap-1 rounded-full px-2 py-0.5",
              tile.activeBg
            )}
            data-testid={`file-card-category-${file.id}`}
          >
            <CategoryBadgeIcon className={cn("size-2.5", tile.activeColor)} aria-hidden="true" />
            <span className={cn("text-[10px] font-medium", tile.activeColor)}>{categoryLabel}</span>
          </span>
        </div>
        <div className="flex flex-1 flex-col gap-1 p-3">
          <p className="truncate text-sm font-medium leading-tight" title={title}>
            {title}
          </p>
          {linkedLabel ? (
            <p
              className="truncate text-xs text-muted-foreground"
              title={linkedLabel}
              data-testid={`file-card-linked-${file.id}`}
            >
              {linkedLabel}
            </p>
          ) : file.created_at ? (
            <p className="truncate text-xs text-muted-foreground">
              {t("files:list.uploadDate", {
                date: formatDate(file.created_at),
                defaultValue: `Uploaded ${formatDate(file.created_at)}`,
              })}
            </p>
          ) : null}
          {matchedTags.length > 0 || sizeStr ? (
            <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-0.5">
              {matchedTags.slice(0, 3).map((tag) => (
                <span
                  key={tag.id}
                  className={cn("text-[10px] font-medium", tag.colorClass)}
                  data-testid={`file-card-tag-${file.id}-${tag.id.toLowerCase()}`}
                >
                  #{tag.label}
                </span>
              ))}
              {matchedTags.length > 3 ? (
                <span className="text-[10px] font-medium text-muted-foreground">
                  +{matchedTags.length - 3}
                </span>
              ) : null}
              {sizeStr ? (
                <span className="ml-auto text-[10px] text-muted-foreground tabular-nums">
                  {sizeStr}
                </span>
              ) : null}
            </div>
          ) : null}
        </div>
      </button>
    </Card>
  )
}
