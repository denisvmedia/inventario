// Pure data-layer functions for the files feature slice. Hooks live in
// `./hooks.ts`. Backed by the unified `/files` surface introduced under
// #1398 (category enum) + #1399 (legacy backfill); the FE consumes a
// single endpoint regardless of whether a row originated as an image,
// invoice, or manual on the legacy split-table side.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type FileEntity = Schema<"models.FileEntity">
export type FileCategory = Schema<"models.FileCategory">
export type FileType = Schema<"models.FileType">
export type FileCategoryCounts = Schema<"jsonapi.FileCategoryCounts">

// Signed-url payload returned alongside list/detail responses. Keys are
// file IDs; values carry a primary URL plus an optional thumbnail map
// keyed by size name (`small` / `medium` / `large` per
// services/file_signing_service.go). The BE
// (apiserver/files.go::generateSignedURLsForFiles) best-effort populates
// this for every file the caller can see.
export interface URLData {
  url: string
  thumbnails?: Record<string, string>
}

// The list endpoint (apiserver/files.go::listFiles → jsonapi.FilesResponse)
// returns FileEntity records FLAT inside `data` — NOT wrapped in the
// `{id, type, attributes}` envelope the singular detail endpoint uses.
// Mirrors the legacy split between FilesResponse and FileResponse types
// in go/jsonapi/files.go; this comment exists so a future maintainer
// doesn't try to "normalise" the shapes back into one.
interface FilesListEnvelope {
  data: Array<FileEntity & { id: string }>
  meta?: {
    files?: number
    total?: number
    signed_urls?: Record<string, URLData>
  }
}

// jsonapi.FileResponse renders FLAT at the top level — no `data` wrapper.
// (See go/jsonapi/files.go::FileResponse.) This is intentional in the
// existing apiserver and matches how detail / create / update / upload
// all return file payloads. The legacy commodities feature DOES use a
// `data: {...}` wrapper, so don't copy that shape here.
interface FileDetailEnvelope {
  id?: string
  type?: string
  attributes?: FileEntity
  meta?: { signed_urls?: Record<string, URLData> }
}

interface CategoryCountsEnvelope {
  data: FileCategoryCounts
}

// What the list endpoint accepts. The BE (#1398) rejects multi-value
// `category` with 400; the FE side never builds a multi-value request,
// so we expose it as a single optional string.
//
// linkedEntityType + linkedEntityId narrow the result to files attached
// to a specific commodity / location / export. The BE rejects partial
// pairs with 400 — both must be supplied together. Used by the
// EntityFilesPanel on commodity / location detail pages.
export interface ListFilesOptions {
  page?: number
  perPage?: number
  category?: FileCategory
  type?: FileType
  search?: string
  tags?: string[]
  linkedEntityType?: string
  linkedEntityId?: string
  signal?: AbortSignal
}

// Listed file rows carry their signed URL + thumbnails inline so the
// list can render previews without an N+1 fetch loop.
export interface ListedFile {
  file: FileEntity & { id: string }
  signedUrl?: URLData
}

export async function listFiles(
  options: ListFilesOptions = {}
): Promise<{ files: ListedFile[]; total: number }> {
  const params = new URLSearchParams()
  if (options.page !== undefined) params.set("page", String(options.page))
  if (options.perPage !== undefined) params.set("limit", String(options.perPage))
  if (options.category) params.set("category", options.category)
  if (options.type) params.set("type", options.type)
  if (options.search?.trim()) params.set("search", options.search.trim())
  if (options.tags?.length) params.set("tags", options.tags.join(","))
  if (options.linkedEntityType && options.linkedEntityId) {
    params.set("linked_entity_type", options.linkedEntityType)
    params.set("linked_entity_id", options.linkedEntityId)
  }
  const qs = params.toString()
  const path = qs ? `/files?${qs}` : "/files"
  const body = await http.get<FilesListEnvelope>(path, { signal: options.signal })
  const signed = body.meta?.signed_urls ?? {}
  return {
    files: (body.data ?? []).map((row) => ({
      file: row,
      signedUrl: signed[row.id ?? ""],
    })),
    total: body.meta?.total ?? body.data?.length ?? 0,
  }
}

export async function getCategoryCounts(
  options: Omit<ListFilesOptions, "category" | "page" | "perPage"> = {}
): Promise<FileCategoryCounts> {
  const params = new URLSearchParams()
  if (options.type) params.set("type", options.type)
  if (options.search?.trim()) params.set("search", options.search.trim())
  if (options.tags?.length) params.set("tags", options.tags.join(","))
  const qs = params.toString()
  const path = qs ? `/files/category-counts?${qs}` : "/files/category-counts"
  const body = await http.get<CategoryCountsEnvelope>(path, { signal: options.signal })
  return body.data
}

