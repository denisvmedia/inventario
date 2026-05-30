import { Building2, Calendar, DollarSign, Hash, Package } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Separator } from "@/components/ui/separator"
import { WARRANTY_STATUS_CONFIG } from "@/components/warranty/config"
import type { Commodity, CommodityCover } from "@/features/commodities/api"
import { warrantyStatus, type CommodityTypeValue } from "@/features/commodities/constants"
import { formatCurrency, formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

import { PhotoSection, type PhotoSize } from "./PhotoSection"
import { aggregateLocationTotals } from "../aggregate"
import { coverToPhotos } from "../photos"

// LocationReport is the whole-location insurance report body (#1370).
// Mirrors the design mock's LocationReport: a hero header, three summary
// value cards (count / total purchase / est. value), then a per-item
// section for every commodity in the location. Photos per item are the
// item's cover thumbnail (the list endpoint surfaces covers, not full
// galleries — see the design-deviations entry).
interface LocationReportProps {
  locationName: string
  locationIcon?: string
  groupName: string
  // Commodities that belong to this location (already filtered by the
  // page from `area_id ∈ location's areas`).
  commodities: Commodity[]
  // Per-id cover descriptors from the list endpoint's `meta.covers`.
  covers: Record<string, CommodityCover>
  // "{Location} · {Area}" resolver for each item's area_id.
  areaLabelFor: (areaId?: string) => string
  imageSize: PhotoSize
  groupCurrency: string
  generatedDate: string
}

export function LocationReport({
  locationName,
  locationIcon,
  groupName,
  commodities,
  covers,
  areaLabelFor,
  imageSize,
  groupCurrency,
  generatedDate,
}: LocationReportProps) {
  const { t } = useTranslation()
  const noValue = t("reports:insurance.noValue")
  const totals = aggregateLocationTotals(commodities)

  return (
    <div className="space-y-10" data-testid="report-location">
      {/* Header */}
      <div className="-mx-10 -mt-8 bg-primary px-10 py-8 text-primary-foreground print:-mx-0">
        <div className="flex items-start justify-between gap-6">
          <div>
            <div className="mb-3 flex items-center gap-2">
              <Building2 className="size-5 opacity-70" aria-hidden="true" />
              <span className="text-sm font-medium uppercase tracking-widest opacity-70">
                {t("reports:insurance.locationReportTitle")}
              </span>
            </div>
            <h1 className="text-3xl font-bold leading-tight tracking-tight">
              {locationIcon ? <span aria-hidden="true">{locationIcon} </span> : null}
              {locationName}
            </h1>
            {groupName ? <p className="mt-1 text-sm opacity-70">{groupName}</p> : null}
          </div>
          <div className="shrink-0 text-right">
            <div className="mb-1 text-xs uppercase tracking-widest opacity-60">
              {t("reports:insurance.generated")}
            </div>
            <div className="text-sm font-medium">{generatedDate}</div>
            <div className="mt-1 text-xs opacity-60">
              {t("reports:insurance.cards.totalItems")}: {totals.count}
            </div>
          </div>
        </div>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-3 gap-4">
        <SummaryCard
          icon={Package}
          label={t("reports:insurance.cards.totalItems")}
          value={String(totals.count)}
        />
        <SummaryCard
          icon={DollarSign}
          label={t("reports:insurance.cards.totalPurchase")}
          value={formatCurrency(totals.purchase, groupCurrency)}
        />
        <SummaryCard
          icon={DollarSign}
          label={t("reports:insurance.cards.totalValue")}
          value={formatCurrency(totals.value, groupCurrency)}
        />
      </div>

      {/* Per-item sections */}
      {commodities.map((item, idx) => {
        const type = item.type as CommodityTypeValue | undefined
        const status = warrantyStatus({ warranty_expires_at: item.warranty_expires_at })
        const wConfig = WARRANTY_STATUS_CONFIG[status]
        const WarrantyIcon = wConfig.icon
        const photos = coverToPhotos(
          item.id ? covers[item.id] : undefined,
          imageSize,
          item.name ?? ""
        )
        const purchase =
          item.converted_original_price !== undefined
            ? formatCurrency(Number(item.converted_original_price), groupCurrency)
            : noValue
        const value =
          item.current_price !== undefined
            ? formatCurrency(Number(item.current_price), groupCurrency)
            : noValue
        return (
          <div
            key={item.id}
            className="print:break-inside-avoid"
            data-testid="report-location-item"
          >
            {idx > 0 ? <Separator className="mb-10" /> : null}

            {/* Item heading */}
            <div className="mb-6 flex items-start justify-between gap-4">
              <div>
                <div className="mb-1 flex items-center gap-2">
                  <span className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
                    {type ? t(`commodities:type.${type}`) : noValue}
                  </span>
                  <span className="text-xs text-muted-foreground">·</span>
                  <span className="text-xs text-muted-foreground">
                    {areaLabelFor(item.area_id)}
                  </span>
                </div>
                <h2 className="text-xl font-bold tracking-tight">{item.name}</h2>
                {item.short_name && item.short_name !== item.name ? (
                  <p className="text-sm text-muted-foreground">{item.short_name}</p>
                ) : null}
              </div>
              <div
                className={cn(
                  "flex shrink-0 items-center gap-1.5 rounded-full border border-border px-3 py-1 text-xs font-semibold",
                  wConfig.bg,
                  wConfig.text
                )}
              >
                <WarrantyIcon className="size-3" aria-hidden="true" />
                {t(wConfig.i18nKey)}
              </div>
            </div>

            {/* Financials + details */}
            <div className="mb-6 grid grid-cols-2 gap-6">
              <div className="space-y-3">
                <ItemDetail
                  icon={DollarSign}
                  label={t("reports:insurance.cards.purchasePrice")}
                  value={purchase}
                  mono
                />
                <ItemDetail
                  icon={DollarSign}
                  label={t("reports:insurance.cards.estimatedValue")}
                  value={value}
                  mono
                />
              </div>
              <div className="space-y-3">
                <ItemDetail
                  icon={Hash}
                  label={t("reports:insurance.fields.serialNumber")}
                  value={item.serial_number || noValue}
                  mono
                />
                <ItemDetail
                  icon={Calendar}
                  label={t("reports:insurance.fields.purchaseDate")}
                  value={
                    item.purchase_date ? formatDate(item.purchase_date, { style: "long" }) : noValue
                  }
                />
              </div>
            </div>

            {/* Warranty note */}
            {item.warranty_expires_at || item.warranty_notes ? (
              <div
                className={cn("mb-6 rounded-lg border border-border px-4 py-3 text-sm", wConfig.bg)}
              >
                <span className={cn("font-medium", wConfig.text)}>
                  {t("reports:insurance.warranty.label")}{" "}
                </span>
                {item.warranty_expires_at ? (
                  <span className="text-muted-foreground">
                    {t("reports:insurance.warranty.expires", {
                      date: formatDate(item.warranty_expires_at, { style: "long" }),
                    })}
                    .{" "}
                  </span>
                ) : null}
                {item.warranty_notes ? (
                  <span className="text-muted-foreground">{item.warranty_notes}</span>
                ) : null}
              </div>
            ) : null}

            {/* Photos */}
            <PhotoSection photos={photos} imageSize={imageSize} />
          </div>
        )
      })}

      {commodities.length === 0 ? (
        <div
          className="rounded-xl border border-dashed border-border p-10 text-center"
          data-testid="report-location-empty"
        >
          <Package className="mx-auto mb-3 size-10 text-muted-foreground/30" aria-hidden="true" />
          <p className="text-sm text-muted-foreground">{t("reports:insurance.empty.noItems")}</p>
        </div>
      ) : null}
    </div>
  )
}

function SummaryCard({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Package
  label: string
  value: string
}) {
  return (
    <div className="rounded-xl border border-border bg-muted/30 p-5">
      <div className="mb-2 flex items-center gap-2">
        <Icon className="size-4 text-muted-foreground" aria-hidden="true" />
        <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          {label}
        </span>
      </div>
      <p className="text-2xl font-bold tabular-nums">{value}</p>
    </div>
  )
}

function ItemDetail({
  icon: Icon,
  label,
  value,
  mono,
}: {
  icon: typeof Package
  label: string
  value: string
  mono?: boolean
}) {
  return (
    <div className="flex items-start gap-3">
      <div className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-md bg-muted">
        <Icon className="size-3 text-muted-foreground" aria-hidden="true" />
      </div>
      <div>
        <p className="text-xs text-muted-foreground">{label}</p>
        <p className={cn("text-sm font-medium", mono ? "font-mono tabular-nums" : "")}>{value}</p>
      </div>
    </div>
  )
}
