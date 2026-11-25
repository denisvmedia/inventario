import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useMemo } from "react"

import { useCurrentGroup } from "@/features/group/GroupContext"
import { useSellers } from "@/features/sellers/hooks"

import {
  countSavedViews,
  createSavedView,
  deleteSavedView,
  listSavedViews,
  reorderSavedViews,
  updateSavedView,
  type SavedView,
  type SavedViewInput,
  type SavedViewList,
  type UpdateSavedViewRequest,
} from "./api"
import { savedViewKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

function useGroupSlug(): string {
  const { currentGroup } = useCurrentGroup()
  return currentGroup?.slug ?? ""
}

// The saved views the member sees in the current group, in display order:
// their own and every shared one.
export function useSavedViews({ enabled = true }: QueryOptions = {}) {
  const slug = useGroupSlug()
  return useQuery<SavedViewList>({
    queryKey: savedViewKeys.list(slug),
    queryFn: ({ signal }) => listSavedViews(signal),
    enabled: enabled && slug !== "",
  })
}

// Item counts per view ID, for the sidebar and the dashboard.
export function useSavedViewCounts({ enabled = true }: QueryOptions = {}) {
  const slug = useGroupSlug()
  return useQuery<Record<string, number>>({
    queryKey: savedViewKeys.counts(slug),
    queryFn: ({ signal }) => countSavedViews(signal),
    enabled: enabled && slug !== "",
    placeholderData: (prev) => prev,
  })
}

function useInvalidateSavedViews() {
  const qc = useQueryClient()
  const slug = useGroupSlug()
  return () => qc.invalidateQueries({ queryKey: savedViewKeys.all(slug) })
}

export function useCreateSavedView() {
  const invalidate = useInvalidateSavedViews()
  return useMutation<SavedView, Error, SavedViewInput>({
    mutationFn: createSavedView,
    onSuccess: () => invalidate(),
  })
}

export function useUpdateSavedView() {
  const invalidate = useInvalidateSavedViews()
  return useMutation<SavedView, Error, UpdateSavedViewRequest>({
    mutationFn: updateSavedView,
    onSuccess: () => invalidate(),
  })
}

export function useDeleteSavedView() {
  const invalidate = useInvalidateSavedViews()
  return useMutation<void, Error, string>({
    mutationFn: deleteSavedView,
    onSuccess: () => invalidate(),
  })
}

export function useReorderSavedViews() {
  const invalidate = useInvalidateSavedViews()
  return useMutation<void, Error, string[]>({
    mutationFn: reorderSavedViews,
    onSuccess: () => invalidate(),
  })
}

export interface SellerCatalog {
  // Seller names by ID, for the seller conditions.
  names: ReadonlyMap<string, string>
  // The group's seller IDs once the sellers have loaded for this group;
  // undefined while they load, after a failed load, and while the previous
  // group's list stands in. A list that is not this group's own would mark
  // the view's sellers as deleted.
  ids?: ReadonlySet<string>
}

export function useSellerCatalog(): SellerCatalog {
  const sellers = useSellers()
  const known = sellers.isSuccess && !sellers.isPlaceholderData
  return useMemo(() => {
    const names = new Map((sellers.data ?? []).map((s) => [s.id, s.name ?? ""]))
    return { names, ids: known ? new Set(names.keys()) : undefined }
  }, [sellers.data, known])
}
