import { describe, expect, it } from "vitest"

import { formatAssetId } from "@/features/commodities/asset-id"

describe("formatAssetId", () => {
  it.each([
    [1, "A-0001"],
    [42, "A-0042"],
    [9999, "A-9999"],
    [12345, "A-12345"],
  ])("formats %i as %s", (n, want) => {
    expect(formatAssetId(n)).toBe(want)
  })

  it.each([undefined, null, 0, -3, 1.5])("returns an empty string for %s", (n) => {
    expect(formatAssetId(n)).toBe("")
  })
})
