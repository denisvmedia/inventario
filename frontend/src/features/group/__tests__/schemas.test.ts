import { describe, expect, it } from "vitest"

import { createGroupSchema, deleteGroupSchema, updateGroupSchema } from "@/features/group/schemas"

describe("group schemas", () => {
  it("createGroupSchema requires name + valid currency", () => {
    expect(
      createGroupSchema.safeParse({
        name: "Household",
        icon: "",
        group_currency: "USD",
        kind: "household",
      }).success
    ).toBe(true)
    expect(
      createGroupSchema.safeParse({ name: "", icon: "", group_currency: "USD", kind: "household" })
        .success
    ).toBe(false)
    expect(
      createGroupSchema.safeParse({
        name: "Household",
        icon: "",
        group_currency: "us",
        kind: "household",
      }).success
    ).toBe(false)
  })

  it("createGroupSchema rejects unknown emoji icons", () => {
    // 🦄 isn't in the curated GROUP_ICONS list — it must be rejected.
    expect(
      createGroupSchema.safeParse({
        name: "Household",
        icon: "🦄",
        group_currency: "USD",
        kind: "household",
      }).success
    ).toBe(false)
    // The empty string is the explicit "no icon" sentinel.
    expect(
      createGroupSchema.safeParse({
        name: "Household",
        icon: "",
        group_currency: "USD",
        kind: "household",
      }).success
    ).toBe(true)
  })

  it("createGroupSchema takes a household or a collection group", () => {
    const base = { name: "Coins", icon: "", group_currency: "EUR" }
    expect(createGroupSchema.safeParse({ ...base, kind: "collection" }).success).toBe(true)
    expect(createGroupSchema.safeParse({ ...base, kind: "museum" }).success).toBe(false)
    expect(createGroupSchema.safeParse(base).success).toBe(false)
  })

  it("updateGroupSchema enforces name length cap", () => {
    expect(updateGroupSchema.safeParse({ name: "x".repeat(101), icon: "" }).success).toBe(false)
    expect(updateGroupSchema.safeParse({ name: "x".repeat(100), icon: "" }).success).toBe(true)
  })

  it("deleteGroupSchema requires both fields", () => {
    expect(deleteGroupSchema.safeParse({ confirmWord: "", password: "x" }).success).toBe(false)
    expect(deleteGroupSchema.safeParse({ confirmWord: "x", password: "" }).success).toBe(false)
    expect(deleteGroupSchema.safeParse({ confirmWord: "x", password: "x" }).success).toBe(true)
  })

  it("createGroupSchema trims and uppercases the currency", () => {
    const result = createGroupSchema.safeParse({
      name: "Household",
      icon: "",
      group_currency: " czk ",
      kind: "household",
    })
    expect(result.success).toBe(true)
    if (result.success) {
      expect(result.data.group_currency).toBe("CZK")
    }
  })
})
