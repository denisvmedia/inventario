// Pure data-layer functions for the exports / imports / restores feature
// slice. Hooks live in `./hooks.ts`. Backed by the `/exports`, `/exports/{id}`,
// `/exports/{id}/restores`, `/exports/{id}/signed-url`, `/exports/import` and
// `/uploads/restores` surfaces.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type ExportEntity = Schema<"models.Export">
export type ExportStatus = Schema<"models.ExportStatus">
export type ExportType = Schema<"models.ExportType">
export type ExportSelectedItem = Schema<"models.ExportSelectedItem">
export type ExportSelectedItemType = Schema<"models.ExportSelectedItemType">
export type RestoreOperation = Schema<"models.RestoreOperation">
export type RestoreOptions = Schema<"models.RestoreOptions">
export type RestoreStatus = Schema<"models.RestoreStatus">
export type RestoreStep = Schema<"models.RestoreStep">

// `as const` keeps the literal-union type so callers can `z.enum(...)`
// without widening to string. The `satisfies` clauses guard against a
// silent BE drift — adding a status to the OpenAPI model without
// updating the FE list will fail the typecheck here.
export const EXPORT_STATUSES = [
  "pending",
  "in_progress",
  "completed",
  "failed",
] as const satisfies readonly ExportStatus[]

export const EXPORT_TYPES = [
  "full_database",
  "selected_items",
  "locations",
  "areas",
  "commodities",
  "imported",
] as const satisfies readonly ExportType[]

export const RESTORE_STATUSES = [
  "pending",
  "running",
  "completed",
  "failed",
] as const satisfies readonly RestoreStatus[]

// Strategy strings live on the BE in go/backup/restore/types/types.go.
// Kept as a literal-union here because models.RestoreOptions.strategy is
// typed as plain string in the generated schema.
export const RESTORE_STRATEGIES = ["full_replace", "merge_add", "merge_update"] as const
export type RestoreStrategy = (typeof RESTORE_STRATEGIES)[number]

const TERMINAL_EXPORT_STATUSES: ReadonlySet<ExportStatus> = new Set(["completed", "failed"])
const TERMINAL_RESTORE_STATUSES: ReadonlySet<RestoreStatus> = new Set(["completed", "failed"])

export function isExportTerminal(status: ExportStatus | undefined): boolean {
  return !!status && TERMINAL_EXPORT_STATUSES.has(status)
}

export function isRestoreTerminal(status: RestoreStatus | undefined): boolean {
  return !!status && TERMINAL_RESTORE_STATUSES.has(status)
}

// Identity-resolved entity types we expose in TS. The OpenAPI types make
// `id` optional on every payload but in practice the BE always returns
// one — we throw on malformed responses rather than letting an empty id
// propagate into cache keys, navigation URLs, and follow-up calls.
export type Export = ExportEntity & { id: string }
export type Restore = RestoreOperation & { id: string }

interface ExportEnvelope {
  id?: string
  type?: string
  attributes?: ExportEntity
}

interface ExportsListEnvelope {
  data?: ExportEnvelope[]
  meta?: {
    exports?: number
  }
}

interface RestoreEnvelope {
  id?: string
  type?: string
  attributes?: RestoreOperation
}

interface RestoresListEnvelope {
  data?: RestoreEnvelope[]
}

interface UploadEnvelope {
  id?: string
  type?: string
  attributes?: {
    fileNames?: string[]
    type?: string
  }
}

function resolveExport(envelope: ExportEnvelope, fallbackId?: string): Export {
  if (!envelope.attributes) {
    throw new Error("Malformed exports response: missing attributes")
  }
  const id = envelope.id ?? fallbackId
  if (!id) {
    throw new Error("Malformed exports response: missing id")
  }
  return { ...envelope.attributes, id }
}

function resolveRestore(envelope: RestoreEnvelope, fallbackId?: string): Restore {
  if (!envelope.attributes) {
    throw new Error("Malformed restores response: missing attributes")
  }
  const id = envelope.id ?? fallbackId
  if (!id) {
    throw new Error("Malformed restores response: missing id")
  }
  return { ...envelope.attributes, id }
}

export interface ListExportsOptions {
  includeDeleted?: boolean
  signal?: AbortSignal
}

export async function listExports(
  options: ListExportsOptions = {}
): Promise<{ exports: Export[]; total: number }> {
  const params = new URLSearchParams()
  if (options.includeDeleted) params.set("include_deleted", "true")
  const qs = params.toString()
  const path = qs ? `/exports?${qs}` : "/exports"
  const body = await http.get<ExportsListEnvelope>(path, { signal: options.signal })
  const exports = (body.data ?? []).map((row) => resolveExport(row))
  return { exports, total: body.meta?.exports ?? exports.length }
}

