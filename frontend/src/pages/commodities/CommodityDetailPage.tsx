import { useEffect, useMemo, useState } from "react"
import { Link, useMatch, useNavigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import {
  ArrowLeft,
  Calendar,
  ExternalLink,
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
import { Skeleton } from "@/components/ui/skeleton"
import { ComingSoonBanner } from "@/components/coming-soon/ComingSoonBanner"
import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAreas } from "@/features/areas/hooks"
import { useDeleteCommodity, useCommodity, useUpdateCommodity } from "@/features/commodities/hooks"
import {
  COMMODITY_STATUS_TONES,
  COMMODITY_TYPE_ICONS,
  type CommodityStatusValue,
  type CommodityTypeValue,
} from "@/features/commodities/constants"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatCurrency, formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

type TabKey = "details" | "warranty" | "files"

// /commodities/:id — full-page detail. The design mock renders this as
// a Sheet overlay over the list; that variant is deferred to a follow-up
// because the deep-link case (a shared link or back-button reload) needs
// a stable URL anyway, and "full page everywhere" is simpler than
// switching modes based on navigation provenance.
//
// Tabs: Details, Warranty, Files. Warranty + Files are stubbed because
// first-class warranties (#1367) and the unified Files surface
// (#1398/#1399) haven't shipped — the tabs render coming-soon banners
// linking the trackers so the UI doesn't pretend.
export function CommodityDetailPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { id = "" } = useParams<{ id: string }>()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug
  const enabled = !!currentGroup
  const detail = useCommodity(id, { enabled })
  const areas = useAreas({ enabled })
  const update = useUpdateCommodity(id)
  const remove = useDeleteCommodity()
  const toast = useAppToast()
  const confirm = useConfirm()

  const [tab, setTab] = useState<TabKey>("details")
  // /commodities/:id/edit deep-link: open the edit dialog immediately.
  // Closing the dialog navigates back to /commodities/:id (sans /edit)
  // so the URL stays meaningful.
  const editMatch = useMatch({ path: "/g/:groupSlug/commodities/:id/edit", end: true })
  const [editOpen, setEditOpen] = useState(false)
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
  const meta = detail.data?.meta

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
      <div className="p-6 max-w-4xl mx-auto w-full" data-testid="commodity-detail-loading">
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
      <div className="p-6 max-w-4xl mx-auto w-full">
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
      <div className="p-6 max-w-4xl mx-auto w-full">
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
  const icon = type ? COMMODITY_TYPE_ICONS[type] : "📦"
  // Per the BE, `original_price` is denominated in the purchase
  // currency (`original_price_currency`); `converted_original_price`
  // and `current_price` are denominated in the group's main currency.
  // Pass both down so DetailsTab can format each row correctly rather
  // than mixing the symbols.
  const groupCurrency = currentGroup?.main_currency ?? "USD"
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

  return (
    <>
      <RouteTitle title={t("commodities:detail.documentTitle")} />
      <div
        className="flex flex-col gap-6 p-6 max-w-4xl mx-auto w-full"
        data-testid="page-commodity-detail"
      >
        <Link
          to={listHref}
          className="text-sm text-muted-foreground hover:underline inline-flex items-center gap-1"
        >
          <ArrowLeft className="size-3.5" aria-hidden="true" />
          {t("commodities:detail.backToList")}
        </Link>

        <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex items-start gap-3 min-w-0">
            <div className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-muted text-2xl">
              {icon}
            </div>
            <div className="min-w-0">
              <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight truncate">
                {commodity.name}
              </h1>
              <div className="mt-1 flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                {commodity.short_name ? (
                  <span data-testid="commodity-detail-short-name">{commodity.short_name}</span>
                ) : null}
                {type ? <span>· {t(`commodities:type.${type}`)}</span> : null}
                {commodity.draft ? (
                  <Badge variant="outline" className="border-dashed text-[10px] h-4 px-1">
                    draft
                  </Badge>
                ) : null}
                {status && status !== "in_use" ? (
                  <span
                    className={cn(
                      "text-[10px] font-medium px-1.5 py-0.5 rounded-full border",
                      tone
                    )}
                  >
                    {t(`commodities:status.${status}`)}
                  </span>
                ) : null}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-1.5">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setEditOpen(true)}
              data-testid="commodity-detail-edit"
              className="gap-1.5"
            >
              <Pencil className="size-3.5" aria-hidden="true" />
              {t("commodities:detail.edit")}
            </Button>
            <Button asChild type="button" variant="ghost" size="sm" className="gap-1.5">
              <Link to={printHref} data-testid="commodity-detail-print">
                <Printer className="size-3.5" aria-hidden="true" />
                {t("commodities:detail.print")}
              </Link>
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={handleDelete}
              data-testid="commodity-detail-delete"
              className="gap-1.5 text-destructive"
            >
              <Trash2 className="size-3.5" aria-hidden="true" />
              {t("commodities:detail.delete")}
            </Button>
          </div>
        </header>

        <Tabs value={tab} onChange={setTab} />

        {tab === "details" ? (
          <>
            <DetailsTab
              commodity={commodity}
              groupCurrency={groupCurrency}
              purchaseCurrency={purchaseCurrency}
              areaName={areaName(commodity.area_id)}
            />
            <StatusHistoryCard commodity={commodity} />
          </>
        ) : tab === "warranty" ? (
          <Card data-testid="commodity-detail-warranty">
            <CardContent className="py-6">
              <ComingSoonBanner surface="warranties" />
            </CardContent>
          </Card>
        ) : (
          <FilesTab meta={meta} />
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
    </>
  )
}

