import { describe, expect, it, beforeEach } from "vitest"
import { act, render, renderHook } from "@testing-library/react"
import type { ReactNode } from "react"

import { DensityProvider, useDensity } from "@/hooks/useDensity"

beforeEach(() => {
  window.localStorage.clear()
  document.documentElement.removeAttribute("data-density")
})

function makeWrapper(storageKey = "test-density") {
  return function DensityWrapper({ children }: { children: ReactNode }) {
    return <DensityProvider storageKey={storageKey}>{children}</DensityProvider>
  }
}

describe("useDensity", () => {
  it("defaults to comfortable and writes data-density on <html>", () => {
    render(<DensityProvider />)
    expect(document.documentElement.getAttribute("data-density")).toBe("comfortable")
  })

  it("setDensity persists the choice in localStorage and updates the attribute", () => {
    const { result } = renderHook(() => useDensity(), { wrapper: makeWrapper("test-density") })
    act(() => result.current.setDensity("compact"))
    expect(result.current.density).toBe("compact")
    expect(document.documentElement.getAttribute("data-density")).toBe("compact")
    expect(window.localStorage.getItem("test-density")).toBe("compact")
  })

  it("rehydrates from localStorage on mount", () => {
    window.localStorage.setItem("test-density", "cozy")
    const { result } = renderHook(() => useDensity(), { wrapper: makeWrapper("test-density") })
    expect(result.current.density).toBe("cozy")
    expect(document.documentElement.getAttribute("data-density")).toBe("cozy")
  })

  it("ignores garbage in localStorage and falls back to the default", () => {
    window.localStorage.setItem("test-density", "ultra-tight")
    const { result } = renderHook(() => useDensity(), { wrapper: makeWrapper("test-density") })
    expect(result.current.density).toBe("comfortable")
  })
})
