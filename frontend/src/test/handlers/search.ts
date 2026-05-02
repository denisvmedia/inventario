import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// Search lives under /g/{slug}/search per the legacy backend. The endpoint
// returns a flat result list — the empty case ([]) is what the command
// palette's "no results" branch tests against.
export function results(slug: string, items: unknown[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/search`), () =>
      HttpResponse.json({ data: items })
    ),
  ]
}