interface TabsProps {
  value: TabKey
  onChange: (v: TabKey) => void
}

function Tabs({ value, onChange }: TabsProps) {
  const { t } = useTranslation()
  const tabs: { key: TabKey; label: string }[] = [
    { key: "details", label: t("commodities:detail.tabs.details") },
    { key: "warranty", label: t("commodities:detail.tabs.warranty") },
    { key: "files", label: t("commodities:detail.tabs.files") },
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
            "px-3 py-2 text-sm border-b-2 -mb-px transition-colors",
            value === tb.key
              ? "border-primary text-foreground"
              : "border-transparent text-muted-foreground hover:text-foreground"
          )}
          data-testid={`commodity-detail-tab-${tb.key}`}
        >
          {tb.label}
        </button>
      ))}
    </div>
  )
}

// StatusHistoryCard renders a minimal activity timeline using the only
// timestamps the BE exposes today: registered_date (when the item was
// created) and last_modified_date (most recent edit). A real status log
// is BE-side work — first-class warranties / status transitions land
// with #1367; this card upgrades whenever that ships. Until then it's
// the user's "when did I add this?" reference.
function StatusHistoryCard({
  commodity,
}: {
  commodity: import("@/features/commodities/api").Commodity
}) {
  const { t } = useTranslation()
  const status = commodity.status as CommodityStatusValue | undefined
  const tone = status ? COMMODITY_STATUS_TONES[status] : ""
  const registered = commodity.registered_date
    ? formatDate(commodity.registered_date as string, { style: "medium" })
    : null
  const lastModified = commodity.last_modified_date
    ? formatDate(commodity.last_modified_date as string, { style: "medium" })
    : null
  const sameDay = registered && lastModified && registered === lastModified
  return (
    <Card data-testid="commodity-detail-history">
      <CardHeader>
        <CardTitle className="text-base">{t("commodities:detail.historyTitle")}</CardTitle>
        <CardDescription>{t("commodities:detail.historyDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <ol className="relative ml-2 border-l border-border pl-4 space-y-3">
          {registered ? (
            <li className="text-sm" data-testid="history-row-registered">
              <span className="absolute -ml-[18px] mt-1.5 size-2 rounded-full bg-muted-foreground" />
              <span className="font-medium">
                {t("commodities:detail.historyRegistered", { date: registered })}
              </span>
            </li>
          ) : null}
          {lastModified && !sameDay ? (
            <li className="text-sm" data-testid="history-row-modified">
              <span className="absolute -ml-[18px] mt-1.5 size-2 rounded-full bg-muted-foreground" />
              <span>{t("commodities:detail.historyModified", { date: lastModified })}</span>
            </li>
          ) : null}
          {status ? (
            <li className="text-sm flex items-center gap-2" data-testid="history-row-current">
              <span className="absolute -ml-[18px] mt-1.5 size-2 rounded-full bg-primary" />
              <span>{t("commodities:detail.historyCurrent")}</span>
              <span className={cn("text-xs font-medium px-2 py-0.5 rounded-full border", tone)}>
                {t(`commodities:status.${status}`)}
              </span>
            </li>
          ) : null}
        </ol>
      </CardContent>
    </Card>
  )
}

interface DetailsTabProps {
  commodity: import("@/features/commodities/api").Commodity
  // Currency the BE stores `original_price` in (taken from
  // `original_price_currency`).
  purchaseCurrency: string
  // Currency the BE stores `converted_original_price` and
  // `current_price` in — always the active group's main currency.
  groupCurrency: string
  areaName: string
}

function DetailsTab({ commodity, purchaseCurrency, groupCurrency, areaName }: DetailsTabProps) {
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
  return (
    <Card data-testid="commodity-detail-details">
      <CardContent className="grid grid-cols-1 sm:grid-cols-2 gap-4 py-6">
        {rows.map((r) => (
          <DetailRow key={r.label} icon={r.icon} label={r.label} value={r.value} />
        ))}
        {commodity.tags && commodity.tags.length > 0 ? (
          <div className="sm:col-span-2 flex flex-col gap-1.5">
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
          <div className="sm:col-span-2 flex flex-col gap-1.5">
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
          <div className="sm:col-span-2 flex flex-col gap-1.5">
            <DetailLabel icon={Hash} label={t("commodities:detail.fields.comments")} />
            <p className="text-sm whitespace-pre-wrap">{commodity.comments}</p>
          </div>
        ) : null}
        {commodity.extra_serial_numbers && commodity.extra_serial_numbers.length > 0 ? (
          <div className="sm:col-span-2 flex flex-col gap-1.5">
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
          <div className="sm:col-span-2 flex flex-col gap-1.5">
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
        <Separator className="sm:col-span-2" />
        <DetailRow
          icon={Calendar}
          label={t("commodities:detail.fields.registeredDate")}
          value={
            commodity.registered_date
              ? formatDate(commodity.registered_date as string, { style: "short" })
              : noValue
          }
        />
        <DetailRow
          icon={Calendar}
          label={t("commodities:detail.fields.lastModifiedDate")}
          value={
            commodity.last_modified_date
              ? formatDate(commodity.last_modified_date as string, { style: "short" })
              : noValue
          }
        />
      </CardContent>
    </Card>
  )
}

function DetailRow({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof MapPin
  label: string
  value: React.ReactNode
}) {
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
  meta: import("@/features/commodities/api").CommodityMeta | undefined
}

function FilesTab({ meta }: FilesTabProps) {
  const totalLegacy =
    (meta?.images?.length ?? 0) + (meta?.invoices?.length ?? 0) + (meta?.manuals?.length ?? 0)
  return (
    <Card data-testid="commodity-detail-files">
      <CardContent className="py-6">
        {totalLegacy > 0 ? (
          <p className="mb-4 text-sm text-muted-foreground">
            {totalLegacy} attached file{totalLegacy === 1 ? "" : "s"} (legacy storage).
          </p>
        ) : null}
        <ComingSoonBanner surface="filesUnification" />
      </CardContent>
    </Card>
  )
}
