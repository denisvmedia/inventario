import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// CoverFixture mirrors the `meta.covers[id]` payload the BE attaches to
// commodity list responses (issue #1451 option A — first photo). Pass a
// map `{commodityId: cover}` to `list()` to opt in; absent ids render
// the type-icon fallback, same as the real handler.
export interface CoverFixture {
  file_id: string
  thumbnails: Record<string, string>
  source?: "first_photo" | "explicit"
}

// Backend mounts commodity routes inside /g/{slug}/commodities. Tests pass
// the slug they expect to see in the URL so MSW exact-matches the
// http-client rewrite output.
export function list(slug: string, items: unknown[] = [], covers?: Record<string, CoverFixture>) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/commodities`), () => {
      const body: Record<string, unknown> = { data: items }
      if (covers && Object.keys(covers).length > 0) {
        body.meta = { covers }
      }
      return HttpResponse.json(body)
    }),
  ]
}

// tagCounts backs GET /commodities/tag-counts. onRequest sees the query
// string, for tests that check which filters reach the counts.
export function tagCounts(
  slug: string,
  data: { tags: Record<string, number>; untagged: number },
  onRequest?: (params: URLSearchParams) => void
) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/tag-counts`), ({ request }) => {
      onRequest?.(new URL(request.url).searchParams)
      return HttpResponse.json({ data })
    }),
  ]
}

// brands backs GET /commodities/brands: it returns the fixture brands that
// contain the q parameter, ignoring case, up to the limit parameter.
export function brands(slug: string, data: { brand: string; count: number }[]) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/brands`), ({ request }) => {
      const params = new URL(request.url).searchParams
      const q = (params.get("q") ?? "").toLowerCase()
      const limit = Number(params.get("limit") ?? "20")
      const matching = data.filter((b) => b.brand.toLowerCase().includes(q)).slice(0, limit)
      return HttpResponse.json({ data: matching })
    }),
  ]
}

export function detail(slug: string, id: string, item: unknown, cover?: CoverFixture) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`), () => {
      const body: Record<string, unknown> = { data: item }
      if (cover) {
        body.meta = { cover }
      }
      return HttpResponse.json(body)
    }),
  ]
}

export function error(slug: string, status = 500) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/commodities`), () =>
      HttpResponse.json({ error: "boom" }, { status })
    ),
  ]
}

// Mocks /commodities/values — the dashboard's "Estimated total value"
// card and the area-detail Est. value stat hit this endpoint. The
// shape mirrors `jsonapi.ValueResponse` + `jsonapi.NamedTotal` from
// the OpenAPI codegen: per-location/area entries carry `{id, name,
// value}` (NOT `total`; the BE renamed in #1632 to align FE with the
// generated schema). The legacy `{name, total}` aliases are still
// accepted by this factory so older Dashboard tests don't have to
// rename in lockstep.
export interface NamedTotalFixture {
  id?: string
  name: string
  value?: number
  unvaluedCount?: number
  /** @deprecated alias for `value` — kept for back-compat with pre-#1632 tests. */
  total?: number
}

export function values(
  slug: string,
  attrs: {
    globalTotal?: number
    valuedCount?: number
    unvaluedCount?: number
    locationTotals?: NamedTotalFixture[]
    areaTotals?: NamedTotalFixture[]
  } = {}
) {
  const toEntry = (t: NamedTotalFixture) => ({
    id: t.id ?? "",
    name: t.name,
    value: t.value ?? t.total ?? 0,
    unvalued_count: t.unvaluedCount ?? 0,
  })
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/values`), () =>
      HttpResponse.json({
        data: {
          id: "value",
          type: "values",
          attributes: {
            global_total: attrs.globalTotal ?? 0,
            valued_count: attrs.valuedCount ?? 0,
            unvalued_count: attrs.unvaluedCount ?? 0,
            location_totals: (attrs.locationTotals ?? []).map(toEntry),
            area_totals: (attrs.areaTotals ?? []).map(toEntry),
          },
        },
      })
    ),
  ]
}

export function valuesError(slug: string, status = 500) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/values`), () =>
      HttpResponse.json({ error: "boom" }, { status })
    ),
  ]
}

// CRUD + bulk handlers used by the items page (#1410). Each returns
// the JSON:API envelope the real handler would produce; tests don't
// need to assert the request body (MSW lets them check via `.use()`).
export function create(slug: string, response: unknown) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/commodities`), () =>
      HttpResponse.json({ data: response }, { status: 201 })
    ),
  ]
}

export function update(slug: string, id: string, response: unknown) {
  return [
    http.put(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({ data: response })
    ),
  ]
}

export function remove(slug: string, id: string) {
  return [
    http.delete(
      apiUrl(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`),
      () => new HttpResponse(null, { status: 204 })
    ),
  ]
}

// The bulk handlers answer like the server does when every id succeeds.
// onRequest sees the request attributes; respond overrides the response, for
// partial results and errors.
type BulkAttributes = Record<string, unknown> & { ids?: string[] }
interface BulkHandlerOptions {
  onRequest?: (attributes: BulkAttributes) => void
  respond?: (attributes: BulkAttributes) => Response | Promise<Response>
}

export function bulkResultResponse(result: {
  succeeded?: string[]
  skipped?: { id: string; name?: string }[]
  failed?: { id: string; name?: string; code?: string; error?: string }[]
}) {
  return HttpResponse.json({
    data: {
      id: "bulk-result",
      type: "commodities.bulk_result",
      attributes: {
        succeeded: result.succeeded ?? [],
        failed: result.failed ?? [],
        ...(result.skipped ? { skipped: result.skipped } : {}),
      },
    },
  })
}

function bulkHandler(slug: string, action: string, opts: BulkHandlerOptions) {
  return [
    http.post(
      apiUrl(`/g/${encodeURIComponent(slug)}/commodities/${action}`),
      async ({ request }) => {
        const body = (await request.json()) as { data?: { attributes?: BulkAttributes } }
        const attributes = body.data?.attributes ?? {}
        opts.onRequest?.(attributes)
        if (opts.respond) return opts.respond(attributes)
        return bulkResultResponse({ succeeded: attributes.ids ?? [] })
      }
    ),
  ]
}

export function bulkDelete(slug: string, opts: BulkHandlerOptions = {}) {
  return bulkHandler(slug, "bulk-delete", opts)
}

export function bulkMove(slug: string, opts: BulkHandlerOptions = {}) {
  return bulkHandler(slug, "bulk-move", opts)
}

export function bulkUpdate(slug: string, opts: BulkHandlerOptions = {}) {
  return bulkHandler(slug, "bulk-update", opts)
}

// CommodityEventFixture mirrors the BE list-row shape returned by
// GET /commodities/{id}/events (issue #1450). Pass an array to `events()`
// to opt in; absent calls 200 with no rows.
export interface CommodityEventFixture {
  id: string
  type?: "commodity_events"
  commodity_id?: string
  kind: string
  occurred_at: string
  before?: Record<string, unknown>
  after?: Record<string, unknown>
  note?: string
  meta?: { actor?: { id: string; name?: string; email?: string } }
}

// Mocks /commodities/{id}/events. The handler returns the rows passed
// in; the FE timeline renders newest-first so callers should already
// pass them in that order (the BE does the same).
export function events(slug: string, id: string, rows: CommodityEventFixture[] = []) {
  return [
    http.get(
      apiUrl(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}/events`),
      () =>
        HttpResponse.json({
          data: rows.map((r) => ({
            type: "commodity_events",
            commodity_id: id,
            ...r,
          })),
          meta: { events: rows.length, total: rows.length },
        })
    ),
  ]
}
