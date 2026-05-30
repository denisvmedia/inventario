import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import {
  bulkDeleteCommodities,
  bulkMoveCommodities,
  createCommodity,
  deleteCommodity,
  getCommoditiesValue,
  getCommodity,
  listCommodities,
  listCommodityEvents,
  setCommodityCover,
  updateCommodity,
  type CommoditiesValue,
  type Commodity,
  type CommodityCover,
  type CommodityEvent,
  type CommodityEventKind,
  type CommodityMeta,
  type CreateCommodityRequest,
  type ListCommoditiesOptions,
  type UpdateCommodityRequest,
} from "./api"
import { commodityKeys } from "./keys"

interface QueryOptions {
  // Gate the query — typically the caller wires this to
  // `!!currentGroup` so the request doesn't fire before the
  // GroupProvider has populated the http client's slug slot. Defaults
  // to `true` for non-group-aware callers.
  enabled?: boolean
}

// Fetches commodities for the active group with full filter/sort/search
// support. The dashboard (#1408) calls this with `{}` for the default
// "active items, name asc" view. The items list page (#1410) wires
// every toolbar control through the same call.
export function useCommodities(opts: ListCommoditiesOptions = {}, query: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const enabled = query.enabled ?? true
  // `covers` is additive (#1370): existing consumers destructure only
  // `commodities` / `total`, so surfacing the per-id cover map alongside
  // them is non-breaking. The insurance report's Location mode reads it
  // to render each item's cover thumbnail.
  return useQuery<{
    commodities: Commodity[]
    total: number
    covers: Record<string, CommodityCover>
  }>({
    queryKey: commodityKeys.list(slug, opts),
    queryFn: ({ signal }) => listCommodities({ ...opts, signal }),
    enabled,
    placeholderData: (prev) => prev,
  })
}

// Page size to request per loop iteration. The list endpoint caps
// `per_page` at 100 (apiserver `parsePagination`: values >100 fall back to
// the default 50), so 100 is the largest honored page and the minimum
// number of round-trips.
const ALL_COMMODITIES_PAGE_SIZE = 100
// Defensive ceiling so a misbehaving endpoint (e.g. one that never returns
// a short page) can't spin forever — 100 pages × 100 rows = 10k items.
const ALL_COMMODITIES_MAX_PAGES = 100

// Fetches EVERY commodity for the active group by paging the list endpoint
// in 100-row batches and accumulating the rows + merging the per-id cover
// map. `useCommodities({ perPage: 1000 })` silently truncates at the BE's
// 100-cap, which would give an insurance report incomplete location lists
// and wrong per-location totals (#1370 review). This hook loops until a
// page returns fewer than a full batch OR the accumulated count reaches the
// reported `total`. Returns the same `{ commodities, total, covers }` shape
// as a single `listCommodities` call so consumers don't special-case it.
export function useAllCommodities(opts: ListCommoditiesOptions = {}, query: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const enabled = query.enabled ?? true
  return useQuery<{
    commodities: Commodity[]
    total: number
    covers: Record<string, CommodityCover>
  }>({
    queryKey: commodityKeys.allList(slug, opts),
    queryFn: async ({ signal }) => {
      const commodities: Commodity[] = []
      const covers: Record<string, CommodityCover> = {}
      let total = 0
      for (let page = 1; page <= ALL_COMMODITIES_MAX_PAGES; page++) {
        const res = await listCommodities({
          ...opts,
          page,
          perPage: ALL_COMMODITIES_PAGE_SIZE,
          signal,
        })
        commodities.push(...res.commodities)
        Object.assign(covers, res.covers)
        total = res.total
        // Stop on a short (final) page or once we've collected everything
        // the server says exists. The length guard alone would loop one
        // extra empty page when total is an exact multiple of the batch.
        if (res.commodities.length < ALL_COMMODITIES_PAGE_SIZE || commodities.length >= total) {
          break
        }
      }
      return { commodities, total, covers }
    },
    enabled,
    placeholderData: (prev) => prev,
  })
}

