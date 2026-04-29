import { http, HttpResponse } from "msw"

import { apiUrl } from "."

export function list(slug: string, items: unknown[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/files`), () =>
      HttpResponse.json({ data: items })
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
