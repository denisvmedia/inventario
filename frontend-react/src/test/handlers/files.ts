import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// MSW factory for the unified Files surface (#1398/#1399). Each helper
// returns a list of handlers so callers compose what they need:
//
//   server.use(...fileHandlers.list("g", items), ...fileHandlers.counts("g", { all: 3 }))
//
// Handlers wrap their JSON in a JSON:API-shaped envelope so the http
// client (lib/http.ts) parses them the same way it does in production.

export function list(
  slug: string,
  items: Array<{ id: string; attributes: unknown }> = [],
  meta: Record<string, unknown> = {}
) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/files`), () =>
      HttpResponse.json({
        data: items.map((it) => ({ id: it.id, type: "files", attributes: it.attributes })),
        meta: { files: items.length, total: items.length, ...meta },
      })
    ),
  ]
}

export function counts(
  slug: string,
  data: { photos?: number; invoices?: number; documents?: number; other?: number; all?: number }
) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/files/category-counts`), () =>
      HttpResponse.json({
        data: {
          photos: 0,
          invoices: 0,
          documents: 0,
          other: 0,
          all: 0,
          ...data,
        },
      })
    ),
  ]
}

export function detail(
  slug: string,
  id: string,
  attributes: unknown,
  signedUrl?: { url: string; thumbnails?: Record<string, string> }
) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/files/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({
        data: {
          id,
          type: "files",
          attributes,
          meta: signedUrl ? { signed_urls: { [id]: signedUrl } } : undefined,
        },
      })
    ),
  ]
}

export function update(slug: string, id: string, attributes: unknown) {
  return [
    http.put(apiUrl(`/g/${encodeURIComponent(slug)}/files/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({
        data: { id, type: "files", attributes },
      })
    ),
  ]
}

export function deleteOk(slug: string, id: string) {
  return [
    http.delete(apiUrl(`/g/${encodeURIComponent(slug)}/files/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({}, { status: 204 })
    ),
  ]
}

export function bulkDelete(
  slug: string,
  succeeded: string[] = [],
  failed: { id: string; error: string }[] = []
) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/files/bulk-delete`), () =>
      HttpResponse.json({
        data: { type: "files", attributes: { succeeded, failed } },
      })
    ),
  ]
}

export function uploadCapacity(
  slug: string,
  opts: { canStart?: boolean; retryAfter?: number } = {}
) {
  const canStart = opts.canStart ?? true
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/upload-slots/check`), () =>
      HttpResponse.json({
        data: {
          attributes: {
            operation_name: "files-upload",
            active_uploads: canStart ? 0 : 4,
            max_uploads: 4,
            available_uploads: canStart ? 4 : 0,
            can_start_upload: canStart,
            retry_after_seconds: opts.retryAfter,
          },
        },
      })
    ),
  ]
}

export function upload(slug: string, attributes: unknown, id = "uploaded-1") {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/uploads/file`), () =>
      HttpResponse.json(
        {
          data: { id, type: "files", attributes },
        },
        { status: 201 }
      )
    ),
  ]
}

export function error(slug: string, status = 500) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/files`), () =>
      HttpResponse.json({ error: "boom" }, { status })
    ),
  ]
}
