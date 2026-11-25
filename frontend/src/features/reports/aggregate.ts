import type { Commodity } from "@/features/commodities/api"
import {
  commodityValue,
  purchasePriceInGroupCurrency,
  sumKnown,
  type KnownTotal,
} from "@/features/commodities/price"

// Location-report totals (#1370), all in the group currency. The purchase
// total sums what each item cost in the group currency (its purchase price
// when paid in the group currency, otherwise the converted amount); the
// value total sums each item's value by the rule the dashboard and the area
// totals use (its current value, else its purchase price), so one item has
// one estimated value everywhere. Each total covers only the items whose
// amount is known and says how many it leaves out, so an unknown price never
// passes for a zero.
export interface LocationTotals {
  count: number
  purchase: KnownTotal
  value: KnownTotal
}

export function aggregateLocationTotals(
  commodities: Commodity[],
  groupCurrency: string
): LocationTotals {
  return {
    count: commodities.length,
    purchase: sumKnown(commodities.map((c) => purchasePriceInGroupCurrency(c, groupCurrency))),
    value: sumKnown(commodities.map((c) => commodityValue(c, groupCurrency))),
  }
}
