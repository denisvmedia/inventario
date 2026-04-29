import { describe, expect, it, vi, beforeEach, afterEach } from "vitest"
import { act, renderHook } from "@testing-library/react"

import { useIsMobile } from "@/hooks/use-mobile"

interface MqlMock {
  matches: boolean
  listeners: Array<(event: { matches: boolean }) => void>
  fire(matches: boolean): void
}

let mql: MqlMock
let originalMatchMedia: typeof window.matchMedia

beforeEach(() => {
  mql = {
    matches: false,
    listeners: [],
    fire(matches: boolean) {
      this.matches = matches
      this.listeners.forEach((cb) => cb({ matches }))
    },
  }
  originalMatchMedia = window.matchMedia
  // Override the default jsdom stub from setup.ts with a controllable mock
  // so we can flip "mobile" on/off and assert the change-listener wiring.
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    media: query,
    get matches() {
      return mql.matches
    },
    addEventListener: (_type: string, cb: (event: { matches: boolean }) => void) => {
      mql.listeners.push(cb)
    },
    removeEventListener: (_type: string, cb: (event: { matches: boolean }) => void) => {
      mql.listeners = mql.listeners.filter((l) => l !== cb)
    },
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
    onchange: null,
  })) as unknown as typeof window.matchMedia
})

afterEach(() => {
  window.matchMedia = originalMatchMedia
})

describe("useIsMobile", () => {
  it("reads the initial value synchronously from matchMedia", () => {
    mql.matches = true
    const { result } = renderHook(() => useIsMobile())
    expect(result.current).toBe(true)
  })

  it("flips on media-query change events", () => {
    mql.matches = false
    const { result } = renderHook(() => useIsMobile())
    expect(result.current).toBe(false)
    act(() => mql.fire(true))
    expect(result.current).toBe(true)
    act(() => mql.fire(false))
    expect(result.current).toBe(false)
  })
})
