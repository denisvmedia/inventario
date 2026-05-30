import type { Commodity } from "@/features/commodities/api"

// Location-report totals (#1370). All money sums are in the group
// currency: `converted_original_price` is the per-row purchase price
// converted to the group currency, and `current_price` is the live
// value already denominated in the group currency. Values arrive from
// the BE as decimal strings, so each is coerced via Number() and any
// NaN / missing entry contributes 0.
export interface LocationTotals {
  count: number
  purchase: number
  value: number
}

function toAmount(raw: unknown): number {
  if (raw === null || raw === undefined || raw === "") return 0
  const n = typeof raw === "string" ? Number(raw) : (raw as number)
  return Number.isFinite(n) ? n : 0
}

export function aggregateLocationTotals(commodities: Commodity[]): LocationTotals {
  let purchase = 0
  let value = 0
  for (const c of commodities) {
    purchase += toAmount(c.converted_original_price)
    value += toAmount(c.current_price)
  }
  return { count: commodities.length, purchase, value }
}
