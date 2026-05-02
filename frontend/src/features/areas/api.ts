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
  options: { perPage?: number; signal?: AbortSignal } = {}
): Promise<Area[]> {
  const params = new URLSearchParams()
  if (options.perPage !== undefined) params.set("per_page", String(options.perPage))
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
}

export async function updateArea(id: string, req: UpdateAreaRequest): Promise<Area> {
  const body = await http.put<AreaResponse>(`/areas/${encodeURIComponent(id)}`, envelope(req))
  if (!body.data?.attributes) {
    throw new Error("Update-area response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

export async function deleteArea(id: string): Promise<void> {
  await http.del<void>(`/areas/${encodeURIComponent(id)}`)
}
