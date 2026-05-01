// Single-resource search against GET /search?q=…&type=…. The BE returns
// `{ data: [{ id, type, attributes }], meta: { entity_type, query, total } }`
// — `data` is untyped in the OpenAPI spec because the shape varies by
// `type`. We surface a typed `SearchResult` per resource and keep the
// raw attributes loose at the boundary so consumers can narrow safely.
import { http, HttpError } from "@/lib/http"
import type { Schema } from "@/types"

export type SearchableType = "commodities" | "files" | "areas" | "locations"

export interface SearchResource<TAttrs = Record<string, unknown>> {
  id: string
  type: string
  attributes: TAttrs
}

export type CommodityAttrs = Schema<"models.Commodity">
export type FileAttrs = Schema<"models.FileEntity">
export type LocationAttrs = Schema<"models.Location">
export type AreaAttrs = Schema<"models.Area">

export interface SearchPage<TAttrs = Record<string, unknown>> {
  results: Array<SearchResource<TAttrs>>
  // total comes from `meta.total`; falls back to the array length when
  // the BE hasn't computed an exact count (the basic-fallback path in
  // go/apiserver/search.go doesn't always populate it).
  total: number
  // True when the BE returned 501 Not Implemented for this resource
  // type (e.g. files search before #1398, tags before #1400). Lets the
  // page tell "no matches" apart from "this resource isn't searchable
  // yet" without leaking transport errors into TanStack's `error` slot.
  unavailable: boolean
}

interface RawSearchResponse {
  data?: unknown
  meta?: { total?: number; entity_type?: string; query?: string }
}

export interface SearchOptions {
  type: SearchableType
  // BE caps at server-side default; we ask for 5 in the grouped page
  // and 3 in the palette to stay snappy. `limit` is forwarded as-is.
  limit?: number
  offset?: number
  signal?: AbortSignal
}

// search hits the type-scoped endpoint and unwraps the JSON:API list.
// 501 Not Implemented (the BE returns this for entity types whose search
// path isn't wired up yet, e.g. files until #1398) is caught and folded
// into `{ results: [], total: 0, unavailable: true }` — the grouped page
// renders that as a "coming soon" stub rather than a transport error.
// Other failures (5xx, network blip, abort) propagate so TanStack's
// `error` slot can carry them to the consumer.
export async function search<TAttrs>(
  query: string,
  opts: SearchOptions
): Promise<SearchPage<TAttrs>> {
  const trimmed = query.trim()
  if (!trimmed) return { results: [], total: 0, unavailable: false }
  const params = new URLSearchParams()
  params.set("q", trimmed)
  params.set("type", opts.type)
  if (opts.limit !== undefined) params.set("limit", String(opts.limit))
  if (opts.offset !== undefined) params.set("offset", String(opts.offset))
  try {
    const body = await http.get<RawSearchResponse>(`/search?${params.toString()}`, {
      signal: opts.signal,
    })
    const list = Array.isArray(body.data) ? (body.data as Array<SearchResource<TAttrs>>) : []
    const total = body.meta?.total ?? list.length
    return { results: list, total, unavailable: false }
  } catch (err) {
    if (err instanceof HttpError && err.status === 501) {
      return { results: [], total: 0, unavailable: true }
    }
    throw err
  }
}
