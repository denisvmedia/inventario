// Pure data-layer functions for the supply-links feature slice (#1369).
// Hooks live in `./hooks.ts`. Backed by the per-commodity
// `/commodities/{id}/supplies` endpoint family.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type SupplyLinkEntity = Schema<"models.SupplyLink">

interface ListEnvelope {
  data?: Array<SupplyLinkEntity & { id: string }>
  meta?: { supply_links?: number; total?: number }
}

interface DetailEnvelope {
  data?: {
    id?: string
    type?: string
    attributes?: SupplyLinkEntity
  }
}

export interface ListSupplyLinksResult {
  links: Array<SupplyLinkEntity & { id: string }>
  total: number
}

export async function listSupplyLinks(
  commodityID: string,
  signal?: AbortSignal
): Promise<ListSupplyLinksResult> {
  const body = await http.get<ListEnvelope>(
    `/commodities/${encodeURIComponent(commodityID)}/supplies`,
    { signal }
  )
  return {
    links: body.data ?? [],
    total: body.meta?.total ?? body.data?.length ?? 0,
  }
}

export interface CreateSupplyLinkRequest {
  commodity_id: string
  label: string
  url: string
  notes?: string
}

export async function createSupplyLink(req: CreateSupplyLinkRequest): Promise<SupplyLinkEntity & { id: string }> {
  const body = await http.post<DetailEnvelope>(
    `/commodities/${encodeURIComponent(req.commodity_id)}/supplies`,
    {
      data: {
        type: "commodity_supply_links",
        attributes: {
          label: req.label,
          url: req.url,
          notes: req.notes ?? "",
        },
      },
    }
  )
  return extractLink(body)
}

export interface UpdateSupplyLinkRequest {
  commodity_id: string
  supply_id: string
  label?: string
  url?: string
  notes?: string
}

export async function updateSupplyLink(req: UpdateSupplyLinkRequest): Promise<SupplyLinkEntity & { id: string }> {
  const attributes: Record<string, unknown> = {}
  if (req.label !== undefined) attributes.label = req.label
  if (req.url !== undefined) attributes.url = req.url
  if (req.notes !== undefined) attributes.notes = req.notes
  const body = await http.patch<DetailEnvelope>(
    `/commodities/${encodeURIComponent(req.commodity_id)}/supplies/${encodeURIComponent(req.supply_id)}`,
    {
      data: {
        type: "commodity_supply_links",
        attributes,
      },
    }
  )
  return extractLink(body)
}

export interface DeleteSupplyLinkRequest {
  commodity_id: string
  supply_id: string
}

export async function deleteSupplyLink(req: DeleteSupplyLinkRequest): Promise<void> {
  await http.del(
    `/commodities/${encodeURIComponent(req.commodity_id)}/supplies/${encodeURIComponent(req.supply_id)}`
  )
}

export interface ReorderSupplyLinksRequest {
  commodity_id: string
  ids: string[]
}

export async function reorderSupplyLinks(req: ReorderSupplyLinksRequest): Promise<ListSupplyLinksResult> {
  const body = await http.post<ListEnvelope>(
    `/commodities/${encodeURIComponent(req.commodity_id)}/supplies/reorder`,
    {
      data: {
        type: "commodity_supply_links_reorder",
        attributes: { ids: req.ids },
      },
    }
  )
  return {
    links: body.data ?? [],
    total: body.meta?.total ?? body.data?.length ?? 0,
  }
}

// JSON:API puts the resource id at `data.id`. Don't accept a nested
// `attributes.id` — masking a BE bug there hides exactly the regression
// a typed envelope is supposed to catch.
function extractLink(body: DetailEnvelope): SupplyLinkEntity & { id: string } {
  const id = body.data?.id ?? ""
  if (!id) {
    throw new Error("Supply link response missing id")
  }
  const attrs = body.data?.attributes ?? ({} as SupplyLinkEntity)
  return { ...attrs, id }
}
