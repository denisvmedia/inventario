// Pure data-layer functions for the commodities feature slice. Hooks live
// in `./hooks.ts`. The dashboard (#1408) reads `listCommodities()` /
// `getCommoditiesValue()`; the items page (#1410) extends with full CRUD
// + bulk + filter/sort/search query params (BE shipped together with the
// FE in #1410).
import { pinnedPost, sendInChunks } from "@/lib/bulk-chunks"
import { http } from "@/lib/http"
import type { Schema } from "@/types"

import { listOptionsToParams, type CommoditySearchMatch, type TagMatchMode } from "./query"

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
    matches?: Record<string, CommoditySearchMatch[]>
    selection_digest?: string
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

interface ValueResponseTotal {
  id?: string
  name?: string
  value?: number
  unvalued_count?: number
}

interface ValueResponse {
  data?: {
    // Mirrors jsonapi.NamedTotal in src/types/api.d.ts. The BE emits
    // `value` (not `total`) and includes the area/location `id` so
    // consumers can match without relying on names — area names are
    // not unique on the BE (`uuid` is the only unique column).
    attributes?: {
      area_totals?: ValueResponseTotal[]
      global_total?: number
      location_totals?: ValueResponseTotal[]
      unvalued_count?: number
      valued_count?: number
    }
  }
}

export type CommoditySortField =
  "name" | "registered_date" | "purchase_date" | "current_price" | "original_price" | "count"

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
  // Sorts by this custom field (#2803) instead of sort, in the direction of
  // sortDesc. The API param is `sort=cf.<field id>`.
  customFieldSort?: string
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
  // Unassigned filter (#1986). undefined = no filter, true = only items
  // with no area ("no location"). The toolbar chip toggles between
  // undefined and true; ignored by the BE when areaId is also set.
  unassigned?: boolean
  // Seller filter (#2804), by seller id; OR-ed at the BE. The API param is
  // `seller_id` (repeatable).
  sellerIds?: string[]
  // Tag filter, by slug; see CommodityTagFilter in ./query. noTags cannot
  // be combined with tags or tagsExclude (the BE answers 400).
  tags?: string[]
  tagMode?: TagMatchMode
  tagsExclude?: string[]
  noTags?: boolean
  // Missing-data filters (#2794): items without a photo, with an unknown
  // purchase price, with an unknown purchase date. A known zero price is a
  // price. noPhoto cannot be combined with trashed (the BE answers 400).
  noPhoto?: boolean
  noPurchasePrice?: boolean
  noPurchaseDate?: boolean
  // Custom field conditions (#2803), each "<field id>:<op>[:<value>]"; the
  // BE AND-s them. The API param is `cf` (repeatable).
  customFields?: string[]
  // true selects the items in Trash instead of the live ones. Not part of
  // the page URL: the items list shows live items only. Cannot be combined
  // with lentOut or noPhoto (the BE answers 400).
  trashed?: boolean
  // Values the page URL carried for a boolean filter or tag_mode that could
  // not be read, by API parameter name. Sent as written so the BE answers
  // 400 query.invalid_parameter; see CommodityQuery.unreadable.
  unreadable?: Record<string, string>
  signal?: AbortSignal
}

function envelope(attrs: Partial<Commodity>) {
  return { data: { type: "commodities", attributes: attrs } }
}

