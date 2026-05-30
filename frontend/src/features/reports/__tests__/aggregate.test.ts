import { describe, expect, it } from "vitest"

import type { Commodity } from "@/features/commodities/api"

import { aggregateLocationTotals } from "../aggregate"

// Minimal commodity factory — the aggregate only reads
// `converted_original_price` (group-currency purchase) and `current_price`
// (group-currency value), so everything else is filler.
function commodity(overrides: Partial<Commodity> = {}): Commodity {
  return { id: Math.random().toString(36).slice(2), name: "Item", ...overrides } as Commodity
}

describe("aggregateLocationTotals", () => {
  it("counts the commodities and sums the two money columns", () => {
    const totals = aggregateLocationTotals([
      commodity({ converted_original_price: 100, current_price: 80 }),
      commodity({ converted_original_price: 250, current_price: 300 }),
    ])
    expect(totals).toEqual({ count: 2, purchase: 350, value: 380 })
  })

  it("coerces decimal-string amounts the BE sends", () => {
    const totals = aggregateLocationTotals([
      commodity({
        converted_original_price: "19.99" as unknown as number,
        current_price: "10.01" as unknown as number,
      }),
      commodity({
        converted_original_price: "0.01" as unknown as number,
        current_price: "5" as unknown as number,
      }),
    ])
    expect(totals.count).toBe(2)
    expect(totals.purchase).toBeCloseTo(20, 5)
    expect(totals.value).toBeCloseTo(15.01, 5)
  })

  it("treats missing / null / empty / NaN amounts as zero", () => {
    const totals = aggregateLocationTotals([
      commodity({ current_price: 50 }), // no converted_original_price
      commodity({
        converted_original_price: null as unknown as number,
        current_price: "" as unknown as number,
      }),
      commodity({
        converted_original_price: "not-a-number" as unknown as number,
        current_price: undefined,
      }),
    ])
    expect(totals).toEqual({ count: 3, purchase: 0, value: 50 })
  })

  it("returns zeroed totals for an empty location", () => {
    expect(aggregateLocationTotals([])).toEqual({ count: 0, purchase: 0, value: 0 })
  })
})
