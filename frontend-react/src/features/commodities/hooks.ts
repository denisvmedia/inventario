import { useQuery } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import { getCommoditiesValue, listCommodities, type Commodity, type CommoditiesValue } from "./api"
import { commodityKeys } from "./keys"

interface QueryOptions {
  // Gate the query — typically the caller wires this to
  // `!!currentGroup` so the request doesn't fire before the
  // GroupProvider has populated the http client's slug slot. Defaults
  // to `true` for non-group-aware callers.
  enabled?: boolean
  // Pagination. The dashboard requests `perPage=100` (the BE max) so
  // its "recently added" slice has a chance of including new items
  // for groups bigger than the default page; the items list (#1410)
  // will pass paginated cursors. Defaults to the BE default (50).
  perPage?: number
}

// Fetches commodities for the active group.
//
// **Pagination caveat for the dashboard.** The current /commodities
// endpoint orders by name/id, not by registered_date — so even at
// `perPage=100`, a group with 100+ commodities can have a recent
// addition fall outside page 1, in which case the dashboard's
// "Recently added" card would miss it. A dedicated server-side
// `?sort=-registered_date&limit=N` (or a `/dashboard` aggregate
// endpoint) is the correct long-term fix; #1408's AC explicitly
// flags that today's number is "approximate". Tracked under #1410
// follow-ups.
export function useCommodities({ enabled = true, perPage }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<{ commodities: Commodity[]; total: number }>({
    queryKey: commodityKeys.list(slug),
    queryFn: ({ signal }) => listCommodities({ signal, perPage }),
    enabled,
  })
}

// Fetches the precomputed value totals for the active group's commodities.
export function useCommoditiesValue({ enabled = true }: Omit<QueryOptions, "perPage"> = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<CommoditiesValue>({
    queryKey: commodityKeys.values(slug),
    queryFn: ({ signal }) => getCommoditiesValue(signal),
    enabled,
  })
}
