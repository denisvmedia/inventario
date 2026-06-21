// Pure data-layer for areas. An area belongs to exactly one location
// (`location_id` on the BE model). Hooks live in `./hooks.ts`.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type Area = Schema<"models.Area">

interface AreaResource {
  id: string
  type: string
  attributes: Area
}

interface AreasListResponse {
  data: AreaResource[]
  meta?: {
    areas?: number
    page?: number
    per_page?: number
    total_pages?: number
  }
}

interface AreaResponse {
  data?: { id?: string; type?: string; attributes?: Area }
}

function envelope(attrs: Partial<Area>) {
  return { data: { type: "areas", attributes: attrs } }
}

export async function listAreas(
  options: { perPage?: number; locationId?: string; signal?: AbortSignal } = {}
): Promise<Area[]> {
  const params = new URLSearchParams()
  if (options.perPage !== undefined) params.set("per_page", String(options.perPage))
  // `?location_id=` (#1473) restricts the result to a single location;
  // unknown / cross-tenant ids return an empty list, not a 4xx, so callers
  // don't need to special-case errors here.
  if (options.locationId) params.set("location_id", options.locationId)
  const qs = params.toString()
  const path = qs ? `/areas?${qs}` : "/areas"
  const body = await http.get<AreasListResponse>(path, { signal: options.signal })
  return (body.data ?? []).map((item) => ({ ...item.attributes, id: item.id }))
}

export async function getArea(id: string, signal?: AbortSignal): Promise<Area> {
  const body = await http.get<AreaResponse>(`/areas/${encodeURIComponent(id)}`, { signal })
  if (!body.data?.attributes) {
    throw new Error(`Area ${id} response missing data.attributes`)
  }
  return { ...body.data.attributes, id: body.data.id }
}

export interface CreateAreaRequest {
  name: string
  location_id: string
  // Short visual token (emoji) for the avatar tile on the location
  // detail's area grid; empty string ⇒ generic Package fallback.
  icon?: string
}

export async function createArea(req: CreateAreaRequest): Promise<Area> {
  const body = await http.post<AreaResponse>("/areas", envelope(req))
  if (!body.data?.attributes) {
    throw new Error("Create-area response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

export interface UpdateAreaRequest {
  name?: string
  location_id?: string
  icon?: string
}

export async function updateArea(id: string, req: UpdateAreaRequest): Promise<Area> {
  // BE rejects PUTs whose body's `data.id` doesn't match the URL id
  // (apiserver/areas.go:212). The shared `envelope()` helper omits
  // `id` because create-time we don't know it yet; splice it in here
  // for update. Same pattern as commodities + locations.
  const body = await http.put<AreaResponse>(`/areas/${encodeURIComponent(id)}`, {
    data: { ...envelope(req).data, id },
  })
  if (!body.data?.attributes) {
    throw new Error("Update-area response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

// Strategy for deleting a non-empty container (#2137). Absent ⇒ the
// BE's safe default that 422s when the area still holds items.
//   cascade — delete the items inside (and their files).
//   unlink  — un-assign the items (they become un-located) and delete
//             the area; the items survive.
export type DeleteStrategy = "cascade" | "unlink"

export async function deleteArea(id: string, strategy?: DeleteStrategy): Promise<void> {
  const qs = strategy ? `?strategy=${encodeURIComponent(strategy)}` : ""
  await http.del<void>(`/areas/${encodeURIComponent(id)}${qs}`)
}
