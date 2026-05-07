import { useMemo } from "react"
import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { WARRANTY_STATUS_CONFIG } from "@/components/warranty/config"
import { useAreas } from "@/features/areas/hooks"
import { useCommodities } from "@/features/commodities/hooks"
import {
  COMMODITY_WARRANTY_STATUSES,
  effectiveWarrantyExpiry,
  warrantyStatus,
  type CommodityWarrantyStatus,
} from "@/features/commodities/constants"
import type { Commodity } from "@/features/commodities/api"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

// Tabs map 1:1 to the BE `warranty_status=` filter; "all" is intentionally
// dropped — the four summary cards above the tabs already give the
// aggregate view. The default tab is "expiring" (the most actionable
// bucket); the URL omits ?tab when on the default so a fresh load looks
// the same with or without the param.
const VALID_TABS = COMMODITY_WARRANTY_STATUSES
type WarrantyTab = (typeof VALID_TABS)[number]

const DEFAULT_TAB: WarrantyTab = "expiring"
// Tabs that show a counter Badge next to the label. "none" intentionally
// omits the badge per the design mock — that bucket is the catch-all
// rather than something the user is monitoring, so the count would
// just be visual noise.
const TAB_COUNTERS: ReadonlySet<WarrantyTab> = new Set(["expiring", "active", "expired"])

function parseTab(raw: string | null): WarrantyTab {
  return (VALID_TABS as readonly string[]).includes(raw ?? "") ? (raw as WarrantyTab) : DEFAULT_TAB
}

function daysUntil(dateStr: string | undefined): number | null {
  if (!dateStr) return null
  const t = Date.parse(`${dateStr}T00:00:00Z`)
  if (Number.isNaN(t)) return null
  const now = new Date()
  const todayUTC = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate())
  return Math.round((t - todayUTC) / (1000 * 60 * 60 * 24))
}

