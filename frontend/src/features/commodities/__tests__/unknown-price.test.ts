import { describe, expect, it } from "vitest"

import type { Commodity } from "@/features/commodities/api"
import { buildDefaults, toRequest } from "@/features/commodities/draft"
import {
  commodityValue,
  formatPrice,
  parsePrice,
  purchasePriceInGroupCurrency,
} from "@/features/commodities/price"
import { buildCommoditySchema, type CommodityFormInput } from "@/features/commodities/schemas"

// An unknown price or purchase date is not a zero. These cover the four
// acquisitions the distinction exists for: a gift with nothing known, a
// free item, an item with only a current value, and a known price paid in
// a foreign currency.

function item(overrides: Record<string, unknown>): Commodity {
  return {
    id: "c1",
    name: "Clock",
    short_name: "Clock",
    type: "furniture",
    status: "in_use",
    count: 1,
    area_id: "a1",
    draft: false,
    original_price_currency: "USD",
    ...overrides,
  } as Commodity
}

// A new reviewed item's form with the basics filled in and nothing known
// about its purchase.
function formValues(overrides: Partial<CommodityFormInput> = {}): CommodityFormInput {
  return { ...buildDefaults(undefined, "USD"), type: "furniture", area_id: "a1", ...overrides }
}

const gift = item({ original_price: null, converted_original_price: null, current_price: null })
const free = item({ original_price: "0", current_price: "0", purchase_date: "2024-02-01" })
const appraised = item({ original_price: null, current_price: "250.5" })
const imported = item({
  original_price: "100",
  original_price_currency: "EUR",
  converted_original_price: "110",
  purchase_date: "2024-03-01",
})

describe("parsePrice", () => {
  it("reads null, undefined, blank and garbage as unknown", () => {
    expect(parsePrice(null)).toBeNull()
    expect(parsePrice(undefined)).toBeNull()
    expect(parsePrice("")).toBeNull()
    expect(parsePrice("  ")).toBeNull()
    expect(parsePrice("abc")).toBeNull()
  })

  it("reads a zero as a known zero", () => {
    expect(parsePrice("0")).toBe(0)
    expect(parsePrice(0)).toBe(0)
  })

  it("reads decimal strings and numbers", () => {
    expect(parsePrice("19.99")).toBe(19.99)
    expect(parsePrice(5)).toBe(5)
  })
})

describe("commodityValue", () => {
  it("has no value for a gift with nothing known", () => {
    expect(commodityValue(gift, "USD")).toBeNull()
    expect(purchasePriceInGroupCurrency(gift, "USD")).toBeNull()
  })

  it("values a free item at zero", () => {
    expect(commodityValue(free, "USD")).toBe(0)
    expect(purchasePriceInGroupCurrency(free, "USD")).toBe(0)
  })

  it("uses the current value when only that is known", () => {
    expect(commodityValue(appraised, "USD")).toBe(250.5)
    expect(purchasePriceInGroupCurrency(appraised, "USD")).toBeNull()
  })

  it("uses the converted amount of a foreign-currency price", () => {
    expect(purchasePriceInGroupCurrency(imported, "USD")).toBe(110)
    expect(commodityValue(imported, "USD")).toBe(110)
  })

  it("does not read a foreign-currency price as a group-currency amount", () => {
    const unconverted = item({ original_price: "100", original_price_currency: "EUR" })
    expect(purchasePriceInGroupCurrency(unconverted, "USD")).toBeNull()
  })

  it("ignores a converted price on a group-currency item", () => {
    // Older clients send a converted 0 for "not entered".
    const staleZero = item({ original_price: null, converted_original_price: "0" })
    expect(purchasePriceInGroupCurrency(staleZero, "USD")).toBeNull()
    expect(commodityValue(staleZero, "USD")).toBeNull()
  })
})

