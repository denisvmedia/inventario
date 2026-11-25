import { describe, expect, it } from "vitest"

import { validateLeadTimes } from "@/features/settings/leadTimes"

const list = { multiple: true, max: 365 }
const single = { multiple: false, max: 60 }

describe("validateLeadTimes", () => {
  it("treats an empty field as the default", () => {
    expect(validateLeadTimes("  ", list)).toEqual({ ok: true, value: null })
    expect(validateLeadTimes("", single)).toEqual({ ok: true, value: null })
  })

  it("parses and sorts a list, largest first", () => {
    expect(validateLeadTimes("7, 90 30", list)).toEqual({ ok: true, value: [90, 30, 7] })
    expect(validateLeadTimes("14", single)).toEqual({ ok: true, value: [14] })
  })

  it.each([
    ["7, soon", list, "settings:notifications.leadTimes.errors.notNumbers"],
    [", ; ", list, "settings:notifications.leadTimes.errors.notNumbers"],
    [",", single, "settings:notifications.leadTimes.errors.notNumber"],
    ["7.5", single, "settings:notifications.leadTimes.errors.notNumber"],
    ["7, 3", single, "settings:notifications.leadTimes.errors.single"],
    ["1, 2, 3, 4, 5, 6", list, "settings:notifications.leadTimes.errors.tooMany"],
    ["0", list, "settings:notifications.leadTimes.errors.range"],
    ["366", list, "settings:notifications.leadTimes.errors.range"],
    ["61", single, "settings:notifications.leadTimes.errors.range"],
    ["7, 7", list, "settings:notifications.leadTimes.errors.duplicate"],
  ])("rejects %j", (raw, opts, errorKey) => {
    const result = validateLeadTimes(raw, opts)
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.errorKey).toBe(errorKey)
  })
})
