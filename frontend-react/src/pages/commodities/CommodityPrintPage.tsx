import { useEffect } from "react"
import { Link, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { ArrowLeft, Printer } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAreas } from "@/features/areas/hooks"
import { useCommodity } from "@/features/commodities/hooks"
import { COMMODITY_TYPE_ICONS, type CommodityTypeValue } from "@/features/commodities/constants"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { formatCurrency, formatDate } from "@/lib/intl"

// /commodities/:id/print — print-optimized layout for a single
// commodity. The toolbar (Back / Print) is hidden under @media print
// so the rendered page sheet contains only the report sections.
//
// The layout intentionally avoids the app shell (sidebar / topbar) —
// the route mounts directly under the protected outlet rather than the
// Shell layout, so the print preview matches the printed output.
export function CommodityPrintPage() {
  const { t } = useTranslation()
  const { id = "" } = useParams<{ id: string }>()
  const { currentGroup } = useCurrentGroup()
  const enabled = !!currentGroup
  const detail = useCommodity(id, { enabled })
  const areas = useAreas({ enabled })
  const commodity = detail.data?.commodity

  useEffect(() => {
    if (!commodity?.name) return
    if (typeof document !== "undefined") {
      document.title = t("commodities:print.documentTitle", { name: commodity.name })
    }
  }, [commodity?.name, t])

  const slug = currentGroup?.slug
  const detailHref =
    slug && id ? `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}` : "#"

  if (detail.isLoading) {
    return (
      <div className="p-8 max-w-3xl mx-auto" data-testid="commodity-print-loading">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="mt-3 h-4 w-32" />
      </div>
    )
  }
  if (detail.isError || !commodity) {
    return (
      <div className="p-8 max-w-3xl mx-auto">
        <Alert variant="destructive">
          <AlertTitle>{t("commodities:detail.errorTitle")}</AlertTitle>
          <AlertDescription>{t("commodities:detail.errorDescription")}</AlertDescription>
        </Alert>
      </div>
    )
  }

  const type = commodity.type as CommodityTypeValue | undefined
  const icon = type ? COMMODITY_TYPE_ICONS[type] : "📦"
  // Per the BE: `original_price` lives in `original_price_currency`;
  // `converted_original_price` and `current_price` are in the group
  // main currency. Use both.
  const groupCurrency = currentGroup?.main_currency ?? "USD"
  const purchaseCurrency = commodity.original_price_currency ?? groupCurrency
  const noValue = t("commodities:print.noPrice")
  const areaName = (areas.data ?? []).find((a) => a.id === commodity.area_id)?.name ?? noValue

  return (
    <>
      <RouteTitle title={t("commodities:print.documentTitle", { name: commodity.name })} />
      <style>{`
        @media print {
          .print-hide { display: none !important; }
          body { background: white !important; }
          .print-sheet { box-shadow: none !important; padding: 0 !important; }
          .print-section { break-inside: avoid; }
        }
      `}</style>
      <div className="bg-muted min-h-svh py-8" data-testid="page-commodity-print">
        <div className="max-w-3xl mx-auto flex flex-col gap-6">
          <div className="print-hide flex items-center justify-between">
            <Button asChild variant="ghost" size="sm" className="gap-1">
              <Link to={detailHref}>
                <ArrowLeft className="size-4" aria-hidden="true" />
                {t("commodities:print.back")}
              </Link>
            </Button>
            <Button
              type="button"
              onClick={() => window.print()}
              className="gap-1.5"
              data-testid="commodity-print-trigger"
            >
              <Printer className="size-4" aria-hidden="true" />
              {t("commodities:print.print")}
            </Button>
          </div>

          <article className="print-sheet bg-card rounded-md border border-border p-8 shadow-sm">
            <header className="mb-6 flex items-start gap-4 border-b border-border pb-6">
              <div className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-muted text-2xl">
                {icon}
              </div>
              <div className="flex-1 min-w-0">
                <h1 className="text-2xl font-bold tracking-tight">{commodity.name}</h1>
                <p className="mt-1 text-sm text-muted-foreground">
                  {commodity.short_name ? `${commodity.short_name} · ` : ""}
                  {type ? t(`commodities:type.${type}`) : ""}
                </p>
              </div>
              <p className="text-xs text-muted-foreground">
                {formatDate(new Date(), { style: "short" })}
              </p>
            </header>

            <section className="print-section grid grid-cols-2 gap-4 mb-6">
              <h2 className="col-span-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                {t("commodities:print.sectionBasics")}
              </h2>
              <PrintRow label={t("commodities:detail.fields.area")} value={areaName} />
              <PrintRow
                label={t("commodities:detail.fields.count")}
                value={String(commodity.count ?? "—")}
              />
              <PrintRow
                label={t("commodities:detail.fields.status")}
                value={commodity.status ? t(`commodities:status.${commodity.status}`) : noValue}
              />
              <PrintRow
                label={t("commodities:detail.fields.serialNumber")}
                value={commodity.serial_number || noValue}
              />
            </section>

            <section className="print-section grid grid-cols-2 gap-4 mb-6">
              <h2 className="col-span-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                {t("commodities:print.sectionPurchase")}
              </h2>
              <PrintRow
                label={t("commodities:detail.fields.purchaseDate")}
                value={
                  commodity.purchase_date
                    ? formatDate(commodity.purchase_date as string, { style: "medium" })
                    : noValue
                }
              />
              <PrintRow
                label={t("commodities:detail.fields.originalPrice")}
                value={
                  commodity.original_price !== undefined
                    ? formatCurrency(commodity.original_price, purchaseCurrency)
                    : noValue
                }
              />
              <PrintRow
                label={t("commodities:detail.fields.convertedOriginalPrice")}
                value={
                  commodity.converted_original_price !== undefined
                    ? formatCurrency(commodity.converted_original_price, groupCurrency)
                    : noValue
                }
              />
              <PrintRow
                label={t("commodities:detail.fields.currentPrice")}
                value={
                  commodity.current_price !== undefined
                    ? formatCurrency(commodity.current_price, groupCurrency)
                    : noValue
                }
              />
            </section>

            {(commodity.tags && commodity.tags.length > 0) ||
            commodity.comments ||
            (commodity.extra_serial_numbers && commodity.extra_serial_numbers.length > 0) ||
            (commodity.part_numbers && commodity.part_numbers.length > 0) ? (
              <section className="print-section flex flex-col gap-3">
                <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                  {t("commodities:print.sectionExtras")}
                </h2>
                {commodity.tags && commodity.tags.length > 0 ? (
                  <PrintRow
                    label={t("commodities:detail.fields.tags")}
                    value={commodity.tags.join(", ")}
                  />
                ) : null}
                {commodity.extra_serial_numbers && commodity.extra_serial_numbers.length > 0 ? (
                  <PrintRow
                    label={t("commodities:detail.fields.extraSerialNumbers")}
                    value={commodity.extra_serial_numbers.join(", ")}
                  />
                ) : null}
                {commodity.part_numbers && commodity.part_numbers.length > 0 ? (
                  <PrintRow
                    label={t("commodities:detail.fields.partNumbers")}
                    value={commodity.part_numbers.join(", ")}
                  />
                ) : null}
                {commodity.comments ? (
                  <PrintRow
                    label={t("commodities:detail.fields.comments")}
                    value={commodity.comments}
                  />
                ) : null}
              </section>
            ) : null}
          </article>
        </div>
      </div>
    </>
  )
}

function PrintRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs uppercase tracking-wide text-muted-foreground">{label}</span>
      <span className="text-sm">{value}</span>
    </div>
  )
}
