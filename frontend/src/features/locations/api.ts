// Pure data-layer for locations. The BE models a location as
// `name + address + icon + description` (icon + description added by
// #1531 item 4, alongside the long-standing name + address). Hooks
// live in `./hooks.ts`.
import { http } from "@/lib/http"
import { listAllPages } from "@/lib/list-all-pages"
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

// Returns every location of the active group. The endpoint pages its
// result, so this reads page after page until it has them all; pickers
// and name lookups need the whole set.
export async function listLocations(options: { signal?: AbortSignal } = {}): Promise<Location[]> {
  return listAllPages(async (page, perPage) => {
    const params = new URLSearchParams({ page: String(page), per_page: String(perPage) })
    const body = await http.get<LocationsListResponse>(`/locations?${params.toString()}`, {
      signal: options.signal,
    })
    return {
      rows: (body.data ?? []).map((item) => ({ ...item.attributes, id: item.id })),
      total: body.meta?.locations,
    }
  })
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
  // Free-text physical address. Empty string is allowed.
  address?: string
  // Short visual token (emoji) for the avatar tile; empty string ⇒
  // generic MapPin fallback.
  icon?: string
  // One-line muted subtitle under the name on the list / detail.
  description?: string
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
  icon?: string
  description?: string
}

export async function updateLocation(id: string, req: UpdateLocationRequest): Promise<Location> {
  // BE rejects PUTs whose body's `data.id` doesn't match the URL id —
  // mirroring the JSON:API contract that callers identify the resource
  // both ways (apiserver/locations.go:229). The shared `envelope()`
  // helper omits `id` because create-time we don't know it yet; for
  // update we splice it in here. Same pattern as commodities.
  const body = await http.put<LocationResponse>(`/locations/${encodeURIComponent(id)}`, {
    data: { ...envelope(req).data, id },
  })
  if (!body.data?.attributes) {
    throw new Error("Update-location response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

// Strategy for deleting a non-empty container (#2137). Absent ⇒ the
// BE's safe default that 422s when the location still holds areas/items.
//   cascade — delete the items inside (and their files).
//   unlink  — un-assign the items (they become un-located) and delete
//             the location; its areas are also removed, but the items
//             survive.
export type DeleteStrategy = "cascade" | "unlink"

export async function deleteLocation(id: string, strategy?: DeleteStrategy): Promise<void> {
  const qs = strategy ? `?strategy=${encodeURIComponent(strategy)}` : ""
  await http.del<void>(`/locations/${encodeURIComponent(id)}${qs}`)
}
