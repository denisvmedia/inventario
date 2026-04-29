import { http, HttpResponse } from "msw"

import { apiUrl } from "."

export function list(slug: string, items: unknown[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/locations`), () =>
      HttpResponse.json({ data: items })
    ),
  ]
}

export function detail(slug: string, id: string, item: unknown) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({ data: item })
    ),
  ]
}

export function error(slug: string, status = 500) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/locations`), () =>
      HttpResponse.json({ error: "boom" }, { status })
    ),
  ]
}
