// Pure data-layer functions for the files feature slice. Hooks live in
// `./hooks.ts`. Backed by the unified `/files` surface introduced under
// #1398 (category enum) + #1399 (legacy backfill); the FE consumes a
// single endpoint regardless of whether a row originated as an image,
// invoice, or manual on the legacy split-table side.
import { pinnedPost, sendInChunks } from "@/lib/bulk-chunks"
import { http, HttpError, type InGroup } from "@/lib/http"
import { getServerErrorCode } from "@/lib/server-error"
import type { Schema } from "@/types"

export type FileEntity = Schema<"models.FileEntity">
export type FileLink = Schema<"models.FileLink">
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
  // Attachment URL — forces a download (Content-Disposition: attachment).
  url: string
  // Inline URL — serves the file for in-browser viewing ("Open in new
  // tab", #1962). The BE only serves a true inline disposition for
  // preview-safe MIME types (images / PDF / text) and downloads the rest,
  // so it's always safe to point a new tab at this. Absent on older
  // payloads or when signing the inline variant failed — callers fall
  // back to `url`.
  inline_url?: string
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
  group?: string
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
  const body = await http.get<FilesListEnvelope>(path, {
    signal: options.signal,
    group: options.group,
  })
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
  if (options.linkedEntityType && options.linkedEntityId) {
    params.set("linked_entity_type", options.linkedEntityType)
    params.set("linked_entity_id", options.linkedEntityId)
  }
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
}

// The entity kinds a user can attach a file to. Backup export bundles carry
// an "export" link too, but that one is never created or removed here. A
// "draft" link holds a file for an unfinished item (#2812).
export type FileLinkEntityType = "commodity" | "location" | "area" | "draft"

export interface AttachFileRequest {
  entityType: FileLinkEntityType
  entityId: string
  // Bucket on the entity's Files tab. Omitted, the BE picks "images" for an
  // image and the entity's document bucket for anything else.
  meta?: string
}

// Attach an existing file to an entity. Only a link is created: no upload, no
// new file row. Attaching to an entity the file is already attached to is a
// no-op on the BE (200 instead of 201), so a retry is safe.
export async function attachFile(
  fileId: string,
  req: AttachFileRequest,
  opts: InGroup = {}
): Promise<FileEntity & { id: string }> {
  const body = await http.post<FileDetailEnvelope>(
    `/files/${encodeURIComponent(fileId)}/links`,
    {
      data: {
        type: "file-links",
        attributes: { entity_type: req.entityType, entity_id: req.entityId, meta: req.meta },
      },
    },
    { group: opts.group }
  )
  if (!body.attributes) {
    throw new Error(`Malformed POST /files/${fileId}/links response: missing attributes`)
  }
  return { ...body.attributes, id: body.id ?? fileId }
}

// Remove one attachment. The file itself stays; when this was its last
// attachment it is a standalone file afterwards.
export async function detachFile(
  fileId: string,
  linkId: string,
  opts: InGroup = {}
): Promise<FileEntity & { id: string }> {
  const body = await http.del<FileDetailEnvelope>(
    `/files/${encodeURIComponent(fileId)}/links/${encodeURIComponent(linkId)}`,
    { group: opts.group }
  )
  if (!body?.attributes) {
    throw new Error(
      `Malformed DELETE /files/${fileId}/links/${linkId} response: missing attributes`
    )
  }
  return { ...body.attributes, id: body.id ?? fileId }
}

// Other files of the group with the same content (same SHA-256), newest
// first. Empty while the file's checksum is not known yet.
export async function listFileDuplicates(
  fileId: string,
  signal?: AbortSignal
): Promise<ListedFile[]> {
  const body = await http.get<FilesListEnvelope>(
    `/files/${encodeURIComponent(fileId)}/duplicates`,
    { signal }
  )
  const signed = body.meta?.signed_urls ?? {}
  return (body.data ?? []).map((file) => ({ file, signedUrl: signed[file.id] }))
}

// The links of a file that a user manages: everything but the export link of
// a backup bundle.
export function ordinaryLinks(file: Pick<FileEntity, "links">): FileLink[] {
  return (file.links ?? []).filter((l) => l.entity_type !== "export")
}

// Update file metadata. The BE derives `Type` from the MIME type and does
// not take `category` here: a file's category follows the bucket of its
// first attachment (POST /files/{id}/links).
export async function updateFile(
  id: string,
  req: UpdateFileRequest,
  opts: InGroup = {}
): Promise<FileEntity & { id: string }> {
  const body = await http.put<FileDetailEnvelope>(
    `/files/${encodeURIComponent(id)}`,
    { data: { id, type: "files", attributes: req } },
    { group: opts.group }
  )
  if (!body.attributes) {
    throw new Error(`Malformed PUT /files/${id} response: missing attributes`)
  }
  return { ...body.attributes, id: body.id ?? id }
}

