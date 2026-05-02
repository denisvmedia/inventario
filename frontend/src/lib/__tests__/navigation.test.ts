import { describe, expect, it, vi, afterEach } from "vitest"

import { __resetNavigationForTests, navigateToLogin, setNavigateToLogin } from "@/lib/navigation"

afterEach(() => {
  __resetNavigationForTests()
})

describe("navigation", () => {
  it("default navigator writes window.location.href with the redirect query", () => {
    const original = window.location
    // jsdom blocks direct assignment to window.location.href, so swap the
    // whole object with a captured spy. `delete` lets us redefine.
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...original, href: "" },
    })
    navigateToLogin("/some/path", "session_expired")
    expect(window.location.href).toBe("/login?redirect=%2Fsome%2Fpath&reason=session_expired")
    Object.defineProperty(window, "location", { writable: true, value: original })
  })

  it("setNavigateToLogin replaces the active navigator", () => {
    const spy = vi.fn()
    setNavigateToLogin(spy)
    navigateToLogin("/x", "auth_required")
    expect(spy).toHaveBeenCalledWith("/x", "auth_required")
  })

  it("__resetNavigationForTests restores the default", () => {
    const spy = vi.fn()
    setNavigateToLogin(spy)
    __resetNavigationForTests()
    // Default uses window.location which we don't want to bother
    // re-stubbing here; just assert the spy is no longer the active one.
    const original = window.location
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...original, href: "" },
    })
    navigateToLogin("/y")
    expect(spy).not.toHaveBeenCalled()
    Object.defineProperty(window, "location", { writable: true, value: original })
  })
})
