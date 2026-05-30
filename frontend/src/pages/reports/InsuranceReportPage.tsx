import { useEffect, useMemo } from "react"
import { Link, useSearchParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { ArrowLeft, Building2, LayoutGrid, MapPin, Package, Printer, Rows3 } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
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
import { useAreas } from "@/features/areas/hooks"
import { useCommodities, useCommodity } from "@/features/commodities/hooks"
import { useFiles } from "@/features/files/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useLocations } from "@/features/locations/hooks"
import { ItemReport } from "@/features/reports/components/ItemReport"
import { LocationReport } from "@/features/reports/components/LocationReport"
import type { PhotoSize } from "@/features/reports/components/PhotoSection"
import { formatDateTime } from "@/lib/intl"
import { cn } from "@/lib/utils"

type ReportMode = "item" | "location"

function parseMode(raw: string | null): ReportMode {
  return raw === "item" ? "item" : "location"
}

function parsePhotoSize(raw: string | null): PhotoSize {
  return raw === "full" ? "full" : "thumb"
}

// InsuranceReportPage (#1370) — a print-capable report at
// `/g/:slug/reports/insurance`. Two modes (item / location) toggled in a
// toolbar, with a selector + thumb/full photo toggle + Print. Mounted
// inside the protected <Shell> like CommodityPrintPage; the toolbar is
// `print:hidden` so only the report sheet lands on paper. Selection is
// driven by the query string: `?mode=item&item=<id>` /
// `?mode=location&location=<id>`. Default mode is `location` with the
// first location preselected.
export function InsuranceReportPage() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const enabled = !!currentGroup
  const slug = currentGroup?.slug
  const groupCurrency = currentGroup?.group_currency ?? "USD"
  const groupName = currentGroup?.name ?? ""

  const [searchParams, setSearchParams] = useSearchParams()
  const mode = parseMode(searchParams.get("mode"))
  const photoSize = parsePhotoSize(searchParams.get("photos"))
  const itemParam = searchParams.get("item") ?? ""
  const locationParam = searchParams.get("location") ?? ""

  const areas = useAreas({ enabled })
  const locations = useLocations({ enabled })
  // Location mode loads the whole group's items (capped) and filters by
  // the selected location's areas. Item mode doesn't need the list, but
  // the hook is cheap and shares cache with other pages; gate it on mode
  // so item-only landings don't pay for it.
  const commoditiesQuery = useCommodities(
    { perPage: 1000, includeInactive: false },
    { enabled: enabled && mode === "location" }
  )

  // Resolve the effective selection: fall back to the first option when
  // the URL carries no (or a stale) id. Drives both the Select value and
  // the report body.
  const locationList = useMemo(() => locations.data ?? [], [locations.data])
  const selectedLocationId = useMemo(() => {
    if (locationParam && locationList.some((l) => l.id === locationParam)) return locationParam
    return locationList[0]?.id ?? ""
  }, [locationParam, locationList])

  // Item-mode list of items to populate the Select. Reuse the same
  // group-items query (shared cache) but only when in item mode.
  const itemListQuery = useCommodities(
    { perPage: 1000, includeInactive: false },
    { enabled: enabled && mode === "item" }
  )
  const itemList = useMemo(() => itemListQuery.data?.commodities ?? [], [itemListQuery.data])
  const selectedItemId = useMemo(() => {
    if (itemParam && itemList.some((c) => c.id === itemParam)) return itemParam
    return itemList[0]?.id ?? ""
  }, [itemParam, itemList])

  // Item-mode detail + its image files (the photo gallery source).
  const detail = useCommodity(selectedItemId || undefined, {
    enabled: enabled && mode === "item" && !!selectedItemId,
  })
  const imageFilesQuery = useFiles(
    {
      linkedEntityType: "commodity",
      linkedEntityId: selectedItemId,
      category: "images",
      perPage: 100,
    },
    { enabled: enabled && mode === "item" && !!selectedItemId }
  )

  // Resolve "{Location} · {Area}" for any area id.
  const areaLabelFor = useMemo(() => {
    const areaById = new Map<string, { name: string; locationId?: string }>()
    for (const a of areas.data ?? []) {
      if (a.id) areaById.set(a.id, { name: a.name ?? "", locationId: a.location_id })
    }
    const locationById = new Map<string, string>()
    for (const l of locations.data ?? []) {
      if (l.id) locationById.set(l.id, l.name ?? "")
    }
    return (areaId?: string) => {
      if (!areaId) return ""
      const area = areaById.get(areaId)
      if (!area) return ""
      const locationName = area.locationId ? (locationById.get(area.locationId) ?? "") : ""
      return locationName ? `${locationName} · ${area.name}` : area.name
    }
  }, [areas.data, locations.data])

  // Location mode: filter commodities to those whose area belongs to the
  // selected location.
  const locationAreaIds = useMemo(() => {
    const ids = new Set<string>()
    for (const a of areas.data ?? []) {
      if (a.location_id === selectedLocationId && a.id) ids.add(a.id)
    }
    return ids
  }, [areas.data, selectedLocationId])
  const locationCommodities = useMemo(
    () =>
      (commoditiesQuery.data?.commodities ?? []).filter(
        (c) => c.area_id && locationAreaIds.has(c.area_id)
      ),
    [commoditiesQuery.data, locationAreaIds]
  )
  const covers = commoditiesQuery.data?.covers ?? {}

  const selectedLocation = locationList.find((l) => l.id === selectedLocationId)
  const commodity = detail.data?.commodity

  // Document title — mirror CommodityPrintPage. Uses the active subject's
  // name once it resolves.
  const subjectName = mode === "item" ? (commodity?.name ?? "") : (selectedLocation?.name ?? "")
  useEffect(() => {
    if (typeof document === "undefined") return
    if (!subjectName) return
    document.title = t("reports:insurance.documentTitle", { name: subjectName })
  }, [subjectName, t])

  const generatedDate = useMemo(() => formatDateTime(new Date(), { dateStyle: "long" }), [])

  const backHref = slug ? `/g/${encodeURIComponent(slug)}/reports` : "#"

  function setParams(next: {
    mode?: ReportMode
    item?: string
    location?: string
    photos?: PhotoSize
  }) {
    const params = new URLSearchParams(searchParams)
    if (next.mode !== undefined) params.set("mode", next.mode)
    if (next.item !== undefined) params.set("item", next.item)
    if (next.location !== undefined) params.set("location", next.location)
    if (next.photos !== undefined) {
      if (next.photos === "thumb") params.delete("photos")
      else params.set("photos", next.photos)
    }
    setSearchParams(params, { replace: true })
  }

  const isLoading =
    mode === "item"
      ? detail.isLoading || itemListQuery.isLoading
      : commoditiesQuery.isLoading || locations.isLoading
  const isError =
    mode === "item"
      ? detail.isError || itemListQuery.isError
      : commoditiesQuery.isError || locations.isError

  return (
    <>
      <RouteTitle title={t("reports:insurance.title")} />
      <style>{`
        @media print {
          .print-hide { display: none !important; }
          body { background: white !important; }
          .print-sheet { box-shadow: none !important; border: 0 !important; border-radius: 0 !important; }
          .print-section { break-inside: avoid; }
        }
      `}</style>
      <div className="min-h-svh bg-muted py-8" data-testid="page-insurance-report">
        <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
          {/* Toolbar — hidden on paper. */}
          <div
            className="print-hide flex flex-wrap items-center gap-2 print:hidden"
            data-testid="insurance-report-toolbar"
          >
            <Button asChild variant="ghost" size="sm" className="gap-1">
              <Link to={backHref}>
                <ArrowLeft className="size-4" aria-hidden="true" />
                {t("reports:insurance.back")}
              </Link>
            </Button>

            <Separator orientation="vertical" className="h-4" />

            {/* Mode toggle (segmented buttons — the app has no ToggleGroup
                primitive; see design-deviations). */}
            <div
              role="tablist"
              aria-label={t("reports:insurance.title")}
              className="flex items-center gap-1 rounded-lg bg-background p-1"
              data-testid="insurance-report-mode"
            >
              <ModeButton
                active={mode === "item"}
                onClick={() => setParams({ mode: "item" })}
                icon={Package}
                label={t("reports:insurance.modes.item")}
                testId="insurance-report-mode-item"
              />
              <ModeButton
                active={mode === "location"}
                onClick={() => setParams({ mode: "location" })}
                icon={MapPin}
                label={t("reports:insurance.modes.location")}
                testId="insurance-report-mode-location"
              />
            </div>

            {/* Selector */}
            {mode === "item" ? (
              <Select
                value={selectedItemId || undefined}
                onValueChange={(v) => setParams({ item: v })}
              >
                <SelectTrigger
                  size="sm"
                  className="w-52"
                  data-testid="insurance-report-item-select"
                >
                  <SelectValue placeholder={t("reports:insurance.selectItem")} />
                </SelectTrigger>
                <SelectContent>
                  {itemList.map((c) => (
                    <SelectItem key={c.id} value={c.id ?? ""}>
                      {c.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : (
              <Select
                value={selectedLocationId || undefined}
                onValueChange={(v) => setParams({ location: v })}
              >
                <SelectTrigger
                  size="sm"
                  className="w-52"
                  data-testid="insurance-report-location-select"
                >
                  <SelectValue placeholder={t("reports:insurance.selectLocation")} />
                </SelectTrigger>
                <SelectContent>
                  {locationList.map((l) => (
                    <SelectItem key={l.id} value={l.id ?? ""}>
                      {l.icon ? `${l.icon} ` : ""}
                      {l.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}

            {/* Photo size toggle + Print */}
            <div className="ml-auto flex items-center gap-2">
              <span className="hidden text-xs text-muted-foreground sm:block">
                {t("reports:insurance.photos.label")}
              </span>
              <div
                role="group"
                aria-label={t("reports:insurance.photos.label")}
                className="flex items-center gap-1 rounded-lg bg-background p-1"
                data-testid="insurance-report-photo-size"
              >
                <ModeButton
                  active={photoSize === "thumb"}
                  onClick={() => setParams({ photos: "thumb" })}
                  icon={LayoutGrid}
                  label={t("reports:insurance.photos.thumbnails")}
                  testId="insurance-report-photo-thumb"
                  hideLabelOnMobile
                />
                <ModeButton
                  active={photoSize === "full"}
                  onClick={() => setParams({ photos: "full" })}
                  icon={Rows3}
                  label={t("reports:insurance.photos.full")}
                  testId="insurance-report-photo-full"
                  hideLabelOnMobile
                />
              </div>

              <Separator orientation="vertical" className="h-4" />

              <Button
                type="button"
                size="sm"
                className="gap-1.5"
                onClick={() => window.print()}
                data-testid="insurance-report-print"
              >
                <Printer className="size-4" aria-hidden="true" />
                <span className="hidden sm:inline">{t("reports:insurance.print")}</span>
              </Button>
            </div>
          </div>

          {/* Report sheet */}
          <article
            className="print-sheet overflow-hidden rounded-2xl border border-border bg-card shadow-sm print:rounded-none print:border-0 print:shadow-none"
            data-testid="insurance-report-sheet"
          >
            <div className="px-10 py-8">
              {isError ? (
                <Alert variant="destructive" data-testid="insurance-report-error">
                  <AlertTitle>{t("commodities:detail.errorTitle")}</AlertTitle>
                  <AlertDescription>{t("commodities:detail.errorDescription")}</AlertDescription>
                </Alert>
              ) : isLoading ? (
                <div data-testid="insurance-report-loading">
                  <Skeleton className="h-8 w-64" />
                  <Skeleton className="mt-3 h-4 w-40" />
                  <div className="mt-6 grid grid-cols-2 gap-4">
                    {Array.from({ length: 2 }).map((_, i) => (
                      <Skeleton key={i} className="h-24 rounded-xl" />
                    ))}
                  </div>
                </div>
              ) : mode === "item" ? (
                commodity ? (
                  <ItemReport
                    commodity={commodity}
                    imageFiles={imageFilesQuery.data?.files ?? []}
                    imageSize={photoSize}
                    areaLabel={areaLabelFor(commodity.area_id)}
                    groupCurrency={groupCurrency}
                    purchaseCurrency={commodity.original_price_currency ?? groupCurrency}
                    generatedDate={generatedDate}
                  />
                ) : (
                  <EmptyState message={t("reports:insurance.empty.noItem")} />
                )
              ) : selectedLocation ? (
                <LocationReport
                  locationName={selectedLocation.name ?? ""}
                  locationIcon={selectedLocation.icon}
                  groupName={groupName}
                  commodities={locationCommodities}
                  covers={covers}
                  areaLabelFor={areaLabelFor}
                  imageSize={photoSize}
                  groupCurrency={groupCurrency}
                  generatedDate={generatedDate}
                />
              ) : (
                <EmptyState message={t("reports:insurance.empty.noLocation")} />
              )}

              {/* Footer */}
              <Separator className="mb-6 mt-10" />
              <div
                className="flex items-center justify-between pb-2 text-xs text-muted-foreground"
                data-testid="insurance-report-footer"
              >
                <div className="flex items-center gap-2">
                  <Building2 className="size-3.5" aria-hidden="true" />
                  <span>{t("reports:insurance.footer", { group: groupName })}</span>
                </div>
                <span>{t("reports:insurance.footerGenerated", { date: generatedDate })}</span>
              </div>
            </div>
          </article>
        </div>
      </div>
    </>
  )
}

interface ModeButtonProps {
  active: boolean
  onClick: () => void
  icon: typeof Package
  label: string
  testId: string
  hideLabelOnMobile?: boolean
}

function ModeButton({
  active,
  onClick,
  icon: Icon,
  label,
  testId,
  hideLabelOnMobile,
}: ModeButtonProps) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={cn(
        "flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium transition-all",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        active
          ? "bg-muted text-foreground shadow-sm"
          : "text-muted-foreground hover:text-foreground"
      )}
      data-testid={testId}
      data-state={active ? "active" : "inactive"}
    >
      <Icon className="size-3.5" aria-hidden="true" />
      <span className={hideLabelOnMobile ? "hidden sm:inline" : undefined}>{label}</span>
    </button>
  )
}

function EmptyState({ message }: { message: string }) {
  return (
    <div
      className="rounded-xl border border-dashed border-border p-10 text-center"
      data-testid="insurance-report-empty"
    >
      <Package className="mx-auto mb-3 size-10 text-muted-foreground/30" aria-hidden="true" />
      <p className="text-sm text-muted-foreground">{message}</p>
    </div>
  )
}
