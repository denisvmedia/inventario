// Pure data-layer functions for the commodities feature slice. Hooks live
// in `./hooks.ts`. The dashboard (#1408) reads `listCommodities()` /
// `getCommoditiesValue()`; the items page (#1410) extends with full CRUD
// + bulk + filter/sort/search query params (BE shipped together with the
// FE in #1410).
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type Commodity = Schema<"models.Commodity">
export type CommodityType = Schema<"models.CommodityType">
export type CommodityStatus = Schema<"models.CommodityStatus">

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

interface CommodityResponseEnvelope {
  data?: { id?: string; type?: string; attributes?: Commodity; meta?: CommodityMeta }
}

// Sub-resources returned alongside a single-commodity GET. Today's BE
// surfaces images / manuals / invoices via the legacy commodity-scoped
// endpoints; the unified Files surface lands with #1398/#1399.
export interface CommodityMeta {
  images?: string[]
  manuals?: string[]
  invoices?: string[]
  images_error?: string
  manuals_error?: string
  invoices_error?: string
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

export type CommoditySortField =
  | "name"
  | "registered_date"
  | "purchase_date"
  | "current_price"
  | "original_price"
  | "count"

// What the list endpoint accepts. The handler treats each field as
// optional + zero-value-means-no-filter, mirroring the BE
// `registry.CommodityListOptions` doc.
export interface ListCommoditiesOptions {
  page?: number
  perPage?: number
  types?: string[]
  statuses?: string[]
  areaId?: string
  search?: string
  // Default false. true surfaces drafts and non-`in_use` commodities;
  // the FE wires this to the "Show inactive" toggle in the toolbar.
  includeInactive?: boolean
  sort?: CommoditySortField
  sortDesc?: boolean
  signal?: AbortSignal
}

function envelope(attrs: Partial<Commodity>) {
  return { data: { type: "commodities", attributes: attrs } }
}

// Returns the commodities for the active group. JSON:API envelope is
// unwrapped here so consumers see a plain Commodity[]. The total comes
// from the `meta.commodities` field, used by paginators.
export async function listCommodities(
  options: ListCommoditiesOptions = {}
): Promise<{ commodities: Commodity[]; total: number }> {
  const params = new URLSearchParams()
  if (options.page !== undefined) params.set("page", String(options.page))
  if (options.perPage !== undefined) params.set("per_page", String(options.perPage))
  for (const t of options.types ?? []) params.append("type", t)
  for (const s of options.statuses ?? []) params.append("status", s)
  if (options.areaId) params.set("area_id", options.areaId)
  if (options.search?.trim()) params.set("q", options.search.trim())
  if (options.includeInactive) params.set("include_inactive", "true")
  if (options.sort) {
    params.set("sort", options.sortDesc ? `-${options.sort}` : options.sort)
  }
  const qs = params.toString()
  const path = qs ? `/commodities?${qs}` : "/commodities"
  const body = await http.get<CommoditiesListResponse>(path, { signal: options.signal })
  return {
    commodities: (body.data ?? []).map((item) => ({ ...item.attributes, id: item.id })),
    total: body.meta?.commodities ?? body.data?.length ?? 0,
  }
}

// Fetches a single commodity with its attached file lists (legacy-shape,
// pending #1398's unified Files API).
export async function getCommodity(
  id: string,
  signal?: AbortSignal
): Promise<{ commodity: Commodity; meta: CommodityMeta }> {
  const body = await http.get<CommodityResponseEnvelope>(
    `/commodities/${encodeURIComponent(id)}`,
    { signal }
  )
  if (!body.data?.attributes) {
    throw new Error(`Commodity ${id} response missing data.attributes`)
  }
  return {
    commodity: { ...body.data.attributes, id: body.data.id },
    meta: body.data.meta ?? {},
  }
}

// CreateCommodityRequest mirrors the BE's CommodityRequest envelope's
// `attributes` shape. Optional fields can be omitted; required ones
// (name, type, area_id, status) match the model's NOT NULL columns.
export type CreateCommodityRequest = Partial<Commodity> & {
  name: string
  type: string
  area_id: string
  status: string
  count: number
}

export async function createCommodity(req: CreateCommodityRequest): Promise<Commodity> {
  const body = await http.post<CommodityResponseEnvelope>("/commodities", envelope(req))
  if (!body.data?.attributes) {
    throw new Error("Create-commodity response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

export type UpdateCommodityRequest = Partial<Commodity>

export async function updateCommodity(
  id: string,
  req: UpdateCommodityRequest
): Promise<Commodity> {
  const body = await http.put<CommodityResponseEnvelope>(
    `/commodities/${encodeURIComponent(id)}`,
    envelope(req)
  )
  if (!body.data?.attributes) {
    throw new Error("Update-commodity response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

export async function deleteCommodity(id: string): Promise<void> {
  await http.del<void>(`/commodities/${encodeURIComponent(id)}`)
}

// Bulk delete: BE accepts `{ ids: [...] }` and returns no body on 204.
export async function bulkDeleteCommodities(ids: string[]): Promise<void> {
  await http.post<void>("/commodities/bulk-delete", { ids })
}

// Bulk move: relocates the listed commodities to a single target area.
// BE returns the updated rows; we don't propagate them — the optimistic
// cache update in the hook is enough.
export async function bulkMoveCommodities(ids: string[], areaId: string): Promise<void> {
  await http.post<void>("/commodities/bulk-move", { ids, area_id: areaId })
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
