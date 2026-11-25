import type { LucideIcon } from "lucide-react"
import { useEffect, useState } from "react"

import { FadeInImage } from "@/components/ui/fade-in-image"
import { cn } from "@/lib/utils"
import type { CommodityCover } from "@/features/commodities/api"
import { BUILTIN_ITEM_TYPE_ICON_LOOKUP } from "@/features/itemTypes/display"
import { ITEM_TYPE_FALLBACK_ICON } from "@/features/itemTypes/icons"

// The thumbnail sizes the BE renders (services/thumbnail_sizes.go), by the
// longest side in pixels. The cover carries one signed URL per size.
const THUMBNAIL_WIDTHS: Record<string, number> = {
  small: 150,
  medium: 300,
  large: 600,
}

// `auto` lets the browser pick from every size by the slot's width and the
// screen's pixel density (srcset + sizes). The others force one size.
export type CommodityThumbVariant = "small" | "medium" | "large" | "auto"

export interface CommodityThumbProps {
  cover?: CommodityCover
  // The item type's slug, shown as data-commodity-type.
  type?: string
  // The icon shown without a photo: the item type's icon, which the caller
  // resolves from the group's types (useItemTypeLookup). Without it a
  // built-in slug shows its default icon and anything else a generic one.
  icon?: LucideIcon
  // Visible name used as the alt text. Falls back to a generic
  // "Commodity photo" when omitted so screen readers always have
  // something to announce.
  name?: string
  // `size` sets a fixed square box in pixels. Omit it with `fill`.
  size?: number
  // `fill` makes the thumbnail fill its parent instead: the caller sizes
  // the box (a fixed aspect ratio keeps the layout from shifting) and
  // passes `sizes`, the width the box is drawn at.
  fill?: boolean
  // The `sizes` attribute for the srcset. Defaults to `${size}px`.
  sizes?: string
  variant?: CommodityThumbVariant
  // Tailwind classes appended to the outer slot. The default rounded
  // muted box mirrors the existing icon-only styling.
  className?: string
  // `imgClassName` is forwarded onto the `<img>` itself when a cover is
  // rendered.
  imgClassName?: string
  // Classes for the fallback icon in `fill` mode, where there is no pixel
  // size to scale it from. Defaults to `size-10`.
  iconClassName?: string
  // `data-testid` propagates onto the outer slot so tests can locate
  // the thumbnail without descending into the DOM.
  testId?: string
}

// pickThumbnailURL chooses the `src`: the forced variant, or for `auto` the
// smallest size at least as large as the slot, falling back to any URL the
// BE returned. Returns undefined when the cover map is empty.
function pickThumbnailURL(
  cover: CommodityCover,
  variant: CommodityThumbVariant,
  size: number
): string | undefined {
  const map = cover.thumbnails
  if (!map) return undefined
  if (variant !== "auto" && map[variant]) return map[variant]
  const bySize = Object.entries(THUMBNAIL_WIDTHS)
    .filter(([name]) => map[name])
    .sort(([, a], [, b]) => a - b)
  const fit = bySize.find(([, width]) => width >= size) ?? bySize.at(-1)
  if (fit) return map[fit[0]]
  return Object.values(map).find(Boolean)
}

// thumbnailSrcSet lists every known size of the cover with its width, so
// the browser fetches no more pixels than the slot draws on this screen.
function thumbnailSrcSet(cover: CommodityCover): string | undefined {
  const entries = Object.entries(THUMBNAIL_WIDTHS)
    .filter(([name]) => cover.thumbnails?.[name])
    .sort(([, a], [, b]) => a - b)
    .map(([name, width]) => `${cover.thumbnails[name]} ${width}w`)
  return entries.length > 1 ? entries.join(", ") : undefined
}

// CommodityThumb renders the cover photo for a commodity, falling back
// to the type Lucide icon when no cover is available or the image
// fails to load. Used by the list cards and rows and the item page
// header (issue #1451 option A; icons in #1392).
export function CommodityThumb({
  cover,
  type,
  icon,
  name,
  size,
  fill = false,
  sizes,
  variant = "auto",
  className,
  imgClassName,
  iconClassName,
  testId,
}: CommodityThumbProps) {
  // `fill` boxes are drawn at whatever the layout gives them; `medium` is
  // the `src` for the rare browser without srcset support.
  const slotPx = size ?? THUMBNAIL_WIDTHS.medium
  const url = cover ? pickThumbnailURL(cover, variant, slotPx) : undefined
  const srcSet = cover && variant === "auto" ? thumbnailSrcSet(cover) : undefined
  // Reset the load-failure flag when the cover URL changes — otherwise
  // a once-failed image keeps rendering the fallback icon even after
  // the user re-uploads a new working photo.
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    // Reset failure flag when the URL changes (sync external prop → state).
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setFailed(false)
  }, [url])

  const Icon = icon ?? BUILTIN_ITEM_TYPE_ICON_LOOKUP[type ?? ""] ?? ITEM_TYPE_FALLBACK_ICON
  const showImage = Boolean(url) && !failed
  // Icon glyph scales with the slot; cap at ~50% so it reads as a
  // pictogram inside the rounded-lg tile rather than dominating it.
  const iconPx = Math.max(12, Math.round(slotPx * 0.5))

  return (
    <div
      className={cn(
        "relative flex shrink-0 items-center justify-center overflow-hidden rounded-lg bg-muted",
        fill && "size-full",
        className
      )}
      style={fill ? undefined : { width: `${slotPx}px`, height: `${slotPx}px` }}
      data-testid={testId}
      data-state={showImage ? "image" : "fallback"}
      data-commodity-type={type ?? "unknown"}
    >
      {showImage ? (
        <FadeInImage
          src={url}
          srcSet={srcSet}
          sizes={srcSet ? (sizes ?? `${slotPx}px`) : undefined}
          alt={name ?? "Commodity photo"}
          loading="lazy"
          width={fill ? undefined : slotPx}
          height={fill ? undefined : slotPx}
          className={cn("h-full w-full object-cover", imgClassName)}
          onError={() => setFailed(true)}
          data-testid={testId ? `${testId}-img` : undefined}
        />
      ) : fill ? (
        <Icon
          aria-hidden="true"
          className={cn("size-10 text-muted-foreground", iconClassName)}
          data-testid={testId ? `${testId}-icon` : undefined}
        />
      ) : (
        <Icon
          aria-hidden="true"
          className="text-muted-foreground"
          width={iconPx}
          height={iconPx}
          data-testid={testId ? `${testId}-icon` : undefined}
        />
      )}
    </div>
  )
}
