import { describe, expect, it } from "vitest"
import { renderHook } from "@testing-library/react"

import { useNavLabel } from "@/lib/nav-labels"

describe("useNavLabel", () => {
  // Each known key resolves through a per-arm translator call against the
  // common namespace. This grid asserts both that the catalog entries
  // exist (the en bundle is preloaded by setup.ts) and that the helper
  // covers every nav item the sidebar / palette declare.
  const cases: Array<[string, string]> = [
    ["common:nav.dashboard", "Dashboard"],
    ["common:nav.locations", "Locations"],
    ["common:nav.items", "All Items"],
    ["common:nav.warranties", "Warranties"],
    ["common:nav.tags", "Tags"],
    ["common:nav.files", "Files"],
    ["common:nav.members", "Members"],
    ["common:nav.backup", "Backup"],
    ["common:nav.system", "Settings"],
    ["common:nav.profile", "Profile"],
    ["common:nav.preferences", "Preferences"],
    ["common:nav.search", "Search"],
  ]

  it.each(cases)("resolves %s → %s", (key, expected) => {
    const { result } = renderHook(() => useNavLabel(key))
    expect(result.current).toBe(expected)
  })

  it("falls back to the raw key for unknown values (defensive)", () => {
    const { result } = renderHook(() => useNavLabel("common:nav.does-not-exist"))
    expect(result.current).toBe("common:nav.does-not-exist")
  })
})
