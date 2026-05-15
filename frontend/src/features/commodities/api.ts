// Pure data-layer functions for the commodities feature slice. Hooks live
// in `./hooks.ts`. The dashboard (#1408) reads `listCommodities()` /
// `getCommoditiesValue()`; the items page (#1410) extends with full CRUD
// + bulk + filter/sort/search query params (BE shipped together with the
// FE in #1410).
import { http } from "@/lib/http"
import type { Schema } from "@/types"

// CommodityCover is the resolved cover image for a single commodity
// (issue #1451 option A — first photo by `created_at`). The BE returns
// it under `meta.covers[id]` on the list response and `meta.cover` on
// the single-commodity GET; the FE merges it onto the commodity object
// so consumers see one shape regardless of which endpoint they hit.
export interface CommodityCover {
  fileId: string
  thumbnails: Record<string, string>
  source: "first_photo" | "explicit"
}

// Commodity is the BE-generated model with the FE-only `cover` field
// spliced on. The BE model itself doesn't carry the cover — it lives in
// the response meta — so the field is optional and comes from the API
// helpers below.
export type Commodity = Schema<"models.Commodity"> & { cover?: CommodityCover }
export type CommodityType = Schema<"models.CommodityType">
export type CommodityStatus = Schema<"models.CommodityStatus">

interface CommodityResource {
  id: string
  type: string
  attributes: Schema<"models.Commodity">
}

interface CoverPayload {
  file_id?: string
  thumbnails?: Record<string, string>
  source?: string
}

interface CommoditiesListResponse {
  data: CommodityResource[]
  meta?: {
    commodities?: number
    page?: number
    per_page?: number
    total_pages?: number
    covers?: Record<string, CoverPayload>
  }
}

