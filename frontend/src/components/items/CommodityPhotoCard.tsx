import type { ReactNode } from "react"
import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { Calendar } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Card } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Skeleton } from "@/components/ui/skeleton"
import { SearchMatchHint } from "@/components/items/SearchMatchHint"
import { TagPill } from "@/components/tags/TagPill"
import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import type { Commodity } from "@/features/commodities/api"
import { CommodityThumb } from "@/features/commodities/CommodityThumb"
import {
  COMMODITY_STATUS_TONES,
  type CommodityStatusValue,
  type CommodityTypeValue,
} from "@/features/commodities/constants"
import { commodityHref } from "@/features/commodities/listContext"
import type { CommoditySearchMatch } from "@/features/commodities/query"
import type { ResolvedTag } from "@/features/tags/hooks"
import { formatCurrency, formatDate } from "@/lib/intl"
import type { ListContext } from "@/lib/list-context"
import { cn } from "@/lib/utils"

// What a card of the Items list needs, in either card layout.
export interface CommodityCardProps {
  row: Commodity
  slug?: string
  listContext: ListContext
  selected: boolean
  onToggleSelected: (id: string) => void
  onPreview: (id: string) => void
  areaName: (id?: string) => string
  tagLabel: (slug: string) => string
  resolveTag: (slug: string) => ResolvedTag
  tagHref: (tagSlug: string) => string | undefined
  currency: string
  lent?: boolean
  inService?: boolean
  match?: CommoditySearchMatch
}

// The photo card grid: two columns on a phone, up to six on a wide window.
// The columns follow the width the grid gets (a container query), not the
// window, so an open or collapsed sidebar does not change the card size.
// The cards stay between about 180 and 280 px wide.
export function CommodityPhotoGrid({
  children,
  ...rest
}: {
  children: ReactNode
  "data-testid"?: string
  "data-view"?: string
  "aria-busy"?: boolean
}) {
  return (
    <div className="@container">
      <div
        className="grid grid-cols-2 gap-4 @xl:grid-cols-3 @3xl:grid-cols-4 @5xl:grid-cols-5 @6xl:grid-cols-6"
        {...rest}
      >
        {children}
      </div>
    </div>
  )
}

// The width a photo card's image is drawn at, for the browser to choose a
// thumbnail size from: half the window on a phone, a third on a tablet,
// at most 240 px on a desktop.
const PHOTO_CARD_IMAGE_SIZES = "(min-width: 1280px) 240px, (min-width: 640px) 30vw, 50vw"