// deleteFile moves the file to the group's Trash (#2799).
export async function deleteFile(id: string, opts: InGroup = {}): Promise<void> {
  await http.del<void>(`/files/${encodeURIComponent(id)}`, { group: opts.group })
}

// trashUnlinkedFile moves a file a form uploaded to Trash when nothing links
// it (#1626): an upload whose attach failed, left behind when the form is
// cancelled. A file that something links after all (an attach whose answer
// was lost, or another tab that attached it meanwhile) stays, and so does one
// already gone. The server checks the links and trashes the file in one
// request (DELETE ?unless_linked=true, #2930). The file is looked up in
// filesPath, the group's files path read before the uploads started (see
// groupScopedPath), or in the group named by group; without either, in the
// current group. Resolves whether the file went to Trash.
export async function trashUnlinkedFile(
  id: string,
  where: { filesPath?: string } & InGroup = {}
): Promise<boolean> {
  const { filesPath, group } = where
  const path = `${filesPath ?? "/files"}/${encodeURIComponent(id)}?unless_linked=true`
  const init = filesPath ? { skipGroupRewrite: true } : { group }
  try {
    await http.del<void>(path, init)
    return true
  } catch (err) {
    if (err instanceof HttpError && err.status === 404) return false
    if (getServerErrorCode(err) === FILE_LINKED_CODE) return false
    throw err
  }
}

// The code of a DELETE ?unless_linked=true that kept the file because a
// link holds it.
const FILE_LINKED_CODE = "file.linked"

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

// The server takes at most this many ids per bulk request.
const MAX_BULK_FILE_IDS = 100

// What a bulk tag change did to each file: changed its tags, left it as it
// was because it already had them, or could not change it.
export interface FileBulkResult {
  succeeded: string[]
  skipped: { id: string; name: string }[]
  failed: { id: string; name: string; error: string }[]
  // Files of a request that failed without an answer: the server may have
  // changed some of them (sendInChunks).
  unknown: string[]
}

export interface FileBulkTagsChange {
  /** Tag slugs, or the text of a tag the group does not have yet. */
  add: string[]
  /** Tag slugs. */
  remove: string[]
}

// Adds tags to and removes tags from the files; tags the change does not
// name stay on each file. Sends at most 100 ids per request, all to the
// group active when the change started. When a request fails without an
// answer, what the earlier ones changed is kept, its ids are reported as
// unknown, since the server may have changed some of them, and the ids after
// it as failed (sendInChunks).
export async function bulkUpdateFileTags(
  ids: string[],
  change: FileBulkTagsChange
): Promise<FileBulkResult> {
  const tags: NonNullable<Schema<"jsonapi.FileBulkPatch">["tags"]> = {}
  if (change.add.length > 0) tags.add = change.add
  if (change.remove.length > 0) tags.remove = change.remove
  const post = pinnedPost<Schema<"jsonapi.BulkResultResponse">>("/files/bulk-update")
  const { parts, unknown, unsent } = await sendInChunks(ids, MAX_BULK_FILE_IDS, (chunk) =>
    post({ data: { type: "files", attributes: { ids: chunk, patch: { tags } } } })
  )
  const merged: FileBulkResult = { succeeded: [], skipped: [], failed: [], unknown }
  for (const body of parts) {
    const attrs = body?.data?.attributes
    merged.succeeded.push(...(attrs?.succeeded ?? []))
    merged.skipped.push(
      ...(attrs?.skipped ?? []).map((sk) => ({ id: sk.id ?? "", name: sk.name ?? "" }))
    )
    merged.failed.push(
      ...(attrs?.failed ?? []).map((f) => ({
        id: f.id ?? "",
        name: f.name ?? "",
        error: f.error ?? "",
      }))
    )
  }
  for (const id of unsent) merged.failed.push({ id, name: "", error: "" })
  return merged
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
export async function uploadFile(file: File, opts: InGroup = {}): Promise<UploadResult> {
  const form = new FormData()
  form.append("file", file)
  const body = await http.post<FileDetailEnvelope>(`/uploads/file`, form, { group: opts.group })
  // Strict envelope: a missing id or attributes here would otherwise
  // propagate as empty strings into cache keys, navigation URLs, and
  // follow-up updateFile() calls.
  if (!body.id || !body.attributes) {
    throw new Error("Malformed /uploads/file response: missing id or attributes")
  }
  const signed = body.meta?.signed_urls?.[body.id]
  return { file: { ...body.attributes, id: body.id }, signedUrl: signed }
}