// WarrantiesListPage is the dedicated `/g/:slug/warranties` surface
// (#1367, polish #1529). Group-wide list of commodities partitioned by
// warranty status with four tabs (Expiring / Active / Expired / No
// Warranty), four tinted summary cards above, and a row layout that
// surfaces the next-action ("N days left" or "N days ago") in the
// status colour. The status pill itself is the canonical
// `WarrantyBadge` so all four warranty surfaces (badge / list rows /
// dashboard / item-detail tab) read the same `--status-*` tokens.
//
// Counts: the per-tab badges + summary cards are computed from the
// "all rows" query — a single perPage=200 fetch — rather than four
// concurrent filtered queries, so the badges stay accurate when the
// user moves between tabs without a refetch.
export function WarrantiesListPage() {
  const { t } = useTranslation(["warranties", "commodities", "common"])
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const [searchParams, setSearchParams] = useSearchParams()
  const tab = parseTab(searchParams.get("tab"))

  // Single fetch for everything. perPage=200 covers all but the largest
  // groups; if a user blows past that we'd add server-side pagination
  // here, but the BE's `warranty_status=` filter still works either way
  // since the tab partitioning happens client-side from this dataset.
  //
  // Both queries are gated on `currentGroup` because the http client
  // rewrites /commodities → /g/{slug}/commodities only after the
  // GroupProvider's useEffect has populated the slug slot (same
  // pattern useDashboardData uses). Firing before that would 404.
  const enabled = !!currentGroup
  const list = useCommodities({ perPage: 200, includeInactive: true }, { enabled })
  const areas = useAreas({ enabled })
  const areaName = useMemo(() => {
    const map = new Map<string, string>()
    for (const a of areas.data ?? []) {
      if (a.id && a.name) map.set(a.id, a.name)
    }
    return (id: string | undefined) => (id ? (map.get(id) ?? "") : "")
  }, [areas.data])

  // Bucket every commodity into one of the four warranty statuses.
  // Used both for the per-tab counters and the active-tab list. Using
  // a single pass keeps the page responsive even when the dataset
  // grows; the alternative (four `filter()` calls) re-walks the array
  // four times for the same result.
  //
  // We carry the resolved (effective) expiry date through the bucket
  // so that legacy `warranty:YYYY-MM-DD` tag-only rows render the
  // right "N days left/ago" line and the right Expires date — without
  // it, the bucketing happily picks them up but the row UI would say
  // "No date".
  const buckets = useMemo(() => {
    const out: Record<CommodityWarrantyStatus, BucketRow[]> = {
      active: [],
      expiring: [],
      expired: [],
      none: [],
    }
    for (const c of list.data?.commodities ?? []) {
      const expiresAt = effectiveWarrantyExpiry({
        warranty_expires_at: c.warranty_expires_at,
        tags: c.tags,
      })
      const s = warrantyStatus({
        warranty_expires_at: c.warranty_expires_at,
        tags: c.tags,
      })
      out[s].push({ commodity: c, expiresAt })
    }
    // Sort by expiry ascending: surfaces the most-actionable rows
    // first inside Expiring (next to lapse on top), and Active (next
    // to enter the expiring window on top). Expired is reversed so
    // the *most recently* lapsed rows are on top — those are the
    // ones the user is most likely to act on (renew / replace).
    for (const status of COMMODITY_WARRANTY_STATUSES) {
      out[status].sort((a, b) => (a.expiresAt ?? "").localeCompare(b.expiresAt ?? ""))
    }
    out.expired.reverse()
    return out
  }, [list.data?.commodities])

  const counts: Record<CommodityWarrantyStatus, number> = {
    active: buckets.active.length,
    expiring: buckets.expiring.length,
    expired: buckets.expired.length,
    none: buckets.none.length,
  }

  function setTab(next: WarrantyTab) {
    const params = new URLSearchParams(searchParams)
    if (next === DEFAULT_TAB) params.delete("tab")
    else params.set("tab", next)
    setSearchParams(params, { replace: true })
  }

  const rows = buckets[tab]

  return (
    <div className="flex flex-col gap-6 p-6 max-w-4xl mx-auto w-full" data-testid="page-warranties">
      <header className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold tracking-tight">{t("warranties:list.title")}</h1>
        <p className="text-muted-foreground">{t("warranties:list.subtitle")}</p>
      </header>

      {/* Status summary cards — counts mirror the per-tab badges. */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4" data-testid="warranties-summary">
        {VALID_TABS.map((status) => {
          const visual = WARRANTY_STATUS_CONFIG[status]
          const Icon = visual.icon
          return (
            <Card
              key={status}
              className={cn("gap-3 border", visual.bg, visual.border)}
              data-testid={`warranties-summary-${status}`}
            >
              <CardHeader className="pb-1">
                <Icon className={cn("size-5", visual.text)} aria-hidden="true" />
              </CardHeader>
              <CardContent>
                <p className={cn("text-2xl font-bold", visual.text)}>{counts[status]}</p>
                <p className="text-xs text-muted-foreground mt-0.5">{t(visual.i18nKey)}</p>
              </CardContent>
            </Card>
          )
        })}
      </div>

      <div
        role="tablist"
        className="flex gap-1 border-b border-border"
        data-testid="warranties-tabs"
      >
        {VALID_TABS.map((s) => {
          const visual = WARRANTY_STATUS_CONFIG[s]
          const showCounter = TAB_COUNTERS.has(s) && counts[s] > 0
          return (
            <button
              key={s}
              role="tab"
              type="button"
              aria-selected={tab === s}
              onClick={() => setTab(s)}
              data-testid={`warranties-tab-${s}`}
              className={cn(
                "inline-flex items-center gap-1.5 px-3 py-2 text-sm border-b-2 -mb-px",
                tab === s
                  ? "border-primary text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              )}
            >
              {t(`warranties:list.tab.${s}`)}
              {showCounter ? (
                <Badge
                  variant="outline"
                  className={cn("h-4 px-1 text-[10px]", visual.text, visual.border)}
                  data-testid={`warranties-tab-${s}-count`}
                >
                  {counts[s]}
                </Badge>
              ) : null}
            </button>
          )
        })}
      </div>

      {list.isLoading || areas.isLoading ? (
        <div className="flex flex-col gap-2" data-testid="warranties-loading">
          <Skeleton className="h-14" />
          <Skeleton className="h-14" />
          <Skeleton className="h-14" />
        </div>
      ) : rows.length === 0 ? (
        <EmptyState tab={tab} />
      ) : (
        <Card className="overflow-hidden p-0" data-testid="warranties-list">
          <ul>
            {rows.map(({ commodity, expiresAt }, i) => (
              <li key={commodity.id ?? `idx-${i}`}>
                {i > 0 ? <Separator /> : null}
                <WarrantyRow
                  commodity={commodity}
                  expiresAt={expiresAt}
                  slug={slug}
                  status={tab}
                  areaName={areaName(commodity.area_id)}
                />
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  )
}

function EmptyState({ tab }: { tab: WarrantyTab }) {
  const { t } = useTranslation(["warranties"])
  const visual = WARRANTY_STATUS_CONFIG[tab]
  const Icon = visual.icon
  return (
    <div
      className="flex flex-col items-center gap-2 py-12 text-center"
      data-testid="warranties-empty"
    >
      <Icon className="size-8 text-muted-foreground/30" aria-hidden="true" />
      <p className="text-sm text-muted-foreground">{t(`warranties:list.empty.${tab}`)}</p>
    </div>
  )
}

interface BucketRow {
  commodity: Commodity
  // Resolved expiry date — either `warranty_expires_at` or the
  // legacy `warranty:YYYY-MM-DD` tag (see `effectiveWarrantyExpiry`).
  // Undefined only for rows in the "none" bucket where neither signal
  // is present.
  expiresAt: string | undefined
}

interface WarrantyRowProps extends BucketRow {
  slug: string
  status: WarrantyTab
  areaName: string
}

function WarrantyRow({ commodity, expiresAt, slug, status, areaName }: WarrantyRowProps) {
  const { t } = useTranslation(["warranties", "commodities"])
  const visual = WARRANTY_STATUS_CONFIG[status]
  const Icon = visual.icon
  const days = daysUntil(expiresAt)
  const id = commodity.id ?? ""
  const subtitle = [
    commodity.short_name && commodity.short_name !== commodity.name ? commodity.short_name : null,
    areaName,
  ]
    .filter((part): part is string => Boolean(part && part.length > 0))
    .join(" · ")
  // `Container` was previously a polymorphic Link|div, which leaked
  // the `to` prop onto a div. Render the two branches explicitly so
  // each tag carries only its own props.
  const className =
    "flex w-full items-center gap-4 px-5 py-4 text-left transition-colors hover:bg-muted/50"
  const inner = (
    <>
      <Icon className={cn("size-5 shrink-0", visual.text)} aria-hidden="true" />
      <div className="flex-1 min-w-0">
        <p className="truncate text-sm font-medium">{commodity.name}</p>
        {subtitle ? <p className="text-xs text-muted-foreground">{subtitle}</p> : null}
      </div>
      <div className="text-right shrink-0">
        {expiresAt ? (
          <>
            <p className="text-sm font-medium">{formatDate(expiresAt)}</p>
            {days !== null ? (
              <p className={cn("text-xs", visual.text)}>
                {days >= 0
                  ? t("warranties:list.row.daysLeft", { count: days })
                  : t("warranties:list.row.daysAgo", { count: -days })}
              </p>
            ) : null}
          </>
        ) : (
          <p className="text-xs text-muted-foreground">{t("warranties:list.row.noDate")}</p>
        )}
      </div>
    </>
  )
  if (id) {
    return (
      <Link
        to={`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}?tab=warranty`}
        className={className}
        data-testid={`warranties-row-${id}`}
      >
        {inner}
      </Link>
    )
  }
  return (
    <div className={className} data-testid="warranties-row-unknown">
      {inner}
    </div>
  )
}
