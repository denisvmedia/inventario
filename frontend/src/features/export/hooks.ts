import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import {
  type CreateExportRequest,
  type CreateRestoreRequest,
  type Export,
  type ImportBackupRequest,
  type ListExportsOptions,
  type Restore,
  type UploadRestoreFileResult,
  createExport,
  createRestore,
  deleteExport,
  getExport,
  getRestore,
  importBackup,
  isExportTerminal,
  isRestoreTerminal,
  listExports,
  listRestores,
  uploadRestoreFile,
} from "./api"
import { exportKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

// Polling cadence for in-progress entities. Picked at 2s — short enough
// that the wizard's "creating…" step feels live, long enough that an
// idle list page stays cheap. TanStack Query backs off when the tab is
// hidden so this is safe to leave on every list mount.
const POLL_INTERVAL_MS = 2000

export function useExports(opts: ListExportsOptions = {}, query: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const enabled = query.enabled ?? true
  return useQuery<{ exports: Export[]; total: number }>({
    queryKey: exportKeys.list(slug, opts),
    queryFn: ({ signal }) => listExports({ ...opts, signal }),
    enabled,
    placeholderData: (prev) => prev,
    refetchInterval: (query) => {
      // Poll while ANY row is non-terminal so the user sees status flips
      // without a manual refresh. Once all rows are completed/failed the
      // interval drops to false (no polling) and we rely on mutations
      // to invalidate.
      const data = query.state.data
      if (!data) return false
      const anyInFlight = data.exports.some((e) => !isExportTerminal(e.status))
      return anyInFlight ? POLL_INTERVAL_MS : false
    },
  })
}

export function useExport(id: string | undefined, { enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<Export>({
    queryKey: exportKeys.detail(slug, id ?? ""),
    queryFn: ({ signal }) => {
      if (!id) throw new Error("useExport called without an id")
      return getExport(id, signal)
    },
    enabled: enabled && !!id,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return false
      return isExportTerminal(data.status) ? false : POLL_INTERVAL_MS
    },
  })
}

export function useExportRestores(
  exportId: string | undefined,
  { enabled = true }: QueryOptions = {}
) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<{ restores: Restore[] }>({
    queryKey: exportKeys.restoreList(slug, exportId ?? ""),
    queryFn: ({ signal }) => {
      if (!exportId) throw new Error("useExportRestores called without an exportId")
      return listRestores(exportId, signal)
    },
    enabled: enabled && !!exportId,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return false
      const anyInFlight = data.restores.some((r) => !isRestoreTerminal(r.status))
      return anyInFlight ? POLL_INTERVAL_MS : false
    },
  })
}

export function useRestore(
  exportId: string | undefined,
  restoreId: string | undefined,
  { enabled = true }: QueryOptions = {}
) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<Restore>({
    queryKey: exportKeys.restoreDetail(slug, exportId ?? "", restoreId ?? ""),
    queryFn: ({ signal }) => {
      if (!exportId || !restoreId) {
        throw new Error("useRestore called without exportId/restoreId")
      }
      return getRestore(exportId, restoreId, signal)
    },
    enabled: enabled && !!exportId && !!restoreId,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return false
      return isRestoreTerminal(data.status) ? false : POLL_INTERVAL_MS
    },
  })
}

// invalidateAll wipes the entire exports namespace for the active group.
// Used after every mutation because list and detail queries hold derived
// state (counts, status, restore lists) that any mutation can shift.
function useInvalidate() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return {
    all: () => qc.invalidateQueries({ queryKey: exportKeys.group(slug) }),
    detail: (id: string) => qc.invalidateQueries({ queryKey: exportKeys.detail(slug, id) }),
    restores: (exportId: string) =>
      qc.invalidateQueries({ queryKey: exportKeys.restoreList(slug, exportId) }),
  }
}

export function useCreateExport() {
  const invalidate = useInvalidate()
  return useMutation<Export, Error, CreateExportRequest>({
    mutationFn: (req) => createExport(req),
    onSuccess: () => invalidate.all(),
  })
}

export function useDeleteExport() {
  const invalidate = useInvalidate()
  return useMutation<void, Error, string>({
    mutationFn: (id) => deleteExport(id),
    onSuccess: () => invalidate.all(),
  })
}

export function useUploadRestoreFile() {
  // No invalidation — the upload alone does not create or modify any
  // exports/restores rows; the follow-up importBackup() does that.
  return useMutation<UploadRestoreFileResult, Error, File>({
    mutationFn: (file) => uploadRestoreFile(file),
  })
}

export function useImportBackup() {
  const invalidate = useInvalidate()
  return useMutation<Export, Error, ImportBackupRequest>({
    mutationFn: (req) => importBackup(req),
    onSuccess: () => invalidate.all(),
  })
}

interface CreateRestoreVars {
  exportId: string
  req: CreateRestoreRequest
}

export function useCreateRestore() {
  const invalidate = useInvalidate()
  return useMutation<Restore, Error, CreateRestoreVars>({
    mutationFn: ({ exportId, req }) => createRestore(exportId, req),
    onSuccess: (_data, { exportId }) => {
      invalidate.detail(exportId)
      invalidate.restores(exportId)
    },
  })
}
