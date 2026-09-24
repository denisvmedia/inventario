import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// MSW factory for the unified Files surface (#1398/#1399). Each helper
// returns a list of handlers so callers compose what they need:
//
//   server.use(...fileHandlers.list("g", items), ...fileHandlers.counts("g", { all: 3 }))
//
// Handlers wrap their JSON in a JSON:API-shaped envelope so the http
// client (lib/http.ts) parses them the same way it does in production.

// list mirrors apiserver/files.go::listFiles → FilesResponse — the
// FileEntity records are FLAT inside `data`, not wrapped in the
// `{id, type, attributes}` envelope the detail endpoint uses. Tests
// pass `{ id, attributes }` so the fixture is readable; the handler
// flattens them into the on-the-wire shape.
export function list(
  slug: string,
  items: Array<{ id: string; attributes: Record<string, unknown> }> = [],
  meta: Record<string, unknown> = {}
) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/files`), ({ request }) => {
      const params = new URL(request.url).searchParams
      const matched = items.filter((it) => matches(it.attributes, params))
      // `limit` caps at 100 on the BE and defaults to 20; mirroring the
      // default matters, because a caller that forgets `perPage` gets a
      // truncated page in production too.
      const limit = Math.min(Number(params.get("limit")) || 20, 100)
      const page = Math.max(Number(params.get("page")) || 1, 1)
      const start = Math.min((page - 1) * limit, matched.length)
      const paged = matched.slice(start, start + limit)
      return HttpResponse.json({
        data: paged.map((it) => ({ ...it.attributes, id: it.id })),
        meta: { files: paged.length, total: matched.length, ...meta },
      })
    }),
  ]
}

// matches applies the filters apiserver/files.go::listFiles applies, so a
// component that ignored its own filter state would fail its test instead of
// being handed a conveniently pre-filtered fixture. `tags` is containment
// (`tags @> $1`), so every requested tag has to be present; `search` is an
// ILIKE substring over the same four columns the registry searches.
function matches(attrs: Record<string, unknown>, params: URLSearchParams): boolean {
  const category = params.get("category")
  if (category && attrs.category !== category) return false

  const linkedType = params.get("linked_entity_type")
  const linkedId = params.get("linked_entity_id")
  if (linkedType && attrs.linked_entity_type !== linkedType) return false
  if (linkedId && attrs.linked_entity_id !== linkedId) return false

  const type = params.get("type")
  if (type && attrs.type !== type) return false

  const tags = (params.get("tags") ?? "")
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean)
  const own = Array.isArray(attrs.tags) ? (attrs.tags as string[]) : []
  if (!tags.every((t) => own.includes(t))) return false

  const search = params.get("search")?.trim().toLowerCase()
  if (search) {
    const haystack = ["title", "description", "path", "original_path"]
      .map((k) => String(attrs[k] ?? "").toLowerCase())
      .join("\u0000")
    if (!haystack.includes(search)) return false
  }

  return true
}

// countsFromFiles answers /files/category-counts out of the same fixture the
// list handler serves, honoring the same query parameters — including the
// linked-entity pair, which is what an entity's Files tab sends so its chips
// agree with the list beneath them. Prefer it over `counts` whenever a test
// renders both surfaces: literal numbers cannot disagree with the list, which
// is exactly the bug worth catching.
export function countsFromFiles(
  slug: string,
  items: Array<{ id: string; attributes: Record<string, unknown> }> = []
) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/files/category-counts`), ({ request }) => {
      const params = new URL(request.url).searchParams
      const matched = items.filter((it) => matches(it.attributes, params))
      const inCategory = (c: string) => matched.filter((it) => it.attributes.category === c).length
      const data = {
        images: inCategory("images"),
        documents: inCategory("documents"),
        other: inCategory("other"),
        all: matched.length,
        bytes: { images: 0, documents: 0, other: 0, all: 0 },
      }
      return HttpResponse.json({ data })
    }),
  ]
}

export function counts(
  slug: string,
  data: {
    images?: number
    documents?: number
    other?: number
    all?: number
    bytes?: {
      images?: number
      documents?: number
      other?: number
      all?: number
    }
  }
) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/files/category-counts`), () =>
      HttpResponse.json({
        data: {
          images: 0,
          documents: 0,
          other: 0,
          all: 0,
          bytes: { images: 0, documents: 0, other: 0, all: 0 },
          ...data,
        },
      })
    ),
  ]
}

// detail mirrors apiserver/files.go::apiGetFile → FileResponse — the
// payload renders FLAT at the top level, NOT nested under `data:`.
// Same for the create/update/upload paths.
export function detail(
  slug: string,
  id: string,
  attributes: unknown,
  signedUrl?: { url: string; inline_url?: string; thumbnails?: Record<string, string> }
) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/files/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({
        id,
        type: "files",
        attributes,
        meta: signedUrl ? { signed_urls: { [id]: signedUrl } } : undefined,
      })
    ),
  ]
}

export function update(slug: string, id: string, attributes: unknown) {
  return [
    http.put(apiUrl(`/g/${encodeURIComponent(slug)}/files/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({
        id,
        type: "files",
        attributes,
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
          id,
          type: "files",
          attributes,
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
