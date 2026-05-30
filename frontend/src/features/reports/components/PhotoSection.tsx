import { Image as ImageIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"

// A single report photo. `url` is the displayable image source (thumbnail
// or full-size); `name` becomes the alt text.
export interface ReportPhoto {
  url: string
  name: string
}

export type PhotoSize = "thumb" | "full"

// PhotoSection renders the item photographs block (#1370). `thumb` lays
// the images out in a 3-col square grid; `full` stacks them at a larger
// contained size. Mirrors the design mock's PhotoSection. Renders nothing
// when there are no photos.
interface PhotoSectionProps {
  photos: ReportPhoto[]
  imageSize: PhotoSize
}

export function PhotoSection({ photos, imageSize }: PhotoSectionProps) {
  const { t } = useTranslation()
  if (photos.length === 0) return null
  return (
    <div data-testid="report-photo-section">
      <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-widest text-muted-foreground">
        <ImageIcon className="size-3.5" aria-hidden="true" />
        {t("reports:insurance.photos.heading", { count: photos.length })}
        {/* Single non-plural key (count is interpolated) to keep en/cs/ru
            key-sets identical for the i18n parity check. */}
      </h3>
      {imageSize === "thumb" ? (
        <div className="grid grid-cols-3 gap-2">
          {photos.map((p, i) => (
            <div
              key={`${p.url}-${i}`}
              className="aspect-square overflow-hidden rounded-lg border border-border bg-muted"
            >
              <img src={p.url} alt={p.name} className={cn("size-full object-cover")} />
            </div>
          ))}
        </div>
      ) : (
        <div className="space-y-3">
          {photos.map((p, i) => (
            <div
              key={`${p.url}-${i}`}
              className="overflow-hidden rounded-xl border border-border bg-muted"
            >
              <img src={p.url} alt={p.name} className="max-h-[480px] w-full object-contain" />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
