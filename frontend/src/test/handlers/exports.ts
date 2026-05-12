import { http, HttpResponse } from "msw"

import type { Export, Restore, ExportStatus, RestoreStatus } from "@/features/export/api"

import { apiUrl } from "."

type ExportFixture = Partial<Export> & { id: string }
type RestoreFixture = Partial<Restore> & { id: string }

function exportEnvelope(fix: ExportFixture) {
  const { id, ...attributes } = fix
  return { id, type: "exports", attributes }
}

function restoreEnvelope(fix: RestoreFixture) {
  const { id, ...attributes } = fix
  return { id, type: "restores", attributes }
}

export function list(slug: string, items: ExportFixture[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/exports`), () =>
      HttpResponse.json({
        data: items.map(exportEnvelope),
        meta: { exports: items.length },
      })
    ),
  ]
}

export function detail(slug: string, item: ExportFixture) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/exports/${encodeURIComponent(item.id)}`), () =>
      HttpResponse.json({ data: exportEnvelope(item) })
    ),
  ]
}

export function create(slug: string, item: ExportFixture) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/exports`), () =>
      HttpResponse.json({ data: exportEnvelope(item) }, { status: 201 })
    ),
  ]
}

export function remove(slug: string, id: string) {
  return [
    http.delete(
      apiUrl(`/g/${encodeURIComponent(slug)}/exports/${encodeURIComponent(id)}`),
      () => new HttpResponse(null, { status: 204 })
    ),
  ]
}

export function importBackup(slug: string, item: ExportFixture) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/exports/import`), () =>
      HttpResponse.json({ data: exportEnvelope(item) }, { status: 201 })
    ),
  ]
}

export function uploadRestore(slug: string, fileName: string) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/uploads/restores`), () =>
      HttpResponse.json(
        {
          id: "uploads",
          type: "uploads",
          attributes: { fileNames: [fileName], type: "restores" },
        },
        { status: 200 }
      )
    ),
  ]
}

export function listRestores(slug: string, exportId: string, restores: RestoreFixture[] = []) {
  return [
    http.get(
      apiUrl(`/g/${encodeURIComponent(slug)}/exports/${encodeURIComponent(exportId)}/restores`),
      () => HttpResponse.json({ data: restores.map(restoreEnvelope) })
    ),
  ]
}

export function createRestore(slug: string, exportId: string, item: RestoreFixture) {
  return [
    http.post(
      apiUrl(`/g/${encodeURIComponent(slug)}/exports/${encodeURIComponent(exportId)}/restores`),
      () => HttpResponse.json({ data: restoreEnvelope(item) }, { status: 201 })
    ),
  ]
}

export function getRestore(slug: string, exportId: string, item: RestoreFixture) {
  return [
    http.get(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/exports/${encodeURIComponent(exportId)}/restores/${encodeURIComponent(item.id)}`
      ),
      () => HttpResponse.json({ data: restoreEnvelope(item) })
    ),
  ]
}

// Convenience builder: most tests just need a single export with a status
// and the four counts populated. Defaults are tuned for the happy-path
// "list page renders one completed export" case; override per-test.
export function exportFixture(overrides: Partial<Export> & { id?: string } = {}): ExportFixture {
  const id = overrides.id ?? "exp-1"
  return {
    id,
    type: "full_database",
    status: "completed" as ExportStatus,
    description: "",
    include_file_data: true,
    file_size: 1024,
    binary_data_size: 0,
    location_count: 1,
    area_count: 1,
    commodity_count: 1,
    file_count: 1,
    image_count: 0,
    invoice_count: 0,
    manual_count: 0,
    created_date: "2026-05-01T10:00:00Z",
    completed_date: "2026-05-01T10:00:30Z",
    ...overrides,
  }
}

export function restoreFixture(overrides: Partial<Restore> & { id?: string } = {}): RestoreFixture {
  const id = overrides.id ?? "rest-1"
  return {
    id,
    status: "completed" as RestoreStatus,
    description: "",
    options: { strategy: "merge_add", include_file_data: false, dry_run: true },
    created_date: "2026-05-01T11:00:00Z",
    completed_date: "2026-05-01T11:00:30Z",
    ...overrides,
  }
}
