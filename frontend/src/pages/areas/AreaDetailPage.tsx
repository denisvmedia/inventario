import { useEffect, useState } from "react"
import { Link, useMatch, useNavigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { ChevronRight, Package, Pencil, Trash2, TrendingUp } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { LocationsBreadcrumb } from "@/components/locations/LocationsBreadcrumb"
import { AreaFormDialog } from "@/components/locations/AreaFormDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import { useArea, useDeleteArea, useUpdateArea } from "@/features/areas/hooks"
import { useCommodities, useCommoditiesValue } from "@/features/commodities/hooks"
import {
  COMMODITY_STATUS_TONES,
  COMMODITY_TYPE_ICONS,
  warrantyStatus,
  type CommodityStatusValue,
  type CommodityTypeValue,
} from "@/features/commodities/constants"
import type { Commodity } from "@/features/commodities/api"
import { useLocation, useLocations } from "@/features/locations/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatCurrency } from "@/lib/intl"
import { cn } from "@/lib/utils"

interface AreaDetailPageProps {
  initialMode?: "edit"
}

const ITEMS_PAGE_SIZE = 24

// /areas/:id — single-area detail. Header + edit/delete actions plus an
// inline items panel modelled on `design-mocks/src/views/LocationPickerView.tsx`
// Level 3 (item count + value stats, then the area's commodities as a
// scoped list). v1 trims the mock's full toolbar / bulk actions / area-files
// panel — those land in follow-ups under #1531.
export function AreaDetailPage({ initialMode }: AreaDetailPageProps = {}) {
  const { t } = useTranslation()
  const params = useParams<{ id: string }>()
  const id = params.id ?? ""
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug

  const area = useArea(id, { enabled: !!currentGroup })
  // Fetch the parent location for the breadcrumb. The detail endpoint
  // doesn't include the parent's name, so we hit /locations/:id once
  // the area resolves.
  const parent = useLocation(area.data?.location_id, {
    enabled: !!area.data?.location_id,
  })
  const allLocations = useLocations({ enabled: !!currentGroup })
  const updateArea = useUpdateArea(id)
  const deleteArea = useDeleteArea()

  const toast = useAppToast()
  const confirm = useConfirm()

  const [editOpen, setEditOpen] = useState(initialMode === "edit")

  const editMatch = useMatch({ path: "/g/:groupSlug/areas/:id/edit", end: true })
  useEffect(() => {
    // Deep-link sync from URL → local dialog state.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (editMatch && !editOpen) setEditOpen(true)
  }, [editMatch, editOpen])

  function closeDialog() {
    setEditOpen(false)
    if (editMatch && slug && id) {
      navigate(`/g/${encodeURIComponent(slug)}/areas/${encodeURIComponent(id)}`, {
        replace: true,
      })
    }
  }

  async function handleEdit(values: { name: string; location_id: string }) {
    await updateArea.mutateAsync(values)
    toast.success(t("locations:toast.areaUpdated"))
  }

  async function handleDelete() {
    if (!id) return
    const ok = await confirm({
      title: t("locations:delete.areaTitle", { name: area.data?.name ?? "" }),
      description: t("locations:delete.areaDescription"),
      confirmLabel: t("common:actions.delete"),
      destructive: true,
    })
    if (!ok) return
    try {
      await deleteArea.mutateAsync(id)
      toast.success(t("locations:toast.areaDeleted"))
      if (slug && area.data?.location_id) {
        navigate(
          `/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(area.data.location_id)}`,
          { replace: true }
        )
      } else if (slug) {
        navigate(`/g/${encodeURIComponent(slug)}/locations`, { replace: true })
      }
    } catch {
      toast.error(t("locations:toast.areaDeleteError"))
    }
  }

  if (area.isError) {
    return (
      <div className="flex flex-col gap-6 p-6 max-w-4xl mx-auto w-full">
        <RouteTitle title={t("locations:areaDetail.errorTitle")} />
        <Alert variant="destructive" data-testid="area-detail-error">
          <AlertTitle>{t("locations:areaDetail.errorTitle")}</AlertTitle>
          <AlertDescription>{t("locations:areaDetail.errorDescription")}</AlertDescription>
        </Alert>
      </div>
    )
  }

  const locationsHref = slug ? `/g/${encodeURIComponent(slug)}/locations` : "#"
  const parentHref =
    slug && area.data?.location_id
      ? `/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(area.data.location_id)}`
      : undefined
  // The breadcrumb's back chevron mirrors the most-natural "one level
  // up" target: parent location when known, locations list otherwise.
  const breadcrumbBackHref = parentHref ?? locationsHref

  return (
    <>
      <RouteTitle title={area.data?.name ?? t("locations:areaDetail.fallbackTitle")} />
      <div
        className="flex flex-col gap-6 p-6 max-w-4xl mx-auto w-full"
        data-testid="page-area-detail"
      >
        <LocationsBreadcrumb
          backHref={breadcrumbBackHref}
          backLabel={t("locations:areaDetail.back")}
          segments={[
            {
              label: t("locations:breadcrumb.locations"),
              to: locationsHref,
              testId: "breadcrumb-locations",
            },
            {
              label: parent.data?.name ?? t("locations:detail.fallbackTitle"),
              to: parentHref,
              testId: "breadcrumb-location",
            },
            {
              label: area.data?.name ?? t("locations:areaDetail.fallbackTitle"),
              testId: "breadcrumb-current",
            },
          ]}
          testId="area-detail-breadcrumb"
        />

        {area.isLoading ? (
          <div className="space-y-3" data-testid="area-detail-loading">
            <Skeleton className="h-8 w-64" />
            <Skeleton className="h-4 w-96" />
          </div>
        ) : area.data ? (
          <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0">
              <h1 className="flex items-center gap-2 text-2xl font-semibold tracking-tight">
                <Package className="size-5 text-muted-foreground" aria-hidden="true" />
                <span className="truncate">{area.data.name}</span>
              </h1>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <Button
                type="button"
                variant="outline"
                onClick={() => setEditOpen(true)}
                data-testid="area-detail-edit"
                className="gap-2"
              >
                <Pencil className="size-4" aria-hidden="true" />
                {t("locations:detail.edit")}
              </Button>
              <Button
                type="button"
                variant="outline"
                onClick={handleDelete}
                data-testid="area-detail-delete"
                className="gap-2"
              >
                <Trash2 className="size-4 text-destructive" aria-hidden="true" />
                {t("common:actions.delete")}
              </Button>
            </div>
          </header>
        ) : null}

        {area.data ? <AreaItemsSection areaId={id} slug={slug} /> : null}
      </div>

      <AreaFormDialog
        open={editOpen}
        onOpenChange={(open) => (open ? null : closeDialog())}
        area={area.data}
        locations={allLocations.data ?? []}
        onSubmit={handleEdit}
        isPending={updateArea.isPending}
      />
    </>
  )
}

interface AreaItemsSectionProps {
  areaId: string
  slug?: string
}

function AreaItemsSection({ areaId, slug }: AreaItemsSectionProps) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  // Default to active-only items — this page has no inactive/draft
  // toggle yet (the toolbar follow-up under #1531 owns it). Mirrors
  // the active-only first paint on `CommoditiesListPage` so drilling
  // into an area doesn't surface sold/lost/disposed rows.
  const items = useCommodities(
    { areaId, perPage: ITEMS_PAGE_SIZE, includeInactive: false },
    { enabled: !!currentGroup && !!areaId }
  )
  // jsonapi.NamedTotal entries carry a stable `id` alongside name +
  // value, so we match by id directly (area names aren't unique — only
  // `uuid` is).
  const values = useCommoditiesValue({ enabled: !!currentGroup })
  const currency = currentGroup?.group_currency ?? "USD"
  const total = items.data?.total ?? 0
  const rows = items.data?.commodities ?? []
  const areaValueEntry = values.data?.areaTotals.find((entry) => entry.id === areaId)
  // values pending → render skeleton; failed or no entry for this
  // area in the totals payload → "—". A literal $0.00 would
  // mis-represent both.
  const valueCell: { value: string; loading: boolean } = values.isLoading
    ? { value: "", loading: true }
    : values.isError || !areaValueEntry
      ? { value: "—", loading: false }
      : { value: formatCurrency(areaValueEntry.value, currency), loading: false }

  if (items.isLoading) {
    return <ItemsLoading />
  }

  if (items.isError) {
    return (
      <Alert variant="destructive" data-testid="area-detail-items-error">
        <AlertTitle>{t("locations:areaDetail.items.errorTitle")}</AlertTitle>
        <AlertDescription>{t("locations:areaDetail.items.errorDescription")}</AlertDescription>
      </Alert>
    )
  }

  return (
    <section className="flex flex-col gap-4" data-testid="area-detail-items">
      <div className="grid grid-cols-2 gap-3" data-testid="area-detail-items-stats">
        <StatCell
          icon={Package}
          label={t("locations:areaDetail.items.statsItems")}
          value={String(total)}
        />
        <StatCell
          icon={TrendingUp}
          label={t("locations:areaDetail.items.statsValue")}
          value={valueCell.value}
          loading={valueCell.loading}
          testId="area-detail-items-value"
        />
      </div>

      {total === 0 ? (
        <ItemsEmpty />
      ) : (
        <Card className="overflow-hidden p-0" data-testid="area-detail-items-list">
          <ul>
            {rows
              // BE invariant: list rows always carry `id`. Filtering
              // here keeps the type narrowed for `<ItemRow>` (which
              // requires a non-empty id to build its detail link) and
              // skips rather than emits a degenerate row if a future
              // schema drift breaks the invariant.
              .filter((row): row is Commodity & { id: string } => Boolean(row.id))
              .map((row, index) => (
                <ItemRow
                  key={row.id}
                  row={row}
                  slug={slug}
                  currency={currency}
                  showSeparator={index > 0}
                />
              ))}
          </ul>
        </Card>
      )}

      {total > rows.length && slug ? (
        <Link
          to={`/g/${encodeURIComponent(slug)}/commodities?area=${encodeURIComponent(areaId)}`}
          className="inline-flex items-center gap-1.5 self-start text-sm text-muted-foreground hover:text-foreground"
          data-testid="area-detail-items-overflow"
        >
          {t("locations:areaDetail.items.viewAll", { count: total })}
          <ChevronRight className="size-3.5" aria-hidden="true" />
        </Link>
      ) : null}
    </section>
  )
}

