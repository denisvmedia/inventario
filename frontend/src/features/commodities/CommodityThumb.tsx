import { useEffect, useState } from "react"

import { cn } from "@/lib/utils"
import {
  COMMODITY_TYPE_FALLBACK_ICON,
  COMMODITY_TYPE_ICONS,
  type CommodityTypeValue,
} from "@/features/commodities/constants"
import type { CommodityCover } from "@/features/commodities/api"

// Thumbnail variant the BE generates today (`small` = 150px,
// `medium` = 300px). `auto` picks the smallest variant that fits the
// requested slot — matches the `<img>` sizing rule of "never serve more
// pixels than will be drawn". The BE may add a `large` later; that goes
// here without touching call sites.
export type CommodityThumbVariant = "small" | "medium" | "auto"

export interface CommodityThumbProps {
  cover?: CommodityCover
  // Type drives the Lucide icon fallback; mirrors `COMMODITY_TYPE_ICONS`.
  type?: CommodityTypeValue
  // Visible name used as the alt text. Falls back to a generic
  // "Commodity photo" when omitted so screen readers always have
  // something to announce.
  name?: string
  // `size` sets the actual pixel box. The component clamps `<img>` to
  // these dimensions to avoid CLS, so the caller controls layout.
  size: number
  // `variant` lets the caller force a specific thumbnail size. Default
  // `auto` picks `small` for boxes ≤ 150px and `medium` otherwise.
  variant?: CommodityThumbVariant
  // Tailwind classes appended to the outer slot. The default rounded
  // muted box mirrors the existing icon-only styling.
  className?: string
  // `imgClassName` is forwarded onto the `<img>` itself when a cover is
  // rendered. Useful for the detail-page hero which wants
  // `object-cover` on a wider aspect ratio.
  imgClassName?: string
  // `data-testid` propagates onto the outer slot so tests can locate
  // the thumbnail without descending into the DOM.
  testId?: string
}

// pickThumbnailURL chooses a usable URL from the variant map. `variant`
// "auto" prefers the smallest variant that's at least as large as the
// requested slot, falling back to the first available URL. Returns
// undefined when the cover map is empty.
function pickThumbnailURL(
  cover: CommodityCover,
  variant: CommodityThumbVariant,
  size: number
): string | undefined {
  const map = cover.thumbnails
  if (!map) return undefined
  if (variant === "small" && map.small) return map.small
  if (variant === "medium" && map.medium) return map.medium
  // Auto: pick the smallest variant that meets the requested pixel
  // size. `small` is 150px today; bump to `medium` (300px) for boxes
  // that would visibly upscale `small`.
  if (variant === "auto") {
    if (size <= 150 && map.small) return map.small
    if (map.medium) return map.medium
    if (map.small) return map.small
  }
  // Last-resort fallback: any URL the BE returned.
  const first = Object.values(map).find(Boolean)
  return first
}

// CommodityThumb renders the cover photo for a commodity, falling back
// to the type Lucide icon when no cover is available or the image
// fails to load. Used by the list card / table row, the Sheet preview,
// and the detail-page hero (issue #1451 option A; icons in #1392).
export function CommodityThumb({
  cover,
  type,
  name,
  size,
  variant = "auto",
  className,
  imgClassName,
  testId,
}: CommodityThumbProps) {
  // Reset the load-failure flag when the cover URL changes — otherwise
  // a once-failed image keeps rendering the fallback icon even after
  // the user re-uploads a new working photo.
  const url = cover ? pickThumbnailURL(cover, variant, size) : undefined
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    // Reset failure flag when the URL changes (sync external prop → state).
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setFailed(false)
  }, [url])

  const Icon = type ? COMMODITY_TYPE_ICONS[type] : COMMODITY_TYPE_FALLBACK_ICON
  const dim = `${size}px`
  const showImage = Boolean(url) && !failed
  // Icon glyph scales with the slot; cap at ~50% so it reads as a
  // pictogram inside the rounded-lg tile rather than dominating it.
  const iconPx = Math.max(12, Math.round(size * 0.5))

  return (
    <div
      className={cn(
        "flex shrink-0 items-center justify-center overflow-hidden rounded-lg bg-muted",
        className
      )}
      style={{ width: dim, height: dim }}
      data-testid={testId}
      data-state={showImage ? "image" : "fallback"}
      data-commodity-type={type ?? "unknown"}
    >
      {showImage ? (
        <img
          src={url}
          alt={name ?? "Commodity photo"}
          loading="lazy"
          width={size}
          height={size}
          className={cn("h-full w-full object-cover", imgClassName)}
          onError={() => setFailed(true)}
          data-testid={testId ? `${testId}-img` : undefined}
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
