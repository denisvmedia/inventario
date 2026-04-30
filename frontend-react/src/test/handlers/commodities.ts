import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// Backend mounts commodity routes inside /g/{slug}/commodities. Tests pass
// the slug they expect to see in the URL so MSW exact-matches the
// http-client rewrite output.
export function list(slug: string, items: unknown[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/commodities`), () =>
      HttpResponse.json({ data: items })
    ),
  ]
}

export function detail(slug: string, id: string, item: unknown) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({ data: item })
    ),
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
  attrs: { globalTotal?: number; locationTotals?: { name: string; total: number }[]; areaTotals?: { name: string; total: number }[] } = {}
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
