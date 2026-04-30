import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import {
  createArea,
  deleteArea,
  getArea,
  listAreas,
  updateArea,
  type Area,
  type CreateAreaRequest,
  type UpdateAreaRequest,
} from "./api"
import { areaKeys } from "./keys"

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
// `useUpdateLocation` — patch the list snapshot, roll back on
// error, settle by invalidating both list and detail.
export function useUpdateArea(id: string) {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const listKey = areaKeys.list(slug)
  const detailKey = areaKeys.detail(slug, id)
  return useMutation<Area, Error, UpdateAreaRequest, { previousList?: Area[] }>({
    mutationFn: (req) => updateArea(id, req),
    onMutate: async (req) => {
      await qc.cancelQueries({ queryKey: listKey })
      const previousList = qc.getQueryData<Area[]>(listKey)
      if (previousList) {
        qc.setQueryData<Area[]>(
          listKey,
          previousList.map((a) => (a.id === id ? { ...a, ...req } : a))
        )
      }
      return { previousList }
    },
    onError: (_err, _req, ctx) => {
      if (ctx?.previousList) qc.setQueryData(listKey, ctx.previousList)
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
  return useMutation<void, Error, string, { previousList?: Area[] }>({
    mutationFn: (id) => deleteArea(id),
    onMutate: async (id) => {
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
    onError: (_err, _id, ctx) => {
      if (ctx?.previousList) qc.setQueryData(listKey, ctx.previousList)
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: listKey })
    },
  })
}