function StatCell({
  icon: Icon,
  label,
  value,
  loading,
  testId,
}: {
  icon: React.ElementType
  label: string
  value: string
  loading?: boolean
  testId?: string
}) {
  return (
    <div
      className="flex items-center gap-3 rounded-xl border border-border bg-card px-4 py-3"
      data-testid={testId}
    >
      <Icon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      <div className="min-w-0">
        {loading ? (
          <Skeleton className="h-4 w-16" />
        ) : (
          <p className="text-sm font-semibold leading-tight">{value}</p>
        )}
        <p className="text-xs text-muted-foreground">{label}</p>
      </div>
    </div>
  )
}

interface ItemRowProps {
  // Narrowed to require `id` at the call site (BE invariant filtered
  // upstream) so the detail link is never built with a `"#"` fallback.
  row: Commodity & { id: string }
  slug?: string
  currency: string
  showSeparator: boolean
}

function ItemRow({ row, slug, currency, showSeparator }: ItemRowProps) {
  const { t } = useTranslation()
  // /g/:slug/* routes guarantee the slug is present when this page
  // mounts; `useCommodities` is gated on `!!currentGroup` so a missing
  // slug means no rows anyway. Guard defensively rather than render a
  // `to="#"` link that quietly mutates the URL hash on click.
  if (!slug) return null
  const detailHref = `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(row.id)}`
  const status = row.status as CommodityStatusValue | undefined
  const tone = status ? COMMODITY_STATUS_TONES[status] : ""
  const typeIcon = COMMODITY_TYPE_ICONS[row.type as CommodityTypeValue] ?? "📦"
  const showStatusPill = status !== undefined && status !== "in_use"
  // Pre-#1535 the classifier accepted a `tags` fallback for the legacy
  // `warranty:YYYY-MM-DD` convention; migration 1779400000 drained
  // those tags into `warranty_expires_at`, so the column is now the
  // only signal.
  const wStatus = warrantyStatus({ warranty_expires_at: row.warranty_expires_at })
  return (
    <li>
      {showSeparator ? <Separator /> : null}
      <Link
        to={detailHref}
        className={cn(
          "flex w-full items-center gap-4 px-5 py-3.5 text-left transition-colors hover:bg-muted/50",
          row.draft && "opacity-70"
        )}
        data-testid="area-detail-items-row"
        data-commodity-id={row.id}
      >
        <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-lg">
          <span aria-hidden="true">{typeIcon}</span>
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
            <p className="truncate text-sm font-medium">{row.name}</p>
            {row.draft ? (
              <Badge
                variant="outline"
                className="h-4 shrink-0 border-dashed px-1 text-[10px] text-muted-foreground"
              >
                {t("commodities:list.draftBadge")}
              </Badge>
            ) : null}
          </div>
          {row.short_name ? (
            <p className="truncate text-xs text-muted-foreground">{row.short_name}</p>
          ) : null}
        </div>
        {showStatusPill && status ? (
          <span
            className={cn("shrink-0 rounded-full border px-2 py-0.5 text-xs font-medium", tone)}
          >
            {t(`commodities:status.${status}`)}
          </span>
        ) : (
          <WarrantyBadge status={wStatus} showIcon={false} className="shrink-0" />
        )}
        <p className="hidden w-20 shrink-0 text-right text-sm font-medium sm:block">
          {formatCurrency(Number(row.current_price ?? 0), currency)}
        </p>
      </Link>
    </li>
  )
}

