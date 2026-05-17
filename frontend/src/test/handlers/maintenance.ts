import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// Maintenance schedule attributes mirror models.MaintenanceSchedule —
// title + interval_days + next_due_at + nullable last_done_at +
// enabled flag (#1368).
type ScheduleAttrs = {
  id: string
  commodity_id: string
  title: string
  interval_days: number
  next_due_at: string
  last_done_at?: string | null
  notes?: string
  enabled?: boolean
}

type ScheduleWithCommodity = ScheduleAttrs & {
  commodity?: { id: string; name: string; short_name?: string }
}

// listForCommodity backs GET /commodities/{id}/maintenance — flat
// entities under `data`. Mirrors the per-commodity envelope used by
// the Maintenance tab.
export function listForCommodity(slug: string, commodityID: string, items: ScheduleAttrs[] = []) {
  return [
    http.get(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/maintenance`
      ),
      () =>
        HttpResponse.json({
          data: items,
          meta: { schedules: items.length, total: items.length },
        })
    ),
  ]
}

// listGroup backs GET /maintenance — group-wide list with the per-row
// `commodity` denorm block.
export function listGroup(slug: string, items: ScheduleWithCommodity[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/maintenance`), () =>
      HttpResponse.json({
        data: items,
        meta: { schedules: items.length, total: items.length },
      })
    ),
  ]
}

export function create(slug: string, commodityID: string, response: ScheduleAttrs) {
  return [
    http.post(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/maintenance`
      ),
      () =>
        HttpResponse.json(
          { data: { id: response.id, type: "maintenance_schedules", attributes: response } },
          { status: 201 }
        )
    ),
  ]
}

export function update(slug: string, scheduleID: string, response: ScheduleAttrs) {
  return [
    http.patch(
      apiUrl(`/g/${encodeURIComponent(slug)}/maintenance/${encodeURIComponent(scheduleID)}`),
      () =>
        HttpResponse.json({
          data: { id: response.id, type: "maintenance_schedules", attributes: response },
        })
    ),
  ]
}

export function markDone(slug: string, scheduleID: string, response: ScheduleAttrs) {
  return [
    http.post(
      apiUrl(`/g/${encodeURIComponent(slug)}/maintenance/${encodeURIComponent(scheduleID)}/done`),
      () =>
        HttpResponse.json({
          data: { id: response.id, type: "maintenance_schedules", attributes: response },
        })
    ),
  ]
}

export function remove(slug: string, scheduleID: string) {
  return [
    http.delete(
      apiUrl(`/g/${encodeURIComponent(slug)}/maintenance/${encodeURIComponent(scheduleID)}`),
      () => new HttpResponse(null, { status: 204 })
    ),
  ]
}
