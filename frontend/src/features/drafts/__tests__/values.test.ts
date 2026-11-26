import { describe, expect, it } from "vitest"

import { draftSellerName, parseDraftValues } from "@/features/drafts/values"

describe("parseDraftValues", () => {
  it("keeps the values the form can use", () => {
    expect(
      parseDraftValues({
        name: "Drill",
        short_name: "Drill",
        count: "2",
        tags: ["tools"],
        draft: false,
      })
    ).toEqual({ name: "Drill", short_name: "Drill", count: "2", tags: ["tools"], draft: false })
  })

  it("drops a known field of the wrong type so the form default applies", () => {
    expect(
      parseDraftValues({
        name: "Drill",
        short_name: null,
        brand: 42,
        tags: "tools",
        draft: "yes",
        urls: ["https://example.com", 7],
      })
    ).toEqual({ name: "Drill", urls: ["https://example.com"] })
  })

  it("reads numbers as the text the form keeps", () => {
    expect(parseDraftValues({ count: 3, original_price: 12.5 })).toEqual({
      count: "3",
      original_price: "12.5",
    })
  })

  it("keeps keys the form does not know", () => {
    expect(parseDraftValues({ name: "Drill", source: { kind: "intake" } })).toEqual({
      name: "Drill",
      source: { kind: "intake" },
    })
  })

  it("gives nothing for values that are not an object", () => {
    expect(parseDraftValues(null)).toEqual({})
    expect(parseDraftValues("Drill")).toEqual({})
    expect(parseDraftValues([1, 2])).toEqual({})
  })
})

describe("draftSellerName", () => {
  it("reads the seller a batch intake draft names", () => {
    expect(draftSellerName(parseDraftValues({ seller_name: " Sample Store " }))).toBe(
      "Sample Store"
    )
  })

  it("gives nothing when a seller is picked or the name is not text", () => {
    expect(draftSellerName(parseDraftValues({ seller_name: "Shop", seller_id: "s1" }))).toBe("")
    expect(draftSellerName(parseDraftValues({ seller_name: 7 }))).toBe("")
    expect(draftSellerName({})).toBe("")
  })
})
