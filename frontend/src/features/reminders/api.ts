// Data layer for item reminders and the group-wide Upcoming list (#2805).
// Hooks live in `./hooks.ts`. Backed by
// `/g/{slug}/commodities/{id}/reminders`, `/g/{slug}/reminders/{id}` and
// `/g/{slug}/upcoming`.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type ReminderRepeatUnit = Schema<"models.ReminderRepeatUnit">
export type Reminder = Schema<"jsonapi.CommodityReminderView"> & { id: string }
export type UpcomingItem = Schema<"jsonapi.UpcomingItem">
export type UpcomingKind = NonNullable<UpcomingItem["kind"]>

export const REMINDER_REPEAT_UNITS: readonly ReminderRepeatUnit[] = [
  "none",
  "day",
  "week",
  "month",
  "year",
] as const

export const UPCOMING_KINDS: readonly UpcomingKind[] = [
  "reminder",
  "warranty",
  "loan",
  "maintenance",
] as const

// Limits mirror go/models/commodity_reminder.go.
export const REMINDER_TITLE_MAX = 200
export const REMINDER_MAX_LEAD_DAYS = 365
export const REMINDER_MAX_LEAD_TIMES = 5
export const REMINDER_MAX_INTERVAL = 999

interface ReminderListEnvelope {
  data?: Reminder[]
  meta?: { reminders?: number }
}

interface ReminderEnvelope {
  data?: {
    id?: string
    type?: string
    attributes?: Schema<"jsonapi.CommodityReminderView">
  }
}

interface UpcomingEnvelope {
  data?: UpcomingItem[]
  meta?: { days?: number; total?: number }
}

export interface ReminderInput {
  title: string
  due_on: string // YYYY-MM-DD
  remind_days_before: number[]
  repeat_unit: ReminderRepeatUnit
  repeat_interval: number
}

function unwrap(body: ReminderEnvelope, what: string): Reminder {
  if (!body.data?.attributes || !body.data.id) {
    throw new Error(`Malformed ${what} response: missing data`)
  }
  return { ...body.data.attributes, id: body.data.id }
}

export async function listRemindersForCommodity(
  commodityID: string,
  signal?: AbortSignal
): Promise<Reminder[]> {
  const body = await http.get<ReminderListEnvelope>(
    `/commodities/${encodeURIComponent(commodityID)}/reminders`,
    { signal }
  )
  return body.data ?? []
}

export async function createReminder(commodityID: string, input: ReminderInput): Promise<Reminder> {
  const body = await http.post<ReminderEnvelope>(
    `/commodities/${encodeURIComponent(commodityID)}/reminders`,
    { data: { type: "commodity_reminders", attributes: input } }
  )
  return unwrap(body, "POST /reminders")
}

export async function updateReminder(
  reminderID: string,
  input: Partial<ReminderInput>
): Promise<Reminder> {
  const body = await http.patch<ReminderEnvelope>(`/reminders/${encodeURIComponent(reminderID)}`, {
    data: { id: reminderID, type: "commodity_reminders", attributes: input },
  })
  return unwrap(body, `PATCH /reminders/${reminderID}`)
}

export async function deleteReminder(reminderID: string): Promise<void> {
  await http.del<void>(`/reminders/${encodeURIComponent(reminderID)}`)
}

export async function listUpcoming(
  days: number,
  signal?: AbortSignal
): Promise<{ items: UpcomingItem[]; days: number }> {
  const body = await http.get<UpcomingEnvelope>(`/upcoming?days=${days}`, { signal })
  return { items: body.data ?? [], days: body.meta?.days ?? days }
}
