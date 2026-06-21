import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"
import { areaKeys } from "@/features/areas/keys"
import { commodityKeys } from "@/features/commodities/keys"

import {
  createLocation,
  deleteLocation,
  getLocation,
  listLocations,
  updateLocation,
  type CreateLocationRequest,
  type DeleteStrategy,
  type Location,
  type UpdateLocationRequest,
} from "./api"
import { locationKeys } from "./keys"

// Variables for the delete mutation. `strategy` is omitted for an empty
// location (BE safe default) and set when the user picks one in the
// non-empty delete dialog (#2137).
export interface DeleteLocationVars {
  id: string
  strategy?: DeleteStrategy
}

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

// Optimistic rename: both the list and the detail query get patched
// in-place so the new name shows immediately wherever the location is
// rendered. On failure we restore the snapshots taken before the
// mutation fired. (The detail patch matters when the user edits from
// LocationDetailPage — without it the heading wouldn't update until
// onSettled refetched.)
export function useUpdateLocation(id: string) {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const listKey = locationKeys.list(slug)
  const detailKey = locationKeys.detail(slug, id)
  return useMutation<
    Location,
    Error,
    UpdateLocationRequest,
    { previousList?: Location[]; previousDetail?: Location }
  >({
    mutationFn: (req) => updateLocation(id, req),
    onMutate: async (req) => {
      await qc.cancelQueries({ queryKey: listKey })
      await qc.cancelQueries({ queryKey: detailKey })
      const previousList = qc.getQueryData<Location[]>(listKey)
      const previousDetail = qc.getQueryData<Location>(detailKey)
      if (previousList) {
        qc.setQueryData<Location[]>(
          listKey,
          previousList.map((l) => (l.id === id ? { ...l, ...req } : l))
        )
      }
      if (previousDetail) {
        qc.setQueryData<Location>(detailKey, { ...previousDetail, ...req })
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

// Optimistic delete: the list query removes the row immediately so
// the UI doesn't lag the action. On failure the snapshot is restored
// (and the user sees the row reappear, which is the "rollback" the
// AC asks for).
export function useDeleteLocation() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const listKey = locationKeys.list(slug)
  return useMutation<void, Error, DeleteLocationVars, { previousList?: Location[] }>({
    mutationFn: ({ id, strategy }) => deleteLocation(id, strategy),
    onMutate: async ({ id }) => {
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
    onError: (_err, _vars, ctx) => {
      if (ctx?.previousList) qc.setQueryData(listKey, ctx.previousList)
    },
    onSettled: (_data, _err, vars) => {
      qc.invalidateQueries({ queryKey: listKey })
      // A strategy-delete touches the location's areas (all removed) and
      // its items (cascade ⇒ deleted, unlink ⇒ un-located). Refresh the
      // group's areas list + commodity caches so the freed items and the
      // emptied areas drop out of every view. The empty-location default
      // (no strategy) never touches either, so skip the extra work. #2137
      if (vars.strategy) {
        qc.invalidateQueries({ queryKey: areaKeys.list(slug) })
        qc.invalidateQueries({ queryKey: commodityKeys.group(slug) })
      }
    },
  })
}
