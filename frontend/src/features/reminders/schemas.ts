import { z } from "zod"

import {
  REMINDER_MAX_INTERVAL,
  REMINDER_MAX_LEAD_DAYS,
  REMINDER_MAX_LEAD_TIMES,
  REMINDER_MAX_YEAR,
  REMINDER_MIN_YEAR,
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

// shortestRepeatDays mirrors CommodityReminder.ShortestRepeatDays in Go: the
// fewest days between two occurrences, counting a month as 28 days and a
// year as 365, or 0 for a reminder that does not repeat.
export function shortestRepeatDays(unit: ReminderRepeatUnit, interval: number): number {
  const n = Math.max(interval, 1)
  switch (unit) {
    case "day":
      return n
    case "week":
      return n * 7
    case "month":
      return n * 28
    case "year":
      return n * 365
    default:
      return 0
  }
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
    const intervalValid =
      Number.isInteger(v.repeat_interval) &&
      v.repeat_interval >= 1 &&
      v.repeat_interval <= REMINDER_MAX_INTERVAL
    const year = Number(v.due_on.slice(0, 4))
    if (year < REMINDER_MIN_YEAR || year > REMINDER_MAX_YEAR) {
      ctx.addIssue({
        code: "custom",
        path: ["due_on"],
        message: "reminders:validation.dateRange",
      })
    }
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
    } else if (intervalValid) {
      // A lead time as long as the repeat would fire the day after the
      // previous occurrence, not on the day asked for. The server rejects it
      // too.
      const period = shortestRepeatDays(v.repeat_unit, v.repeat_interval)
      if (period > 0 && leads.some((d) => d >= period)) {
        ctx.addIssue({
          code: "custom",
          path: ["remind_days_before"],
          message: "reminders:validation.leadTimesExceedRepeat",
        })
      }
    }
    if (v.repeat_unit !== "none" && !intervalValid) {
      ctx.addIssue({
        code: "custom",
        path: ["repeat_interval"],
        message: "reminders:validation.intervalRange",
      })
    }
  })

export type ReminderFormInput = z.input<typeof reminderFormSchema>
export type ReminderFormOutput = z.output<typeof reminderFormSchema>
