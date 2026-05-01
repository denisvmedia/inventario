// Pure data-layer for locations. The Inventario backend models a
// location as `name + address`; the design mock's "icon" / "description"
// pair maps to "address" (no icon column server-side). Hooks live in
// `./hooks.ts`.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type Location = Schema<"models.Location">

interface LocationResource {
  id: string
  type: string
  attributes: Location
}

interface LocationsListResponse {
  data: LocationResource[]
  meta?: {
    locations?: number
    page?: number
    per_page?: number
    total_pages?: number
  }
}

interface LocationResponse {
  data?: { id?: string; type?: string; attributes?: Location }
}

function envelope(attrs: Partial<Location>) {
  return { data: { type: "locations", attributes: attrs } }
}

// Returns every location for the active group. The list is small
// (typically single-digit) so the dashboard / location picker can
// reasonably load it in one go without paging.
export async function listLocations(
  options: { perPage?: number; signal?: AbortSignal } = {}
): Promise<Location[]> {
  const params = new URLSearchParams()
  if (options.perPage !== undefined) params.set("per_page", String(options.perPage))
  const qs = params.toString()
  const path = qs ? `/locations?${qs}` : "/locations"
  const body = await http.get<LocationsListResponse>(path, { signal: options.signal })
  return (body.data ?? []).map((item) => ({ ...item.attributes, id: item.id }))
}

export async function getLocation(id: string, signal?: AbortSignal): Promise<Location> {
  const body = await http.get<LocationResponse>(`/locations/${encodeURIComponent(id)}`, { signal })
  if (!body.data?.attributes) {
    throw new Error(`Location ${id} response missing data.attributes`)
  }
  return { ...body.data.attributes, id: body.data.id }
}

export interface CreateLocationRequest {
  name: string
  // Free-text "where is this" (the mock calls it "description"; the BE
  // calls it "address"). Empty string is allowed — the mock has no
  // required-field constraint here.
  address?: string
}

export async function createLocation(req: CreateLocationRequest): Promise<Location> {
  const body = await http.post<LocationResponse>("/locations", envelope(req))
  if (!body.data?.attributes) {
    throw new Error("Create-location response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

export interface UpdateLocationRequest {
  name?: string
  address?: string
}

export async function updateLocation(id: string, req: UpdateLocationRequest): Promise<Location> {
  const body = await http.put<LocationResponse>(
    `/locations/${encodeURIComponent(id)}`,
    envelope(req)
  )
  if (!body.data?.attributes) {
    throw new Error("Update-location response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

export async function deleteLocation(id: string): Promise<void> {
  await http.del<void>(`/locations/${encodeURIComponent(id)}`)
}
