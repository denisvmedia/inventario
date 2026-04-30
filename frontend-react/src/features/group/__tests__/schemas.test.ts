import { describe, expect, it } from "vitest"

import { createGroupSchema, deleteGroupSchema, updateGroupSchema } from "@/features/group/schemas"

describe("group schemas", () => {
  it("createGroupSchema requires name + valid currency", () => {
    expect(
      createGroupSchema.safeParse({ name: "Household", icon: "", main_currency: "USD" }).success
    ).toBe(true)
    expect(createGroupSchema.safeParse({ name: "", icon: "", main_currency: "USD" }).success).toBe(
      false
    )
    expect(
      createGroupSchema.safeParse({ name: "Household", icon: "", main_currency: "us" }).success
    ).toBe(false)
  })

  it("createGroupSchema rejects unknown emoji icons", () => {
    // 🦄 isn't in the curated GROUP_ICONS list — it must be rejected.
    expect(
      createGroupSchema.safeParse({ name: "Household", icon: "🦄", main_currency: "USD" }).success
    ).toBe(false)
    // The empty string is the explicit "no icon" sentinel.
    expect(
      createGroupSchema.safeParse({ name: "Household", icon: "", main_currency: "USD" }).success
    ).toBe(true)
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
      main_currency: " czk ",
    })
    expect(result.success).toBe(true)
    if (result.success) {
      expect(result.data.main_currency).toBe("CZK")
    }
  })
})