// Returns the commodities for the active group. JSON:API envelope is
// unwrapped here so consumers see a plain Commodity[]. The total comes
// from the `meta.commodities` field, used by paginators. `covers` carries
// the per-id resolved cover descriptors parsed from `meta.covers` —
// additive (#1370): existing consumers ignore it, and each commodity also
// already carries its own `cover` field; the standalone map is exposed so
// the insurance report's Location mode can render each item's cover
// thumbnail without re-deriving from the list rows.
//
// `matches` carries `meta.matches`: per commodity id, the fields the search
// text was found in. It is empty when the request has no search text.
//
// `selectionDigest` carries `meta.selection_digest`: a digest of the ids of
// every item the query matches, on every page, which a bulk action on "all
// matching" sends back (CommoditySelection). It is empty when a bulk action
// cannot take the query (no items, more than MAX_BULK_QUERY_ITEMS, Trash).
export async function listCommodities(options: ListCommoditiesOptions = {}): Promise<{
  commodities: Commodity[]
  total: number
  covers: Record<string, CommodityCover>
  matches: Record<string, CommoditySearchMatch[]>
  selectionDigest: string
}> {
  const qs = listOptionsToParams(options).toString()
  const path = qs ? `/commodities?${qs}` : "/commodities"
  const body = await http.get<CommoditiesListResponse>(path, { signal: options.signal })
  const coverPayloads = body.meta?.covers ?? {}
  const covers: Record<string, CommodityCover> = {}
  for (const [id, payload] of Object.entries(coverPayloads)) {
    const normalized = normalizeCover(payload)
    if (normalized) covers[id] = normalized
  }
  return {
    commodities: (body.data ?? []).map((item) => ({
      ...item.attributes,
      id: item.id,
      cover: covers[item.id],
    })),
    total: body.meta?.commodities ?? body.data?.length ?? 0,
    covers,
    matches: body.meta?.matches ?? {},
    selectionDigest: body.meta?.selection_digest ?? "",
  }
}

// Per tag slug, how many items a query selects once its tag filter is set
// aside, plus how many have no tags (GET /commodities/tag-counts).
export type CommodityTagCounts = Schema<"registry.CommodityTagCounts">

// The options that reach the tag counts: the query without paging, sort
// or tag filter, which the endpoint would ignore or set aside. An unreadable
// tag_mode or no_tags is part of the tag filter and goes with it.
export function tagCountsOptions(options: ListCommoditiesOptions): ListCommoditiesOptions {
  const unreadable = Object.fromEntries(
    Object.entries(options.unreadable ?? {}).filter(
      ([name]) => name !== "tag_mode" && name !== "no_tags"
    )
  )
  return {
    ...options,
    page: undefined,
    perPage: undefined,
    sort: undefined,
    sortDesc: undefined,
    customFieldSort: undefined,
    tags: undefined,
    tagMode: undefined,
    tagsExclude: undefined,
    noTags: undefined,
    unreadable: Object.keys(unreadable).length > 0 ? unreadable : undefined,
    signal: undefined,
  }
}

export async function getCommodityTagCounts(
  options: ListCommoditiesOptions = {}
): Promise<{ tags: Record<string, number>; untagged: number }> {
  const qs = listOptionsToParams(tagCountsOptions(options)).toString()
  const path = qs ? `/commodities/tag-counts?${qs}` : "/commodities/tag-counts"
  const body = await http.get<{ data?: CommodityTagCounts }>(path, { signal: options.signal })
  return { tags: body.data?.tags ?? {}, untagged: body.data?.untagged ?? 0 }
}

// A brand the group's items carry and how many carry it
// (GET /commodities/brands).
export type CommodityBrandCount = Schema<"registry.CommodityBrandCount">

// listCommodityBrands returns the brands the group's items carry that
// contain query (any case), most used first, at most limit of them.
export async function listCommodityBrands(
  query: string,
  limit: number,
  signal?: AbortSignal
): Promise<CommodityBrandCount[]> {
  const params = new URLSearchParams()
  if (query.trim()) params.set("q", query.trim())
  params.set("limit", String(limit))
  const body = await http.get<{ data?: CommodityBrandCount[] }>(
    `/commodities/brands?${params.toString()}`,
    { signal }
  )
  return body.data ?? []
}

