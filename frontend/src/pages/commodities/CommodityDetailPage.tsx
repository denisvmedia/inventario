import { useEffect, useMemo, useState } from "react"
import {
  Link,
  useLocation,
  useMatch,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom"
import { useTranslation } from "react-i18next"
import {
  ArrowLeft,
  BarChart3,
  Calendar,
  CircleDot,
  DollarSign,
  ExternalLink,
  FileText,
  Hash,
  MapPin,
  Package,
  Paperclip,
  Pencil,
  Printer,
  Tag,
  Trash2,
  TriangleAlert,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Page } from "@/components/ui/page"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { CommodityFilesTab } from "@/components/files/CommodityFilesTab"
import { DropOverlay } from "@/components/files/DropOverlay"
import { UploadFilesDialog } from "@/components/files/UploadFilesDialog"
import { useFileDropZone } from "@/components/files/useFileDropZone"
import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import {
  StatusTransitionDialog,
  type StatusTransitionPayload,
} from "@/components/items/StatusTransitionDialog"
import { LendTab } from "@/components/loans/LendTab"
import { MaintenanceTab } from "@/components/maintenance/MaintenanceTab"
import { ServiceTab } from "@/components/services/ServiceTab"
import { SuppliesTab } from "@/components/supplies/SuppliesTab"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAreas } from "@/features/areas/hooks"
import { useFiles } from "@/features/files/hooks"
import { useLocations } from "@/features/locations/hooks"
import { CommodityHistoryTimeline } from "@/features/commodities/CommodityHistoryTimeline"
import { CommodityThumb } from "@/features/commodities/CommodityThumb"
import {
  useCommodity,
  useDeleteCommodity,
  useSetCommodityCover,
  useUpdateCommodity,
} from "@/features/commodities/hooks"
import {
  COMMODITY_STATUS_TONES,
  warrantyStatus,
  type CommodityStatusValue,
  type CommodityTypeValue,
  type CommodityWarrantyStatus,
} from "@/features/commodities/constants"

// CHANGE-STATUS bar: terminal status transitions surfaced as quick
// buttons on the detail surface (mock: "Change Status" section).
// Mirror the mock's set sans `in_use`, since `in_use` is what we're
// transitioning *from*. Order matches the mock's row.
const TERMINAL_STATUSES = ["sold", "lost", "disposed", "written_off"] as const

// Mock parity colour-only mapping for the CHANGE STATUS quick
// buttons. Lifted from `inventario-design`'s
// `COMMODITY_STATUS_CONFIG.color` field — text-only, no bg/border
// tint, so the buttons are plain outline pills with coloured
// labels (the mock's `cn("gap-1.5 text-xs h-7", c.color)` pattern).
// Distinct from the project's `COMMODITY_STATUS_TONES` (which adds
// bg + border for the inline status pills elsewhere).
const STATUS_TRANSITION_TEXT_TONES: Record<(typeof TERMINAL_STATUSES)[number], string> = {
  sold: "text-chart-2",
  lost: "text-status-expiring",
  disposed: "text-muted-foreground",
  written_off: "text-status-expired",
}
import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import { WARRANTY_STATUS_CONFIG } from "@/components/warranty/config"
import type { Commodity } from "@/features/commodities/api"
import { useGroupMigrationLock } from "@/features/currency-migration/lock"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatCurrency, formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

type TabKey = "details" | "warranty" | "files" | "lend" | "service" | "supplies" | "maintenance"

const TAB_KEYS = [
  "details",
  "warranty",
  "files",
  "lend",
  "service",
  "supplies",
  "maintenance",
] as const

function parseTab(raw: string | null): TabKey {
  return (TAB_KEYS as readonly string[]).includes(raw ?? "") ? (raw as TabKey) : "details"
}

// CommodityDetailContent is the rendered body of the item detail surface
// — header, action buttons, tab strip, and the per-tab bodies. The same
// component is rendered in two visual frames:
//
//   - `variant="page"` (default): standard full-page wrapper at
//     `/g/:slug/commodities/:id`. Stable URL, deep-link friendly,
//     refresh-survivable. This is what the catch-all route and
//     direct-link arrivals see (#1546 AC2).
//   - `variant="sheet"`: rendered inside a right-side `<Sheet>` overlay
//     on top of the items list. Used when the user drills in from the
//     list — the URL still updates to the same `/commodities/:id` so
//     back/forward and `?tab=` deep-links work, but the list page
//     stays mounted behind the sheet (#1546 AC1).
//
// Tabs: Details, Warranty, Files, Lend, Service. The same hooks /
// query-string mirror / edit dialog / drop zone all run in both
// variants — the only difference is the outer chrome and a "Back to
// list" affordance vs. the Sheet's built-in close button.
// PageFrame swaps between a plain sheet body and the canonical Page wrapper.
// Keeps every render site (loading / error / not-found / main) routed through
// the same width token without duplicating the branching. Hoisted to module
// scope to satisfy react-hooks/static-components.
function PageFrame({
  isSheet,
  className,
  children,
  ...rest
}: React.ComponentProps<"div"> & { isSheet: boolean }) {
  if (isSheet) {
    return (
      <div className={className} {...rest}>
        {children}
      </div>
    )
  }
  return (
    <Page width="wide" className={className} {...rest}>
      {children}
    </Page>
  )
}

export interface CommodityDetailContentProps {
  /**
   * The commodity id to load. Pre-extracted from the URL by the page
   * or sheet wrapper so this component stays unaware of the route shape.
   */
  id: string
  /**
   * Outer-frame variant. Default `"page"`. `"sheet"` drops the
   * page-level max-width wrapper and the "Back to list" link (the
   * Sheet provides its own X close button), but otherwise renders
   * the same content with the same hooks and behaviour.
   */
  variant?: "page" | "sheet"
}

