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
    // The limit counts code points: an emoji is one character, never cut
    // in half.
    ["🎸".repeat(25), "🎸".repeat(20)],
    ["Žluťoučký kůň 🐎 úpěl ďábelské ódy", "Žluťoučký kůň 🐎 úpěl"],
  ])("suggests %j → %j", (name, want) => {
    const got = suggestShortName(name)
    expect(got).toBe(want)
    expect(Array.from(got).length).toBeLessThanOrEqual(SUGGESTED_SHORT_NAME_MAX)
    expect(got).not.toMatch(/[\uD800-\uDBFF](?![\uDC00-\uDFFF])/)
  })
})
