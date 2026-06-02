import { describe, expect, it } from "vitest"

import { buildDefaults } from "@/features/commodities/draft"
import type { Commodity } from "@/features/commodities/api"

// buildDefaults' `defaultDraft` seam backs the anonymous "add your first
// item" flow (#1988): the landing dialog passes true so a first-time
// visitor only has to fill name/short_name/type (price/date relax to
// optional in the draft schema).
describe("buildDefaults", () => {
  it("defaults a new item to non-draft", () => {
    expect(buildDefaults(undefined, "USD").draft).toBe(false)
  })

  it("honours defaultDraft=true for a brand-new item", () => {
    expect(buildDefaults(undefined, "USD", true).draft).toBe(true)
  })

  it("lets an existing record's own draft value win over defaultDraft", () => {
    const existing = { draft: false } as unknown as Commodity
    expect(buildDefaults(existing, "USD", true).draft).toBe(false)
  })
})
