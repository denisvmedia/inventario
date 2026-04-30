import { useMemo } from "react"

import { useCommodities, useCommoditiesValue } from "@/features/commodities/hooks"
import type { Commodity } from "@/features/commodities/api"
import { useCurrentGroup } from "@/features/group/GroupContext"

// What the Dashboard page renders. Aggregates two upstream queries
// (commodities list + values endpoint) into a single shape so the page
// component stays dumb. Warranty status counts are stubbed at zero
// today — first-class warranties land in #1367; until then the cards
// surface a "Coming soon" affordance instead of derived numbers.
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
  // main currency (precomputed by `/commodities/values`).
  totalValue: number
  // Five most recently added commodities, sorted by registered_date
  // descending (falling back to last_modified_date if registered_date
  // is missing). The list is what the "Recently Added" card renders.
  recent: Commodity[]
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
  const commodities = useCommodities({ enabled })
  const values = useCommoditiesValue({ enabled })

  return useMemo<DashboardData>(() => {
    const list = commodities.data?.commodities ?? []
    return {
      // Treat "waiting for group" as still-loading so the page renders
      // skeletons rather than an empty state.
      isLoading: !enabled || commodities.isLoading || values.isLoading,
      isError: commodities.isError || values.isError,
      totalItems: commodities.data?.total ?? list.length,
      totalValue: values.data?.globalTotal ?? 0,
      recent: recentlyAdded(list, 5),
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
