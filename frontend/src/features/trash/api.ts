// Data-layer functions for the Trash. Hooks live in `./hooks.ts`.
// Deleted items and files wait under /trash until they are restored, deleted
// for good, or purged by the server once the retention runs out.
import { sendInChunks } from "@/lib/bulk-chunks"
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type TrashedCommodityAttrs = Schema<"models.Commodity">
export type TrashedFileAttrs = Schema<"models.FileEntity">

export interface TrashedCommodity {
  id: string
  commodity: TrashedCommodityAttrs
  // Name of the member who deleted the item; empty when that user is gone.
  deletedByName: string
  // When the server deletes the item for good.
  expiresAt: string
  // Files that went to Trash with the item and come back with it.
  fileCount: number
  fileBytes: number
}

export interface TrashedFile {
  id: string
  file: TrashedFileAttrs
  deletedByName: string
  expiresAt: string
}

export interface TrashPage<T> {
  entries: T[]
  total: number
  // How many days an entry stays in Trash, as the server is configured.
  retentionDays: number
}

export interface TrashListOptions {
  page?: number
  perPage?: number
}

export interface BulkTrashResult {
  succeeded: string[]
  failed: { id: string; error: string }[]
}

export interface BulkRestoreResult extends BulkTrashResult {
  // Restored items whose area was deleted while they were in Trash; they
  // came back unassigned.
  areaRemoved: string[]
  // Items of a request that failed after an earlier one went through: the
  // server may have restored some of them (sendInChunks).
  unknown: string[]
}

export interface CommodityRestoreResult {
  areaRemoved: boolean
  filesRestored: number
}

export interface EmptyTrashResult {
  commodities: number
  files: number
}

type CommoditiesEnvelope = Schema<"jsonapi.TrashedCommoditiesResponse">
type FilesEnvelope = Schema<"jsonapi.TrashedFilesResponse">
type CommodityRestoreEnvelope = Schema<"jsonapi.CommodityRestoreResponse">
type BulkRestoreEnvelope = Schema<"jsonapi.TrashBulkRestoreResponse">
type BulkResultEnvelope = Schema<"jsonapi.BulkResultResponse">
type EmptyTrashEnvelope = Schema<"jsonapi.EmptyTrashResponse">

function listQuery(path: string, opts: TrashListOptions): string {
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  const qs = params.toString()
  return qs ? `${path}?${qs}` : path
}

function bulkBody(type: "commodities" | "files", ids: string[]) {
  return { data: { type, attributes: { ids } } }
}

function bulkResult(body: BulkResultEnvelope): BulkTrashResult {
  return {
    succeeded: body.data?.attributes?.succeeded ?? [],
    failed: (body.data?.attributes?.failed ?? []).map((f) => ({
      id: f.id ?? "",
      error: f.error ?? "",
    })),
  }
}

export async function listTrashedCommodities(
  opts: TrashListOptions = {},
  signal?: AbortSignal
): Promise<TrashPage<TrashedCommodity>> {
  const body = await http.get<CommoditiesEnvelope>(listQuery("/trash/commodities", opts), {
    signal,
  })
  return {
    entries: (body.data ?? []).map((row) => ({
      id: row.id ?? "",
      commodity: row.attributes ?? {},
      deletedByName: row.meta?.deleted_by_name ?? "",
      expiresAt: row.meta?.expires_at ?? "",
      fileCount: row.meta?.file_count ?? 0,
      fileBytes: row.meta?.file_bytes ?? 0,
    })),
    total: body.meta?.total ?? 0,
    retentionDays: body.meta?.retention_days ?? 0,
  }
}

export async function listTrashedFiles(
  opts: TrashListOptions = {},
  signal?: AbortSignal
): Promise<TrashPage<TrashedFile>> {
  const body = await http.get<FilesEnvelope>(listQuery("/trash/files", opts), { signal })
  return {
    entries: (body.data ?? []).map((row) => ({
      id: row.id ?? "",
      file: row.attributes ?? {},
      deletedByName: row.meta?.deleted_by_name ?? "",
      expiresAt: row.meta?.expires_at ?? "",
    })),
    total: body.meta?.total ?? 0,
    retentionDays: body.meta?.retention_days ?? 0,
  }
}

export async function restoreCommodity(id: string): Promise<CommodityRestoreResult> {
  const body = await http.post<CommodityRestoreEnvelope>(
    `/trash/commodities/${encodeURIComponent(id)}/restore`
  )
  return {
    areaRemoved: body.meta?.area_removed ?? false,
    filesRestored: body.meta?.files_restored ?? 0,
  }
}

// The server restores at most this many items per request (jsonapi.MaxBulkIDs).
const MAX_RESTORE_IDS = 100

// Restores items from Trash. More ids than one request takes, as Undo after
// a bulk delete of "all matching" items sends, go in several requests. When
// a later one fails, what the earlier ones restored is kept, its ids are
// reported as unknown, since the server may have restored some of them, and
// the ids after it as failed (sendInChunks).
export async function restoreCommodities(ids: string[]): Promise<BulkRestoreResult> {
  const { parts, unknown, unsent } = await sendInChunks(ids, MAX_RESTORE_IDS, (chunk) =>
    http.post<BulkRestoreEnvelope>("/trash/commodities/restore", bulkBody("commodities", chunk))
  )
  const merged: BulkRestoreResult = { succeeded: [], failed: [], areaRemoved: [], unknown }
  for (const body of parts) {
    const part = bulkResult(body)
    merged.succeeded.push(...part.succeeded)
    merged.failed.push(...part.failed)
    merged.areaRemoved.push(...(body.data?.attributes?.area_removed ?? []))
  }
  for (const id of unsent) merged.failed.push({ id, error: "" })
  return merged
}

export async function purgeCommodity(id: string): Promise<void> {
  await http.del<void>(`/trash/commodities/${encodeURIComponent(id)}`)
}

export async function purgeCommodities(ids: string[]): Promise<BulkTrashResult> {
  const body = await http.post<BulkResultEnvelope>(
    "/trash/commodities/purge",
    bulkBody("commodities", ids)
  )
  return bulkResult(body)
}

export async function restoreFile(id: string): Promise<void> {
  await http.post<void>(`/trash/files/${encodeURIComponent(id)}/restore`)
}

export async function restoreFiles(ids: string[]): Promise<BulkTrashResult> {
  const body = await http.post<BulkResultEnvelope>("/trash/files/restore", bulkBody("files", ids))
  return bulkResult(body)
}

export async function purgeFile(id: string): Promise<void> {
  await http.del<void>(`/trash/files/${encodeURIComponent(id)}`)
}

export async function purgeFiles(ids: string[]): Promise<BulkTrashResult> {
  const body = await http.post<BulkResultEnvelope>("/trash/files/purge", bulkBody("files", ids))
  return bulkResult(body)
}

export async function emptyTrash(): Promise<EmptyTrashResult> {
  const body = await http.del<EmptyTrashEnvelope>("/trash")
  return {
    commodities: body?.data?.attributes?.commodities ?? 0,
    files: body?.data?.attributes?.files ?? 0,
  }
}