export async function getExport(id: string, signal?: AbortSignal): Promise<Export> {
  const body = await http.get<{ data?: ExportEnvelope }>(`/exports/${encodeURIComponent(id)}`, {
    signal,
  })
  if (!body.data) {
    throw new Error(`Malformed /exports/${id} response: missing data`)
  }
  return resolveExport(body.data, id)
}

export interface CreateExportRequest {
  type: ExportType
  description: string
  include_file_data: boolean
  selected_items?: ExportSelectedItem[]
}

export async function createExport(req: CreateExportRequest): Promise<Export> {
  const body = await http.post<{ data?: ExportEnvelope }>("/exports", {
    data: { type: "exports", attributes: req },
  })
  if (!body.data) {
    throw new Error("Malformed POST /exports response: missing data")
  }
  return resolveExport(body.data)
}

export async function deleteExport(id: string): Promise<void> {
  await http.del<void>(`/exports/${encodeURIComponent(id)}`)
}

// Downloads no longer put a JWT in the URL (#1780). Instead we ask the BE
// to mint a short-lived HMAC-signed, app-absolute download URL via an
// authenticated request; the browser then navigates to that signed URL,
// which streams the file with `Content-Disposition: attachment`. No
// session token ever appears in referer / history / proxy logs.
type SignedFileURLResponse = Schema<"jsonapi.SignedFileURLResponse">

// fetchExportDownloadUrl performs an authenticated GET to mint a
// short-lived signed download URL for a completed export. The returned
// string is an app-absolute URL (`/api/v1/...`) ready to navigate to.
// It is a GET because minting the URL is side-effect-free — and because
// the group-scoped write gate is admin-only for non-GET methods, so a
// GET keeps export downloads available to every group member.
// Throws on a malformed response or a BE error (404 when the export is
// deleted, not completed, or has no backing file entity).
export async function fetchExportDownloadUrl(slug: string, id: string): Promise<string> {
  const body = await http.get<SignedFileURLResponse>(
    `/g/${encodeURIComponent(slug)}/exports/${encodeURIComponent(id)}/signed-url`
  )
  const url = body.attributes?.url
  if (!url) {
    throw new Error("Malformed signed-url response: missing attributes.url")
  }
  return url
}

export interface UploadRestoreFileResult {
  sourceFilePath: string
}

// Two-step "import a backup": first upload the `.inb` archive, then create an
// "imported" export record from it. The first step intentionally lives
// under /uploads/restores (not /exports/import) — the BE writes the
// blob to a sandboxed bucket and only acknowledges the path; nothing
// is parsed until the second call.
export async function uploadRestoreFile(file: File): Promise<UploadRestoreFileResult> {
  const form = new FormData()
  form.append("file", file)
  const body = await http.post<UploadEnvelope>("/uploads/restores", form)
  const fileNames = body.attributes?.fileNames ?? []
  const sourceFilePath = fileNames[0]
  if (!sourceFilePath) {
    throw new Error("Malformed /uploads/restores response: missing fileNames[0]")
  }
  return { sourceFilePath }
}

export interface ImportBackupRequest {
  description: string
  source_file_path: string
}

export async function importBackup(req: ImportBackupRequest): Promise<Export> {
  const body = await http.post<{ data?: ExportEnvelope }>("/exports/import", {
    data: { type: "exports", attributes: req },
  })
  if (!body.data) {
    throw new Error("Malformed POST /exports/import response: missing data")
  }
  return resolveExport(body.data)
}

export async function listRestores(
  exportId: string,
  signal?: AbortSignal
): Promise<{ restores: Restore[] }> {
  const body = await http.get<RestoresListEnvelope>(
    `/exports/${encodeURIComponent(exportId)}/restores`,
    { signal }
  )
  return { restores: (body.data ?? []).map((row) => resolveRestore(row)) }
}

export async function getRestore(
  exportId: string,
  restoreId: string,
  signal?: AbortSignal
): Promise<Restore> {
  const body = await http.get<{ data?: RestoreEnvelope }>(
    `/exports/${encodeURIComponent(exportId)}/restores/${encodeURIComponent(restoreId)}`,
    { signal }
  )
  if (!body.data) {
    throw new Error(`Malformed restore detail response: missing data`)
  }
  return resolveRestore(body.data, restoreId)
}

export interface CreateRestoreRequest {
  description: string
  options: {
    strategy: RestoreStrategy
    include_file_data: boolean
    dry_run: boolean
  }
}

export async function createRestore(exportId: string, req: CreateRestoreRequest): Promise<Restore> {
  const body = await http.post<{ data?: RestoreEnvelope }>(
    `/exports/${encodeURIComponent(exportId)}/restores`,
    { data: { type: "restores", attributes: req } }
  )
  if (!body.data) {
    throw new Error("Malformed POST /restores response: missing data")
  }
  return resolveRestore(body.data)
}
