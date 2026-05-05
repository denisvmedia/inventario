import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// CoverFixture mirrors the `meta.covers[id]` payload the BE attaches to
// commodity list responses (issue #1451 option A — first photo). Pass a
// map `{commodityId: cover}` to `list()` to opt in; absent ids render
// the type-emoji fallback, same as the real handler.
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
// card hits this endpoint. The shape mirrors `jsonapi.ValueResponse`
// from the OpenAPI codegen.
export function values(
  slug: string,
  attrs: {
    globalTotal?: number
    locationTotals?: { name: string; total: number }[]
    areaTotals?: { name: string; total: number }[]
  } = {}
) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/values`), () =>
      HttpResponse.json({
        data: {
          id: "value",
          type: "values",
          attributes: {
            global_total: attrs.globalTotal ?? 0,
            location_totals: attrs.locationTotals ?? [],
            area_totals: attrs.areaTotals ?? [],
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

export function bulkDelete(slug: string) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/bulk-delete`), () =>
      HttpResponse.json({}, { status: 200 })
    ),
  ]
}

export function bulkMove(slug: string) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/bulk-move`), () =>
      HttpResponse.json({}, { status: 200 })
    ),
  ]
}
