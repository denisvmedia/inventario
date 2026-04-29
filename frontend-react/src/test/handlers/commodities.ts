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
