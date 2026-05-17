import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// Supply-link attributes mirror models.SupplyLink — label/url/notes +
// sort_order (densely renumbered by the BE on reorder).
export type SupplyLinkAttrs = {
  id: string
  commodity_id: string
  label: string
  url: string
  notes?: string
  sort_order: number
}

// listForCommodity backs GET /commodities/{id}/supplies — flat list
// inside `data`, ordered by sort_order ASC. Tests that exercise the
// reorder flow pre-sort the fixture themselves so the handler stays
// dumb (no implicit re-ordering on the wire).
export function listForCommodity(slug: string, commodityID: string, items: SupplyLinkAttrs[] = []) {
  return [
    http.get(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/supplies`
      ),
      () =>
        HttpResponse.json({
          data: items,
          meta: { supply_links: items.length, total: items.length },
        })
    ),
  ]
}

// create backs POST /commodities/{id}/supplies. Echoes the supplied
// response payload as a 201 — fine for tests that only need to observe
// the mutation fired.
export function create(slug: string, commodityID: string, response: SupplyLinkAttrs) {
  return [
    http.post(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/supplies`
      ),
      () =>
        HttpResponse.json(
          {
            data: { id: response.id, type: "commodity_supply_links", attributes: response },
          },
          { status: 201 }
        )
    ),
  ]
}

// patch backs PATCH /commodities/{id}/supplies/{supplyID}.
export function patch(
  slug: string,
  commodityID: string,
  supplyID: string,
  response: SupplyLinkAttrs
) {
  return [
    http.patch(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/supplies/${encodeURIComponent(supplyID)}`
      ),
      () =>
        HttpResponse.json({
          data: { id: response.id, type: "commodity_supply_links", attributes: response },
        })
    ),
  ]
}

// remove backs DELETE /commodities/{id}/supplies/{supplyID}.
export function remove(slug: string, commodityID: string, supplyID: string) {
  return [
    http.delete(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/supplies/${encodeURIComponent(supplyID)}`
      ),
      () => new HttpResponse(null, { status: 204 })
    ),
  ]
}

// reorder backs POST /commodities/{id}/supplies/reorder — returns the
// re-sorted list. Tests pass a `response` slice in the new order; the
// handler doesn't validate the request body so a test can assert on
// the payload via msw's `request.json()` separately if needed.
export function reorder(slug: string, commodityID: string, response: SupplyLinkAttrs[]) {
  return [
    http.post(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/supplies/reorder`
      ),
      () =>
        HttpResponse.json({
          data: response,
          meta: { supply_links: response.length, total: response.length },
        })
    ),
  ]
}