// Fetches the precomputed value totals for the active group's commodities.
export function useCommoditiesValue({ enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<CommoditiesValue>({
    queryKey: commodityKeys.values(slug),
    queryFn: ({ signal }) => getCommoditiesValue(signal),
    enabled,
  })
}

interface CommodityEventsOptions {
  page?: number
  perPage?: number
  kinds?: CommodityEventKind[]
}

// Fetches the audit timeline for a commodity (issue #1450). Defaults to
// page 1, per_page=50 — large enough to render the timeline without
// infinite-scrolling while staying under the BE's 100-cap.
export function useCommodityEvents(
  id: string | undefined,
  opts: CommodityEventsOptions = {},
  query: QueryOptions = {}
) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  // Gate on currentGroup so the request doesn't fire before the http
  // client's /g/{slug}/ rewrite has been populated — same pattern as
  // useCommodities / useTags / etc. The id check covers callers that
  // pass `undefined` while a sibling query loads.
  const enabled = (query.enabled ?? true) && !!id && !!currentGroup
  return useQuery<{ events: CommodityEvent[]; total: number }>({
    queryKey: commodityKeys.events(slug, id ?? "", {
      page: opts.page,
      perPage: opts.perPage,
      kinds: opts.kinds,
    }),
    queryFn: ({ signal }) => {
      if (!id) throw new Error("useCommodityEvents called without an id")
      return listCommodityEvents(id, { ...opts, signal })
    },
    enabled,
    placeholderData: (prev) => prev,
  })
}

// Fetches a single commodity + its attached file lists.
export function useCommodity(id: string | undefined, { enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<{ commodity: Commodity; meta: CommodityMeta }>({
    queryKey: commodityKeys.detail(slug, id ?? ""),
    queryFn: ({ signal }) => {
      if (!id) throw new Error("useCommodity called without an id")
      return getCommodity(id, signal)
    },
    enabled: enabled && !!id,
  })
}

// invalidateAll wipes the entire commodities namespace for the active
// group. Used after any mutation — list pages can be sliced by a dozen
// different filter keys, so a focused invalidation would miss the cached
// permutations the user might switch back to.
function useInvalidate() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return {
    all: () => qc.invalidateQueries({ queryKey: commodityKeys.group(slug) }),
    detail: (id: string) => qc.invalidateQueries({ queryKey: commodityKeys.detail(slug, id) }),
  }
}

export function useCreateCommodity() {
  const invalidate = useInvalidate()
  return useMutation<Commodity, Error, CreateCommodityRequest>({
    mutationFn: (req) => createCommodity(req),
    onSuccess: () => {
      invalidate.all()
    },
  })
}

export function useUpdateCommodity(id: string) {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const detailKey = commodityKeys.detail(slug, id)
  return useMutation<Commodity, Error, UpdateCommodityRequest>({
    mutationFn: (req) => updateCommodity(id, req),
    onSuccess: (commodity) => {
      // Patch the cached detail query in place so the page rerenders
      // immediately with the new values. The detail GET also returns
      // attached file lists in `meta`; preserve whatever was cached
      // there since the update endpoint's response doesn't repopulate
      // it.
      const cached = qc.getQueryData<{ commodity: Commodity; meta: CommodityMeta }>(detailKey)
      qc.setQueryData(detailKey, {
        commodity: { ...commodity, id },
        meta: cached?.meta ?? {},
      })
      qc.invalidateQueries({ queryKey: commodityKeys.group(slug) })
    },
  })
}

export function useDeleteCommodity() {
  const invalidate = useInvalidate()
  return useMutation<void, Error, string>({
    mutationFn: (id) => deleteCommodity(id),
    onSuccess: () => {
      invalidate.all()
    },
  })
}

// Mutation for setting / clearing the explicit cover-photo override
// (issue #1451 option B). The mutation result patches the cached detail
// query so the hero / cards re-render with the new cover without a
// round-trip; the list cache is invalidated since the resolved cover
// also surfaces under `meta.covers[id]`.
export function useSetCommodityCover(id: string) {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const detailKey = commodityKeys.detail(slug, id)
  return useMutation<{ commodity: Commodity; meta: CommodityMeta }, Error, string | null>({
    mutationFn: (fileId) => setCommodityCover(id, fileId),
    onSuccess: ({ commodity, meta }) => {
      qc.setQueryData(detailKey, { commodity: { ...commodity, id }, meta })
      qc.invalidateQueries({ queryKey: commodityKeys.group(slug) })
    },
  })
}

export function useBulkDeleteCommodities() {
  const invalidate = useInvalidate()
  return useMutation<void, Error, string[]>({
    mutationFn: (ids) => bulkDeleteCommodities(ids),
    onSuccess: () => {
      invalidate.all()
    },
  })
}

interface BulkMoveVars {
  ids: string[]
  areaId: string
}

export function useBulkMoveCommodities() {
  const invalidate = useInvalidate()
  return useMutation<void, Error, BulkMoveVars>({
    mutationFn: ({ ids, areaId }) => bulkMoveCommodities(ids, areaId),
    onSuccess: () => {
      invalidate.all()
    },
  })
}