interface CommodityResponseEnvelope {
  data?: {
    id?: string
    type?: string
    attributes?: Schema<"models.Commodity">
    meta?: CommodityMeta
  }
  meta?: { cover?: CoverPayload }
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
    // Mirrors jsonapi.NamedTotal in src/types/api.d.ts. The BE emits
    // `value` (not `total`) and includes the area/location `id` so
    // consumers can match without relying on names — area names are
    // not unique on the BE (`uuid` is the only unique column).
    attributes?: {
      area_totals?: { id?: string; name?: string; value?: number }[]
      global_total?: number
      location_totals?: { id?: string; name?: string; value?: number }[]
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
  // Warranty status filter (#1367). Multi: each entry is OR-ed at the
  // BE; the API param is `warranty_status` (repeatable). Allowed values
  // mirror models.WarrantyStatus on the BE — "active" | "expiring" |
  // "expired" | "none".
  warrantyStatuses?: string[]
  // Restricts to commodities whose `warranty_expires_at` is strictly
  // before this YYYY-MM-DD date. Combined with warrantyStatuses via AND.
  warrantyExpiresBefore?: string
  // Lent-out filter (#1510). undefined = no filter, true = only items
  // currently on an open loan, false = only items NOT currently lent.
  // The toolbar chip toggles between undefined and true.
  lentOut?: boolean
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
  // Always send include_inactive — its absence means "BE default" which
  // is true (= no filter) for legacy compat. The React list page opts
  // IN to the active-only view by sending `false`.
  if (options.includeInactive !== undefined) {
    params.set("include_inactive", options.includeInactive ? "true" : "false")
  }
  if (options.sort) {
    params.set("sort", options.sortDesc ? `-${options.sort}` : options.sort)
  }
  for (const w of options.warrantyStatuses ?? []) params.append("warranty_status", w)
  if (options.warrantyExpiresBefore?.trim()) {
    params.set("warranty_expires_before", options.warrantyExpiresBefore.trim())
  }
  if (options.lentOut !== undefined) {
    params.set("lent_out", options.lentOut ? "true" : "false")
  }
  const qs = params.toString()
  const path = qs ? `/commodities?${qs}` : "/commodities"
  const body = await http.get<CommoditiesListResponse>(path, { signal: options.signal })
  const covers = body.meta?.covers ?? {}
  return {
    commodities: (body.data ?? []).map((item) => ({
      ...item.attributes,
      id: item.id,
      cover: normalizeCover(covers[item.id]),
    })),
    total: body.meta?.commodities ?? body.data?.length ?? 0,
  }
}

// normalizeCover folds a BE `meta.covers[id]` payload into the FE's
// `CommodityCover` shape. Returns undefined when the payload is missing
// or doesn't carry the minimum (`file_id` + at least one thumbnail) the
// renderer needs — same condition the FE treats as "fall back to emoji",
// so the absent state is a single `cover === undefined` check.
function normalizeCover(payload?: CoverPayload): CommodityCover | undefined {
  if (!payload?.file_id || !payload.thumbnails || Object.keys(payload.thumbnails).length === 0) {
    return undefined
  }
  return {
    fileId: payload.file_id,
    thumbnails: payload.thumbnails,
    source: payload.source === "explicit" ? "explicit" : "first_photo",
  }
}

// Fetches a single commodity with its attached file lists (legacy-shape,
// pending #1398's unified Files API).
export async function getCommodity(
  id: string,
  signal?: AbortSignal
): Promise<{ commodity: Commodity; meta: CommodityMeta }> {
  const body = await http.get<CommodityResponseEnvelope>(`/commodities/${encodeURIComponent(id)}`, {
    signal,
  })
  if (!body.data?.attributes) {
    throw new Error(`Commodity ${id} response missing data.attributes`)
  }
  return {
    commodity: {
      ...body.data.attributes,
      id: body.data.id,
      cover: normalizeCover(body.meta?.cover),
    },
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

export async function updateCommodity(id: string, req: UpdateCommodityRequest): Promise<Commodity> {
  // BE rejects PUTs whose body's `data.id` doesn't match the URL id —
  // mirroring the JSON:API contract that callers identify the resource
  // both ways. The shared `envelope()` helper omits `id` because
  // create-time we don't know it yet; for update we splice it in here.
  const body = await http.put<CommodityResponseEnvelope>(`/commodities/${encodeURIComponent(id)}`, {
    data: { ...envelope(req).data, id },
  })
  if (!body.data?.attributes) {
    throw new Error("Update-commodity response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

export async function deleteCommodity(id: string): Promise<void> {
  await http.del<void>(`/commodities/${encodeURIComponent(id)}`)
}

// Sets or clears the explicit cover-photo override for a commodity
// (issue #1451 option B). `fileId === null` clears the override; the
// resolver falls back to the auto-pick first-photo path on the next
// read. Returns the updated commodity (with the new resolved `cover`
// folded in) so the caller can patch its cache without a refetch.
export async function setCommodityCover(
  id: string,
  fileId: string | null
): Promise<{ commodity: Commodity; meta: CommodityMeta }> {
  const body = await http.patch<CommodityResponseEnvelope>(
    `/commodities/${encodeURIComponent(id)}/cover`,
    {
      data: {
        type: "commodity_cover",
        attributes: { file_id: fileId },
      },
    }
  )
  if (!body.data?.attributes) {
    throw new Error(`setCommodityCover ${id} response missing data.attributes`)
  }
  return {
    commodity: {
      ...body.data.attributes,
      id: body.data.id,
      cover: normalizeCover(body.meta?.cover),
    },
    meta: body.data.meta ?? {},
  }
}

// CommodityEventKind mirrors the BE enum (issues #1450 + #1507 + #1508).
// Keep in sync with `models.CommodityEventKind` — the FE renders
// kind-aware copy off this union and unknown kinds fall through to a
// generic "updated" line.
export type CommodityEventKind =
  | "created"
  | "updated"
  | "status_changed"
  | "moved"
  | "price_changed"
  | "cover_changed"
  | "lent_out"
  | "returned"
  | "loan_updated"
  | "sent_for_service"
  | "back_from_service"
  | "service_updated"
  | "deleted"

export interface CommodityEventActor {
  id: string
  name?: string
  email?: string
}

// CommodityEvent is the FE-shaped row of the audit timeline.
export interface CommodityEvent {
  id: string
  commodityId: string
  kind: CommodityEventKind
  occurredAt: string
  before?: Record<string, unknown>
  after?: Record<string, unknown>
  note?: string
  actor?: CommodityEventActor
}

// listCommodityEvents fetches the audit timeline for one commodity. The
// FE always reads newest-first; the BE enforces the order via the
// composite index. `kinds` narrows by event kind and is repeatable on
// the URL — `?kind=status_changed&kind=moved` returns the union.
export async function listCommodityEvents(
  commodityId: string,
  options: {
    page?: number
    perPage?: number
    kinds?: CommodityEventKind[]
    signal?: AbortSignal
  } = {}
): Promise<{ events: CommodityEvent[]; total: number }> {
  const params = new URLSearchParams()
  if (options.page !== undefined) params.set("page", String(options.page))
  if (options.perPage !== undefined) params.set("per_page", String(options.perPage))
  for (const k of options.kinds ?? []) params.append("kind", k)
  const qs = params.toString()
  const path = qs
    ? `/commodities/${encodeURIComponent(commodityId)}/events?${qs}`
    : `/commodities/${encodeURIComponent(commodityId)}/events`
  const body = await http.get<{
    data?: Array<{
      id?: string
      commodity_id?: string
      kind?: CommodityEventKind
      occurred_at?: string
      before?: Record<string, unknown>
      after?: Record<string, unknown>
      note?: string
      meta?: { actor?: CommodityEventActor }
    }>
    meta?: { events?: number; total?: number }
  }>(path, { signal: options.signal })
  return {
    events: (body.data ?? []).map((row) => ({
      id: row.id ?? "",
      commodityId: row.commodity_id ?? "",
      kind: (row.kind ?? "updated") as CommodityEventKind,
      occurredAt: row.occurred_at ?? "",
      before: row.before,
      after: row.after,
      note: row.note,
      actor: row.meta?.actor,
    })),
    total: body.meta?.total ?? body.data?.length ?? 0,
  }
}

// Bulk delete: BE binds `BulkIDsRequest` (jsonapi/bulk.go) which
// requires the `{data:{type,attributes:{ids}}}` JSON:API envelope —
// passing `{ids}` flat used to 422 with "missing data.attributes".
// Discovered while writing the e2e bulk-delete round-trip in #1449.
export async function bulkDeleteCommodities(ids: string[]): Promise<void> {
  await http.post<void>("/commodities/bulk-delete", {
    data: { type: "commodities", attributes: { ids } },
  })
}

// Bulk move: relocates the listed commodities to a single target area.
// Same envelope shape as bulk-delete (BulkMoveRequest, with the
// destination area_id under attributes). BE returns the updated rows;
// we don't propagate them — the optimistic cache update in the hook
// is enough.
export async function bulkMoveCommodities(ids: string[], areaId: string): Promise<void> {
  await http.post<void>("/commodities/bulk-move", {
    data: { type: "commodities", attributes: { ids, area_id: areaId } },
  })
}

// Returns the global / per-location / per-area value totals for the
// active group, in the group currency. The dashboard reads only
// `globalValue`; per-location/area breakdowns are kept here for the
// items page (#1410) and the area detail (#1531) to reuse.
//
// Each named total carries the area/location `id` alongside its name
// + value, mirroring `jsonapi.NamedTotal` on the BE. Consumers should
// match by `id` — area names are not unique (only `uuid` is).
export interface NamedTotal {
  id: string
  name: string
  value: number
}

export interface CommoditiesValue {
  globalValue: number
  locationTotals: NamedTotal[]
  areaTotals: NamedTotal[]
}

export async function getCommoditiesValue(signal?: AbortSignal): Promise<CommoditiesValue> {
  const body = await http.get<ValueResponse>("/commodities/values", { signal })
  const attrs = body.data?.attributes ?? {}
  const mapEntry = (t: { id?: string; name?: string; value?: number }): NamedTotal => ({
    id: t.id ?? "",
    name: t.name ?? "",
    value: t.value ?? 0,
  })
  return {
    globalValue: attrs.global_total ?? 0,
    locationTotals: (attrs.location_totals ?? []).map(mapEntry),
    areaTotals: (attrs.area_totals ?? []).map(mapEntry),
  }
}
