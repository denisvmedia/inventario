import { describe, expect, it } from "vitest"

import { recentlyAdded } from "@/features/dashboard/hooks"
import type { Commodity } from "@/features/commodities/api"

function commodity(over: Partial<Commodity>): Commodity {
  return { id: "c", name: "Item", ...over }
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
