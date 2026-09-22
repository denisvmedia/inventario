import { describe, expect, it } from "vitest"

import { withId } from "@/lib/with-id"

describe("withId", () => {
  it("drops rows without an id", () => {
    const rows = [{ id: "a", name: "A" }, { name: "no id" }, { id: "b", name: "B" }]
    expect(withId(rows).map((r) => r.id)).toEqual(["a", "b"])
  })

  it("drops an empty-string id, which is the value Radix reserves", () => {
    expect(withId([{ id: "" }, { id: "x" }]).map((r) => r.id)).toEqual(["x"])
  })

  it("accepts undefined and null so a call site need not guard first", () => {
    expect(withId(undefined)).toEqual([])
    expect(withId(null)).toEqual([])
  })

  it("keeps the rest of the row", () => {
    expect(withId([{ id: "a", name: "A", extra: 1 }])).toEqual([{ id: "a", name: "A", extra: 1 }])
  })
})
