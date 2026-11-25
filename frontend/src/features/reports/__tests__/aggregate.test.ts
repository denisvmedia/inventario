import { describe, expect, it } from "vitest"

import type { Commodity } from "@/features/commodities/api"

import { aggregateLocationTotals } from "../aggregate"

// Minimal commodity factory — the aggregate only reads the prices and the
// purchase currency, so everything else is filler.
function commodity(overrides: Record<string, unknown> = {}): Commodity {
  return {
    id: Math.random().toString(36).slice(2),
    name: "Item",
    original_price_currency: "USD",
    ...overrides,
  } as Commodity
}

describe("aggregateLocationTotals", () => {
  it("sums the purchase price in the group currency and the current value", () => {
    const totals = aggregateLocationTotals(
      [
        // Paid in the group currency: the purchase price itself counts.
        commodity({ original_price: "100", current_price: "80" }),
        // Paid in a foreign currency: the converted amount counts.
        commodity({
          original_price: "200",
          original_price_currency: "EUR",
          converted_original_price: "250",
          current_price: "300",
        }),
      ],
      "USD"
    )
    expect(totals.count).toBe(2)
    expect(totals.purchase).toEqual({ total: 350, included: 2, excluded: 0 })
    expect(totals.value).toEqual({ total: 380, included: 2, excluded: 0 })
  })

  it("coerces decimal-string amounts the BE sends", () => {
    const totals = aggregateLocationTotals(
      [
        commodity({ original_price: "19.99", current_price: "10.01" }),
        commodity({ original_price: "0.01", current_price: "5" }),
      ],
      "USD"
    )
    expect(totals.purchase.total).toBeCloseTo(20, 5)
    expect(totals.value.total).toBeCloseTo(15.01, 5)
  })

  it("leaves unknown amounts out and counts them, but keeps a known zero", () => {
    const totals = aggregateLocationTotals(
      [
        // A gift: nothing known.
        commodity({ original_price: null, current_price: null }),
        // A free item: a known zero on both.
        commodity({ original_price: "0", current_price: "0" }),
        // Only a current value, from an appraisal.
        commodity({ original_price: null, current_price: "1200" }),
        // A foreign-currency price with no converted amount.
        commodity({ original_price: "50", original_price_currency: "EUR" }),
      ],
      "USD"
    )
    expect(totals.count).toBe(4)
    expect(totals.purchase).toEqual({ total: 0, included: 1, excluded: 3 })
    expect(totals.value).toEqual({ total: 1200, included: 2, excluded: 2 })
  })

  it("returns zeroed totals for an empty location", () => {
    expect(aggregateLocationTotals([], "USD")).toEqual({
      count: 0,
      purchase: { total: 0, included: 0, excluded: 0 },
      value: { total: 0, included: 0, excluded: 0 },
    })
  })
})
