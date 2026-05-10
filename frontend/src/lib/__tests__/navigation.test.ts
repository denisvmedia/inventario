import { describe, expect, it, vi, afterEach } from "vitest"

import { __resetNavigationForTests, navigateToLogin, setNavigateToLogin } from "@/lib/navigation"

afterEach(() => {
  __resetNavigationForTests()
})

describe("navigation", () => {
  it("default navigator does NOT touch window.location (no full-page reload)", () => {
    // The default navigator was a hard `window.location.href = …` until we
    // tracked a tab-reload-after-idle bug to that path firing whenever the
    // SPA navigator hadn't installed yet. The default now noops + warns —
    // anything that needs an actual route change must install via
    // `setNavigateToLogin` first. This test guards against re-introducing
    // the location.href assignment.
    const original = window.location
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...original, href: "about:blank" },
    })
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => undefined)
    navigateToLogin("/some/path", "session_expired")
    expect(window.location.href).toBe("about:blank")
    expect(warnSpy).toHaveBeenCalled()
    warnSpy.mockRestore()
    Object.defineProperty(window, "location", { writable: true, value: original })
  })

  it("setNavigateToLogin replaces the active navigator", () => {
    const spy = vi.fn()
    setNavigateToLogin(spy)
    navigateToLogin("/x", "auth_required")
    expect(spy).toHaveBeenCalledWith("/x", "auth_required")
  })

  it("__resetNavigationForTests restores the default (now a noop+warn)", () => {
    const spy = vi.fn()
    setNavigateToLogin(spy)
    __resetNavigationForTests()
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => undefined)
    navigateToLogin("/y")
    expect(spy).not.toHaveBeenCalled()
    expect(warnSpy).toHaveBeenCalled()
    warnSpy.mockRestore()
  })
})
