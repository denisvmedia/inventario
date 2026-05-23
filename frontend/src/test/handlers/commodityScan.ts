import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// MSW factory for the AI-vision scan endpoint (#1720). Mirrors the BE
// handler `apiserver/commodity_scan.go::scanCommodity` — the response
// is wrapped in a JSON:API `data.attributes` envelope with `fields`
// and `warnings` objects.
//
// Usage:
//   server.use(...commodityScanHandlers.ok("g", { name: { value: "Sony", confidence: 0.9 } }))
//   server.use(...commodityScanHandlers.error("g", 503, "commodity_scan.provider_disabled"))

type Field =
  | { value: string; confidence: number }
  | { value: number; confidence: number }
  | { value: string[]; confidence: number }

interface OkAttributes {
  fields: Partial<Record<string, Field>>
  warnings?: Array<{ code: string; field?: string; detail?: string }>
}

export function ok(slug: string, attrs: OkAttributes) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/scan`), async () =>
      HttpResponse.json({
        data: {
          type: "commodity_scan",
          attributes: {
            fields: attrs.fields,
            warnings: attrs.warnings ?? [],
          },
        },
      })
    ),
  ]
}

export function error(slug: string, status: number, code: string, detail = "scan failed") {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/scan`), () =>
      HttpResponse.json(
        {
          errors: [{ code, status: String(status), detail, title: detail }],
        },
        { status }
      )
    ),
  ]
}

// Helper that delays the response so tests can assert the
// "scanning" phase before resolving.
export function slow(slug: string, attrs: OkAttributes, delayMs = 200): ReturnType<typeof ok> {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/commodities/scan`), async () => {
      await new Promise((resolve) => setTimeout(resolve, delayMs))
      return HttpResponse.json({
        data: {
          type: "commodity_scan",
          attributes: {
            fields: attrs.fields,
            warnings: attrs.warnings ?? [],
          },
        },
      })
    }),
  ]
}