function ItemsLoading() {
  return (
    <section className="flex flex-col gap-4" data-testid="area-detail-items-loading">
      <div className="grid grid-cols-2 gap-3">
        <Skeleton className="h-[58px] rounded-xl" />
        <Skeleton className="h-[58px] rounded-xl" />
      </div>
      <Card className="overflow-hidden p-0">
        <ul>
          {[0, 1, 2].map((i) => (
            <li key={i}>
              {i > 0 ? <Separator /> : null}
              <div className="flex items-center gap-4 px-5 py-3.5">
                <Skeleton className="size-9 shrink-0 rounded-lg" />
                <div className="flex flex-1 flex-col gap-2">
                  <Skeleton className="h-3 w-40" />
                  <Skeleton className="h-3 w-24" />
                </div>
                <Skeleton className="hidden h-4 w-20 sm:block" />
              </div>
            </li>
          ))}
        </ul>
      </Card>
    </section>
  )
}

function ItemsEmpty() {
  const { t } = useTranslation()
  return (
    <div
      className="flex flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border py-16"
      data-testid="area-detail-items-empty"
    >
      <Package className="size-8 text-muted-foreground/30" aria-hidden="true" />
      <p className="text-sm text-muted-foreground">{t("locations:areaDetail.items.empty")}</p>
    </div>
  )
}