// CommodityPhotoCard is the photo-forward card of the Items list (#2797):
// the cover photo on top at a fixed 4:3 ratio, then the name, the area, the
// status and warranty pills, the tags and the value. Without a photo, or
// when it fails to load, the image box shows the item type's icon.
export function CommodityPhotoCard({
  row,
  slug,
  listContext,
  selected,
  onToggleSelected,
  onPreview,
  areaName,
  tagLabel,
  resolveTag,
  tagHref,
  currency,
  lent,
  inService,
  match,
}: CommodityCardProps) {
  const { t } = useTranslation()
  const id = row.id ?? ""
  const detailHref = slug && id ? commodityHref(slug, id, listContext) : "#"
  const status = row.status as CommodityStatusValue | undefined
  const tone = status ? COMMODITY_STATUS_TONES[status] : ""
  const area = areaName(row.area_id)
  const purchaseDateShort = row.purchase_date
    ? formatDate(row.purchase_date, { style: "short" })
    : null
  const purchaseDateLabel = purchaseDateShort
    ? t("commodities:card.purchasedOn", { date: purchaseDateShort })
    : undefined
  // A plain click opens the item over the list; a modified or middle click
  // falls through to the link, for a new tab.
  function handleCardClick(e: React.MouseEvent<HTMLAnchorElement>) {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return
    e.preventDefault()
    onPreview(id)
  }
  return (
    <Card
      className={cn(
        "relative gap-0 py-0 transition-all hover:-translate-y-0.5 hover:shadow-md",
        row.draft && "border-dashed opacity-70"
      )}
      data-testid="commodity-card"
      data-commodity-id={id}
      data-variant="photo"
    >
      {/* The card-wide link sits under the content, which lets clicks
          through (`pointer-events-none`); the checkbox and the tag pills
          take their own clicks above it. Same pattern as the compact card. */}
      <Link
        to={detailHref}
        aria-label={row.name ?? ""}
        onClick={handleCardClick}
        className="absolute inset-0 rounded-xl focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring/50"
        data-testid="commodity-card-link"
      />
      <div className="pointer-events-none relative aspect-[4/3] overflow-hidden rounded-t-xl border-b">
        <CommodityThumb
          fill
          cover={row.cover}
          type={row.type as CommodityTypeValue | undefined}
          name={row.name}
          sizes={PHOTO_CARD_IMAGE_SIZES}
          className="rounded-none"
          testId="commodity-card-thumb"
        />
      </div>
      <Checkbox
        checked={selected}
        onCheckedChange={() => onToggleSelected(id)}
        aria-label={`select ${row.name ?? ""}`}
        data-testid="commodity-select"
        className="pointer-events-auto absolute top-2 left-2 z-10 bg-background dark:bg-background"
      />
      <div className="pointer-events-none flex flex-1 flex-col gap-1.5 p-3">
        <div className="min-w-0">
          <p
            className="truncate text-sm font-semibold leading-tight"
            title={row.name}
            data-testid="commodity-card-title"
          >
            {row.name}
          </p>
          {area ? (
            <p className="mt-0.5 truncate text-xs text-muted-foreground" title={area}>
              {area}
            </p>
          ) : null}
          <SearchMatchHint match={match} tagLabel={tagLabel} />
        </div>
        <div className="flex flex-wrap items-center gap-1">
          {row.draft ? (
            <Badge variant="outline" className="h-4 border-dashed px-1 text-[10px]">
              {t("commodities:list.draftBadge")}
            </Badge>
          ) : null}
          {lent ? (
            <Badge
              variant="secondary"
              className="h-4 px-1 text-[10px]"
              data-testid="commodity-lent-badge"
            >
              {t("loans:badge.lentOut")}
            </Badge>
          ) : null}
          {inService ? (
            <Badge
              variant="secondary"
              className="h-4 px-1 text-[10px]"
              data-testid="commodity-in-service-badge"
            >
              {t("services:badge.inService")}
            </Badge>
          ) : null}
          {status && status !== "in_use" ? (
            <span className={cn("rounded-full border px-1.5 py-0.5 text-[10px] font-medium", tone)}>
              {t(`commodities:status.${status}`)}
            </span>
          ) : null}
          <WarrantyBadge
            source={{ warranty_expires_at: row.warranty_expires_at }}
            showIcon={false}
            className="h-4 px-1.5 text-[10px]"
            data-testid="commodity-card-warranty"
          />
        </div>
        {row.tags && row.tags.length > 0 ? (
          <div className="flex flex-wrap gap-1">
            {row.tags.slice(0, 3).map((tagSlug) => {
              const tag = resolveTag(tagSlug)
              return (
                <TagPill
                  key={tagSlug}
                  label={tag.label}
                  color={tag.color}
                  size="xs"
                  href={tagHref(tagSlug)}
                  className="pointer-events-auto relative z-10"
                  data-testid="commodity-card-tag"
                />
              )
            })}
          </div>
        ) : null}
        <div className="mt-auto flex items-center justify-between gap-2 pt-1 text-xs text-muted-foreground">
          {purchaseDateShort ? (
            <span
              className="inline-flex min-w-0 items-center gap-1"
              aria-label={purchaseDateLabel}
              title={purchaseDateLabel}
              data-testid="commodity-card-purchase-date"
            >
              <Calendar className="size-3 shrink-0" aria-hidden="true" />
              <span className="truncate">{purchaseDateShort}</span>
            </span>
          ) : (
            <span />
          )}
          <span className="shrink-0 font-medium text-foreground">
            {formatCurrency(Number(row.current_price ?? 0), currency)}
          </span>
        </div>
      </div>
    </Card>
  )
}

// Placeholder cards while the first page loads, in the photo grid's shape.
export function CommodityPhotoGridSkeleton() {
  return (
    <CommodityPhotoGrid data-testid="commodities-loading" aria-busy>
      {Array.from({ length: 12 }).map((_, i) => (
        <Card key={i} className="gap-0 overflow-hidden py-0">
          <Skeleton className="aspect-[4/3] w-full rounded-none" />
          <div className="flex flex-col gap-2 p-3">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
            <div className="flex justify-between pt-1">
              <Skeleton className="h-3 w-12" />
              <Skeleton className="h-3 w-10" />
            </div>
          </div>
        </Card>
      ))}
    </CommodityPhotoGrid>
  )
}
