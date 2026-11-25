import { parseLeadTimes } from "@/features/reminders/schemas"

// Maximum number of lead times a list setting holds; mirrors
// models.ReminderMaxLeadTimes.
export const LEAD_TIMES_MAX_COUNT = 5

export type LeadTimesResult =
  | { ok: true; value: number[] | null }
  | { ok: false; errorKey: string; params?: Record<string, number> }

// validateLeadTimes checks the text of a lead-time field in Settings →
// Notifications (#2805). An empty field means "use the default" and comes
// back as null. A field of separators alone (", ;") holds no number, so it
// is an error rather than an empty list, which the server rejects. The rules
// mirror SettingsObject.ValidateWithContext.
export function validateLeadTimes(
  raw: string,
  { multiple, max }: { multiple: boolean; max: number }
): LeadTimesResult {
  if (raw.trim() === "") return { ok: true, value: null }
  const values = parseLeadTimes(raw)
  if (values === null || values.length === 0) {
    return {
      ok: false,
      errorKey: multiple
        ? "settings:notifications.leadTimes.errors.notNumbers"
        : "settings:notifications.leadTimes.errors.notNumber",
    }
  }
  if (!multiple && values.length > 1) {
    return { ok: false, errorKey: "settings:notifications.leadTimes.errors.single" }
  }
  if (values.length > LEAD_TIMES_MAX_COUNT) {
    return {
      ok: false,
      errorKey: "settings:notifications.leadTimes.errors.tooMany",
      params: { max: LEAD_TIMES_MAX_COUNT },
    }
  }
  if (values.some((v) => v < 1 || v > max)) {
    return { ok: false, errorKey: "settings:notifications.leadTimes.errors.range", params: { max } }
  }
  if (new Set(values).size !== values.length) {
    return { ok: false, errorKey: "settings:notifications.leadTimes.errors.duplicate" }
  }
  return { ok: true, value: [...values].sort((a, b) => b - a) }
}

export function formatLeadTimes(values: number[]): string {
  return values.join(", ")
}
