// Reading and combining a commodity's prices. Every price and the purchase
// date can be unknown, and unknown is not zero: a gift has an unknown price,
// a free item has a price of 0. The BE sends a known price as a decimal
// string ("19.99", "0") and an unknown one as null; the generated types call
// it a number, and test fixtures pass numbers, so both are accepted.
//
// The valuation rules mirror models.Commodity.Value on the BE, so the
// figures shown next to an item agree with the totals the BE computes.

export type ApiPrice = number | string | null | undefined

// parsePrice returns the amount of a price, or null when it is unknown. A
// value that does not parse counts as unknown rather than as zero.
export function parsePrice(price: ApiPrice): number | null {
  if (price === null || price === undefined) return null
  if (typeof price === "string" && price.trim() === "") return null
  const n = typeof price === "number" ? price : Number(price)
  return Number.isFinite(n) ? n : null
}

export interface PricedCommodity {
  original_price?: ApiPrice
  original_price_currency?: string
  converted_original_price?: ApiPrice
  current_price?: ApiPrice
}

function sameCurrency(a: string | undefined, b: string): boolean {
  return !!a && !!b && a.trim().toUpperCase() === b.trim().toUpperCase()
}

// purchasePriceInGroupCurrency is what the item cost in the group currency:
// the purchase price when it was paid in the group currency, otherwise its
// converted amount. Null when that is unknown.
export function purchasePriceInGroupCurrency(
  commodity: PricedCommodity,
  groupCurrency: string
): number | null {
  const original = parsePrice(commodity.original_price)
  if (original !== null && sameCurrency(commodity.original_price_currency, groupCurrency)) {
    return original
  }
  return parsePrice(commodity.converted_original_price)
}

// commodityValue is the item's value in the group currency: its current
// value when known, otherwise its purchase price. Null when neither is
// known; a known zero is a value.
export function commodityValue(commodity: PricedCommodity, groupCurrency: string): number | null {
  const current = parsePrice(commodity.current_price)
  if (current !== null) return current
  return purchasePriceInGroupCurrency(commodity, groupCurrency)
}

// KnownTotal is a sum over the amounts that are known, with the number of
// items it left out because their amount is unknown.
export interface KnownTotal {
  total: number
  included: number
  excluded: number
}

export function sumKnown(amounts: (number | null)[]): KnownTotal {
  let total = 0
  let included = 0
  let excluded = 0
  for (const amount of amounts) {
    if (amount === null) {
      excluded++
      continue
    }
    total += amount
    included++
  }
  return { total, included, excluded }
}

// formatPrice formats a known amount with `format` and returns `unknown`
// for an unknown one, so a zero still reads as a formatted zero.
export function formatPrice(
  price: ApiPrice,
  currency: string,
  unknown: string,
  format: (amount: number, currency: string) => string
): string {
  const amount = parsePrice(price)
  return amount === null ? unknown : format(amount, currency)
}

// priceToFormValue turns a price into the string a form input holds:
// "" when unknown, so a blank field means unknown and "0" means zero.
export function priceToFormValue(price: ApiPrice): string {
  const amount = parsePrice(price)
  return amount === null ? "" : String(amount)
}

// formValueToPrice is the reverse of priceToFormValue: a blank field is an
// unknown price (null), anything else is the amount typed.
export function formValueToPrice(value: string | undefined): number | null {
  if (value === undefined) return null
  const trimmed = value.trim()
  if (trimmed === "") return null
  const n = Number(trimmed)
  return Number.isFinite(n) ? n : null
}
