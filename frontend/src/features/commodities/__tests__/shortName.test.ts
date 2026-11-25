import { describe, expect, it } from "vitest"

import { SUGGESTED_SHORT_NAME_MAX, suggestShortName } from "@/features/commodities/shortName"

describe("suggestShortName", () => {
  it.each([
    ["Drill", "Drill"],
    ["  Cordless   drill  ", "Cordless drill"],
    ["Bosch Professional GSR 18V-55 cordless drill", "Bosch Professional"],
    ["Samsung, Galaxy S24 Ultra", "Samsung, Galaxy S24"],
    ["Supercalifragilisticexpialidocious", "Supercalifragilistic"],
    ["", ""],
  ])("suggests %j → %j", (name, want) => {
    expect(suggestShortName(name)).toBe(want)
    expect(suggestShortName(name).length).toBeLessThanOrEqual(SUGGESTED_SHORT_NAME_MAX)
  })
})
