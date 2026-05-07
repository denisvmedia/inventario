import { useMemo } from "react"

import { useCommodities, useCommoditiesValue } from "@/features/commodities/hooks"
import type { Commodity } from "@/features/commodities/api"
import { warrantyStatus, type CommodityWarrantyStatus } from "@/features/commodities/constants"
import { useCurrentGroup } from "@/features/group/GroupContext"

// What the Dashboard page renders. Aggregates two upstream queries
// (commodities list + values endpoint) into a single shape so the page
// component stays dumb. Warranty status counts ship now (#1367 / #1529)
// — both `warrantyStatusCounts` and `expiringWarranties` are derived
// from the same commodities list query, no extra round-trip.
export interface DashboardData {
  // True while either upstream query is on its first fetch. `isLoading`
  // (not `isFetching`) so a stale-while-revalidate refetch doesn't
  // flicker the skeleton state.
  isLoading: boolean
  // True if either upstream query errored out. The page renders an
  // empty-state-plus-toast in this case rather than partial data.
  isError: boolean
  // Total number of commodities in the active group. Drives the
  // "Total Items" stat card.
  totalItems: number
  // Sum of `current_price` across all commodities, in the group's
  // group currency (precomputed by `/commodities/values`).
  totalValue: number
  // Five most recently added commodities, sorted by registered_date
  // descending (falling back to last_modified_date if registered_date
  // is missing). The list is what the "Recently Added" card renders.
  recent: Commodity[]
  // Warranty bucket counts (active / expiring / expired / none) over
  // the loaded commodities slice. Drives the "Warranty Health" panel
  // bars + the upper bound for the bar widths.
  warrantyStatusCounts: Record<CommodityWarrantyStatus, number>
  // Items whose warranty status is "expiring" (≤60 days from expiry),
  // sorted by expiry ascending (next-to-expire first). Drives the
  // "Expiring Warranties" panel — capped at five rows so the panel
  // doesn't outgrow its tile.
  expiringWarranties: Commodity[]
}

// Date string ↦ Unix epoch (ms). Returns 0 for missing/unparseable
// values so they sort last when a sibling has a real date.
function parseDate(value: string | undefined): number {
  if (!value) return 0
  const ms = new Date(value).getTime()
  return Number.isNaN(ms) ? 0 : ms
}

// recentlyAdded picks the N most recently added commodities. Sort key is
// registered_date (when the user added the item to Inventario), falling
// back to last_modified_date when registered_date is unset (older
// commodities created before the field was a thing). Pure function so it
// can be unit-tested without hooks/MSW.
export function recentlyAdded(commodities: Commodity[], limit: number): Commodity[] {
  return [...commodities]
    .sort((a, b) => {
      const ad = parseDate(a.registered_date) || parseDate(a.last_modified_date)
      const bd = parseDate(b.registered_date) || parseDate(b.last_modified_date)
      return bd - ad
    })
    .slice(0, limit)
}

// warrantyBuckets walks the commodity list once and returns the
// per-status counts plus the slice destined for the "Expiring
// Warranties" panel. Single-pass so adding a third derived view
// later doesn't multiply the work.
export function warrantyBuckets(
  commodities: Commodity[],
  expiringLimit: number
): {
  counts: Record<CommodityWarrantyStatus, number>
  expiring: Commodity[]
} {
  const counts: Record<CommodityWarrantyStatus, number> = {
    active: 0,
    expiring: 0,
    expired: 0,
    none: 0,
  }
  const expiringRows: Commodity[] = []
  for (const c of commodities) {
    const s = warrantyStatus({
      warranty_expires_at: c.warranty_expires_at,
      tags: c.tags,
    })
    counts[s]++
    if (s === "expiring") expiringRows.push(c)
  }
  expiringRows.sort((a, b) =>
    (a.warranty_expires_at ?? "").localeCompare(b.warranty_expires_at ?? "")
  )
  return { counts, expiring: expiringRows.slice(0, expiringLimit) }
}

// useDashboardData composes the two upstream queries the page needs.
// Returning a single object keeps Dashboard.tsx free of TanStack
// machinery — it just renders against a plain shape.
//
// Both queries are gated behind `currentGroup`: the http client rewrites
// /commodities → /g/{slug}/commodities only after the GroupProvider's
// useEffect has populated the slug slot. Firing before that would issue
// /commodities and 404 in CI; gating here keeps the hook's contract
// "results are scoped to the URL's group" without leaking timing
// concerns to callers.
export function useDashboardData(): DashboardData {
  const { currentGroup } = useCurrentGroup()
  const enabled = !!currentGroup
  // perPage=100 is the BE max — gives the "Recently added" slice the
  // best chance of seeing new items in groups beyond the default 50.
  // For larger groups, the BE's name/id ordering still means a recent
  // addition can fall off page 1; that's a known limitation and lives
  // in the useCommodities() docstring.
  const commodities = useCommodities({ perPage: 100 }, { enabled })
  const values = useCommoditiesValue({ enabled })

  return useMemo<DashboardData>(() => {
    const list = commodities.data?.commodities ?? []
    const { counts, expiring } = warrantyBuckets(list, 5)
    return {
      // Treat "waiting for group" as still-loading so the page renders
      // skeletons rather than an empty state.
      isLoading: !enabled || commodities.isLoading || values.isLoading,
      isError: commodities.isError || values.isError,
      totalItems: commodities.data?.total ?? list.length,
      totalValue: values.data?.globalTotal ?? 0,
      recent: recentlyAdded(list, 5),
      warrantyStatusCounts: counts,
      expiringWarranties: expiring,
    }
  }, [
    enabled,
    commodities.data,
    commodities.isLoading,
    commodities.isError,
    values.data,
    values.isLoading,
    values.isError,
  ])
}
