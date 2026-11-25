import { describe, expect, it } from "vitest"

import { parseLeadTimes, reminderFormSchema } from "@/features/reminders/schemas"

const valid = {
  title: "Inspection",
  due_on: "2027-03-01",
  remind_days_before: "30, 7, 1",
  repeat_unit: "none" as const,
  repeat_interval: 1,
}

function firstError(input: Record<string, unknown>): string | undefined {
  const result = reminderFormSchema.safeParse({ ...valid, ...input })
  return result.success ? undefined : result.error.issues[0]?.message
}

describe("parseLeadTimes", () => {
  it("accepts commas, spaces and semicolons", () => {
    expect(parseLeadTimes("30, 7,1")).toEqual([30, 7, 1])
    expect(parseLeadTimes(" 14 ; 0 ")).toEqual([14, 0])
    expect(parseLeadTimes("")).toEqual([])
  })

  it("returns null for anything that is not a whole number", () => {
    expect(parseLeadTimes("7, soon")).toBeNull()
    expect(parseLeadTimes("1.5")).toBeNull()
    expect(parseLeadTimes("-1")).toBeNull()
  })
})

describe("reminderFormSchema", () => {
  it("accepts a valid reminder", () => {
    expect(firstError({})).toBeUndefined()
  })

  it.each([
    [{ title: "  " }, "reminders:validation.titleRequired"],
    [{ due_on: "" }, "reminders:validation.dateRequired"],
    [{ due_on: "0226-03-01" }, "reminders:validation.dateRange"],
    [{ due_on: "2201-01-01" }, "reminders:validation.dateRange"],
    [
      { remind_days_before: "7", repeat_unit: "week", repeat_interval: 1 },
      "reminders:validation.leadTimesExceedRepeat",
    ],
    [
      { remind_days_before: "30, 7", repeat_unit: "month", repeat_interval: 1 },
      "reminders:validation.leadTimesExceedRepeat",
    ],
    [
      { remind_days_before: "365", repeat_unit: "year", repeat_interval: 1 },
      "reminders:validation.leadTimesExceedRepeat",
    ],
    [{ remind_days_before: "" }, "reminders:validation.leadTimesRequired"],
    [{ remind_days_before: "7, x" }, "reminders:validation.leadTimesInvalid"],
    [{ remind_days_before: "366" }, "reminders:validation.leadTimesRange"],
    [{ remind_days_before: "7, 7" }, "reminders:validation.leadTimesDuplicate"],
    [{ remind_days_before: "1, 2, 3, 4, 5, 6" }, "reminders:validation.leadTimesTooMany"],
    [{ repeat_unit: "month", repeat_interval: 0 }, "reminders:validation.intervalRange"],
    [{ repeat_unit: "month", repeat_interval: 1000 }, "reminders:validation.intervalRange"],
  ])("rejects %o", (input, message) => {
    expect(firstError(input)).toBe(message)
  })

  it("accepts lead times shorter than the repeat", () => {
    expect(firstError({ due_on: "1900-01-01" })).toBeUndefined()
    expect(firstError({ due_on: "2200-12-31" })).toBeUndefined()
    expect(firstError({ remind_days_before: "6, 0", repeat_unit: "week" })).toBeUndefined()
    expect(firstError({ remind_days_before: "27", repeat_unit: "month" })).toBeUndefined()
    expect(
      firstError({ remind_days_before: "60, 30", repeat_unit: "month", repeat_interval: 3 })
    ).toBeUndefined()
    expect(firstError({ remind_days_before: "364", repeat_unit: "year" })).toBeUndefined()
    expect(firstError({ remind_days_before: "365" })).toBeUndefined()
  })

  it("ignores the interval of a reminder that does not repeat", () => {
    expect(firstError({ repeat_unit: "none", repeat_interval: 0 })).toBeUndefined()
  })
})
