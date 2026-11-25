import { http, HttpResponse } from "msw"

import { capture } from "@/test/capture"
import type { Schema } from "@/types"

import { apiUrl } from "."

type ReminderAttrs = Schema<"jsonapi.CommodityReminderView"> & { id: string }
type UpcomingItem = Schema<"jsonapi.UpcomingItem">

// listForCommodity backs GET /commodities/{id}/reminders — flat views
// under `data`, each with its computed next occurrence (#2805).
export function listForCommodity(slug: string, commodityID: string, items: ReminderAttrs[] = []) {
  return [
    http.get(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/reminders`
      ),
      () => HttpResponse.json({ data: items, meta: { reminders: items.length } })
    ),
  ]
}

// create backs POST /commodities/{id}/reminders. The request body lands in
// `body.value` so a test can assert on the payload.
export function create(
  slug: string,
  commodityID: string,
  response: ReminderAttrs,
  body = capture<unknown>()
) {
  return [
    http.post(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/reminders`
      ),
      async ({ request }) => {
        body.value = await request.json()
        return HttpResponse.json(
          { data: { id: response.id, type: "commodity_reminders", attributes: response } },
          { status: 201 }
        )
      }
    ),
  ]
}

export function update(slug: string, response: ReminderAttrs, body = capture<unknown>()) {
  return [
    http.patch(
      apiUrl(`/g/${encodeURIComponent(slug)}/reminders/${encodeURIComponent(response.id)}`),
      async ({ request }) => {
        body.value = await request.json()
        return HttpResponse.json({
          data: { id: response.id, type: "commodity_reminders", attributes: response },
        })
      }
    ),
  ]
}

export function remove(slug: string, reminderID: string) {
  return [
    http.delete(
      apiUrl(`/g/${encodeURIComponent(slug)}/reminders/${encodeURIComponent(reminderID)}`),
      () => new HttpResponse(null, { status: 204 })
    ),
  ]
}

// upcoming backs GET /upcoming?days=N. `days` records each requested window.
export function upcoming(slug: string, items: UpcomingItem[] = [], days: number[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/upcoming`), ({ request }) => {
      const n = Number(new URL(request.url).searchParams.get("days") ?? "30")
      days.push(n)
      return HttpResponse.json({ data: items, meta: { days: n, total: items.length } })
    }),
  ]
}
