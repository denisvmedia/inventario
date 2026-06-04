import { afterEach, describe, expect, it } from "vitest"

import { isUiDebugOverrideEnabled } from "@/lib/ui-debug"

const KEY = "inventario:debug-ui"

afterEach(() => {
  window.localStorage.clear()
  window.history.replaceState({}, "", "/")
})

describe("isUiDebugOverrideEnabled", () => {
  it("returns false with no query param and no stored flag", () => {
    expect(isUiDebugOverrideEnabled()).toBe(false)
  })

  it("enables and persists when ?debug=1 is present", () => {
    window.history.replaceState({}, "", "/?debug=1")
    expect(isUiDebugOverrideEnabled()).toBe(true)
    expect(window.localStorage.getItem(KEY)).toBe("1")
  })

  it("accepts truthy spellings (true/yes/on, case-insensitive)", () => {
    for (const v of ["true", "YES", "On"]) {
      window.history.replaceState({}, "", `/?debug=${v}`)
      expect(isUiDebugOverrideEnabled()).toBe(true)
    }
  })

  it("stays enabled on later loads via the persisted flag (no param)", () => {
    window.localStorage.setItem(KEY, "1")
    expect(isUiDebugOverrideEnabled()).toBe(true)
  })

  it("?debug=0 disables and clears the persisted flag", () => {
    window.localStorage.setItem(KEY, "1")
    window.history.replaceState({}, "", "/?debug=0")
    expect(isUiDebugOverrideEnabled()).toBe(false)
    expect(window.localStorage.getItem(KEY)).toBeNull()
  })
})
