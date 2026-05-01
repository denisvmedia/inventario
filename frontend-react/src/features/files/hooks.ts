import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import {
  bulkDeleteFiles,
  deleteFile,
  getCategoryCounts,
  getFile,
  listFiles,
  updateFile,
  uploadFile,
  type BulkDeleteResult,
  type FileCategoryCounts,
  type FileEntity,
  type ListFilesOptions,
  type ListedFile,
  type UpdateFileRequest,
  type UploadResult,
  type URLData,
} from "./api"
import { fileKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

export function useFiles(opts: ListFilesOptions = {}, query: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const enabled = query.enabled ?? true
  return useQuery<{ files: ListedFile[]; total: number }>({
    queryKey: fileKeys.list(slug, opts),
    queryFn: ({ signal }) => listFiles({ ...opts, signal }),
    enabled,
    placeholderData: (prev) => prev,
  })
}

export function useFileCategoryCounts(
  opts: Omit<ListFilesOptions, "category" | "page" | "perPage"> = {},
  { enabled = true }: QueryOptions = {}
) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<FileCategoryCounts>({
    queryKey: fileKeys.categoryCounts(slug, opts),
    queryFn: ({ signal }) => getCategoryCounts({ ...opts, signal }),
    enabled,
    placeholderData: (prev) => prev,
  })
}

export function useFile(id: string | undefined, { enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<{ file: FileEntity & { id: string }; signedUrl?: URLData }>({
    queryKey: fileKeys.detail(slug, id ?? ""),
    queryFn: ({ signal }) => {
      if (!id) throw new Error("useFile called without an id")
      return getFile(id, signal)
    },
    enabled: enabled && !!id,
  })
}

// invalidateAll wipes the entire files namespace for the active group.
// Used after any mutation since list queries can be sliced by category +
// search + tags + type combinations and a focused invalidation would
// miss the cached permutations the user can switch back to.
function useInvalidate() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return {
    all: () => qc.invalidateQueries({ queryKey: fileKeys.group(slug) }),
    detail: (id: string) => qc.invalidateQueries({ queryKey: fileKeys.detail(slug, id) }),
  }
}

export function useUpdateFile(id: string) {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const detailKey = fileKeys.detail(slug, id)
  return useMutation<FileEntity & { id: string }, Error, UpdateFileRequest>({
    mutationFn: (req) => updateFile(id, req),
    onSuccess: (file) => {
      const cached = qc.getQueryData<{ file: FileEntity & { id: string }; signedUrl?: URLData }>(detailKey)
      qc.setQueryData(detailKey, {
        file: { ...file, id },
        signedUrl: cached?.signedUrl,
      })
      qc.invalidateQueries({ queryKey: fileKeys.group(slug) })
    },
  })
}

export function useDeleteFile() {
  const invalidate = useInvalidate()
  return useMutation<void, Error, string>({
    mutationFn: (id) => deleteFile(id),
    onSuccess: () => invalidate.all(),
  })
}

export function useBulkDeleteFiles() {
  const invalidate = useInvalidate()
  return useMutation<BulkDeleteResult, Error, string[]>({
    mutationFn: (ids) => bulkDeleteFiles(ids),
    onSuccess: () => invalidate.all(),
  })
}

export function useUploadFile() {
  const invalidate = useInvalidate()
  return useMutation<UploadResult, Error, File>({
    mutationFn: (f) => uploadFile(f),
    onSuccess: () => invalidate.all(),
  })
}
