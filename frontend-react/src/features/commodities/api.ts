// Pure data-layer functions for the commodities feature slice. Hooks live
// in `./hooks.ts`. The dashboard (#1408) is the first consumer; the
// items list (#1410) will reuse the same `listCommodities()` helper.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type Commodity = Schema<"models.Commodity">

interface CommodityResource {
  id: string
  type: string
  attributes: Commodity
}

interface CommoditiesListResponse {
  data: CommodityResource[]
  meta?: {
    commodities?: number
    page?: number
    per_page?: number
    total_pages?: number
  }
}

interface ValueResponse {
  data?: {
    attributes?: {
      area_totals?: { name?: string; total?: number }[]
      global_total?: number
      location_totals?: { name?: string; total?: number }[]
    }
  }
}

// Returns the commodities for the active group. JSON:API envelope is
// unwrapped here so consumers see a plain Commodity[]. `per_page` is
// passed through unchanged — the dashboard cares about an aggregate
// count + a "recently added" slice, the items list (#1410) will need
// proper pagination.
export async function listCommodities(
  options: { perPage?: number; page?: number; signal?: AbortSignal } = {}
): Promise<{ commodities: Commodity[]; total: number }> {
  const params = new URLSearchParams()
  if (options.perPage !== undefined) params.set("per_page", String(options.perPage))
  if (options.page !== undefined) params.set("page", String(options.page))
  const qs = params.toString()
  const path = qs ? `/commodities?${qs}` : "/commodities"
  const body = await http.get<CommoditiesListResponse>(path, { signal: options.signal })
  return {
    commodities: (body.data ?? []).map((item) => ({ ...item.attributes, id: item.id })),
    total: body.meta?.commodities ?? body.data?.length ?? 0,
  }
}

// Returns the global / per-location / per-area value totals for the
// active group, in the group's main currency. The dashboard reads only
// `global_total`; per-location/area breakdowns are kept here for the
// items page (#1410) to reuse.
export interface CommoditiesValue {
  globalTotal: number
  locationTotals: { name: string; total: number }[]
  areaTotals: { name: string; total: number }[]
}

export async function getCommoditiesValue(signal?: AbortSignal): Promise<CommoditiesValue> {
  const body = await http.get<ValueResponse>("/commodities/values", { signal })
  const attrs = body.data?.attributes ?? {}
  return {
    globalTotal: attrs.global_total ?? 0,
    locationTotals: (attrs.location_totals ?? []).map((t) => ({
      name: t.name ?? "",
      total: t.total ?? 0,
    })),
    areaTotals: (attrs.area_totals ?? []).map((t) => ({
      name: t.name ?? "",
      total: t.total ?? 0,
    })),
  }
}
