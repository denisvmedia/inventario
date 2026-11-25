import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Images } from "lucide-react"

import { ImageViewer, type GalleryImage } from "@/components/files/ImageViewer"
import type { Commodity } from "@/features/commodities/api"
import { CommodityThumb } from "@/features/commodities/CommodityThumb"
import type { CommodityTypeValue } from "@/features/commodities/constants"
import { isImageMime } from "@/features/files/constants"
import { useFiles } from "@/features/files/hooks"
import { cn } from "@/lib/utils"

// How many of the item's photos the viewer cycles through. The Files tab
// pages through the rest.
const GALLERY_LIMIT = 48

// The width the cover is drawn at: the full width on a phone, the header's
// photo column from `sm` up (see CommodityDetailPage).
const COVER_SIZES = "(min-width: 1024px) 224px, (min-width: 640px) 192px, 100vw"

export interface CommodityCoverPanelProps {
  commodity: Commodity
  className?: string
}

// CommodityCoverPanel is the item page's cover photo (#2797): the cover at
// a fixed 4:3 ratio, and a count of the item's photos when there is more
// than one. Clicking it, or Enter / Space on it, opens the fullscreen
// viewer on the cover, with Previous / Next through the item's photos.
// The caller renders it only for an item that has a cover.
export function CommodityCoverPanel({ commodity, className }: CommodityCoverPanelProps) {
  const { t } = useTranslation()
  const id = commodity.id ?? ""
  const cover = commodity.cover
  const name = commodity.name ?? ""
  // The same request as the Files tab's Images chip, without its paging.
  const photos = useFiles(
    {
      linkedEntityType: "commodity",
      linkedEntityId: id,
      category: "images",
      perPage: GALLERY_LIMIT,
    },
    { enabled: !!id && !!cover }
  )
  // useFiles keeps the previous key's data while the next loads, which on
  // Previous / Next would be the previous item's photos.
  const current = photos.isPlaceholderData ? undefined : photos.data
  const gallery: GalleryImage[] = useMemo(
    () =>
      (current?.files ?? [])
        .filter(({ file, signedUrl }) => isImageMime(file.mime_type) && !!signedUrl?.url)
        .map(({ file, signedUrl }) => ({
          id: file.id,
          url: signedUrl?.url ?? "",
          alt: file.title?.trim() || file.path?.trim() || name,
        })),
    [current, name]
  )
  const photoCount = current?.total ?? 0
  const [viewerIndex, setViewerIndex] = useState<number | null>(null)
  // Before the photo list arrives, the viewer shows the cover's largest
  // thumbnail on its own.
  const fallbackUrl =
    cover?.thumbnails.large ?? cover?.thumbnails.medium ?? cover?.thumbnails.small ?? ""

  function openViewer() {
    const index = gallery.findIndex((photo) => photo.id === cover?.fileId)
    setViewerIndex(index >= 0 ? index : 0)
  }
  function onViewerOpenChange(open: boolean) {
    if (!open) setViewerIndex(null)
  }

  return (
    <>
      <button
        type="button"
        onClick={openViewer}
        aria-label={t("commodities:detail.cover.open", { name })}
        className={cn(
          "relative block aspect-[4/3] cursor-zoom-in overflow-hidden rounded-xl border bg-muted",
          "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none",
          className
        )}
        data-testid="commodity-detail-cover"
      >
        <CommodityThumb
          fill
          cover={cover}
          type={commodity.type as CommodityTypeValue | undefined}
          name={name}
          sizes={COVER_SIZES}
          className="rounded-none"
          iconClassName="size-12"
          testId="commodity-detail-thumb"
        />
        {photoCount > 1 ? (
          <span
            className="pointer-events-none absolute right-2 bottom-2 inline-flex items-center gap-1 rounded-full bg-background/90 px-2 py-0.5 text-xs font-medium text-foreground backdrop-blur"
            data-testid="commodity-detail-photo-count"
          >
            <Images className="size-3" aria-hidden="true" />
            {t("commodities:detail.cover.photoCount", { count: photoCount })}
          </span>
        ) : null}
      </button>
      {gallery.length > 0 ? (
        <ImageViewer
          open={viewerIndex !== null}
          onOpenChange={onViewerOpenChange}
          siblings={gallery}
          index={viewerIndex ?? 0}
          onIndexChange={setViewerIndex}
        />
      ) : fallbackUrl ? (
        <ImageViewer
          open={viewerIndex !== null}
          onOpenChange={onViewerOpenChange}
          url={fallbackUrl}
          alt={name}
        />
      ) : null}
    </>
  )
}
