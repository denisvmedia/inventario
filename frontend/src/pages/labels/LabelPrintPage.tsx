import { Link, useSearchParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { useQueries } from "@tanstack/react-query"
import { ArrowLeft, Printer } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { getArea } from "@/features/areas/api"
import { areaKeys } from "@/features/areas/keys"
import { getCommodity, type Commodity } from "@/features/commodities/api"
import { formatAssetId } from "@/features/commodities/asset-id"
import { useAllCommodities } from "@/features/commodities/hooks"
import { commodityKeys } from "@/features/commodities/keys"
import { useCurrentGroup } from "@/features/group/GroupContext"
import {
  LABEL_FORMATS,
  LABEL_FORMAT_IDS,
  defaultSheetFormat,
  isLabelFormatId,
  labelsPerSheet,
  paginateLabels,
  type LabelFormat,
  type LabelFormatId,
} from "@/features/labels/formats"
import { LabelSheets, type LabelData } from "@/features/labels/LabelSheets"
import { labelTargetUrl } from "@/features/labels/links"
import { useLocations } from "@/features/locations/hooks"
import { cn } from "@/lib/utils"

function unique(values: string[]): string[] {
  return [...new Set(values.filter(Boolean))]
}

// Items in an area print in asset-ID order, the order the numbers were
// handed out; items without a number yet go last, by name.
function byAssetNumber(a: Commodity, b: Commodity): number {
  const an = a.asset_number ?? Number.POSITIVE_INFINITY
  const bn = b.asset_number ?? Number.POSITIVE_INFINITY
  if (an !== bn) return an < bn ? -1 : 1
  return (a.name ?? "").localeCompare(b.name ?? "")
}

function pageSize(format: LabelFormat): string {
  return `${format.page.width}${format.unit} ${format.page.height}${format.unit}`
}

// /g/:slug/labels — printable labels with a QR code that opens the item
// or area page. What to print comes from the query string:
//   item=<id>        an item label (repeatable)
//   area=<id>        an area label (repeatable)
//   area_items=<id>  a label for every active item in that area
//   format=<id>      single | a4-3x7 | letter-3x10
//   skip=<n>         empty positions at the start of the first sheet
//
// The page lives inside the Shell like the other print pages: the global
// print rules in index.css hide the chrome and drop the container
// padding, and this page drops its own padding on paper, so the sheet
// starts at the corner of the page and the labels land on the die-cut
// positions. @page sets the paper size and removes the printer margins.
export function LabelPrintPage() {
  const { t } = useTranslation()
  const [params, setParams] = useSearchParams()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const enabled = !!currentGroup

  const itemIds = unique(params.getAll("item"))
  const areaIds = unique(params.getAll("area"))
  const areaItemsId = params.get("area_items") ?? ""

  const itemQueries = useQueries({
    queries: itemIds.map((id) => ({
      queryKey: commodityKeys.detail(slug, id),
      queryFn: ({ signal }: { signal: AbortSignal }) => getCommodity(id, signal),
      enabled,
    })),
  })
  const areaQueries = useQueries({
    queries: areaIds.map((id) => ({
      queryKey: areaKeys.detail(slug, id),
      queryFn: ({ signal }: { signal: AbortSignal }) => getArea(id, signal),
      enabled,
    })),
  })
  const areaItems = useAllCommodities(
    { areaId: areaItemsId },
    { enabled: enabled && !!areaItemsId }
  )
  const locations = useLocations({ enabled: enabled && areaIds.length > 0 })

  const singleRequest = itemIds.length + areaIds.length === 1 && !areaItemsId
  const formatParam = params.get("format")
  const formatId: LabelFormatId = isLabelFormatId(formatParam)
    ? formatParam
    : singleRequest
      ? "single"
      : defaultSheetFormat(navigator.language)
  const format = LABEL_FORMATS[formatId]
  const isSheet = labelsPerSheet(format) > 1
  const maxSkip = labelsPerSheet(format) - 1
  const skip = isSheet
    ? Math.min(Math.max(0, Number.parseInt(params.get("skip") ?? "", 10) || 0), maxSkip)
    : 0

  const formatLabels: Record<LabelFormatId, string> = {
    single: t("commodities:labels.formats.single"),
    "a4-3x7": t("commodities:labels.formats.a4Sheet"),
    "letter-3x10": t("commodities:labels.formats.letterSheet"),
  }

  const origin = window.location.origin
  const labels: LabelData[] = []
  if (slug) {
    const locationNames = new Map((locations.data ?? []).map((l) => [l.id, l.name]))
    for (const [index, query] of areaQueries.entries()) {
      const area = query.data
      if (!area) continue
      const id = area.id ?? areaIds[index]
      labels.push({
        key: `area-${id}`,
        url: labelTargetUrl(origin, slug, "areas", id),
        qrTitle: t("commodities:labels.areaQrTitle", { name: area.name }),
        title: area.name ?? "",
        detail: (area.location_id && locationNames.get(area.location_id)) || undefined,
      })
    }
    const seen = new Set<string>()
    const itemLabel = (commodity: Commodity, fallbackId: string) => {
      const id = commodity.id ?? fallbackId
      if (!id || seen.has(id)) return
      seen.add(id)
      labels.push({
        key: `item-${id}`,
        url: labelTargetUrl(origin, slug, "commodities", id),
        qrTitle: t("commodities:labels.itemQrTitle", { name: commodity.name }),
        code: formatAssetId(commodity.asset_number) || undefined,
        title: commodity.name ?? "",
      })
    }
    for (const commodity of [...(areaItems.data?.commodities ?? [])].sort(byAssetNumber)) {
      itemLabel(commodity, "")
    }
    for (const [index, query] of itemQueries.entries()) {
      if (query.data) itemLabel(query.data.commodity, itemIds[index])
    }
  }
  const sheets = paginateLabels(labels, format, skip)

  const isLoading =
    itemQueries.some((q) => q.isLoading) ||
    areaQueries.some((q) => q.isLoading) ||
    areaItems.isLoading
  const hasError =
    itemQueries.some((q) => q.isError) || areaQueries.some((q) => q.isError) || areaItems.isError

  const updateParams = (patch: Record<string, string | null>) => {
    const next = new URLSearchParams(params)
    for (const [key, value] of Object.entries(patch)) {
      if (value === null) next.delete(key)
      else next.set(key, value)
    }
    setParams(next, { replace: true })
  }

  const group = slug ? `/g/${encodeURIComponent(slug)}` : ""
  const backHref = !slug
    ? "/"
    : areaIds.length === 1 && itemIds.length === 0
      ? `${group}/areas/${encodeURIComponent(areaIds[0])}`
      : areaItemsId && itemIds.length === 0
        ? `${group}/areas/${encodeURIComponent(areaItemsId)}`
        : itemIds.length === 1 && areaIds.length === 0 && !areaItemsId
          ? `${group}/commodities/${encodeURIComponent(itemIds[0])}`
          : `${group}/commodities`

  return (
    <>
      <RouteTitle title={t("commodities:labels.title")} />
      <style>{`
        @page { size: ${pageSize(format)}; margin: 0; }
        .label-paper { background: white; color: black; }
        @media screen {
          .label-slot { outline: 1px dashed currentColor; outline-offset: -1px; }
          .label-slot-empty { outline: 1px dashed currentColor; outline-offset: -1px; opacity: 0.25; }
        }
        @media print {
          .label-sheet { break-after: page; }
          .label-sheet:last-child { break-after: auto; }
        }
      `}</style>
      <div
        className="min-h-svh bg-muted py-8 print:min-h-0 print:bg-transparent print:p-0"
        data-testid="page-labels"
      >
        <div className="mx-auto flex w-full max-w-4xl flex-col gap-6 px-4 print:block print:max-w-none print:p-0">
          <div
            className="flex flex-wrap items-center gap-2 print:hidden"
            data-testid="labels-toolbar"
          >
            <Button asChild variant="ghost" size="sm" className="gap-1">
              <Link to={backHref}>
                <ArrowLeft className="size-4" aria-hidden="true" />
                {t("commodities:labels.back")}
              </Link>
            </Button>

            <Separator orientation="vertical" className="h-4" />

            <Select
              value={formatId}
              onValueChange={(value) => updateParams({ format: value, skip: null })}
            >
              <SelectTrigger
                size="sm"
                className="w-64"
                aria-label={t("commodities:labels.format")}
                data-testid="labels-format-select"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {LABEL_FORMAT_IDS.map((id) => (
                  <SelectItem key={id} value={id} data-testid={`labels-format-${id}`}>
                    {formatLabels[id]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            {isSheet ? (
              <div className="flex items-center gap-2">
                <Label htmlFor="labels-skip" className="text-xs text-muted-foreground">
                  {t("commodities:labels.skip")}
                </Label>
                <Input
                  id="labels-skip"
                  type="number"
                  inputMode="numeric"
                  min={0}
                  max={maxSkip}
                  value={skip}
                  onChange={(event) => {
                    const value = Number.parseInt(event.target.value, 10)
                    updateParams({
                      skip: Number.isFinite(value) && value > 0 ? String(value) : null,
                    })
                  }}
                  className="h-8 w-16"
                  title={t("commodities:labels.skipHint")}
                  data-testid="labels-skip"
                />
              </div>
            ) : null}

            <div className="ml-auto flex items-center gap-2">
              {labels.length > 0 ? (
                <span className="text-xs text-muted-foreground" data-testid="labels-summary">
                  {t("commodities:labels.summary", { count: labels.length })}
                  {" · "}
                  {t("commodities:labels.pages", { count: sheets.length })}
                </span>
              ) : null}
              <Button
                type="button"
                size="sm"
                className="gap-1.5"
                onClick={() => window.print()}
                disabled={labels.length === 0}
                data-testid="labels-print"
              >
                <Printer className="size-4" aria-hidden="true" />
                {t("commodities:labels.print")}
              </Button>
            </div>
          </div>

          <p className="text-xs text-muted-foreground print:hidden">
            {t("commodities:labels.printHint")}
          </p>

          {hasError ? (
            <Alert variant="destructive" className="print:hidden" data-testid="labels-error">
              <AlertTitle>{t("commodities:labels.error.title")}</AlertTitle>
              <AlertDescription>{t("commodities:labels.error.description")}</AlertDescription>
            </Alert>
          ) : null}

          {isLoading ? (
            <div className="flex flex-col gap-3" data-testid="labels-loading">
              <Skeleton className="h-36 w-60" />
              <Skeleton className="h-4 w-40" />
            </div>
          ) : labels.length === 0 ? (
            <Alert data-testid="labels-empty">
              <AlertTitle>{t("commodities:labels.empty.title")}</AlertTitle>
              <AlertDescription>{t("commodities:labels.empty.description")}</AlertDescription>
            </Alert>
          ) : (
            <div className="overflow-x-auto pb-2 print:overflow-visible print:p-0">
              <div
                className={cn(
                  "print:block",
                  isSheet ? "flex flex-col gap-6" : "flex flex-wrap gap-4"
                )}
                data-testid="labels-preview"
                data-format={formatId}
              >
                <LabelSheets sheets={sheets} format={format} />
              </div>
            </div>
          )}
        </div>
      </div>
    </>
  )
}
