import { describe, expect, it, beforeEach } from "vitest"
import { act, render, renderHook } from "@testing-library/react"
import type { ReactNode } from "react"

import { ThemeProvider, useTheme } from "@/components/theme-provider"

beforeEach(() => {
  window.localStorage.clear()
  document.documentElement.classList.remove("light", "dark")
})

function makeWrapper(storageKey = "test-theme") {
  return ({ children }: { children: ReactNode }) => (
    <ThemeProvider storageKey={storageKey}>{children}</ThemeProvider>
  )
}

describe("useTheme", () => {
  it("setTheme writes the .dark class on <html> and persists to localStorage", () => {
    const { result } = renderHook(() => useTheme(), { wrapper: makeWrapper("test-theme") })
    act(() => result.current.setTheme("dark"))
    expect(result.current.theme).toBe("dark")
    expect(document.documentElement.classList.contains("dark")).toBe(true)
    expect(window.localStorage.getItem("test-theme")).toBe("dark")
  })

  it("setTheme('light') flips the resolved class to light", () => {
    const { result } = renderHook(() => useTheme(), { wrapper: makeWrapper("test-theme") })
    act(() => result.current.setTheme("dark"))
    act(() => result.current.setTheme("light"))
    expect(document.documentElement.classList.contains("light")).toBe(true)
    expect(document.documentElement.classList.contains("dark")).toBe(false)
  })

  it("rehydrates from localStorage on mount", () => {
    window.localStorage.setItem("test-theme", "dark")
    render(<ThemeProvider storageKey="test-theme" />)
    expect(document.documentElement.classList.contains("dark")).toBe(true)
  })
})
