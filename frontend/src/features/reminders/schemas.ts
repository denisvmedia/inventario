import { z } from "zod"

import {
  REMINDER_MAX_INTERVAL,
  REMINDER_MAX_LEAD_DAYS,
  REMINDER_MAX_LEAD_TIMES,
  REMINDER_REPEAT_UNITS,
  REMINDER_TITLE_MAX,
  type ReminderRepeatUnit,
} from "./api"

// parseLeadTimes turns "30, 7, 1" into [30, 7, 1]. It returns null when a
// part is not a whole number, so the schema can report it.
export function parseLeadTimes(raw: string): number[] | null {
  const parts = raw
    .split(/[\s,;]+/)
    .map((p) => p.trim())
    .filter((p) => p.length > 0)
  const out: number[] = []
  for (const p of parts) {
    if (!/^\d+$/.test(p)) return null
    out.push(Number(p))
  }
  return out
}

// reminderFormSchema backs ReminderDialog (#2805). Messages are i18n keys,
// resolved by <FieldError>. The lead times are typed as one comma-separated
// field and parsed here; the limits mirror go/models/commodity_reminder.go.
export const reminderFormSchema = z
  .object({
    title: z
      .string()
      .trim()
      .min(1, "reminders:validation.titleRequired")
      .max(REMINDER_TITLE_MAX, "reminders:validation.titleTooLong"),
    due_on: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, "reminders:validation.dateRequired"),
    remind_days_before: z.string(),
    repeat_unit: z.enum(REMINDER_REPEAT_UNITS as [ReminderRepeatUnit, ...ReminderRepeatUnit[]]),
    repeat_interval: z.coerce.number(),
  })
  .superRefine((v, ctx) => {
    const leads = parseLeadTimes(v.remind_days_before)
    if (leads === null) {
      ctx.addIssue({
        code: "custom",
        path: ["remind_days_before"],
        message: "reminders:validation.leadTimesInvalid",
      })
    } else if (leads.length === 0) {
      ctx.addIssue({
        code: "custom",
        path: ["remind_days_before"],
        message: "reminders:validation.leadTimesRequired",
      })
    } else if (leads.length > REMINDER_MAX_LEAD_TIMES) {
      ctx.addIssue({
        code: "custom",
        path: ["remind_days_before"],
        message: "reminders:validation.leadTimesTooMany",
      })
    } else if (leads.some((d) => d > REMINDER_MAX_LEAD_DAYS)) {
      ctx.addIssue({
        code: "custom",
        path: ["remind_days_before"],
        message: "reminders:validation.leadTimesRange",
      })
    } else if (new Set(leads).size !== leads.length) {
      ctx.addIssue({
        code: "custom",
        path: ["remind_days_before"],
        message: "reminders:validation.leadTimesDuplicate",
      })
    }
    if (
      v.repeat_unit !== "none" &&
      (!Number.isInteger(v.repeat_interval) ||
        v.repeat_interval < 1 ||
        v.repeat_interval > REMINDER_MAX_INTERVAL)
    ) {
      ctx.addIssue({
        code: "custom",
        path: ["repeat_interval"],
        message: "reminders:validation.intervalRange",
      })
    }
  })

export type ReminderFormInput = z.input<typeof reminderFormSchema>
export type ReminderFormOutput = z.output<typeof reminderFormSchema>
