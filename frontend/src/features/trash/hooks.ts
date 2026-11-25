import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useAuth } from "@/features/auth/AuthContext"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useMembers } from "@/features/group/hooks"
import { storageKeys } from "@/features/storage/keys"

import {
  emptyTrash,
  listTrashedCommodities,
  listTrashedFiles,
  purgeCommodities,
  purgeCommodity,
  purgeFile,
  purgeFiles,
  restoreCommodities,
  restoreCommodity,
  restoreFile,
  restoreFiles,
  type BulkRestoreResult,
  type BulkTrashResult,
  type CommodityRestoreResult,
  type EmptyTrashResult,
  type TrashListOptions,
  type TrashPage,
  type TrashedCommodity,
  type TrashedFile,
} from "./api"
import { trashKeys } from "./keys"

export function useTrashedCommodities(opts: TrashListOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<TrashPage<TrashedCommodity>>({
    queryKey: trashKeys.commodities(slug, opts),
    queryFn: ({ signal }) => listTrashedCommodities(opts, signal),
    enabled: !!slug,
    placeholderData: (prev) => prev,
  })
}

export function useTrashedFiles(opts: TrashListOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<TrashPage<TrashedFile>>({
    queryKey: trashKeys.files(slug, opts),
    queryFn: ({ signal }) => listTrashedFiles(opts, signal),
    enabled: !!slug,
    placeholderData: (prev) => prev,
  })
}

// useInvalidateTrash refreshes the Trash lists and the storage usage, which
// counts trashed bytes in their own bucket. Every delete that moves rows to
// Trash calls it.
export function useInvalidateTrash() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: trashKeys.group(slug) }),
      qc.invalidateQueries({ queryKey: storageKeys.group(slug) }),
    ])
}

// A restore brings back an item with its files, loans, service records and
// schedules, so nearly every list and count of the group can change. Marking
// everything stale is simpler than naming each slice, and only the queries on
// screen refetch.
function useInvalidateAfterRestore() {
  const qc = useQueryClient()
  return () => qc.invalidateQueries()
}

export function useRestoreCommodity() {
  const invalidate = useInvalidateAfterRestore()
  return useMutation<CommodityRestoreResult, Error, string>({
    mutationFn: (id) => restoreCommodity(id),
    onSuccess: () => invalidate(),
  })
}

export function useRestoreCommodities() {
  const invalidate = useInvalidateAfterRestore()
  return useMutation<BulkRestoreResult, Error, string[]>({
    mutationFn: (ids) => restoreCommodities(ids),
    onSuccess: () => invalidate(),
  })
}

export function useRestoreFile() {
  const invalidate = useInvalidateAfterRestore()
  return useMutation<void, Error, string>({
    mutationFn: (id) => restoreFile(id),
    onSuccess: () => invalidate(),
  })
}

export function useRestoreFiles() {
  const invalidate = useInvalidateAfterRestore()
  return useMutation<BulkTrashResult, Error, string[]>({
    mutationFn: (ids) => restoreFiles(ids),
    onSuccess: () => invalidate(),
  })
}

export function usePurgeCommodity() {
  const invalidate = useInvalidateTrash()
  return useMutation<void, Error, string>({
    mutationFn: (id) => purgeCommodity(id),
    onSuccess: () => invalidate(),
  })
}

export function usePurgeCommodities() {
  const invalidate = useInvalidateTrash()
  return useMutation<BulkTrashResult, Error, string[]>({
    mutationFn: (ids) => purgeCommodities(ids),
    onSuccess: () => invalidate(),
  })
}

export function usePurgeFile() {
  const invalidate = useInvalidateTrash()
  return useMutation<void, Error, string>({
    mutationFn: (id) => purgeFile(id),
    onSuccess: () => invalidate(),
  })
}

export function usePurgeFiles() {
  const invalidate = useInvalidateTrash()
  return useMutation<BulkTrashResult, Error, string[]>({
    mutationFn: (ids) => purgeFiles(ids),
    onSuccess: () => invalidate(),
  })
}

export function useEmptyTrash() {
  const invalidate = useInvalidateTrash()
  return useMutation<EmptyTrashResult, Error, void>({
    mutationFn: () => emptyTrash(),
    onSuccess: () => invalidate(),
  })
}

export interface TrashPermissions {
  // Restore needs the user role or higher.
  canRestore: boolean
  // Deleting for good and emptying the Trash need admin or owner.
  canPurge: boolean
}

// useTrashPermissions mirrors the server's role gates so the page offers only
// the actions the member may take. The server enforces them either way.
export function useTrashPermissions(): TrashPermissions {
  const { user } = useAuth()
  const { currentGroup } = useCurrentGroup()
  const members = useMembers(currentGroup?.id)
  const role = members.data?.find((m) => m.member_user_id === user?.id)?.role
  return {
    canRestore: role === "user" || role === "admin" || role === "owner",
    canPurge: role === "admin" || role === "owner",
  }
}