describe("formatPrice", () => {
  const format = (amount: number, currency: string) => `${currency} ${amount}`

  it("shows a known zero as zero and an unknown price as unknown", () => {
    expect(formatPrice("0", "USD", "—", format)).toBe("USD 0")
    expect(formatPrice(null, "USD", "—", format)).toBe("—")
    expect(formatPrice(undefined, "USD", "—", format)).toBe("—")
  })
})

describe("buildCommoditySchema for a reviewed item", () => {
  const schema = buildCommoditySchema("USD")

  it("accepts a gift with no purchase date and no prices", () => {
    expect(schema.safeParse(formValues({ name: "Clock", short_name: "Clock" })).success).toBe(true)
  })

  it("accepts a free item with zero prices", () => {
    const values = formValues({
      name: "Chair",
      short_name: "Chair",
      original_price: "0",
      current_price: "0",
    })
    expect(schema.safeParse(values).success).toBe(true)
  })

  it("rejects a negative price", () => {
    const result = schema.safeParse(formValues({ name: "X", short_name: "X", current_price: "-1" }))
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues.map((i) => i.message)).toContain(
        "commodities:validation.priceNegative"
      )
    }
  })

  it("needs a group-currency amount for a known foreign-currency price", () => {
    const result = schema.safeParse(
      formValues({
        name: "Lamp",
        short_name: "Lamp",
        original_price: "100",
        original_price_currency: "EUR",
      })
    )
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues.map((i) => i.message)).toContain(
        "commodities:validation.noPriceInGroupCurrency"
      )
    }
  })

  it("accepts a known zero as the group-currency amount", () => {
    const values = formValues({
      name: "Lamp",
      short_name: "Lamp",
      original_price: "100",
      original_price_currency: "EUR",
      converted_original_price: "0",
    })
    expect(schema.safeParse(values).success).toBe(true)
  })

  it("asks for no conversion when the foreign-currency price is unknown", () => {
    const values = formValues({ name: "Lamp", short_name: "Lamp", original_price_currency: "EUR" })
    expect(schema.safeParse(values).success).toBe(true)
  })
})

describe("form round trip", () => {
  it("keeps a gift's prices and purchase date unknown", () => {
    const form = buildDefaults(gift, "USD")
    expect(form.original_price).toBe("")
    expect(form.converted_original_price).toBe("")
    expect(form.current_price).toBe("")
    expect(form.purchase_date).toBe("")

    const request = toRequest(form, "USD")
    expect(request.original_price).toBeNull()
    expect(request.converted_original_price).toBeNull()
    expect(request.current_price).toBeNull()
    // Left out of the request, which the BE stores as no purchase date.
    expect(request.purchase_date).toBeUndefined()
  })

  it("keeps a free item's zeros", () => {
    const form = buildDefaults(free, "USD")
    expect(form.original_price).toBe("0")
    expect(form.current_price).toBe("0")

    const request = toRequest(form, "USD")
    expect(request.original_price).toBe(0)
    expect(request.current_price).toBe(0)
    // Paid in the group currency: there is nothing to convert.
    expect(request.converted_original_price).toBeNull()
    expect(request.purchase_date).toBe("2024-02-01")
  })

  it("keeps an appraised item's purchase price unknown", () => {
    const request = toRequest(buildDefaults(appraised, "USD"), "USD")
    expect(request.original_price).toBeNull()
    expect(request.current_price).toBe(250.5)
  })

  it("keeps a foreign-currency price and its converted amount", () => {
    const request = toRequest(buildDefaults(imported, "USD"), "USD")
    expect(request.original_price).toBe(100)
    expect(request.original_price_currency).toBe("EUR")
    expect(request.converted_original_price).toBe(110)
    expect(request.current_price).toBeNull()
  })

  it("does not copy the current value into the converted price", () => {
    const request = toRequest(
      formValues({
        original_price: "100",
        original_price_currency: "EUR",
        current_price: "90",
      }),
      "USD"
    )
    expect(request.converted_original_price).toBeNull()
    expect(request.current_price).toBe(90)
  })
})
