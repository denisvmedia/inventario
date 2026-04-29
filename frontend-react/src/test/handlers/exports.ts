import { http, HttpResponse } from "msw"

import { apiUrl } from "."

export function list(slug: string, items: unknown[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/exports`), () =>
      HttpResponse.json({ data: items })
    ),
  ]
}
