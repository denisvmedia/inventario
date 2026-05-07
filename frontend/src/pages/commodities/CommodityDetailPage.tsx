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
  Calendar,
  CircleDot,
  ExternalLink,
  FileBarChart2,
  FileText,
  Hash,
  MapPin,
  Package,
  Pencil,
  Printer,
  Tag,
  Trash2,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Sheet, SheetContent } from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { DropOverlay } from "@/components/files/DropOverlay"
import { EntityFilesPanel } from "@/components/files/EntityFilesPanel"
import { UploadFilesDialog } from "@/components/files/UploadFilesDialog"
import { useFileDropZone } from "@/components/files/useFileDropZone"
import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { LendTab } from "@/components/loans/LendTab"
import { ServiceTab } from "@/components/services/ServiceTab"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAreas } from "@/features/areas/hooks"
import { useFiles } from "@/features/files/hooks"
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
import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import { WARRANTY_STATUS_CONFIG } from "@/components/warranty/config"
import type { Commodity } from "@/features/commodities/api"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatCurrency, formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

type TabKey = "details" | "warranty" | "files" | "lend" | "service"

const TAB_KEYS = ["details", "warranty", "files", "lend", "service"] as const

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
  // Outer-frame classes: page mode uses the canonical 4xl-max-width
  // centered-column layout; sheet mode lets the SheetContent control
  // width and just provides padding + scroll inside the panel.
  const isSheet = variant === "sheet"
  const errorWrapperClass = isSheet ? "p-6" : "p-6 max-w-4xl mx-auto w-full"
  const mainWrapperClass = isSheet
    ? "relative flex flex-col gap-6 p-6 overflow-y-auto"
    : "relative flex flex-col gap-6 p-6 max-w-4xl mx-auto w-full"
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug
  const enabled = !!currentGroup
  const detail = useCommodity(id, { enabled })
  const areas = useAreas({ enabled })
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
    if (editMatch && !editOpen) setEditOpen(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only react to URL match changes; editOpen is intentionally read once
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

  // The detail page heading mirrors the commodity name; once it's
  // loaded we update the document title via RouteTitle so browser tabs
  // are useful in long sessions.
  useEffect(() => {
    if (!commodity?.id) return
    if (typeof document !== "undefined") {
      document.title = commodity.name
        ? `${commodity.name} — Inventario`
        : t("commodities:detail.documentTitle")
    }
  }, [commodity?.id, commodity?.name, t])

  if (detail.isLoading) {
    return (
      <div className={errorWrapperClass} data-testid="commodity-detail-loading">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="mt-2 h-4 w-32" />
        <div className="mt-6 grid grid-cols-2 gap-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-12 rounded-md" />
          ))}
        </div>
      </div>
    )
  }
  if (detail.isError) {
    return (
      <div className={errorWrapperClass}>
        <Alert variant="destructive" data-testid="commodity-detail-error">
          <AlertTitle>{t("commodities:detail.errorTitle")}</AlertTitle>
          <AlertDescription>{t("commodities:detail.errorDescription")}</AlertDescription>
        </Alert>
        <Button variant="ghost" className="mt-4 gap-1" onClick={() => navigate(-1)}>
          <ArrowLeft className="size-4" aria-hidden="true" />
          {t("commodities:detail.backToList")}
        </Button>
      </div>
    )
  }
  if (!commodity) {
    return (
      <div className={errorWrapperClass}>
        <Card data-testid="commodity-detail-not-found">
          <CardHeader>
            <CardTitle>{t("commodities:detail.notFoundTitle")}</CardTitle>
            <CardDescription>{t("commodities:detail.notFoundDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={() => navigate(-1)}>{t("commodities:detail.backToList")}</Button>
          </CardContent>
        </Card>
      </div>
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

  // CHANGE STATUS quick action: confirm + PATCH the commodity's
  // `status`. The mock's StatusTransitionDialog also captures a
  // status_date / status_note / sale_price triple, but our BE schema
  // doesn't carry those fields — we just transition the status and
  // surface a toast. Adding the metadata is a follow-up that needs
  // BE work first.
  async function handleStatusTransition(next: CommodityStatusValue) {
    if (!commodity) return
    const ok = await confirm({
      title: t("commodities:detail.statusTransition.title", {
        label: t(`commodities:status.${next}`),
      }),
      description: t("commodities:detail.statusTransition.description", {
        label: t(`commodities:status.${next}`),
      }),
      confirmLabel: t("common:actions.confirm"),
      destructive: next === "lost" || next === "written_off",
    })
    if (!ok) return
    try {
      await update.mutateAsync({ ...commodity, status: next })
      toast.success(t("commodities:toast.statusUpdated"))
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
      <div
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

        {/* Header — name + identity. The mock keeps the title at
            text-lg even on the full-page version, so both variants
            share the size. The status pills row that used to live
            inline in the description has been hoisted into its own
            row below so the badges don't compete for vertical
            rhythm with the brand subtitle. */}
        <header className="flex items-start gap-3">
          <CommodityThumb
            cover={commodity.cover}
            type={type}
            name={commodity.name}
            size={48}
            testId="commodity-detail-thumb"
          />
          <div className="min-w-0 flex-1">
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
        </header>

        {/* Status pills row — commodity status + warranty + days
            remaining. Mock shows this directly under the header,
            outside the action row, so glanceable signals don't
            crowd the buttons. */}
        <div
          className="flex flex-wrap items-center gap-2 -mt-2"
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
          <WarrantyBadge
            source={{
              warranty_expires_at: commodity.warranty_expires_at,
              tags: commodity.tags,
            }}
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
        </div>

        {/* Action buttons row — Edit (primary, flex-1), Insurance
            Report (mock parity, links to the per-item insurance
            report stub at /insurance/:id), and a small icon-only
            destructive Delete on the right. Print stays as a small
            icon button next to Delete so the action stays reachable
            without the row taking three lines on narrow viewports. */}
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setEditOpen(true)}
            data-testid="commodity-detail-edit"
            className="flex-1 gap-1.5"
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
                to={`/g/${encodeURIComponent(slug)}/insurance/${encodeURIComponent(commodity.id)}`}
              >
                <FileBarChart2 className="size-3.5" aria-hidden="true" />
                {t("commodities:detail.insuranceReport")}
              </Link>
            </Button>
          ) : null}
          <Button
            asChild
            type="button"
            variant="outline"
            size="icon"
            className="size-8"
            title={t("commodities:detail.print")}
            aria-label={t("commodities:detail.print")}
          >
            <Link to={printHref} data-testid="commodity-detail-print">
              <Printer className="size-3.5" aria-hidden="true" />
            </Link>
          </Button>
          <Button
            type="button"
            variant="outline"
            size="icon"
            className="size-8 text-destructive hover:bg-destructive/10"
            onClick={handleDelete}
            data-testid="commodity-detail-delete"
            title={t("commodities:detail.delete")}
            aria-label={t("commodities:detail.delete")}
          >
            <Trash2 className="size-3.5" aria-hidden="true" />
          </Button>
        </div>

        {/* CHANGE STATUS bar — only meaningful while the item is
            still `in_use`. Once it transitions to a terminal status
            (sold/lost/disposed/written_off) the bar disappears and
            the user can revert via the edit dialog. Each button
            opens a confirm; on confirm we PATCH `status` only — the
            mock's optional date/note/sale-price capture needs new
            BE columns first. */}
        {status === "in_use" ? (
          <div
            className="rounded-xl border border-border bg-muted/30 p-3 space-y-2"
            data-testid="commodity-detail-change-status"
          >
            <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {t("commodities:detail.statusTransition.heading")}
            </p>
            <div className="flex flex-wrap gap-1.5">
              {TERMINAL_STATUSES.map((s) => (
                <Button
                  key={s}
                  type="button"
                  variant="outline"
                  size="sm"
                  className={cn("gap-1.5 text-xs h-7", COMMODITY_STATUS_TONES[s])}
                  onClick={() => handleStatusTransition(s)}
                  data-testid={`commodity-detail-transition-${s}`}
                  disabled={update.isPending}
                >
                  {t(`commodities:status.${s}`)}
                </Button>
              ))}
            </div>
          </div>
        ) : null}

        <Tabs value={tab} onChange={setTab} fileCount={fileCount} />

        {tab === "details" ? (
          <>
            <DetailsTab
              commodity={commodity}
              groupCurrency={groupCurrency}
              purchaseCurrency={purchaseCurrency}
              areaName={areaName(commodity.area_id)}
              variant={variant}
            />
            {commodity.id ? <CommodityHistoryTimeline commodityId={commodity.id} /> : null}
          </>
        ) : tab === "warranty" ? (
          <WarrantyTab commodity={commodity} onSwitchToFiles={() => setTab("files")} />
        ) : tab === "lend" ? (
          <LendTab commodityId={commodity?.id ?? id} />
        ) : tab === "service" ? (
          <ServiceTab commodityId={commodity?.id ?? id} />
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
                      ? t("commodities:cover.setSuccess", { defaultValue: "Cover photo updated." })
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

      <CommodityFormDialog
        open={editOpen}
        onOpenChange={handleEditOpenChange}
        mode="edit"
        initialValues={commodity}
        areas={areas.data ?? []}
        defaultCurrency={currency}
        onSubmit={handleSave}
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
}

function Tabs({ value, onChange, fileCount = 0 }: TabsProps) {
  const { t } = useTranslation()
  const tabs: { key: TabKey; label: string; count?: number }[] = [
    { key: "details", label: t("commodities:detail.tabs.details") },
    { key: "warranty", label: t("commodities:detail.tabs.warranty") },
    { key: "files", label: t("commodities:detail.tabs.files"), count: fileCount },
    { key: "lend", label: t("commodities:detail.tabs.lend") },
    { key: "service", label: t("commodities:detail.tabs.service") },
  ]
  return (
    <div
      role="tablist"
      className="flex gap-1 border-b border-border"
      data-testid="commodity-detail-tabs"
    >
      {tabs.map((tb) => (
        <button
          key={tb.key}
          role="tab"
          type="button"
          aria-selected={value === tb.key}
          onClick={() => onChange(tb.key)}
          className={cn(
            "inline-flex items-center gap-1.5 px-3 py-2 text-sm border-b-2 -mb-px transition-colors",
            value === tb.key
              ? "border-primary text-foreground"
              : "border-transparent text-muted-foreground hover:text-foreground"
          )}
          data-testid={`commodity-detail-tab-${tb.key}`}
        >
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
      ))}
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
  variant = "page",
}: DetailsTabProps) {
  const { t } = useTranslation()
  const noValue = t("commodities:detail.noValue")
  const rows: { icon: typeof MapPin; label: string; value: React.ReactNode; testId?: string }[] = [
    { icon: MapPin, label: t("commodities:detail.fields.area"), value: areaName || noValue },
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
      icon: Hash,
      label: t("commodities:detail.fields.originalPrice"),
      value:
        commodity.original_price !== undefined
          ? formatCurrency(Number(commodity.original_price), purchaseCurrency)
          : noValue,
    },
    {
      icon: Hash,
      label: t("commodities:detail.fields.convertedOriginalPrice"),
      value:
        commodity.converted_original_price !== undefined
          ? formatCurrency(Number(commodity.converted_original_price), groupCurrency)
          : noValue,
    },
    {
      icon: Hash,
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
  // Sheet variant renders a single-column vertical list with each row
  // formatted as `[icon] [label/value]` to match the design mock; the
  // page variant keeps the wider 2-col grid that fills the canvas.
  const isSheet = variant === "sheet"
  const containerClass = isSheet
    ? "flex flex-col py-2"
    : "grid grid-cols-1 sm:grid-cols-2 gap-4 py-6"
  const fullWidthClass = isSheet ? "" : "sm:col-span-2"
  const separatorClass = isSheet ? "" : "sm:col-span-2"
  return (
    <Card data-testid="commodity-detail-details">
      <CardContent className={containerClass}>
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
        {commodity.tags && commodity.tags.length > 0 ? (
          <div className={cn("flex flex-col gap-1.5", fullWidthClass, isSheet && "py-2.5")}>
            <DetailLabel icon={Tag} label={t("commodities:detail.fields.tags")} />
            <div className="flex flex-wrap gap-1.5">
              {commodity.tags.map((tag) => (
                <Badge key={tag} variant="secondary">
                  {tag}
                </Badge>
              ))}
            </div>
          </div>
        ) : null}
        {Array.isArray(commodity.urls) && commodity.urls.length > 0 ? (
          <div className={cn("flex flex-col gap-1.5", fullWidthClass, isSheet && "py-2.5")}>
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
        ) : null}
        {commodity.comments ? (
          <div className={cn("flex flex-col gap-1.5", fullWidthClass, isSheet && "py-2.5")}>
            <DetailLabel icon={Hash} label={t("commodities:detail.fields.comments")} />
            <p className="text-sm whitespace-pre-wrap">{commodity.comments}</p>
          </div>
        ) : null}
        {commodity.extra_serial_numbers && commodity.extra_serial_numbers.length > 0 ? (
          <div className={cn("flex flex-col gap-1.5", fullWidthClass, isSheet && "py-2.5")}>
            <DetailLabel icon={Hash} label={t("commodities:detail.fields.extraSerialNumbers")} />
            <div className="flex flex-wrap gap-1.5">
              {commodity.extra_serial_numbers.map((s) => (
                <Badge key={s} variant="outline">
                  {s}
                </Badge>
              ))}
            </div>
          </div>
        ) : null}
        {commodity.part_numbers && commodity.part_numbers.length > 0 ? (
          <div className={cn("flex flex-col gap-1.5", fullWidthClass, isSheet && "py-2.5")}>
            <DetailLabel icon={Hash} label={t("commodities:detail.fields.partNumbers")} />
            <div className="flex flex-wrap gap-1.5">
              {commodity.part_numbers.map((p) => (
                <Badge key={p} variant="outline">
                  {p}
                </Badge>
              ))}
            </div>
          </div>
        ) : null}
        <Separator className={separatorClass} />
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
      </CardContent>
    </Card>
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
  // Renders attachments for this commodity through the unified
  // /files surface (#1411 AC #4). The legacy meta.images / .invoices /
  // .manuals counters are no longer consulted — once #1399 backfill
  // ran, every legacy row is also a /files row, and the unified
  // surface is the single source of truth.
  //
  // `onAttachClick` opens the page-level upload dialog with the
  // commodity preselected (#1448). The page also exposes a drag-drop
  // overlay that opens the same dialog with files preloaded.
  // `coverState` + `onSetCover` (issue #1451 option B) thread the
  // cover-photo wiring down to the per-file star button.
  return (
    <div data-testid="commodity-detail-files">
      <EntityFilesPanel
        linkedEntityType="commodity"
        linkedEntityId={commodityId}
        onAttachClick={onAttachClick}
        coverState={coverState}
        onSetCover={onSetCover}
        coverBusy={coverBusy}
      />
    </div>
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
  const status: CommodityWarrantyStatus = commodity
    ? warrantyStatus({
        warranty_expires_at: commodity.warranty_expires_at,
        tags: commodity.tags,
      })
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
        // Width matches the design mock 1:1 (`sm:max-w-lg` → 32rem
        // ≈ 512px). The detail surface adapts: in sheet mode the
        // DetailsTab body switches to a vertical icon|label-value
        // list (instead of a 2-col grid) so the narrower panel
        // stays readable. Below `sm` the Sheet primitive falls
        // back to `w-3/4` and the panel takes the whole viewport.
        className="w-full sm:max-w-lg overflow-y-auto p-0"
        data-testid="commodity-detail-sheet"
      >
        <CommodityDetailContent id={id} variant="sheet" />
      </SheetContent>
    </Sheet>
  )
}
