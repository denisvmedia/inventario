// Pure data-layer functions for the commodity-services feature slice
// (#1508). Sibling to features/loans — same shape, distinct namespace
// because the FE renders Lend and Service flows as separate UI surfaces.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type ServiceEntity = Schema<"models.CommodityService">
export type ServiceState = "all" | "open" | "overdue" | "completed"

// ServiceCommodityRef mirrors the BE's tiny denormalised commodity
// summary returned alongside service rows on the group-wide endpoint.
export interface ServiceCommodityRef {
  id: string
  name: string
  short_name?: string
}

export interface ListedService {
  service: ServiceEntity & { id: string }
  commodity?: ServiceCommodityRef
}

// Single-service response envelope mirrors the project-wide JSON:API
// shape (`{data: {id, type, attributes}}`) — same as commodities,
// areas, files, loans (post-#1510). The previous flat shape was a bug
// on the BE side; corrected together with the `data` wrapping.
interface ServiceDetailEnvelope {
  data?: {
    id?: string
    type?: string
    attributes?: ServiceEntity
  }
}

interface PerCommodityListEnvelope {
  data?: Array<ServiceEntity & { id: string }>
  meta?: { services?: number; total?: number }
}

interface GroupListEnvelope {
  data?: Array<ServiceEntity & { id: string; commodity?: ServiceCommodityRef }>
  meta?: { services?: number; total?: number }
}

interface ServiceCountsEnvelope {
  data?: Record<string, number>
}

export interface ListServicesForCommodityResult {
  services: Array<ServiceEntity & { id: string }>
  total: number
}

export async function listServicesForCommodity(
  commodityID: string,
  signal?: AbortSignal
): Promise<ListServicesForCommodityResult> {
  const body = await http.get<PerCommodityListEnvelope>(
    `/commodities/${encodeURIComponent(commodityID)}/services`,
    { signal }
  )
  return {
    services: body.data ?? [],
    total: body.meta?.total ?? body.data?.length ?? 0,
  }
}

export interface ListGroupServicesOptions {
  page?: number
  perPage?: number
  state?: ServiceState
  signal?: AbortSignal
}

export async function listGroupServices(
  opts: ListGroupServicesOptions = {}
): Promise<{ services: ListedService[]; total: number }> {
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  if (opts.state) params.set("state", opts.state)
  const qs = params.toString()
  const path = qs ? `/services?${qs}` : "/services"
  const body = await http.get<GroupListEnvelope>(path, { signal: opts.signal })
  return {
    services: (body.data ?? []).map((row) => {
      const { commodity, ...rest } = row
      return { service: rest, commodity }
    }),
    total: body.meta?.total ?? body.data?.length ?? 0,
  }
}

export async function getServiceCounts(
  commodityIDs: string[],
  signal?: AbortSignal
): Promise<Record<string, number>> {
  if (commodityIDs.length === 0) return {}
  const params = new URLSearchParams()
  for (const id of commodityIDs) params.append("commodity_id", id)
  const body = await http.get<ServiceCountsEnvelope>(`/services/counts?${params.toString()}`, {
    signal,
  })
  return body.data ?? {}
}

export interface StartServiceRequest {
  commodity_id: string
  provider_name: string
  provider_contact?: string
  reason?: string
  sent_at: string // YYYY-MM-DD
  expected_return_at?: string | null
  cost_amount?: string // decimal string for precision
  cost_currency?: string // ISO 4217
}

export async function startService(
  req: StartServiceRequest
): Promise<ServiceEntity & { id: string }> {
  const { commodity_id, ...attrs } = req
  const body = await http.post<ServiceDetailEnvelope>(
    `/commodities/${encodeURIComponent(commodity_id)}/services`,
    {
      data: { type: "commodity_services", attributes: attrs },
    }
  )
  if (!body.data?.attributes) {
    throw new Error(`Malformed POST /services response: missing data.attributes`)
  }
  return { ...body.data.attributes, id: body.data.id ?? "" }
}

export interface UpdateServiceRequest {
  provider_name?: string
  provider_contact?: string
  reason?: string
  expected_return_at?: string
  cost_amount?: string
  cost_currency?: string
}

export async function updateService(
  commodityID: string,
  serviceID: string,
  req: UpdateServiceRequest
): Promise<ServiceEntity & { id: string }> {
  const body = await http.patch<ServiceDetailEnvelope>(
    `/commodities/${encodeURIComponent(commodityID)}/services/${encodeURIComponent(serviceID)}`,
    {
      data: { id: serviceID, type: "commodity_services", attributes: req },
    }
  )
  if (!body.data?.attributes) {
    throw new Error(`Malformed PATCH /services/${serviceID} response: missing data.attributes`)
  }
  return { ...body.data.attributes, id: body.data.id ?? serviceID }
}

// returnedAt defaults to today (server-side). Optional finalCost +
// finalCurrency let the caller record the repair bill on the same call.
export async function returnService(
  commodityID: string,
  serviceID: string,
  options: {
    returnedAt?: string
    costAmount?: string
    costCurrency?: string
  } = {}
): Promise<ServiceEntity & { id: string }> {
  const hasAny = options.returnedAt || options.costAmount || options.costCurrency
  const payload = hasAny
    ? {
        data: {
          type: "commodity_services",
          attributes: {
            ...(options.returnedAt ? { returned_at: options.returnedAt } : {}),
            ...(options.costAmount ? { cost_amount: options.costAmount } : {}),
            ...(options.costCurrency ? { cost_currency: options.costCurrency } : {}),
          },
        },
      }
    : undefined
  const body = await http.post<ServiceDetailEnvelope>(
    `/commodities/${encodeURIComponent(commodityID)}/services/${encodeURIComponent(serviceID)}/return`,
    payload
  )
  if (!body.data?.attributes) {
    throw new Error(
      `Malformed POST /services/${serviceID}/return response: missing data.attributes`
    )
  }
  return { ...body.data.attributes, id: body.data.id ?? serviceID }
}

export async function deleteService(commodityID: string, serviceID: string): Promise<void> {
  await http.del<void>(
    `/commodities/${encodeURIComponent(commodityID)}/services/${encodeURIComponent(serviceID)}`
  )
}

// Display helpers — mirror the BE methods on models.CommodityService so
// the FE-side filtering stays consistent with the /services state list.
export function isOpen(svc: Pick<ServiceEntity, "returned_at">): boolean {
  return !svc.returned_at
}

export function daysOverdue(
  svc: Pick<ServiceEntity, "expected_return_at" | "returned_at">,
  now: Date = new Date()
): number {
  if (!isOpen(svc) || !svc.expected_return_at) return 0
  const due = new Date(`${svc.expected_return_at}T00:00:00`)
  if (Number.isNaN(due.getTime())) return 0
  const diff = now.getTime() - due.getTime()
  if (diff <= 0) return 0
  return Math.floor(diff / (1000 * 60 * 60 * 24))
}

// hasCost reports whether the service row carries a recorded cost. Both
// fields are paired on the BE so checking either is sufficient — but
// the FE list-page formatting reads both; this is a clarity helper.
export function hasCost(svc: Pick<ServiceEntity, "cost_amount" | "cost_currency">): boolean {
  if (!svc.cost_amount || !svc.cost_currency) return false
  const num = Number(svc.cost_amount)
  return !Number.isNaN(num) && num !== 0
}
