import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router-dom"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useCommodities } from "@/features/commodities/hooks"
import {
  COMMODITY_WARRANTY_STATUSES,
  COMMODITY_WARRANTY_TONES,
  warrantyStatus,
  type CommodityWarrantyStatus,
} from "@/features/commodities/constants"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

// VALID_TABS includes the "all" sentinel at the head — it does not map
// to a server-side filter, just renders every commodity that owns a
// `warranty_expires_at` (or a legacy tag). The other entries map 1:1
// to `warranty_status=` on the BE.
const VALID_TABS = ["all", ...COMMODITY_WARRANTY_STATUSES] as const
type WarrantyTab = (typeof VALID_TABS)[number]

function parseTab(raw: string | null): WarrantyTab {
  return (VALID_TABS as readonly string[]).includes(raw ?? "") ? (raw as WarrantyTab) : "all"
}

// WarrantiesListPage is the dedicated `/g/:slug/warranties` surface
// (#1367). Group-wide list of commodities partitioned by warranty
// status with tabs for All / Active / Expiring / Expired / None.
//
// The status pill itself is computed client-side via warrantyStatus()
// — the BE pre-filters via `warranty_status=` so the page is what
// the server returned, and pill rendering doesn't need a second
// round-trip.
export function WarrantiesListPage() {
  const { t } = useTranslation(["warranties", "commodities", "common"])
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const [searchParams, setSearchParams] = useSearchParams()
  const tab = parseTab(searchParams.get("tab"))

  // Default view (All) shows every commodity with a tracked warranty,
  // sorted by purchase_date desc. The status tabs each request a
  // single status from the BE — the BE filter is documented in
  // models.ComputeWarrantyStatus and the worker emits emails on the
  // same boundary.
  const list = useCommodities({
    perPage: 100,
    includeInactive: true,
    warrantyStatuses: tab === "all" ? undefined : [tab],
  })

  function setTab(next: WarrantyTab) {
    const params = new URLSearchParams(searchParams)
    if (next === "all") params.delete("tab")
    else params.set("tab", next)
    setSearchParams(params, { replace: true })
  }

  const rows =
    tab === "all"
      ? // The "all" tab includes both items with a warranty AND items
        // without (so the user can see the universe and notice rows that
        // need a date filling in). Filter to "has any warranty" using
        // the same warrantyStatus() helper the row pills use, so what's
        // counted matches what's rendered.
        (list.data?.commodities.filter((c) => {
          const s = warrantyStatus({
            warranty_expires_at: c.warranty_expires_at,
            tags: c.tags,
          })
          return s !== "none" || !!c.warranty_expires_at
        }) ?? [])
      : (list.data?.commodities ?? [])

  return (
    <div className="flex flex-col gap-6 p-6" data-testid="page-warranties">
      <header className="flex flex-col gap-1">
        <h1 className="text-2xl font-semibold">{t("warranties:list.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("warranties:list.subtitle")}</p>
      </header>

      <div
        role="tablist"
        className="flex gap-1 border-b border-border"
        data-testid="warranties-tabs"
      >
        {VALID_TABS.map((s) => (
          <button
            key={s}
            role="tab"
            type="button"
            aria-selected={tab === s}
            onClick={() => setTab(s)}
            data-testid={`warranties-tab-${s}`}
            className={cn(
              "px-3 py-2 text-sm border-b-2 -mb-px",
              tab === s
                ? "border-primary text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
          >
            {t(`warranties:list.tab.${s}`)}
          </button>
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="sr-only">{t("warranties:list.title")}</CardTitle>
        </CardHeader>
        <CardContent>
          {list.isLoading ? (
            <div className="flex flex-col gap-2" data-testid="warranties-loading">
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
            </div>
          ) : rows.length === 0 ? (
            <p className="text-sm text-muted-foreground" data-testid="warranties-empty">
              {t(`warranties:list.empty.${tab}`)}
            </p>
          ) : (
            <table className="w-full text-sm" data-testid="warranties-table">
              <thead className="text-left text-xs text-muted-foreground">
                <tr>
                  <th className="px-2 py-2 font-medium">{t("warranties:list.headerItem")}</th>
                  <th className="px-2 py-2 font-medium">{t("warranties:list.headerExpires")}</th>
                  <th className="px-2 py-2 font-medium">{t("warranties:list.headerStatus")}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((c) => {
                  const status: CommodityWarrantyStatus = warrantyStatus({
                    warranty_expires_at: c.warranty_expires_at,
                    tags: c.tags,
                  })
                  const id = c.id ?? ""
                  return (
                    <tr
                      key={id}
                      className="border-t border-border"
                      data-testid={`warranties-row-${id}`}
                    >
                      <td className="px-2 py-2">
                        {id ? (
                          <Link
                            to={`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}?tab=warranty`}
                            className="font-medium hover:underline"
                          >
                            {c.name}
                          </Link>
                        ) : (
                          <span className="font-medium">{c.name}</span>
                        )}
                      </td>
                      <td className="px-2 py-2 text-muted-foreground">
                        {c.warranty_expires_at ? formatDate(c.warranty_expires_at) : "—"}
                      </td>
                      <td className="px-2 py-2">
                        <span
                          className={cn(
                            "inline-flex items-center rounded-md border px-2 py-1 text-xs font-medium",
                            COMMODITY_WARRANTY_TONES[status]
                          )}
                          data-testid={`warranties-status-${id}`}
                        >
                          {t(`commodities:warrantyStatus.${status}`)}
                        </span>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