// normalizeCover folds a BE `meta.covers[id]` payload into the FE's
// `CommodityCover` shape. Returns undefined when the payload is missing
// or doesn't carry the minimum (`file_id` + at least one thumbnail) the
// renderer needs — same condition the FE treats as "fall back to type
// icon", so the absent state is a single `cover === undefined` check.
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
// (name, type, status, count) match the model's NOT NULL columns.
// area_id is optional (#1986/#1987): an item may be created unassigned.
export type CreateCommodityRequest = Partial<Commodity> & {
  name: string
  type: string
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

// Sets (true) or clears (false) the draft mark of an item and changes no
// other field, so an edit saved in the meantime stays. The server refuses
// the mark of an item that a full save would refuse, with the same field
// errors.
export async function setCommodityDraftMark(id: string, draft: boolean): Promise<Commodity> {
  const req: Schema<"jsonapi.CommodityDraftMarkRequest"> = {
    data: { type: "commodity_draft_mark", attributes: { draft } },
  }
  const body = await http.patch<CommodityResponseEnvelope>(
    `/commodities/${encodeURIComponent(id)}/draft`,
    req
  )
  if (!body.data?.attributes) {
    throw new Error(`setCommodityDraftMark ${id} response missing data.attributes`)
  }
  return { ...body.data.attributes, id: body.data.id }
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
  | "restored"

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

// The items of a bulk action: the ids the user picked, or every item a list
// query matches ("Select all N matching"). A query goes in the API form of
// the item query contract (commodityQueryParam) with the number of items the
// user saw it match and the list's selection digest of them. The server runs
// the query again and refuses with 409 bulk.selection_changed when it
// selects other items (more, fewer, or as many with one swapped), so a
// confirmed action never reaches items the user did not see.
export type CommoditySelection =
  { ids: string[] } | { query: string; expectedCount: number; expectedDigest: string }

// The server's limits on one bulk request (go/jsonapi/bulk.go and
// go/services/commodity_bulk_service.go).
export const MAX_BULK_IDS = 100
export const MAX_BULK_QUERY_ITEMS = 500

export type CommodityBulkPatch = Schema<"jsonapi.CommodityBulkPatch">
export type CommodityBulkStatusValue = NonNullable<
  Schema<"jsonapi.CommodityBulkStatusPatch">["value"]
>
// The server's codes, plus two for an action sent in several requests that
// stopped when one failed: unknown, the item was in the request that failed,
// so it may have changed; not_sent, it was in a later request, so it did not.
export type CommodityBulkFailureCode =
  NonNullable<Schema<"jsonapi.BulkResultFail">["code"]> | "unknown" | "not_sent"

export interface CommodityBulkFailure {
  id: string
  name: string
  code: CommodityBulkFailureCode
  /** The server's English message. */
  error: string
  /** With invalid: the field the validation refused, e.g. short_name. */
  field?: string
  /** With invalid: the validator's code, e.g. validation_required. */
  reason?: string
}

// What a bulk action did to each item: changed it, left it as it was
// because it already had what the action sets, or could not change it.
// Notes add to changed items: files_kept says a deleted item went to Trash
// while some of its files stayed live.
export interface CommodityBulkResult {
  succeeded: string[]
  skipped: { id: string; name: string }[]
  failed: CommodityBulkFailure[]
  notes: { id: string; name: string; code: string }[]
}

type BulkResultEnvelope = Schema<"jsonapi.BulkResultResponse">

function emptyBulkResult(): CommodityBulkResult {
  return { succeeded: [], skipped: [], failed: [], notes: [] }
}

function readBulkResult(body: BulkResultEnvelope | undefined): CommodityBulkResult {
  const attrs = body?.data?.attributes
  return {
    succeeded: attrs?.succeeded ?? [],
    skipped: (attrs?.skipped ?? []).map((s) => ({ id: s.id ?? "", name: s.name ?? "" })),
    failed: (attrs?.failed ?? []).map((f) => ({
      id: f.id ?? "",
      name: f.name ?? "",
      code: f.code ?? "failed",
      error: f.error ?? "",
      field: f.field || undefined,
      reason: f.reason || undefined,
    })),
    notes: (attrs?.notes ?? []).map((n) => ({
      id: n.id ?? "",
      name: n.name ?? "",
      code: n.code ?? "",
    })),
  }
}

// postBulk sends one bulk action. An id list longer than the server takes
// in one request goes in several, one after the other, all to the group
// active when the action started, and the results are merged. When a
// request fails without an answer, its items are reported as unknown, since
// the server may have changed some of them, and the items after it as
// not_sent (sendInChunks). A query always goes in one request, since the
// server resolves it.
async function postBulk(
  path: string,
  selection: CommoditySelection,
  extra: Record<string, unknown>
): Promise<CommodityBulkResult> {
  const post = pinnedPost<BulkResultEnvelope>(path)
  const send = async (attributes: Record<string, unknown>) =>
    readBulkResult(
      await post({ data: { type: "commodities", attributes: { ...attributes, ...extra } } })
    )
  if ("query" in selection) {
    return send({
      query: selection.query,
      expected_count: selection.expectedCount,
      expected_digest: selection.expectedDigest,
    })
  }
  const { parts, unknown, unsent } = await sendInChunks(selection.ids, MAX_BULK_IDS, (ids) =>
    send({ ids })
  )
  const merged = emptyBulkResult()
  for (const part of parts) {
    merged.succeeded.push(...part.succeeded)
    merged.skipped.push(...part.skipped)
    merged.failed.push(...part.failed)
    merged.notes.push(...part.notes)
  }
  for (const id of unknown) merged.failed.push({ id, name: "", code: "unknown", error: "" })
  for (const id of unsent) merged.failed.push({ id, name: "", code: "not_sent", error: "" })
  return merged
}

// Moves the selected items to Trash, each as deleting one item does.
export function bulkDeleteCommodities(selection: CommoditySelection): Promise<CommodityBulkResult> {
  return postBulk("/commodities/bulk-delete", selection, {})
}

// Puts the selected items into one area. An item already there is skipped.
export function bulkMoveCommodities(
  selection: CommoditySelection,
  areaId: string
): Promise<CommodityBulkResult> {
  return postBulk("/commodities/bulk-move", selection, { area_id: areaId })
}

// Changes the tags, type or status of the selected items. Tags are added
// and removed, never replaced; a part the patch leaves out stays as it is.
export function bulkUpdateCommodities(
  selection: CommoditySelection,
  patch: CommodityBulkPatch
): Promise<CommodityBulkResult> {
  return postBulk("/commodities/bulk-update", selection, { patch })
}

// Returns the global / per-location / per-area value totals for the
// active group, in the group currency. The dashboard reads only
// `globalValue`; per-location/area breakdowns are kept here for the
// items page (#1410) and the area detail (#1531) to reuse.
//
// Each named total carries the area/location `id` alongside its name
// + value, mirroring `jsonapi.NamedTotal` on the BE. Consumers should
// match by `id` — area names are not unique (only `uuid` is).
//
// A total sums the items whose value is known (the current value, else
// the purchase price in the group currency). Items with neither are left
// out rather than counted as zero; `unvaluedCount` says how many.
export interface NamedTotal {
  id: string
  name: string
  value: number
  unvaluedCount: number
}

export interface CommoditiesValue {
  globalValue: number
  valuedCount: number
  unvaluedCount: number
  locationTotals: NamedTotal[]
  areaTotals: NamedTotal[]
}

export async function getCommoditiesValue(signal?: AbortSignal): Promise<CommoditiesValue> {
  const body = await http.get<ValueResponse>("/commodities/values", { signal })
  const attrs = body.data?.attributes ?? {}
  const mapEntry = (t: ValueResponseTotal): NamedTotal => ({
    id: t.id ?? "",
    name: t.name ?? "",
    value: t.value ?? 0,
    unvaluedCount: t.unvalued_count ?? 0,
  })
  return {
    globalValue: attrs.global_total ?? 0,
    valuedCount: attrs.valued_count ?? 0,
    unvaluedCount: attrs.unvalued_count ?? 0,
    locationTotals: (attrs.location_totals ?? []).map(mapEntry),
    areaTotals: (attrs.area_totals ?? []).map(mapEntry),
  }
}