export async function getFile(
  id: string,
  signal?: AbortSignal
): Promise<{ file: FileEntity & { id: string }; signedUrl?: URLData }> {
  const body = await http.get<FileDetailEnvelope>(`/files/${encodeURIComponent(id)}`, { signal })
  // Be strict about the envelope: a missing `attributes` is a backend
  // regression, not a "render an empty form" condition.
  if (!body.attributes) {
    throw new Error(`Malformed /files/${id} response: missing attributes`)
  }
  const fileId = body.id ?? id
  const signed = body.meta?.signed_urls?.[fileId]
  return { file: { ...body.attributes, id: fileId }, signedUrl: signed }
}

export interface UpdateFileRequest {
  title?: string
  description?: string
  tags?: string[]
  path?: string
  category?: FileCategory
  linked_entity_type?: string
  linked_entity_id?: string
  linked_entity_meta?: string
}

// Update file metadata. The BE re-derives `Type` and `Category` server-
// side when MIME info is available; we still send `Category` so explicit
// re-categorisation works regardless of MIME (a PDF that the user
// classifies as Photos via the picker, say).
export async function updateFile(
  id: string,
  req: UpdateFileRequest
): Promise<FileEntity & { id: string }> {
  const body = await http.put<FileDetailEnvelope>(`/files/${encodeURIComponent(id)}`, {
    data: { id, type: "files", attributes: req },
  })
  if (!body.attributes) {
    throw new Error(`Malformed PUT /files/${id} response: missing attributes`)
  }
  return { ...body.attributes, id: body.id ?? id }
}

export async function deleteFile(id: string): Promise<void> {
  await http.del<void>(`/files/${encodeURIComponent(id)}`)
}

export async function bulkDeleteFiles(ids: string[]): Promise<BulkDeleteResult> {
  const body = await http.post<BulkDeleteEnvelope>(`/files/bulk-delete`, {
    data: { type: "files", attributes: { ids } },
  })
  return {
    succeeded: body.data?.attributes?.succeeded ?? [],
    failed: body.data?.attributes?.failed ?? [],
  }
}

// Bulk re-categorize: for each id, fire a metadata-update setting
// `category`. The BE has no dedicated bulk-move endpoint yet, so we
// fan out individual PUTs and aggregate succeeded/failed in the same
// shape as bulkDeleteFiles. Doing it client-side keeps the change a
// pure FE shipment without an extra BE PR.
export async function bulkReclassifyFiles(
  ids: string[],
  category: FileCategory
): Promise<BulkDeleteResult> {
  const succeeded: string[] = []
  const failed: { id: string; error: string }[] = []
  for (const id of ids) {
    try {
      await updateFile(id, { category })
      succeeded.push(id)
    } catch (err) {
      failed.push({ id, error: err instanceof Error ? err.message : String(err) })
    }
  }
  return { succeeded, failed }
}

interface BulkDeleteEnvelope {
  data?: {
    attributes?: {
      succeeded?: string[]
      failed?: { id: string; error: string }[]
    }
  }
}

export interface BulkDeleteResult {
  succeeded: string[]
  failed: { id: string; error: string }[]
}

// Upload-slot status used to gate the upload UI. The BE returns 429 with
// `retry_after_seconds` when the per-user concurrency cap is hit; the
// caller surfaces that to the user instead of failing silently.
export interface UploadCapacity {
  canStart: boolean
  active: number
  max: number
  retryAfterSeconds?: number
}

interface UploadSlotEnvelope {
  data?: {
    attributes?: {
      operation_name?: string
      active_uploads?: number
      max_uploads?: number
      available_uploads?: number
      can_start_upload?: boolean
      retry_after_seconds?: number
    }
  }
}

export async function checkUploadCapacity(operation = "files-upload"): Promise<UploadCapacity> {
  const body = await http.get<UploadSlotEnvelope>(
    `/upload-slots/check?operation=${encodeURIComponent(operation)}`
  )
  const a = body.data?.attributes ?? {}
  return {
    canStart: a.can_start_upload ?? false,
    active: a.active_uploads ?? 0,
    max: a.max_uploads ?? 0,
    retryAfterSeconds: a.retry_after_seconds,
  }
}

export interface UploadResult {
  file: FileEntity & { id: string }
  signedUrl?: URLData
}

// Standalone (non-linked) upload. The BE handler at
// `/uploads/file` derives `Category` from MIME server-side; the caller
// can override it with a follow-up updateFile() if the user picked a
// different bucket in the upload metadata step.
export async function uploadFile(file: File): Promise<UploadResult> {
  const form = new FormData()
  form.append("file", file)
  const body = await http.post<FileDetailEnvelope>(`/uploads/file`, form)
  // Strict envelope: a missing id or attributes here would otherwise
  // propagate as empty strings into cache keys, navigation URLs, and
  // follow-up updateFile() calls.
  if (!body.id || !body.attributes) {
    throw new Error("Malformed /uploads/file response: missing id or attributes")
  }
  const signed = body.meta?.signed_urls?.[body.id]
  return { file: { ...body.attributes, id: body.id }, signedUrl: signed }
}
