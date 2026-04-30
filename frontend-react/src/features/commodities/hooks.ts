import { useQuery } from "@tanstack/react-query"

import {
  getCommoditiesValue,
  listCommodities,
  type Commodity,
  type CommoditiesValue,
} from "./api"
import { commodityKeys } from "./keys"

interface QueryOptions {
  // Gate the query — typically the caller wires this to
  // `!!currentGroup` so the request doesn't fire before the
  // GroupProvider has populated the http client's slug slot. Defaults
  // to `true` for non-group-aware callers.
  enabled?: boolean
}

// Fetches commodities for the active group. The dashboard pulls the full
// list (one page, default 50) to compute its aggregates client-side; the
// items page (#1410) will paginate via separate query keys.
export function useCommodities({ enabled = true }: QueryOptions = {}) {
  return useQuery<{ commodities: Commodity[]; total: number }>({
    queryKey: commodityKeys.list(),
    queryFn: ({ signal }) => listCommodities({ signal }),
    enabled,
  })
}

// Fetches the precomputed value totals for the active group's commodities.
export function useCommoditiesValue({ enabled = true }: QueryOptions = {}) {
  return useQuery<CommoditiesValue>({
    queryKey: commodityKeys.values(),
    queryFn: ({ signal }) => getCommoditiesValue(signal),
    enabled,
  })
}
