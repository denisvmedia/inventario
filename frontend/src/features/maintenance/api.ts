// Pure data-layer functions for the maintenance-schedules feature
// slice (#1368). Hooks live in `./hooks.ts`. Backed by the
// `/g/{slug}/maintenance` + `/g/{slug}/commodities/{id}/maintenance`
// endpoints.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type MaintenanceScheduleEntity = Schema<"models.MaintenanceSchedule">

export interface MaintenanceCommodityRef {
  id: string
  name: string
  short_name?: string
}

export interface ListedMaintenanceSchedule {
  schedule: MaintenanceScheduleEntity & { id: string }
  commodity?: MaintenanceCommodityRef
}

interface MaintenanceDetailEnvelope {
  data?: {
    id?: string
    type?: string
    attributes?: MaintenanceScheduleEntity
  }
}

interface PerCommodityListEnvelope {
  data?: Array<MaintenanceScheduleEntity & { id: string }>
  meta?: { schedules?: number; total?: number }
}

interface GroupListEnvelope {
  data?: Array<MaintenanceScheduleEntity & { id: string; commodity?: MaintenanceCommodityRef }>
  meta?: { schedules?: number; total?: number }
}

export interface ListSchedulesForCommodityResult {
  schedules: Array<MaintenanceScheduleEntity & { id: string }>
  total: number
}

export async function listSchedulesForCommodity(
  commodityID: string,
  signal?: AbortSignal
): Promise<ListSchedulesForCommodityResult> {
  const body = await http.get<PerCommodityListEnvelope>(
    `/commodities/${encodeURIComponent(commodityID)}/maintenance`,
    { signal }
  )
  return {
    schedules: body.data ?? [],
    total: body.meta?.total ?? body.data?.length ?? 0,
  }
}

export interface ListGroupMaintenanceOptions {
  page?: number
  perPage?: number
  dueBefore?: string // YYYY-MM-DD
  enabledOnly?: boolean
  signal?: AbortSignal
}

export async function listGroupMaintenance(
  opts: ListGroupMaintenanceOptions = {}
): Promise<{ schedules: ListedMaintenanceSchedule[]; total: number }> {
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  if (opts.dueBefore) params.set("due_before", opts.dueBefore)
  if (opts.enabledOnly) params.set("enabled_only", "true")
  const qs = params.toString()
  const path = qs ? `/maintenance?${qs}` : "/maintenance"
  const body = await http.get<GroupListEnvelope>(path, { signal: opts.signal })
  return {
    schedules: (body.data ?? []).map((row) => {
      const { commodity, ...rest } = row
      return { schedule: rest, commodity }
    }),
    total: body.meta?.total ?? body.data?.length ?? 0,
  }
}

export interface CreateMaintenanceScheduleRequest {
  commodity_id: string
  title: string
  interval_days: number
  next_due_at?: string // YYYY-MM-DD, optional — BE defaults to today + interval_days
  last_done_at?: string
  notes?: string
  enabled?: boolean
}

export async function createMaintenanceSchedule(
  req: CreateMaintenanceScheduleRequest
): Promise<MaintenanceScheduleEntity & { id: string }> {
  const { commodity_id, ...attrs } = req
  const body = await http.post<MaintenanceDetailEnvelope>(
    `/commodities/${encodeURIComponent(commodity_id)}/maintenance`,
    {
      data: { type: "maintenance_schedules", attributes: attrs },
    }
  )
  if (!body.data?.attributes) {
    throw new Error(`Malformed POST /maintenance response: missing data.attributes`)
  }
  return { ...body.data.attributes, id: body.data.id ?? "" }
}

export interface UpdateMaintenanceScheduleRequest {
  title?: string
  interval_days?: number
  next_due_at?: string
  last_done_at?: string | null
  notes?: string
  enabled?: boolean
}

export async function updateMaintenanceSchedule(
  scheduleID: string,
  req: UpdateMaintenanceScheduleRequest
): Promise<MaintenanceScheduleEntity & { id: string }> {
  const body = await http.patch<MaintenanceDetailEnvelope>(
    `/maintenance/${encodeURIComponent(scheduleID)}`,
    {
      data: { id: scheduleID, type: "maintenance_schedules", attributes: req },
    }
  )
  if (!body.data?.attributes) {
    throw new Error(`Malformed PATCH /maintenance/${scheduleID} response: missing data.attributes`)
  }
  return { ...body.data.attributes, id: body.data.id ?? scheduleID }
}

// doneAt defaults to today (server-side). Pass undefined for the
// canonical "I just did this" case.
export async function markMaintenanceDone(
  scheduleID: string,
  doneAt?: string
): Promise<MaintenanceScheduleEntity & { id: string }> {
  const payload = doneAt
    ? {
        data: {
          type: "maintenance_schedules",
          attributes: { done_at: doneAt },
        },
      }
    : undefined
  const body = await http.post<MaintenanceDetailEnvelope>(
    `/maintenance/${encodeURIComponent(scheduleID)}/done`,
    payload
  )
  if (!body.data?.attributes) {
    throw new Error(
      `Malformed POST /maintenance/${scheduleID}/done response: missing data.attributes`
    )
  }
  return { ...body.data.attributes, id: body.data.id ?? scheduleID }
}

export async function deleteMaintenanceSchedule(scheduleID: string): Promise<void> {
  await http.del<void>(`/maintenance/${encodeURIComponent(scheduleID)}`)
}

// daysUntilDue returns the integer number of days from "today (UTC)"
// until the schedule's next_due_at; negative when overdue, 0 when due
// today. Mirrors the BE's IsDueWithin/IsOverdue clock semantics so the
// FE renders the same overdue badge as the reminder worker.
export function daysUntilDue(
  schedule: Pick<MaintenanceScheduleEntity, "next_due_at">,
  now: Date = new Date()
): number | null {
  if (!schedule.next_due_at) return null
  const t = Date.parse(`${schedule.next_due_at}T00:00:00Z`)
  if (Number.isNaN(t)) return null
  const todayUTC = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate())
  return Math.round((t - todayUTC) / (1000 * 60 * 60 * 24))
}
