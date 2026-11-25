import { describe, expect, it } from "vitest"

import type { CustomField } from "@/features/customFields/api"
import {
  customFieldsRequest,
  toApiValue,
  toFormValue,
  toFormValues,
  validateFormValue,
} from "@/features/customFields/values"

function field(id: string, type: CustomField["type"]): CustomField {
  return { id, name: id, type, options: [], templates: [] }
}

describe("toFormValue / toFormValues", () => {
  it("reads every API form into the form's text", () => {
    expect(toFormValue("AB 123")).toEqual({ value: "AB 123", currency: "" })
    expect(toFormValue(0)).toEqual({ value: "0", currency: "" })
    expect(toFormValue(false)).toEqual({ value: "false", currency: "" })
    expect(toFormValue({ amount: 12.5, currency: "EUR" })).toEqual({
      value: "12.5",
      currency: "EUR",
    })
  })

  it("keeps zero, false and empty text, and drops null", () => {
    expect(toFormValues({ a: 0, b: false, c: "", d: null })).toEqual({
      a: { value: "0", currency: "" },
      b: { value: "false", currency: "" },
      c: { value: "", currency: "" },
    })
    expect(toFormValues(undefined)).toEqual({})
  })
})

describe("toApiValue", () => {
  it("turns the form's text into the API form of the type", () => {
    expect(toApiValue("number", { value: " 12.50 ", currency: "" })).toBe(12.5)
    expect(toApiValue("money", { value: "3", currency: "eur" })).toEqual({
      amount: 3,
      currency: "EUR",
    })
    expect(toApiValue("boolean", { value: "false", currency: "" })).toBe(false)
    expect(toApiValue("text", { value: "  x ", currency: "" })).toBe("x")
    expect(toApiValue("text", { value: "  ", currency: "" })).toBeNull()
  })
})

describe("customFieldsRequest", () => {
  const fields = [field("plate", "text"), field("weight", "number"), field("fuel", "select")]

  it("sends the values held and clears a stored one the form dropped", () => {
    expect(
      customFieldsRequest(
        fields,
        { plate: "AB", weight: 2, unknown: "kept" },
        { plate: { value: "XY", currency: "" }, fuel: { value: "opt-1", currency: "" } }
      )
    ).toEqual({ plate: "XY", weight: null, fuel: "opt-1" })
  })

  it("clears a value handed over as null", () => {
    expect(customFieldsRequest(fields, { weight: null }, {})).toEqual({ weight: null })
  })

  it("sends nothing when there is nothing to send", () => {
    expect(customFieldsRequest(fields, undefined, { plate: { value: "", currency: "" } })).toBe(
      undefined
    )
  })
})

describe("validateFormValue", () => {
  it("refuses what the server refuses", () => {
    const v = (value: string, currency = "") => ({ value, currency })
    expect(validateFormValue(field("n", "number"), v("1e3"))).toBe("number")
    expect(validateFormValue(field("n", "number"), v("1234567890123456"))).toBe("numberTooLong")
    expect(validateFormValue(field("n", "number"), v("-12.5"))).toBeUndefined()
    expect(validateFormValue(field("m", "money"), v("1.234", "EUR"))).toBe("numberTooLong")
    expect(validateFormValue(field("m", "money"), v("1.23", "EU"))).toBe("currency")
    expect(validateFormValue(field("u", "url"), v("ftp://x"))).toBe("url")
    expect(validateFormValue(field("u", "url"), v("https://example.com"))).toBeUndefined()
    expect(validateFormValue(field("t", "text"), v("x".repeat(201)))).toBe("tooLong")
    expect(validateFormValue(field("t", "text"), v(""))).toBeUndefined()
  })
})
