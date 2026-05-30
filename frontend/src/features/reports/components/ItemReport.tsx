import { Calendar, DollarSign, Hash, MapPin, Package, Tag, type LucideIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Separator } from "@/components/ui/separator"
import { WARRANTY_STATUS_CONFIG } from "@/components/warranty/config"
import type { Commodity } from "@/features/commodities/api"
import { warrantyStatus, type CommodityTypeValue } from "@/features/commodities/constants"
import type { ListedFile } from "@/features/files/api"
import { formatCurrency, formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

import { PhotoSection, type PhotoSize } from "./PhotoSection"
import { filesToPhotos } from "../photos"

// ItemReport is the single-item insurance report body (#1370). Mirrors
// the design mock's ItemReport, adapted to real fields: title uses the
// commodity name + short_name (no brand/model in the BE), category →
// `type` enum, currency via the group / purchase currencies, warranty via
// the canonical WARRANTY_STATUS_CONFIG.
interface ItemReportProps {
  commodity: Commodity
  // Image-category files attached to the commodity (from the unified
  // /files surface). Drives the photo gallery.
  imageFiles: ListedFile[]
  imageSize: PhotoSize
  // "{Location} · {Area}" breadcrumb for the item's area.
  areaLabel: string
  // Group currency — `converted_original_price` / `current_price` live here.
  groupCurrency: string
  // Purchase currency — `original_price` lives here.
  purchaseCurrency: string
  generatedDate: string
}

export function ItemReport({
  commodity,
  imageFiles,
  imageSize,
  areaLabel,
  groupCurrency,
  purchaseCurrency,
  generatedDate,
}: ItemReportProps) {
  const { t } = useTranslation()
  const noValue = t("reports:insurance.noValue")
  const type = commodity.type as CommodityTypeValue | undefined
  const status = warrantyStatus({ warranty_expires_at: commodity.warranty_expires_at })
  const wConfig = WARRANTY_STATUS_CONFIG[status]
  const WarrantyIcon = wConfig.icon
  const photos = filesToPhotos(imageFiles, imageSize)

  const purchasePrice =
    commodity.original_price !== undefined
      ? formatCurrency(Number(commodity.original_price), purchaseCurrency)
      : noValue
  const estimatedValue =
    commodity.current_price !== undefined
      ? formatCurrency(Number(commodity.current_price), groupCurrency)
      : noValue

  const details: { icon: LucideIcon; label: string; value: string }[] = [
    {
      icon: Tag,
      label: t("reports:insurance.fields.type"),
      value: type ? t(`commodities:type.${type}`) : noValue,
    },
    {
      icon: Hash,
      label: t("reports:insurance.fields.serialNumber"),
      value: commodity.serial_number || noValue,
    },
    {
      icon: Calendar,
      label: t("reports:insurance.fields.purchaseDate"),
      value: commodity.purchase_date
        ? formatDate(commodity.purchase_date, { style: "long" })
        : noValue,
    },
    {
      icon: MapPin,
      label: t("reports:insurance.fields.location"),
      value: areaLabel || noValue,
    },
  ]

  return (
    <div className="space-y-8" data-testid="report-item">
      {/* Header */}
      <div className="-mx-10 -mt-8 bg-primary px-10 py-8 text-primary-foreground print:-mx-0">
        <div className="flex items-start justify-between gap-6">
          <div>
            <div className="mb-3 flex items-center gap-2">
              <Package className="size-5 opacity-70" aria-hidden="true" />
              <span className="text-sm font-medium uppercase tracking-widest opacity-70">
                {t("reports:insurance.itemReportTitle")}
              </span>
            </div>
            <h1 className="text-3xl font-bold leading-tight tracking-tight">{commodity.name}</h1>
            {commodity.short_name && commodity.short_name !== commodity.name ? (
              <p className="mt-1 text-sm opacity-70">{commodity.short_name}</p>
            ) : null}
          </div>
          <div className="shrink-0 text-right">
            <div className="mb-1 text-xs uppercase tracking-widest opacity-60">
              {t("reports:insurance.generated")}
            </div>
            <div className="text-sm font-medium">{generatedDate}</div>
          </div>
        </div>
      </div>

      {/* Value cards */}
      <div className="grid grid-cols-2 gap-4">
        <div className="rounded-xl border border-border bg-muted/30 p-5">
          <div className="mb-2 flex items-center gap-2">
            <DollarSign className="size-4 text-muted-foreground" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              {t("reports:insurance.cards.purchasePrice")}
            </span>
          </div>
          <p className="text-2xl font-bold tabular-nums">{purchasePrice}</p>
          {commodity.purchase_date ? (
            <p className="mt-1 text-xs text-muted-foreground">
              {formatDate(commodity.purchase_date, { style: "long" })}
            </p>
          ) : null}
        </div>
        <div className="rounded-xl border border-border bg-muted/30 p-5">
          <div className="mb-2 flex items-center gap-2">
            <DollarSign className="size-4 text-muted-foreground" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              {t("reports:insurance.cards.estimatedValue")}
            </span>
          </div>
          <p className="text-2xl font-bold tabular-nums">{estimatedValue}</p>
          <p className="mt-1 text-xs text-muted-foreground">
            {t("reports:insurance.cards.estimatedValueHint")}
          </p>
        </div>
      </div>

      {/* Details */}
      <div>
        <h2 className="mb-4 text-xs font-semibold uppercase tracking-widest text-muted-foreground">
          {t("reports:insurance.sections.itemDetails")}
        </h2>
        <div className="grid grid-cols-2 gap-x-8 gap-y-4">
          {details.map(({ icon: Icon, label, value }) => (
            <div key={label} className="flex items-start gap-3">
              <div className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted">
                <Icon className="size-3.5 text-muted-foreground" aria-hidden="true" />
              </div>
              <div>
                <p className="text-xs text-muted-foreground">{label}</p>
                <p className="mt-0.5 text-sm font-medium">{value}</p>
              </div>
            </div>
          ))}
        </div>
      </div>

      <Separator />

      {/* Warranty */}
      <div>
        <h2 className="mb-4 text-xs font-semibold uppercase tracking-widest text-muted-foreground">
          {t("reports:insurance.sections.warranty")}
        </h2>
        <div
          className={cn("flex items-start gap-3 rounded-xl border border-border p-4", wConfig.bg)}
        >
          <div className="flex size-9 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
            <WarrantyIcon className={cn("size-4", wConfig.text)} aria-hidden="true" />
          </div>
          <div className="flex-1">
            <div className="flex items-center gap-2">
              <span className={cn("text-sm font-semibold", wConfig.text)}>
                {t(wConfig.i18nKey)}
              </span>
              {commodity.warranty_expires_at ? (
                <span className="text-xs text-muted-foreground">
                  {t("reports:insurance.warranty.expires", {
                    date: formatDate(commodity.warranty_expires_at, { style: "long" }),
                  })}
                </span>
              ) : null}
            </div>
            {commodity.warranty_notes ? (
              <p className="mt-1 text-sm text-muted-foreground">{commodity.warranty_notes}</p>
            ) : null}
          </div>
        </div>
      </div>

      {/* Photos */}
      {photos.length > 0 ? (
        <>
          <Separator />
          <PhotoSection photos={photos} imageSize={imageSize} />
        </>
      ) : null}

      {/* Notes */}
      {commodity.comments ? (
        <>
          <Separator />
          <div>
            <h2 className="mb-3 text-xs font-semibold uppercase tracking-widest text-muted-foreground">
              {t("reports:insurance.sections.notes")}
            </h2>
            <p className="rounded-xl border border-border bg-muted/30 px-4 py-3 text-sm leading-relaxed text-foreground whitespace-pre-wrap">
              {commodity.comments}
            </p>
          </div>
        </>
      ) : null}
    </div>
  )
}
