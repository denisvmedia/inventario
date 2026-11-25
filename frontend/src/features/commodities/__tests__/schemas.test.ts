import { describe, expect, it } from "vitest"

import { commoditySchema } from "@/features/commodities/schemas"

// Issue #1554: a count > 1 commodity (a bundle of interchangeable
// units) cannot carry warranty fields. The schema's superRefine
// surfaces the same translation key the BE 422 maps to, so the form
// rejects the pair before the network round-trip.
describe("commoditySchema — quantity-forbids-warranty (#1554)", () => {
  const baseValues = {
    name: "Pack of bulbs",
    short_name: "bulbs",
    brand: "",
    model: "",
    type: "other",
    area_id: "a1",
    seller_id: "",
    status: "in_use",
    count: "12",
    original_price: "",
    original_price_currency: "USD",
    converted_original_price: "",
    current_price: "",
    serial_number: "",
    extra_serial_numbers: [],
    part_numbers: [],
    tags: [],
    purchase_date: "",
    urls: [],
    comments: "",
    draft: true,
    warranty_expires_at: "",
    warranty_notes: "",
  }

  it("rejects count > 1 with a warranty expiry", () => {
    const result = commoditySchema.safeParse({
      ...baseValues,
      warranty_expires_at: "2027-01-01",
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message)
      expect(messages).toContain("commodities:validation.quantityForbidsWarranty")
      const paths = result.error.issues.map((i) => i.path.join("."))
      expect(paths).toContain("warranty_expires_at")
      expect(paths).toContain("count")
    }
  })

  it("rejects count > 1 with warranty notes", () => {
    const result = commoditySchema.safeParse({
      ...baseValues,
      warranty_notes: "covers parts only",
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      const paths = result.error.issues.map((i) => i.path.join("."))
      expect(paths).toContain("warranty_notes")
      expect(paths).toContain("count")
    }
  })

  it("accepts count > 1 with no warranty fields", () => {
    const result = commoditySchema.safeParse(baseValues)
    expect(result.success).toBe(true)
  })

  it("accepts count = 1 with warranty fields", () => {
    const result = commoditySchema.safeParse({
      ...baseValues,
      count: "1",
      warranty_expires_at: "2027-01-01",
      warranty_notes: "covers parts only",
    })
    expect(result.success).toBe(true)
  })
})

describe("commoditySchema — brand and model", () => {
  const values = {
    name: "Dishwasher",
    short_name: "Dishwasher",
    brand: "Bosch",
    model: "SMV6ZCX42E",
    type: "white_goods",
    area_id: "",
    seller_id: "",
    status: "in_use",
    count: "1",
    original_price: "",
    original_price_currency: "USD",
    converted_original_price: "",
    current_price: "",
    serial_number: "",
    extra_serial_numbers: [],
    part_numbers: [],
    tags: [],
    purchase_date: "",
    urls: [],
    comments: "",
    draft: true,
    warranty_expires_at: "",
    warranty_notes: "",
  }

  it("accepts both empty or up to 100 characters", () => {
    expect(commoditySchema.safeParse({ ...values, brand: "", model: "" }).success).toBe(true)
    expect(
      commoditySchema.safeParse({ ...values, brand: "b".repeat(100), model: "m".repeat(100) })
        .success
    ).toBe(true)
  })

  it.each([
    ["brand", "commodities:validation.brandTooLong"],
    ["model", "commodities:validation.modelTooLong"],
  ])("rejects a %s over 100 characters", (field, message) => {
    const result = commoditySchema.safeParse({ ...values, [field]: "x".repeat(101) })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues).toContainEqual(
        expect.objectContaining({ path: [field], message })
      )
    }
  })
})
