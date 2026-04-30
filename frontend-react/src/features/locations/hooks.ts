import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import {
  createLocation,
  deleteLocation,
  getLocation,
  listLocations,
  updateLocation,
  type CreateLocationRequest,
  type Location,
  type UpdateLocationRequest,
} from "./api"
import { locationKeys } from "./keys"

interface QueryOptions {
  // Gate the query — typically `!!currentGroup`. The locations
  // endpoint is /g/{slug}/locations after the http rewrite, so firing
  // before the GroupProvider has populated the slug slot 404s.
  enabled?: boolean
}

export function useLocations({ enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<Location[]>({
    queryKey: locationKeys.list(slug),
    queryFn: ({ signal }) => listLocations({ signal }),
    enabled,
  })
}

export function useLocation(id: string | undefined, { enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<Location>({
    queryKey: locationKeys.detail(slug, id ?? ""),
    queryFn: ({ signal }) => {
      if (!id) throw new Error("useLocation called without an id")
      return getLocation(id, signal)
    },
    enabled: enabled && !!id,
  })
}

// Common refetch on success: invalidate the list (so a fresh
// list-without-the-deleted-row / list-with-the-new-row arrives) plus
// the affected detail key. We do NOT refetch the detail when it's the
// row we just removed — TanStack will discard the cached error itself
// once the matching component unmounts.
function useInvalidate() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return {
    list: () => qc.invalidateQueries({ queryKey: locationKeys.list(slug) }),
    detail: (id: string) => qc.invalidateQueries({ queryKey: locationKeys.detail(slug, id) }),
  }
}

export function useCreateLocation() {
  const invalidate = useInvalidate()
  return useMutation<Location, Error, CreateLocationRequest>({
    mutationFn: (req) => createLocation(req),
    onSuccess: () => {
      invalidate.list()
    },
  })
}

// Optimistic rename: the list query gets patched in-place so the new
// name shows immediately. On failure we restore the snapshot taken
// before the mutation fired.
export function useUpdateLocation(id: string) {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const listKey = locationKeys.list(slug)
  const detailKey = locationKeys.detail(slug, id)
  return useMutation<Location, Error, UpdateLocationRequest, { previousList?: Location[] }>({
    mutationFn: (req) => updateLocation(id, req),
    onMutate: async (req) => {
      await qc.cancelQueries({ queryKey: listKey })
      const previousList = qc.getQueryData<Location[]>(listKey)
      if (previousList) {
        qc.setQueryData<Location[]>(
          listKey,
          previousList.map((l) => (l.id === id ? { ...l, ...req } : l))
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

// Optimistic delete: the list query removes the row immediately so
// the UI doesn't lag the action. On failure the snapshot is restored
// (and the user sees the row reappear, which is the "rollback" the
// AC asks for).
export function useDeleteLocation() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const listKey = locationKeys.list(slug)
  return useMutation<void, Error, string, { previousList?: Location[] }>({
    mutationFn: (id) => deleteLocation(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: listKey })
      const previousList = qc.getQueryData<Location[]>(listKey)
      if (previousList) {
        qc.setQueryData<Location[]>(
          listKey,
          previousList.filter((l) => l.id !== id)
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