export function CommodityDetailContent({ id, variant = "page" }: CommodityDetailContentProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  // Outer-frame classes match the design mock 1:1
  // (`denisvmedia/inventario-design/src/components/ItemDetail.tsx`):
  // sheet mode uses `flex flex-col gap-0 px-5 pb-5` with each section
  // owning its own vertical padding (the mock's `<SheetHeader pt-6
  // pb-4>` + `flex gap-2 pb-4` + `mb-4 rounded-xl …` pattern). Page
  // mode keeps the wider 4xl centered-column layout that direct
  // landings expect. Scroll is owned by `<SheetContent
  // overflow-y-auto>` — leaving it off the inner wrapper avoids the
  // double-scrollbar bug copilot flagged.
  const isSheet = variant === "sheet"
  // Page-mode width comes from the canonical wide token via <Page width="wide">
  // (`PageFrame` below). The literal class strings here stay scoped to the sheet
  // variant so the page-layout convention guard's `max-w-*` check never trips
  // on a top-level page wrapper.
  const errorWrapperClass = isSheet ? "px-5 py-6" : undefined
  const mainWrapperClass = isSheet ? "relative flex flex-col gap-0 px-5 pb-5" : "relative"
  const { currentGroup } = useCurrentGroup()
  const migrationLock = useGroupMigrationLock()
  const slug = currentGroup?.slug
  const enabled = !!currentGroup
  const detail = useCommodity(id, { enabled })
  const areas = useAreas({ enabled })
  const locations = useLocations({ enabled })
  const update = useUpdateCommodity(id)
  const remove = useDeleteCommodity()
  const setCover = useSetCommodityCover(id)
  const toast = useAppToast()
  const confirm = useConfirm()
  // The Files tab label gets a count badge driven by this query.
  // perPage=1 keeps the round-trip cheap — we only need `meta.total`.
  // Gated on `enabled && id` so it doesn't fire before the slug
  // resolves, and the cache key matches the "all files attached to
  // this commodity" view the Files tab reuses.
  const filesCount = useFiles(
    { linkedEntityType: "commodity", linkedEntityId: id, perPage: 1 },
    { enabled: enabled && !!id }
  )
  const fileCount = filesCount.data?.total ?? 0

  // Tab selection is mirrored in the `?tab=` query string so deep
  // links from the warranties list / dashboard expiring panel land on
  // the right tab. `?tab=details` is the default — we strip the param
  // when switching back so the URL stays clean.
  const [searchParams, setSearchParams] = useSearchParams()
  const tab = parseTab(searchParams.get("tab"))
  function setTab(next: TabKey) {
    const params = new URLSearchParams(searchParams)
    if (next === "details") params.delete("tab")
    else params.set("tab", next)
    setSearchParams(params, { replace: true })
  }
  // /commodities/:id/edit deep-link: open the edit dialog immediately.
  // Closing the dialog navigates back to /commodities/:id (sans /edit)
  // so the URL stays meaningful.
  const editMatch = useMatch({ path: "/g/:groupSlug/commodities/:id/edit", end: true })
  const [editOpen, setEditOpen] = useState(false)
  // #1448 quick-attach state. `pendingDropFiles` holds files seeded by
  // the page-level drop catcher; the upload dialog reads them on open
  // and queues them in the select step. The button-triggered open
  // path leaves it empty so the user picks files inside the dialog.
  const [uploadOpen, setUploadOpen] = useState(false)
  const [pendingDropFiles, setPendingDropFiles] = useState<File[]>([])
  // #1611: forward-transition target gates the StatusTransitionDialog.
  // Null = dialog closed. Revert keeps the lighter useConfirm path
  // (no metadata to capture).
  const [transitionTarget, setTransitionTarget] = useState<CommodityStatusValue | null>(null)
  const dropZone = useFileDropZone({
    onFiles: (files) => {
      setPendingDropFiles(files)
      setUploadOpen(true)
    },
    // Don't catch new drags while the dialog is already open — its
    // own dropzone takes over so a second drop doesn't bounce through
    // the page wrapper.
    disabled: uploadOpen,
  })
  useEffect(() => {
    // Deep-link sync from URL → local edit-dialog state.
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one cascading render per nav is acceptable
    if (editMatch && !editOpen) setEditOpen(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentionally only re-run on URL match changes; editOpen is read once
  }, [editMatch?.pathname])
  function handleEditOpenChange(open: boolean) {
    setEditOpen(open)
    if (!open && editMatch && slug && id) {
      navigate(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`, {
        replace: true,
      })
    }
  }

  const commodity = detail.data?.commodity

  const areaName = useMemo(() => {
    const map = new Map<string, string>()
    for (const a of areas.data ?? []) {
      if (a.id) map.set(a.id, a.name ?? "")
    }
    return (areaId?: string) => (areaId ? (map.get(areaId) ?? "") : "")
  }, [areas.data])

  // areaLabel mirrors the design mock's `areaLabel(areaId)` —
  // "{Location.name} · {Area.name}" when both exist, fall back to
  // just the area name (mock fallback), then empty string. Drives
  // the Location row in DetailsTab so the user sees both
  // breadcrumb levels at a glance instead of a single area name.
  const areaLabel = useMemo(() => {
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

  // The detail page heading mirrors the commodity name; once it's
  // loaded we update the document title via RouteTitle so browser
  // tabs are useful in long sessions. Sheet mode is gated out — the
  // backdrop list page already owns the tab title, and stomping it
  // when the user pops the sheet would leak the item name onto the
  // list URL too (copilot review).
  useEffect(() => {
    if (isSheet) return
    if (!commodity?.id) return
    if (typeof document !== "undefined") {
      document.title = commodity.name
        ? `${commodity.name} — Inventario`
        : t("commodities:detail.documentTitle")
    }
  }, [isSheet, commodity?.id, commodity?.name, t])

  if (detail.isLoading) {
    return (
      <PageFrame
        isSheet={isSheet}
        className={errorWrapperClass}
        data-testid="commodity-detail-loading"
      >
        <Skeleton className="h-8 w-64" />
        <Skeleton className="mt-2 h-4 w-32" />
        <div className="mt-6 grid grid-cols-2 gap-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-12 rounded-md" />
          ))}
        </div>
      </PageFrame>
    )
  }
  if (detail.isError) {
    return (
      <PageFrame isSheet={isSheet} className={errorWrapperClass}>
        <Alert variant="destructive" data-testid="commodity-detail-error">
          <AlertTitle>{t("commodities:detail.errorTitle")}</AlertTitle>
          <AlertDescription>{t("commodities:detail.errorDescription")}</AlertDescription>
        </Alert>
        <Button variant="ghost" className="mt-4 gap-1" onClick={() => navigate(-1)}>
          <ArrowLeft className="size-4" aria-hidden="true" />
          {t("commodities:detail.backToList")}
        </Button>
      </PageFrame>
    )
  }
  if (!commodity) {
    return (
      <PageFrame isSheet={isSheet} className={errorWrapperClass}>
        <Card data-testid="commodity-detail-not-found">
          <CardHeader>
            <CardTitle>{t("commodities:detail.notFoundTitle")}</CardTitle>
            <CardDescription>{t("commodities:detail.notFoundDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={() => navigate(-1)}>{t("commodities:detail.backToList")}</Button>
          </CardContent>
        </Card>
      </PageFrame>
    )
  }

  const status = commodity.status as CommodityStatusValue | undefined
  const tone = status ? COMMODITY_STATUS_TONES[status] : ""
  const type = commodity.type as CommodityTypeValue | undefined
  // Per the BE, `original_price` is denominated in the purchase
  // currency (`original_price_currency`); `converted_original_price`
  // and `current_price` are denominated in the group currency.
  // Pass both down so DetailsTab can format each row correctly rather
  // than mixing the symbols.
  const groupCurrency = currentGroup?.group_currency ?? "USD"
  const purchaseCurrency = commodity.original_price_currency ?? groupCurrency
  const currency = groupCurrency // for the (deprecated) edit-dialog default
  const listHref = slug ? `/g/${encodeURIComponent(slug)}/commodities` : "#"
  const printHref =
    slug && commodity.id
      ? `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodity.id)}/print`
      : "#"

  async function handleSave(values: Parameters<typeof update.mutateAsync>[0]) {
    await update.mutateAsync(values)
    toast.success(t("commodities:toast.updated"))
    setEditOpen(false)
  }

  async function handleDelete() {
    if (!commodity?.id) return
    const ok = await confirm({
      title: t("commodities:delete.title", { name: commodity.name ?? "" }),
      description: t("commodities:delete.description"),
      confirmLabel: t("common:actions.delete"),
      destructive: true,
    })
    if (!ok) return
    try {
      await remove.mutateAsync(commodity.id)
      toast.success(t("commodities:toast.deleted"))
      navigate(listHref)
    } catch {
      toast.error(t("commodities:toast.deleteError"))
    }
  }

  // CHANGE STATUS quick action: drives the row's status transitions.
  // Forward transitions (in_use → sold/lost/disposed/written_off) open
  // the StatusTransitionDialog (#1611) so the user can record the
  // status_date / status_note / sale_price triple the BE persists.
  // Revert (terminal → in_use) keeps the lighter useConfirm path and
  // explicitly clears the three metadata fields so the BE's model
  // validation invariant (status_date NULL when status==in_use,
  // sale_price NULL when status!=sold) holds on the next write.
  async function handleStatusTransition(next: CommodityStatusValue) {
    if (!commodity) return
    if (next !== "in_use") {
      setTransitionTarget(next)
      return
    }
    const ok = await confirm({
      title: t("commodities:detail.terminalStatus.revertConfirmTitle"),
      description: t("commodities:detail.terminalStatus.revertConfirmDescription"),
      confirmLabel: t("common:actions.confirm"),
      destructive: false,
    })
    if (!ok) return
    try {
      await update.mutateAsync({
        ...commodity,
        status: next,
        status_date: undefined,
        status_note: "",
        sale_price: undefined,
      })
      toast.success(t("commodities:toast.statusUpdated"))
    } catch {
      toast.error(t("commodities:toast.statusUpdateError"))
    }
  }

  // handleTransitionConfirm bridges the StatusTransitionDialog's
  // captured payload to the PATCH wiring. Threads the three new
  // metadata columns alongside the status flip. `sale_price` is
  // included by the dialog only for `sold`; for other targets it
  // stays absent so the BE's per-row invariant holds.
  async function handleTransitionConfirm(payload: StatusTransitionPayload) {
    if (!commodity || !transitionTarget) return
    try {
      await update.mutateAsync({
        ...commodity,
        status: transitionTarget,
        status_date: payload.status_date,
        status_note: payload.status_note,
        sale_price: payload.sale_price,
      })
      toast.success(t("commodities:toast.statusUpdated"))
      setTransitionTarget(null)
    } catch {
      toast.error(t("commodities:toast.statusUpdateError"))
    }
  }

  return (
    <>
      {/* Document title is owned by the full-page variant only — when
          the sheet is open over the list, the list page already set
          the title and the user expects it to stay. */}
      {isSheet ? null : <RouteTitle title={t("commodities:detail.documentTitle")} />}
      <PageFrame
        isSheet={isSheet}
        className={mainWrapperClass}
        data-testid={isSheet ? "sheet-commodity-detail" : "page-commodity-detail"}
        {...dropZone.bindProps}
      >
        {dropZone.isDragging ? (
          <DropOverlay
            label={t("files:entityPanel.dropOverlay_commodity")}
            hint={t("files:entityPanel.dropHint")}
          />
        ) : null}
        {/* The Sheet variant renders its own X close button (top-right
            of the panel), so the explicit "Back to list" Link is
            page-only — clicking the X dismisses the overlay and lands
            the user back on the list URL the sheet opened over. */}
        {isSheet ? null : (
          <Link
            to={listHref}
            className="text-sm text-muted-foreground hover:underline inline-flex items-center gap-1"
          >
            <ArrowLeft className="size-3.5" aria-hidden="true" />
            {t("commodities:detail.backToList")}
          </Link>
        )}

        {/* Header (mock parity: `<SheetHeader pt-6 pb-4 px-0>`). The
            title block + status pills row both live inside the
            header in the mock. We do the same shape: avatar +
            title + short_name + type subtitle, then status pills
            below. Page mode keeps `gap-6` flow above so the
            existing visual rhythm is unchanged. */}
        <header className={isSheet ? "pt-6 pb-4" : ""}>
          <div className="flex items-start gap-3">
            <CommodityThumb
              cover={commodity.cover}
              type={type}
              name={commodity.name}
              size={48}
              testId="commodity-detail-thumb"
            />
            <div className="min-w-0 flex-1 pr-6">
              <h1
                className={cn(
                  "font-semibold leading-tight tracking-tight",
                  isSheet ? "text-lg" : "text-2xl"
                )}
                data-testid="commodity-detail-name"
              >
                {commodity.name}
              </h1>
              {commodity.short_name && commodity.short_name !== commodity.name ? (
                <p
                  className="text-xs font-mono text-muted-foreground mt-0.5"
                  data-testid="commodity-detail-short-name"
                >
                  {commodity.short_name}
                </p>
              ) : null}
              {type ? (
                <p className="text-sm text-muted-foreground mt-0.5">
                  {t(`commodities:type.${type}`)}
                </p>
              ) : null}
            </div>
          </div>

          {/* Status pills row — commodity status + warranty + days
              remaining. Mock places it inside SheetHeader directly
              under the title block (`pt-1`). */}
          <div
            className="flex flex-wrap items-center gap-2 pt-1"
            data-testid="commodity-detail-pills"
          >
            {status ? (
              <span
                className={cn(
                  "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium border",
                  tone || "border-border text-foreground"
                )}
                data-testid="commodity-detail-status-pill"
              >
                <CircleDot className="size-3" aria-hidden="true" />
                {t(`commodities:status.${status}`)}
              </span>
            ) : null}
            {commodity.draft ? (
              <Badge variant="outline" className="border-dashed text-xs">
                {t("commodities:list.draftBadge")}
              </Badge>
            ) : null}
            {/* #1554: bundles don't carry a warranty — hide the pill
                + days-remaining row entirely so the header doesn't
                advertise an attribute the row can't have. The
                Warranty tab body shows the explanatory empty state. */}
            {(commodity.count ?? 0) > 1 ? null : (
              <>
                <WarrantyBadge
                  source={{ warranty_expires_at: commodity.warranty_expires_at }}
                  data-testid="commodity-detail-warranty-pill"
                />
                {(() => {
                  const days = warrantyDaysRemaining(commodity.warranty_expires_at)
                  return days !== null && days > 0 ? (
                    <span className="text-xs text-muted-foreground">
                      {t("commodities:detail.warranty.daysRemaining", { count: days })}
                    </span>
                  ) : null
                })()}
              </>
            )}
          </div>
        </header>

        {/* Action row — `flex gap-2 pb-4 flex-wrap` per the mock.
            Edit (flex-1) + Insurance Report + icon-only destructive
            Delete. Print is page-only (the mock has no Print
            button); direct landings still have it on the page
            chrome where it doesn't crowd the action row. */}
        <div className={cn("flex flex-wrap items-center gap-2", isSheet && "pb-4")}>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setEditOpen(true)}
            data-testid="commodity-detail-edit"
            className="flex-1 gap-1.5"
            disabled={migrationLock.locked}
            title={migrationLock.locked ? t("errors:lockedDuringMigration") : undefined}
            aria-disabled={migrationLock.locked || undefined}
          >
            <Pencil className="size-3.5" aria-hidden="true" />
            {t("commodities:detail.edit")}
          </Button>
          {slug && commodity.id ? (
            <Button
              asChild
              type="button"
              variant="outline"
              size="sm"
              className="gap-1.5"
              data-testid="commodity-detail-insurance"
            >
              <Link
                to={`/g/${encodeURIComponent(slug)}/reports/insurance?mode=item&item=${encodeURIComponent(commodity.id)}`}
              >
                <BarChart3 className="size-3.5" aria-hidden="true" />
                {t("commodities:detail.insuranceReport")}
              </Link>
            </Button>
          ) : null}
          {isSheet ? null : (
            <Button
              asChild
              type="button"
              variant="outline"
              size="sm"
              title={t("commodities:detail.print")}
              aria-label={t("commodities:detail.print")}
            >
              <Link to={printHref} data-testid="commodity-detail-print">
                <Printer className="size-3.5" aria-hidden="true" />
              </Link>
            </Button>
          )}
          {/* Delete uses `size="sm"` (h-8 px-3 — slightly wider
              than `size="icon"`'s 8x8 square) to match the mock's
              same-row Edit / Insurance Report buttons. The icon
              child stays the same; the destructive tone comes from
              the className. */}
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="text-destructive hover:bg-destructive/10"
            onClick={handleDelete}
            data-testid="commodity-detail-delete"
            title={
              migrationLock.locked
                ? t("errors:lockedDuringMigration")
                : t("commodities:detail.delete")
            }
            aria-label={t("commodities:detail.delete")}
            disabled={migrationLock.locked}
            aria-disabled={migrationLock.locked || undefined}
          >
            <Trash2 className="size-3.5" aria-hidden="true" />
          </Button>
        </div>

        {/* CHANGE STATUS bar — `mb-4 rounded-xl border border-border
            bg-muted/30 p-3 space-y-2` lifted from the mock. Only
            meaningful while the item is still `in_use`. */}
        {status === "in_use" ? (
          <div
            className={cn(
              "rounded-xl border border-border bg-muted/30 p-3 space-y-2",
              isSheet && "mb-4"
            )}
            data-testid="commodity-detail-change-status"
          >
            <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
              {t("commodities:detail.statusTransition.heading")}
            </p>
            <div className="flex flex-wrap gap-1.5">
              {TERMINAL_STATUSES.map((s) => (
                <Button
                  key={s}
                  type="button"
                  variant="outline"
                  size="sm"
                  className={cn("gap-1.5 text-xs h-7", STATUS_TRANSITION_TEXT_TONES[s])}
                  onClick={() => handleStatusTransition(s)}
                  data-testid={`commodity-detail-transition-${s}`}
                  disabled={update.isPending || migrationLock.locked}
                  title={migrationLock.locked ? t("errors:lockedDuringMigration") : undefined}
                  aria-disabled={migrationLock.locked || undefined}
                >
                  {t(`commodities:status.${s}`)}
                </Button>
              ))}
            </div>
          </div>
        ) : null}

        {/* Terminal-status info card (#1530 item 1 + #1611) — surfaces
            the current status, the metadata captured during the
            transition (date / note / sale_price), and the explicit
            "Revert to In Use" CTA the mock requires (lines 736-762).
            Each metadata row gates on its column being set, so rows
            that pre-date #1611 (NULL metadata) collapse to just the
            label + Revert pair — the prior shipped behaviour. */}
        {status && status !== "in_use" ? (
          <div
            className={cn(
              "rounded-xl border p-3 space-y-1",
              tone || "border-border bg-muted text-foreground",
              isSheet && "mb-4"
            )}
            role="status"
            data-testid="commodity-detail-terminal-status"
          >
            <div className="flex items-center gap-1.5">
              <TriangleAlert className="size-3.5 shrink-0" aria-hidden="true" />
              <p
                className="text-xs font-semibold"
                data-testid="commodity-detail-terminal-status-label"
              >
                {t(`commodities:status.${status}`)}
              </p>
            </div>
            {commodity.status_date ? (
              <p
                className="text-xs text-muted-foreground"
                data-testid="commodity-detail-terminal-status-date"
              >
                {t("commodities:detail.terminalStatus.statusDate", {
                  date: formatDate(commodity.status_date),
                })}
              </p>
            ) : null}
            {commodity.status_note ? (
              <p
                className="text-xs text-muted-foreground"
                data-testid="commodity-detail-terminal-status-note"
              >
                {commodity.status_note}
              </p>
            ) : null}
            {commodity.sale_price != null ? (
              <p
                className="text-xs text-muted-foreground"
                data-testid="commodity-detail-terminal-status-sale-price"
              >
                {t("commodities:detail.terminalStatus.salePrice", {
                  value: formatCurrency(Number(commodity.sale_price), purchaseCurrency),
                })}
              </p>
            ) : null}
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="-mb-1 mt-1 h-6 px-1 text-xs text-foreground"
              onClick={() => handleStatusTransition("in_use")}
              disabled={update.isPending || migrationLock.locked}
              title={migrationLock.locked ? t("errors:lockedDuringMigration") : undefined}
              aria-disabled={migrationLock.locked || undefined}
              data-testid="commodity-detail-revert-status"
            >
              {t("commodities:detail.terminalStatus.revert")}
            </Button>
          </div>
        ) : null}

        <Tabs value={tab} onChange={setTab} fileCount={fileCount} variant={variant} />

        {/* Tab content gets `mt-4` in sheet mode to mirror the
            mock's `<TabsContent className="mt-4 space-y-0">`. Page
            mode keeps its parent `gap-6` flow. */}
        <div className={isSheet ? "mt-4" : ""}>
          {tab === "details" ? (
            <>
              <DetailsTab
                commodity={commodity}
                groupCurrency={groupCurrency}
                purchaseCurrency={purchaseCurrency}
                areaName={areaName(commodity.area_id)}
                areaLabel={areaLabel(commodity.area_id)}
                variant={variant}
              />
              {commodity.id ? <CommodityHistoryTimeline commodityId={commodity.id} /> : null}
            </>
          ) : tab === "warranty" ? (
            <WarrantyTab commodity={commodity} onSwitchToFiles={() => setTab("files")} />
          ) : tab === "lend" ? (
            <LendTab
              commodityId={commodity?.id ?? id}
              commodityCount={commodity?.count ?? undefined}
            />
          ) : tab === "service" ? (
            <ServiceTab
              commodityId={commodity?.id ?? id}
              commodityCount={commodity?.count ?? undefined}
            />
          ) : tab === "supplies" ? (
            <SuppliesTab commodityId={commodity?.id ?? id} />
          ) : tab === "maintenance" ? (
            <MaintenanceTab
              commodityId={commodity?.id ?? id}
              commodityCount={commodity?.count ?? undefined}
            />
          ) : (
            <FilesTab
              commodityId={commodity?.id ?? id}
              coverState={{
                // Explicit override (issue #1451 option B) takes precedence;
                // first_photo is the auto-pick when no override is set.
                current: commodity?.cover_file_id ?? undefined,
                auto:
                  commodity?.cover && commodity.cover.source === "first_photo"
                    ? commodity.cover.fileId
                    : undefined,
              }}
              onSetCover={(fileId) => {
                setCover.mutate(fileId, {
                  onSuccess: () => {
                    toast.success(
                      fileId
                        ? t("commodities:cover.setSuccess", {
                            defaultValue: "Cover photo updated.",
                          })
                        : t("commodities:cover.clearSuccess", {
                            defaultValue: "Cover photo cleared.",
                          })
                    )
                  },
                  onError: () =>
                    toast.error(
                      t("commodities:cover.error", {
                        defaultValue: "Couldn't update the cover photo.",
                      })
                    ),
                })
              }}
              coverBusy={setCover.isPending}
              onAttachClick={() => {
                setPendingDropFiles([])
                setUploadOpen(true)
              }}
            />
          )}
        </div>
      </PageFrame>

      <CommodityFormDialog
        open={editOpen}
        onOpenChange={handleEditOpenChange}
        mode="edit"
        initialValues={commodity}
        areas={areas.data ?? []}
        locations={locations.data ?? []}
        defaultCurrency={currency}
        onSubmit={handleSave}
        isPending={update.isPending}
      />

      <StatusTransitionDialog
        open={transitionTarget !== null}
        onOpenChange={(o) => {
          if (!o) setTransitionTarget(null)
        }}
        targetStatus={transitionTarget}
        purchaseCurrency={purchaseCurrency}
        onSubmit={handleTransitionConfirm}
        isPending={update.isPending}
      />

      <UploadFilesDialog
        open={uploadOpen}
        onOpenChange={(open) => {
          setUploadOpen(open)
          if (!open) setPendingDropFiles([])
        }}
        linkedEntity={{
          type: "commodity",
          id: commodity?.id ?? id,
          name: commodity?.name,
        }}
        initialFiles={pendingDropFiles}
        // Issue #1451 option C — promote the first uploaded photo to
        // the cover when the commodity has no explicit cover yet. The
        // checkbox in the metadata step defaults to ON for that case.
        // "Has a cover" means EITHER an explicit `cover_file_id` set on
        // the row (option B) OR an auto-picked first-photo cover (option
        // A). Only `cover_file_id` would silently re-promote new uploads
        // on commodities that already have a perfectly fine first-photo
        // cover (Copilot review on PR #1504). `commodity.cover` is the
        // resolved cover from `meta.cover`, set whenever either path
        // produced a usable image.
        commodityHasCover={!!commodity?.cover}
        onSetCover={(fileId) => {
          setCover.mutate(fileId, {
            onError: () =>
              toast.error(
                t("commodities:cover.error", {
                  defaultValue: "Couldn't update the cover photo.",
                })
              ),
          })
        }}
      />
    </>
  )
}

interface TabsProps {
  value: TabKey
  onChange: (v: TabKey) => void
  // File count surfaced as a badge on the Files tab — mirrors the
  // mock's `<TabsTrigger value="files">Files {n > 0 && <Badge>}`.
  // Zero is hidden so the tab strip doesn't carry visual debt for
  // every empty commodity.
  fileCount?: number
  // `"sheet"` flips the tab strip to `w-full` so it spans the
  // narrower panel; `"page"` keeps the existing left-clumped
  // layout that fits the wider canvas.
  variant?: "page" | "sheet"
}

function Tabs({ value, onChange, fileCount = 0, variant = "page" }: TabsProps) {
  const { t } = useTranslation()
  const isSheet = variant === "sheet"
  // Files tab gets a leading icon — mock parity (`<Paperclip
  // size-3.5 />` inside `<TabsTrigger>`). Other tabs stay
  // text-only; the strip would feel cluttered if every label
  // sprouted an icon.
  const tabs: {
    key: TabKey
    label: string
    count?: number
    icon?: typeof Paperclip
  }[] = [
    { key: "details", label: t("commodities:detail.tabs.details") },
    { key: "warranty", label: t("commodities:detail.tabs.warranty") },
    {
      key: "files",
      label: t("commodities:detail.tabs.files"),
      count: fileCount,
      icon: Paperclip,
    },
    { key: "lend", label: t("commodities:detail.tabs.lend") },
    { key: "service", label: t("commodities:detail.tabs.service") },
    {
      key: "supplies",
      label: t("commodities:detail.tabs.supplies", { defaultValue: "Supplies" }),
    },
    {
      key: "maintenance",
      label: t("commodities:detail.tabs.maintenance", { defaultValue: "Maintenance" }),
    },
  ]
  return (
    <div
      role="tablist"
      className={cn(
        "flex",
        // Sheet variant: no full-width bottom border on the
        // container — the mock's `<TabsList variant="line">` only
        // shows the underline under the *active* trigger. Page
        // mode keeps the strip-wide divider for the wider chrome.
        // `flex-1` triggers below spread the labels evenly across
        // the panel instead of clumping left.
        isSheet ? "w-full gap-1" : "gap-1 border-b border-border"
      )}
      data-testid="commodity-detail-tabs"
    >
      {tabs.map((tb) => {
        const Icon = tb.icon
        return (
          <button
            key={tb.key}
            role="tab"
            type="button"
            aria-selected={value === tb.key}
            onClick={() => onChange(tb.key)}
            className={cn(
              "inline-flex items-center gap-1.5 py-2 text-sm border-b-2 -mb-px transition-colors",
              // Sheet triggers grow to equal width + center their
              // labels so the tab strip mirrors the mock's
              // `flex-1 justify-center` `<TabsTrigger>` shape.
              isSheet ? "flex-1 justify-center px-2" : "px-3",
              value === tb.key
                ? "border-primary text-foreground font-semibold"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
            data-testid={`commodity-detail-tab-${tb.key}`}
          >
            {Icon ? <Icon className="size-3.5" aria-hidden="true" /> : null}
            {tb.label}
            {tb.count && tb.count > 0 ? (
              <span
                className="flex size-4 items-center justify-center rounded-full bg-muted text-[10px] font-medium text-foreground"
                data-testid={`commodity-detail-tab-${tb.key}-count`}
              >
                {tb.count}
              </span>
            ) : null}
          </button>
        )
      })}
    </div>
  )
}

interface DetailsTabProps {
  commodity: import("@/features/commodities/api").Commodity
  // Currency the BE stores `original_price` in (taken from
  // `original_price_currency`).
  purchaseCurrency: string
  // Currency the BE stores `converted_original_price` and
  // `current_price` in — always the active group currency.
  groupCurrency: string
  areaName: string
  // "{Location.name} · {Area.name}" breadcrumb for the Location
  // row (mock parity). Falls back to the area name alone when the
  // location can't be resolved; empty string when the area itself
  // is missing. Sheet mode prefers this richer label since the
  // panel is narrow but the user still wants the full context.
  areaLabel: string
  // Variant matches the parent's: `"sheet"` swaps the 2-col grid
  // for a vertical icon|label-value list (matching the mock and
  // staying readable inside the narrower Sheet panel); `"page"`
  // keeps the existing 2-col layout that fills the wider canvas.
  variant?: "page" | "sheet"
}

function DetailsTab({
  commodity,
  purchaseCurrency,
  groupCurrency,
  areaName,
  areaLabel,
  variant = "page",
}: DetailsTabProps) {
  const { t } = useTranslation()
  const noValue = t("commodities:detail.noValue")
  // Sheet mode shows the breadcrumb-style "Location · Area"
  // value with a "Location" label (mock parity); page mode keeps
  // the bare "Area" / area-name pair to match the existing column
  // layout where adjacent rows already carry their own context.
  const isSheetVariant = variant === "sheet"
  const rows: { icon: typeof MapPin; label: string; value: React.ReactNode; testId?: string }[] = [
    {
      icon: MapPin,
      label: isSheetVariant
        ? t("commodities:detail.fields.location")
        : t("commodities:detail.fields.area"),
      value: (isSheetVariant ? areaLabel : areaName) || noValue,
    },
    {
      icon: Package,
      label: t("commodities:detail.fields.count"),
      value: commodity.count ?? noValue,
    },
    {
      icon: Calendar,
      label: t("commodities:detail.fields.purchaseDate"),
      value: commodity.purchase_date
        ? formatDate(commodity.purchase_date as string, { style: "short" })
        : noValue,
    },
    {
      icon: DollarSign,
      label: t("commodities:detail.fields.originalPrice"),
      // After a currency migration the BE freezes the per-row
      // "as purchased" amount in `acquisition_price` /
      // `acquisition_currency` (issue #202 §2 Case A). Surface it
      // as a subdued line under the live OriginalPrice so users can
      // still see the original purchase amount in the original
      // currency. Hidden when the BE didn't freeze a value (fresh
      // commodity → live OriginalPrice already IS the purchase
      // value, so no second line is needed).
      value: (
        <span className="flex flex-col gap-0.5">
          <span>
            {commodity.original_price !== undefined
              ? formatCurrency(Number(commodity.original_price), purchaseCurrency)
              : noValue}
          </span>
          {commodity.acquisition_price != null && commodity.acquisition_currency ? (
            <span
              className="text-xs text-muted-foreground"
              data-testid="commodity-detail-acquisition"
            >
              {t("commodities:detail.acquisitionPrice", {
                // formatCurrency injects the locale-correct symbol /
                // code on its own; the i18n string interpolates the
                // already-formatted value. Translations that want a
                // bare numeric + uppercase code (e.g. "1,234.56 USD")
                // can switch to a number-only formatter and append
                // {{currency}} themselves once we surface that knob.
                price: formatCurrency(
                  Number(commodity.acquisition_price),
                  commodity.acquisition_currency
                ),
              })}
            </span>
          ) : null}
        </span>
      ),
    },
    {
      icon: DollarSign,
      label: t("commodities:detail.fields.convertedOriginalPrice"),
      value:
        commodity.converted_original_price !== undefined
          ? formatCurrency(Number(commodity.converted_original_price), groupCurrency)
          : noValue,
    },
    {
      icon: DollarSign,
      label: t("commodities:detail.fields.currentPrice"),
      value:
        commodity.current_price !== undefined
          ? formatCurrency(Number(commodity.current_price), groupCurrency)
          : noValue,
    },
    {
      icon: Hash,
      label: t("commodities:detail.fields.serialNumber"),
      value: commodity.serial_number || noValue,
    },
  ]
  // Sheet variant renders the rows directly on the SheetContent
  // surface (NO outer Card / no nested rounded border) — the mock's
  // `<TabsContent className="mt-4 space-y-0">` pattern. Each row +
  // Separator pair is the entire visual rhythm; the panel chrome
  // is everything around it. Page variant keeps the wider 2-col
  // grid wrapped in the existing Card.
  const isSheet = variant === "sheet"
  const containerClass = isSheet ? "flex flex-col" : "grid grid-cols-1 sm:grid-cols-2 gap-4 py-6"
  const fullWidthClass = isSheet ? "" : "sm:col-span-2"
  const separatorClass = isSheet ? "" : "sm:col-span-2"
  const Outer: React.ElementType = isSheet ? "div" : Card
  const Inner: React.ElementType = isSheet ? "div" : CardContent
  return (
    <Outer data-testid="commodity-detail-details">
      <Inner className={containerClass}>
        {rows.map((r, i) => (
          <DetailRow
            key={r.label}
            icon={r.icon}
            label={r.label}
            value={r.value}
            variant={variant}
            withDivider={isSheet && i > 0}
          />
        ))}
        {/* Auxiliary fields (tags / urls / notes / extra serials /
            part numbers). Sheet mode threads them through the same
            DetailRow shape as the main rows so the labels stay
            "Tags" / "Notes" (not "TAGS" / "NOTES" via DetailLabel's
            uppercase tracking) and the icon-on-left layout is
            uniform. Page mode keeps the existing 2-col grid block
            with DetailLabel on top. */}
        {commodity.tags && commodity.tags.length > 0 ? (
          isSheet ? (
            <DetailRow
              icon={Tag}
              label={t("commodities:detail.fields.tags")}
              value={
                <div className="flex flex-wrap gap-1.5 mt-0.5">
                  {commodity.tags.map((tag) => (
                    <Badge key={tag} variant="secondary">
                      {tag}
                    </Badge>
                  ))}
                </div>
              }
              variant={variant}
              withDivider
            />
          ) : (
            <div className={cn("flex flex-col gap-1.5", fullWidthClass)}>
              <DetailLabel icon={Tag} label={t("commodities:detail.fields.tags")} />
              <div className="flex flex-wrap gap-1.5">
                {commodity.tags.map((tag) => (
                  <Badge key={tag} variant="secondary">
                    {tag}
                  </Badge>
                ))}
              </div>
            </div>
          )
        ) : null}
        {Array.isArray(commodity.urls) && commodity.urls.length > 0 ? (
          isSheet ? (
            <DetailRow
              icon={ExternalLink}
              label={t("commodities:detail.fields.urls")}
              value={
                <ul className="text-sm">
                  {(commodity.urls as unknown as string[]).map((u, i) => (
                    <li key={i}>
                      <a
                        href={u}
                        target="_blank"
                        rel="noreferrer noopener"
                        className="text-primary hover:underline"
                      >
                        {u}
                      </a>
                    </li>
                  ))}
                </ul>
              }
              variant={variant}
              withDivider
            />
          ) : (
            <div className={cn("flex flex-col gap-1.5", fullWidthClass)}>
              <DetailLabel icon={ExternalLink} label={t("commodities:detail.fields.urls")} />
              <ul className="text-sm">
                {(commodity.urls as unknown as string[]).map((u, i) => (
                  <li key={i}>
                    <a
                      href={u}
                      target="_blank"
                      rel="noreferrer noopener"
                      className="text-primary hover:underline"
                    >
                      {u}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          )
        ) : null}
        {commodity.comments ? (
          isSheet ? (
            <DetailRow
              icon={FileText}
              label={t("commodities:detail.fields.comments")}
              value={
                <p className="text-sm font-normal whitespace-pre-wrap">{commodity.comments}</p>
              }
              variant={variant}
              withDivider
            />
          ) : (
            <div className={cn("flex flex-col gap-1.5", fullWidthClass)}>
              <DetailLabel icon={Hash} label={t("commodities:detail.fields.comments")} />
              <p className="text-sm whitespace-pre-wrap">{commodity.comments}</p>
            </div>
          )
        ) : null}
        {commodity.extra_serial_numbers && commodity.extra_serial_numbers.length > 0 ? (
          isSheet ? (
            <DetailRow
              icon={Hash}
              label={t("commodities:detail.fields.extraSerialNumbers")}
              value={
                <div className="flex flex-wrap gap-1.5 mt-0.5">
                  {commodity.extra_serial_numbers.map((s) => (
                    <Badge key={s} variant="outline">
                      {s}
                    </Badge>
                  ))}
                </div>
              }
              variant={variant}
              withDivider
            />
          ) : (
            <div className={cn("flex flex-col gap-1.5", fullWidthClass)}>
              <DetailLabel icon={Hash} label={t("commodities:detail.fields.extraSerialNumbers")} />
              <div className="flex flex-wrap gap-1.5">
                {commodity.extra_serial_numbers.map((s) => (
                  <Badge key={s} variant="outline">
                    {s}
                  </Badge>
                ))}
              </div>
            </div>
          )
        ) : null}
        {commodity.part_numbers && commodity.part_numbers.length > 0 ? (
          isSheet ? (
            <DetailRow
              icon={Package}
              label={t("commodities:detail.fields.partNumbers")}
              value={
                <div className="flex flex-wrap gap-1.5 mt-0.5">
                  {commodity.part_numbers.map((p) => (
                    <Badge key={p} variant="outline">
                      {p}
                    </Badge>
                  ))}
                </div>
              }
              variant={variant}
              withDivider
            />
          ) : (
            <div className={cn("flex flex-col gap-1.5", fullWidthClass)}>
              <DetailLabel icon={Hash} label={t("commodities:detail.fields.partNumbers")} />
              <div className="flex flex-wrap gap-1.5">
                {commodity.part_numbers.map((p) => (
                  <Badge key={p} variant="outline">
                    {p}
                  </Badge>
                ))}
              </div>
            </div>
          )
        ) : null}
        {isSheet ? null : <Separator className={separatorClass} />}
        <DetailRow
          icon={Calendar}
          label={t("commodities:detail.fields.registeredDate")}
          value={
            commodity.registered_date
              ? formatDate(commodity.registered_date as string, { style: "short" })
              : noValue
          }
          variant={variant}
          withDivider={isSheet}
        />
        <DetailRow
          icon={Calendar}
          label={t("commodities:detail.fields.lastModifiedDate")}
          value={
            commodity.last_modified_date
              ? formatDate(commodity.last_modified_date as string, { style: "short" })
              : noValue
          }
          variant={variant}
          withDivider={isSheet}
        />
      </Inner>
    </Outer>
  )
}

interface DetailRowProps {
  icon: typeof MapPin
  label: string
  value: React.ReactNode
  // `"sheet"` switches to the mock's `[icon] [label / value]`
  // horizontal arrangement separated by Separator; `"page"` keeps
  // the existing label-stacked-above-value layout.
  variant?: "page" | "sheet"
  // Sheet rows draw a top Separator unless they're first in the
  // list; the parent passes `false` for the leading row.
  withDivider?: boolean
}

function DetailRow({ icon: Icon, label, value, variant = "page", withDivider }: DetailRowProps) {
  if (variant === "sheet") {
    return (
      <>
        {withDivider ? <Separator /> : null}
        <div className="flex items-start gap-3 py-2.5">
          <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          <div className="flex-1 min-w-0">
            <p className="text-xs text-muted-foreground mb-0.5">{label}</p>
            <div className="text-sm font-medium">{value}</div>
          </div>
        </div>
      </>
    )
  }
  return (
    <div className="flex flex-col gap-1">
      <DetailLabel icon={Icon} label={label} />
      <p className="text-sm">{value}</p>
    </div>
  )
}

function DetailLabel({ icon: Icon, label }: { icon: typeof MapPin; label: string }) {
  return (
    <span className="flex items-center gap-1.5 text-xs uppercase tracking-wide text-muted-foreground">
      <Icon className="size-3.5" aria-hidden="true" />
      {label}
    </span>
  )
}

interface FilesTabProps {
  commodityId: string
  onAttachClick: () => void
  coverState?: { current?: string; auto?: string }
  onSetCover?: (fileId: string | null) => void
  coverBusy?: boolean
}

function FilesTab({
  commodityId,
  onAttachClick,
  coverState,
  onSetCover,
  coverBusy,
}: FilesTabProps) {
  // Commodity-detail Files tab — mock-parity surface introduced for
  // #1530 item 3: segmented chip-bar (All / Photos / Invoices /
  // Documents) + contextual upload zone + 3-col aspect-square photo
  // grid + non-photo list. Sits on top of the same unified `/files`
  // surface the global Files page uses (#1411 AC #4) but with a
  // commodity-specific layout. The cover-photo wiring (#1451 option
  // B) and the page-level upload dialog (#1448) are passed through.
  //
  // `EntityFilesPanel` stays as-is for `LocationDetailPage`; only
  // the commodity surface adopts the chip-bar treatment because the
  // mock contract is commodity-specific.
  return (
    <CommodityFilesTab
      commodityId={commodityId}
      onAttachClick={onAttachClick}
      coverState={coverState}
      onSetCover={onSetCover}
      coverBusy={coverBusy}
    />
  )
}

interface WarrantyTabProps {
  commodity?: Commodity
  // Optional. When provided, renders an "Upload Receipt" CTA that
  // jumps to the Files tab so the user can drop the warranty PDF
  // there instead of inventing a per-commodity upload widget.
  onSwitchToFiles?: () => void
}

// WarrantyTab renders the first-class warranty surface (#1367):
// status-coloured card + expiry-with-days-remaining line + notes block
// + an "Upload Receipt" CTA pointing at the Files tab. Layout mirrors
// the design mock (`inventario-design/src/components/ItemDetail.tsx`,
// `<TabsContent value="warranty">`); the design mock CLAUDE.md
// requires that warranty status colours go through the canonical
// WarrantyBadge / `WARRANTY_STATUS_CONFIG` so all four surfaces
// (badge, list rows, dashboard panel, this tab) read the same tokens.
//
// The form inputs themselves live in the commodity edit dialog's
// Warranty step — this tab stays read-only.
function WarrantyTab({ commodity, onSwitchToFiles }: WarrantyTabProps) {
  const { t } = useTranslation()
  // #1554: bundle commodities don't carry a warranty — render the
  // "split into separate items" hint instead of the live pill / notes
  // block. Doing this here (rather than inside the existing layout)
  // avoids leaking the upload-receipt CTA + status pill into a row
  // that can never have either.
  const isBundle = (commodity?.count ?? 0) > 1
  if (isBundle) {
    return (
      <Card data-testid="commodity-detail-warranty">
        <CardHeader>
          <CardTitle>{t("commodities:detail.warranty.title")}</CardTitle>
          <CardDescription>{t("commodities:detail.warranty.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <p
            className="text-sm text-muted-foreground"
            data-testid="commodity-detail-warranty-bundle-empty-state"
          >
            {t("commodities:trackingRestrictions.warrantyDisabled")}
          </p>
        </CardContent>
      </Card>
    )
  }
  const status: CommodityWarrantyStatus = commodity
    ? warrantyStatus({ warranty_expires_at: commodity.warranty_expires_at })
    : "none"
  const daysRemaining = warrantyDaysRemaining(commodity?.warranty_expires_at)
  const visual = WARRANTY_STATUS_CONFIG[status]
  const StatusIcon = visual.icon
  return (
    <Card data-testid="commodity-detail-warranty">
      <CardHeader>
        <CardTitle>{t("commodities:detail.warranty.title")}</CardTitle>
        <CardDescription>{t("commodities:detail.warranty.description")}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className={cn("flex flex-col gap-2 rounded-lg border p-4", visual.bg, visual.border)}>
          <div className="flex items-center gap-2">
            <StatusIcon className={cn("size-4", visual.text)} aria-hidden="true" />
            <WarrantyBadge
              status={status}
              showIcon={false}
              data-testid="commodity-detail-warranty-status"
            />
          </div>
          {commodity?.warranty_expires_at && daysRemaining !== null ? (
            <p className={cn("text-sm", visual.text)}>
              {daysRemaining > 0
                ? t("commodities:detail.warranty.expiresOnIn", {
                    date: formatDate(commodity.warranty_expires_at),
                    count: daysRemaining,
                  })
                : daysRemaining === 0
                  ? t("commodities:detail.warranty.expiresToday")
                  : t("commodities:detail.warranty.expiredOnAgo", {
                      date: formatDate(commodity.warranty_expires_at),
                      count: -daysRemaining,
                    })}
            </p>
          ) : status === "none" ? (
            <p className="text-sm text-muted-foreground">
              {t("commodities:detail.warranty.noneInline")}
            </p>
          ) : null}
        </div>
        {commodity?.warranty_notes ? (
          <div
            className="rounded-lg border border-border bg-muted/30 p-3"
            data-testid="commodity-detail-warranty-notes"
          >
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              {t("commodities:detail.warranty.notesLabel")}
            </p>
            <p className="mt-1 text-sm text-foreground whitespace-pre-wrap">
              {commodity.warranty_notes}
            </p>
          </div>
        ) : null}
        {status === "none" ? (
          <p className="text-sm text-muted-foreground">
            {t("commodities:detail.warranty.emptyState")}
          </p>
        ) : null}
        {onSwitchToFiles ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="w-fit gap-1.5"
            onClick={onSwitchToFiles}
            data-testid="commodity-detail-warranty-upload-receipt"
          >
            <FileText className="size-3.5" aria-hidden="true" />
            {t("commodities:detail.warranty.uploadReceipt")}
          </Button>
        ) : null}
      </CardContent>
    </Card>
  )
}

// warrantyDaysRemaining returns days between today and the expiry
// date. Negative values mean the warranty already expired. Returns
// null when the date is missing or unparseable.
function warrantyDaysRemaining(date: string | undefined): number | null {
  if (!date) return null
  const t = Date.parse(`${date}T00:00:00Z`)
  if (Number.isNaN(t)) return null
  const today = new Date()
  const todayUTC = Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), today.getUTCDate())
  return Math.round((t - todayUTC) / (1000 * 60 * 60 * 24))
}

// CommodityDetailPage is the canonical full-page entry mounted at
// `/g/:slug/commodities/:id` (and `/edit`). Direct landings, refresh,
// "open in new tab", and shared links all hit this page — its URL is
// stable and the layout is the standard centered-column view.
//
// The actual rendering lives in `<CommodityDetailContent>` so the
// sheet variant can reuse the same hooks, dialogs, drag-drop, and
// tab logic without duplication.
export function CommodityDetailPage() {
  const { id = "" } = useParams<{ id: string }>()
  return <CommodityDetailContent id={id} variant="page" />
}

// CommodityDetailSheet renders the same content as the full page,
// but inside a right-side `<Sheet>` overlay. Used by the router when
// the navigation that landed us here carried `state.background` (i.e.
// the user drilled in from the items list). The list page stays
// mounted underneath so closing the sheet just dismisses the overlay
// without a route transition.
//
// Closing the sheet (X / Esc / outside click) navigates to the
// `state.background` URL — that drops the `:id` from the path and
// returns the user to the list they came from. If we can't resolve a
// background (e.g. the user reloaded with a stale state), we fall
// back to going back in history; if even that fails, the route's
// catch-all `<NotFoundPage>` would handle it after the page reloads.
export function CommodityDetailSheet() {
  const { t } = useTranslation()
  const { id = "" } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const location = useLocation()
  const background = (
    location.state as { background?: { pathname: string; search?: string; hash?: string } } | null
  )?.background

  function handleClose() {
    if (background) {
      navigate(`${background.pathname}${background.search ?? ""}${background.hash ?? ""}`, {
        replace: true,
      })
    } else {
      navigate(-1)
    }
  }

  return (
    <Sheet open onOpenChange={(open) => !open && handleClose()}>
      <SheetContent
        side="right"
        // Width + flex shape lifted verbatim from the design mock
        // (`denisvmedia/inventario-design/src/components/ItemDetail.tsx`).
        // `flex flex-col gap-0` overrides the SheetContent default
        // `gap-4` so children layout flush against the header — each
        // section owns its own padding (header `pt-6 pb-4`, action
        // row `pb-4`, status card `mb-4`, tab body `mt-4`). `p-0`
        // lets the inner wrapper supply `px-5 pb-5`. Below `sm` the
        // Sheet primitive falls back to `w-3/4` so the panel takes
        // the whole viewport on narrow screens.
        className="w-full sm:max-w-lg flex flex-col gap-0 overflow-y-auto p-0"
        data-testid="commodity-detail-sheet"
      >
        {/* Radix Dialog (which Sheet wraps) emits a console warning
            unless a DialogTitle/SheetTitle is mounted inside
            DialogContent. The visible commodity name lives inside
            CommodityDetailContent's own header (`<h1
            data-testid="commodity-detail-name">`) which Radix can't
            see, so we mirror it sr-only here. The fallback copy
            covers the brief window before `useCommodity` resolves. */}
        <SheetHeader className="sr-only">
          <SheetTitle>{t("commodities:detail.sheetTitleFallback")}</SheetTitle>
          <SheetDescription>{t("commodities:detail.sheetDescriptionFallback")}</SheetDescription>
        </SheetHeader>
        <CommodityDetailContent id={id} variant="sheet" />
      </SheetContent>
    </Sheet>
  )
}
