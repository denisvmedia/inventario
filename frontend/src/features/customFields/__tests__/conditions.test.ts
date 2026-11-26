import { describe, expect, it } from "vitest"

import {
  formatCustomFieldCondition,
  isSortableField,
  opsForType,
  parseCustomFieldCondition,
  staleCustomFieldIds,
} from "@/features/customFields/conditions"

describe("parseCustomFieldCondition / formatCustomFieldCondition", () => {
  it("reads set and unset without a value and the others with one", () => {
    expect(parseCustomFieldCondition("cf1:set")).toEqual({ fieldId: "cf1", op: "set", value: "" })
    expect(parseCustomFieldCondition("cf1:gte:10")).toEqual({
      fieldId: "cf1",
      op: "gte",
      value: "10",
    })
    // Everything after the second colon is the value.
    expect(parseCustomFieldCondition("cf1:eq:10:30")).toEqual({
      fieldId: "cf1",
      op: "eq",
      value: "10:30",
    })
  })

  it("refuses a condition it cannot read", () => {
    expect(parseCustomFieldCondition("cf1")).toBeNull()
    expect(parseCustomFieldCondition(":set")).toBeNull()
    expect(parseCustomFieldCondition("cf1:like:x")).toBeNull()
    expect(parseCustomFieldCondition("cf1:set:x")).toBeNull()
    expect(parseCustomFieldCondition("cf1:eq")).toBeNull()
  })

  it("writes back what it reads", () => {
    for (const raw of ["cf1:unset", "cf1:in:a,b", "cf1:contains:a:b"]) {
      const cond = parseCustomFieldCondition(raw)
      expect(cond && formatCustomFieldCondition(cond)).toBe(raw)
    }
  })
})

describe("opsForType / isSortableField", () => {
  it("offers the comparisons each type takes", () => {
    expect(opsForType("text")).toContain("contains")
    expect(opsForType("number")).toEqual(["eq", "gte", "lte", "set", "unset"])
    expect(opsForType("select")).toEqual(["in", "set", "unset"])
    expect(opsForType("item_link")).toEqual(["set", "unset"])
  })

  it("sorts by every type but long text and links", () => {
    expect(isSortableField({ type: "date" })).toBe(true)
    expect(isSortableField({ type: "long_text" })).toBe(false)
    expect(isSortableField({ type: "item_link" })).toBe(false)
  })
})

describe("staleCustomFieldIds", () => {
  it("names each deleted field of the conditions and the sort once", () => {
    expect(
      staleCustomFieldIds(["a:set", "gone:set", "gone:eq:1", "bad"], "old", new Set(["a"]))
    ).toEqual(["gone", "old"])
    expect(staleCustomFieldIds(["a:set"], "", new Set(["a"]))).toEqual([])
  })
})
