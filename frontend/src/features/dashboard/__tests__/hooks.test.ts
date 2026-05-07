import { describe, expect, it } from "vitest"

import { recentlyAdded, warrantyBuckets } from "@/features/dashboard/hooks"
import type { Commodity } from "@/features/commodities/api"

function commodity(over: Partial<Commodity>): Commodity {
  return { id: "c", name: "Item", ...over }
}

function daysFromNow(days: number): string {
  return new Date(Date.now() + days * 24 * 60 * 60 * 1000).toISOString().slice(0, 10)
}

describe("recentlyAdded", () => {
  it("sorts by registered_date desc and slices to the limit", () => {
    const items: Commodity[] = [
      commodity({ id: "old", name: "Old", registered_date: "2024-01-01" }),
      commodity({ id: "new", name: "New", registered_date: "2026-04-01" }),
      commodity({ id: "mid", name: "Mid", registered_date: "2025-06-15" }),
    ]
    const result = recentlyAdded(items, 2)
    expect(result.map((c) => c.id)).toEqual(["new", "mid"])
  })

  it("falls back to last_modified_date when registered_date is missing", () => {
    const items: Commodity[] = [
      commodity({ id: "a", registered_date: "2025-01-01" }),
      commodity({ id: "b", last_modified_date: "2026-04-01" }),
      commodity({ id: "c", registered_date: "2024-01-01" }),
    ]
    expect(recentlyAdded(items, 5).map((c) => c.id)).toEqual(["b", "a", "c"])
  })

  it("treats missing dates as oldest", () => {
    const items: Commodity[] = [
      commodity({ id: "no-date" }),
      commodity({ id: "dated", registered_date: "2025-01-01" }),
    ]
    expect(recentlyAdded(items, 5).map((c) => c.id)).toEqual(["dated", "no-date"])
  })

  it("returns an empty array when given no items", () => {
    expect(recentlyAdded([], 5)).toEqual([])
  })

  it("does not mutate the input array", () => {
    const items: Commodity[] = [
      commodity({ id: "a", registered_date: "2024-01-01" }),
      commodity({ id: "b", registered_date: "2026-01-01" }),
    ]
    const snapshot = items.map((c) => c.id)
    recentlyAdded(items, 5)
    expect(items.map((c) => c.id)).toEqual(snapshot)
  })
})

describe("warrantyBuckets", () => {
  it("counts each commodity into the right warranty bucket", () => {
    const items: Commodity[] = [
      commodity({ id: "a", warranty_expires_at: "2099-01-01" }),
      commodity({ id: "b", warranty_expires_at: daysFromNow(30) }),
      commodity({ id: "c", warranty_expires_at: "1999-01-01" }),
      commodity({ id: "d" }),
      commodity({ id: "e", warranty_expires_at: daysFromNow(10) }),
    ]
    const { counts } = warrantyBuckets(items, 5)
    expect(counts).toEqual({ active: 1, expiring: 2, expired: 1, none: 1 })
  })

  it("returns up to N expiring rows sorted by expiry ascending", () => {
    const items: Commodity[] = [
      commodity({ id: "later", warranty_expires_at: daysFromNow(45) }),
      commodity({ id: "soonest", warranty_expires_at: daysFromNow(2) }),
      commodity({ id: "middle", warranty_expires_at: daysFromNow(20) }),
      commodity({ id: "limit-out", warranty_expires_at: daysFromNow(50) }),
    ]
    const { expiring } = warrantyBuckets(items, 2)
    expect(expiring.map((c) => c.id)).toEqual(["soonest", "middle"])
  })

  it("falls back to the legacy warranty:YYYY-MM-DD tag when the dedicated field is missing", () => {
    const items: Commodity[] = [
      commodity({ id: "tagged-active", tags: ["warranty:2099-01-01"] }),
      commodity({ id: "tagged-expired", tags: ["warranty:1999-01-01"] }),
    ]
    const { counts } = warrantyBuckets(items, 5)
    expect(counts.active).toBe(1)
    expect(counts.expired).toBe(1)
  })
})
