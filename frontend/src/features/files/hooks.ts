import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { commodityKeys } from "@/features/commodities/keys"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { tagKeys } from "@/features/tags/keys"

import {
  attachFile,
  bulkDeleteFiles,
  bulkReclassifyFiles,
  deleteFile,
  detachFile,
  getCategoryCounts,
  getFile,
  listFileDuplicates,
  listFiles,
  updateFile,
  uploadFile,
  type AttachFileRequest,
  type BulkDeleteResult,
  type FileCategory,
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
    // Note: we deliberately do NOT gate this on `!!slug`. The http
    // client's group rewrite uses the URL slug, not the GroupProvider
    // state, so a fetch that fires before the provider resolves still
    // hits the right endpoint. The brief empty-slug cache entry the
    // first render creates is overwritten by the next render's keyed
    // entry once `slug` populates — same pattern useFiles uses, and
    // gating it broke the FileEditPage form-reset path because the
    // GroupProvider/test timing in JSDOM races the polling waitFor.
    enabled: enabled && !!id,
  })
}

// invalidateAll wipes the entire files namespace for the active group.
// Used after any mutation since list queries can be sliced by category +
// search + tags + type combinations and a focused invalidation would
// miss the cached permutations the user can switch back to. File writes
// can auto-create file tags and change their usage counts, so the group's
// tag queries are refreshed with them.
function useInvalidate() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return {
    all: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: fileKeys.group(slug) }),
        qc.invalidateQueries({ queryKey: tagKeys.group(slug) }),
      ]),
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
      const cached = qc.getQueryData<{ file: FileEntity & { id: string }; signedUrl?: URLData }>(
        detailKey
      )
      qc.setQueryData(detailKey, {
        file: { ...file, id },
        signedUrl: cached?.signedUrl,
      })
      qc.invalidateQueries({ queryKey: fileKeys.group(slug) })
      qc.invalidateQueries({ queryKey: tagKeys.group(slug) })
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

interface BulkReclassifyVars {
  ids: string[]
  category: FileCategory
}

export function useBulkReclassifyFiles() {
  const invalidate = useInvalidate()
  return useMutation<BulkDeleteResult, Error, BulkReclassifyVars>({
    mutationFn: ({ ids, category }) => bulkReclassifyFiles(ids, category),
    onSuccess: () => invalidate.all(),
  })
}

// Per-file upload mutation. Skips the cache-invalidation onSuccess hook
// the other mutations have because the upload dialog runs many uploads
// in a loop and a per-file invalidation would refetch list + counts on
// every iteration. The dialog calls `useInvalidateFiles().all()` once
// after the batch instead.
export function useUploadFile() {
  return useMutation<UploadResult, Error, File>({
    mutationFn: (f) => uploadFile(f),
  })
}

// Public hook so non-mutation callers (like the upload dialog batch)
// can fire a single invalidation when the batch finishes.
export function useInvalidateFiles() {
  return useInvalidate()
}

// Files of the group with the same content as fileId. The upload dialog asks
// right after an upload; the result never changes for a stored file, so it is
// not refetched on focus.
export function useFileDuplicates(
  fileId: string | undefined,
  { enabled = true }: QueryOptions = {}
) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<ListedFile[]>({
    queryKey: fileKeys.duplicates(slug, fileId ?? ""),
    queryFn: ({ signal }) => {
      if (!fileId) throw new Error("useFileDuplicates called without an id")
      return listFileDuplicates(fileId, signal)
    },
    enabled: enabled && !!fileId,
    staleTime: Infinity,
  })
}

// Attachments change what an entity's Files tab lists and which photo an item
// shows as its cover, so both the files and the commodities caches refresh.
function useInvalidateAttachments() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: fileKeys.group(slug) }),
      qc.invalidateQueries({ queryKey: commodityKeys.group(slug) }),
    ])
}

interface AttachFileVars extends AttachFileRequest {
  fileId: string
}

export function useAttachFile() {
  const invalidate = useInvalidateAttachments()
  return useMutation<FileEntity & { id: string }, Error, AttachFileVars>({
    mutationFn: ({ fileId, ...req }) => attachFile(fileId, req),
    onSuccess: () => invalidate(),
  })
}

interface DetachFileVars {
  fileId: string
  linkId: string
}

export function useDetachFile() {
  const invalidate = useInvalidateAttachments()
  return useMutation<FileEntity & { id: string }, Error, DetachFileVars>({
    mutationFn: ({ fileId, linkId }) => detachFile(fileId, linkId),
    onSuccess: () => invalidate(),
  })
}
