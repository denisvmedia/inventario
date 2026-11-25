import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Images } from "lucide-react"

import { ImageViewer, type GalleryImage } from "@/components/files/ImageViewer"
import type { Commodity } from "@/features/commodities/api"
import { CommodityThumb } from "@/features/commodities/CommodityThumb"
import { isImageMime } from "@/features/files/constants"
import { useFiles } from "@/features/files/hooks"
import { useItemTypeLookup } from "@/features/itemTypes/hooks"
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
  const itemTypes = useItemTypeLookup()
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
  const coverId = cover?.fileId ?? ""
  const coverUrl =
    cover?.thumbnails.large ?? cover?.thumbnails.medium ?? cover?.thumbnails.small ?? ""
  const gallery: GalleryImage[] = useMemo(() => {
    const listed: GalleryImage[] = (current?.files ?? [])
      .filter(({ file, signedUrl }) => isImageMime(file.mime_type) && !!signedUrl?.url)
      .map(({ file, signedUrl }) => ({
        id: file.id,
        url: signedUrl?.url ?? "",
        alt: file.title?.trim() || file.path?.trim() || name,
      }))
    if (listed.some((photo) => photo.id === coverId) || !coverUrl) return listed
    // The list holds the newest photos and the default cover is the oldest,
    // so the cover can be missing from it. It then leads the gallery, drawn
    // from its largest thumbnail; before the list arrives it is the gallery.
    return [{ id: coverId, url: coverUrl, alt: name }, ...listed]
  }, [current, name, coverId, coverUrl])
  const photoCount = current?.total ?? 0
  // The viewer tracks the photo it shows, not a position, so the list
  // arriving while it is open keeps it on the cover.
  const [viewerPhotoId, setViewerPhotoId] = useState<string | null>(null)
  const viewerIndex = Math.max(
    0,
    gallery.findIndex((photo) => photo.id === viewerPhotoId)
  )

  function onViewerOpenChange(open: boolean) {
    if (!open) setViewerPhotoId(null)
  }
  function onViewerIndexChange(index: number) {
    const photo = gallery[index]
    if (photo) setViewerPhotoId(photo.id)
  }

  return (
    <>
      <button
        type="button"
        onClick={() => setViewerPhotoId(coverId)}
        aria-label={
          photoCount > 1
            ? t("commodities:detail.cover.openWithCount", { name, count: photoCount })
            : t("commodities:detail.cover.open", { name })
        }
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
          type={commodity.type}
          icon={itemTypes.get(commodity.type)?.Icon}
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
          open={viewerPhotoId !== null}
          onOpenChange={onViewerOpenChange}
          siblings={gallery}
          index={viewerIndex}
          onIndexChange={onViewerIndexChange}
        />
      ) : null}
    </>
  )
}
