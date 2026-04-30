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
  updateCommodity,
  type CommoditiesValue,
  type Commodity,
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
  return useQuery<{ commodities: Commodity[]; total: number }>({
    queryKey: commodityKeys.list(slug, opts),
    queryFn: ({ signal }) => listCommodities({ ...opts, signal }),
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
