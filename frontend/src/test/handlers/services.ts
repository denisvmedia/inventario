import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// ServiceAttrs mirrors models.CommodityService — `commodity_id`,
// `provider_*`, `reason`, dates, optional cost pair, `returned_at`
// (nullable while open).
type ServiceAttrs = {
  id: string
  commodity_id: string
  provider_name: string
  provider_contact?: string
  reason?: string
  sent_at: string
  expected_return_at?: string | null
  returned_at?: string | null
  cost_amount?: string
  cost_currency?: string
}

type ServiceWithCommodity = ServiceAttrs & {
  commodity?: { id: string; name: string; short_name?: string }
}

export function listForCommodity(
  slug: string,
  commodityID: string,
  items: ServiceAttrs[] = []
) {
  return [
    http.get(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/services`
      ),
      () =>
        HttpResponse.json({
          data: items,
          meta: { services: items.length, total: items.length },
        })
    ),
  ]
}

export function listGroup(slug: string, items: ServiceWithCommodity[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/services`), () =>
      HttpResponse.json({
        data: items,
        meta: { services: items.length, total: items.length },
      })
    ),
  ]
}

export function counts(slug: string, byCommodity: Record<string, number> = {}) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/services/counts`), () =>
      HttpResponse.json({ data: byCommodity })
    ),
  ]
}

export function startService(slug: string, commodityID: string, response: ServiceAttrs) {
  return [
    http.post(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/services`
      ),
      () =>
        HttpResponse.json(
          { id: response.id, type: "commodity_services", attributes: response },
          { status: 201 }
        )
    ),
  ]
}

export function returnService(
  slug: string,
  commodityID: string,
  serviceID: string,
  response: ServiceAttrs
) {
  return [
    http.post(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/services/${encodeURIComponent(serviceID)}/return`
      ),
      () =>
        HttpResponse.json({ id: response.id, type: "commodity_services", attributes: response })
    ),
  ]
}

export function deleteService(slug: string, commodityID: string, serviceID: string) {
  return [
    http.delete(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/services/${encodeURIComponent(serviceID)}`
      ),
      () => new HttpResponse(null, { status: 204 })
    ),
  ]
}
