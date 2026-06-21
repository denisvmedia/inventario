import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import { commodityKeys } from "@/features/commodities/keys"

import {
  createArea,
  deleteArea,
  getArea,
  listAreas,
  updateArea,
  type Area,
  type CreateAreaRequest,
  type DeleteStrategy,
  type UpdateAreaRequest,
} from "./api"
import { areaKeys } from "./keys"

// Variables for the delete mutation. `strategy` is omitted for an empty
// area (BE safe default) and set when the user picks one in the
// non-empty delete dialog (#2137).
export interface DeleteAreaVars {
  id: string
  strategy?: DeleteStrategy
}

interface QueryOptions {
  enabled?: boolean
}

export function useAreas({ enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<Area[]>({
    queryKey: areaKeys.list(slug),
    queryFn: ({ signal }) => listAreas({ signal }),
    enabled,
  })
}

export function useArea(id: string | undefined, { enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<Area>({
    queryKey: areaKeys.detail(slug, id ?? ""),
    queryFn: ({ signal }) => {
      if (!id) throw new Error("useArea called without an id")
      return getArea(id, signal)
    },
    enabled: enabled && !!id,
  })
}

export function useCreateArea() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useMutation<Area, Error, CreateAreaRequest>({
    mutationFn: (req) => createArea(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: areaKeys.list(slug) })
    },
  })
}

// Optimistic rename / re-parent. Same shape as
// `useUpdateLocation` — patch both list AND detail snapshots so a
// rename from the area detail page updates the heading immediately
// rather than waiting for onSettled to refetch. Roll back both on
// error, settle by invalidating both queries.
export function useUpdateArea(id: string) {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const listKey = areaKeys.list(slug)
  const detailKey = areaKeys.detail(slug, id)
  return useMutation<
    Area,
    Error,
    UpdateAreaRequest,
    { previousList?: Area[]; previousDetail?: Area }
  >({
    mutationFn: (req) => updateArea(id, req),
    onMutate: async (req) => {
      await qc.cancelQueries({ queryKey: listKey })
      await qc.cancelQueries({ queryKey: detailKey })
      const previousList = qc.getQueryData<Area[]>(listKey)
      const previousDetail = qc.getQueryData<Area>(detailKey)
      if (previousList) {
        qc.setQueryData<Area[]>(
          listKey,
          previousList.map((a) => (a.id === id ? { ...a, ...req } : a))
        )
      }
      if (previousDetail) {
        qc.setQueryData<Area>(detailKey, { ...previousDetail, ...req })
      }
      return { previousList, previousDetail }
    },
    onError: (_err, _req, ctx) => {
      if (ctx?.previousList) qc.setQueryData(listKey, ctx.previousList)
      if (ctx?.previousDetail) qc.setQueryData(detailKey, ctx.previousDetail)
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: listKey })
      qc.invalidateQueries({ queryKey: detailKey })
    },
  })
}

export function useDeleteArea() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const listKey = areaKeys.list(slug)
  return useMutation<void, Error, DeleteAreaVars, { previousList?: Area[] }>({
    mutationFn: ({ id, strategy }) => deleteArea(id, strategy),
    onMutate: async ({ id }) => {
      await qc.cancelQueries({ queryKey: listKey })
      const previousList = qc.getQueryData<Area[]>(listKey)
      if (previousList) {
        qc.setQueryData<Area[]>(
          listKey,
          previousList.filter((a) => a.id !== id)
        )
      }
      return { previousList }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.previousList) qc.setQueryData(listKey, ctx.previousList)
    },
    onSettled: (_data, _err, vars) => {
      qc.invalidateQueries({ queryKey: listKey })
      // Unlink frees the area's items (they become un-located), so any
      // cached commodity list for this group is now stale. Cascade also
      // removes the items, so refresh in both non-default-strategy
      // branches — the BE only changes commodities when a strategy is
      // given (the empty-area default never touches items). #2137
      if (vars.strategy) {
        qc.invalidateQueries({ queryKey: commodityKeys.group(slug) })
      }
    },
  })
}
