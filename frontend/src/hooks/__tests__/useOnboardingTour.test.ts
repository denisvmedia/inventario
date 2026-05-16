import { afterEach, describe, expect, it, beforeEach } from "vitest"
import { act, renderHook } from "@testing-library/react"

import { TOUR_STEPS } from "@/components/OnboardingTour"
import { useOnboardingTour, __resetTourSeenForTests } from "@/hooks/useOnboardingTour"

const USER = "user-abc"

beforeEach(() => {
  window.localStorage.clear()
})

afterEach(() => {
  __resetTourSeenForTests(USER)
})

describe("useOnboardingTour", () => {
  it("auto-launches on first authenticated render for a brand-new user", () => {
    const { result } = renderHook(() => useOnboardingTour(USER))
    expect(result.current.isOpen).toBe(true)
    expect(result.current.step).toBe(0)
    expect(result.current.totalSteps).toBe(TOUR_STEPS.length)
  })

  it("does not auto-launch when the user has already seen the tour", () => {
    window.localStorage.setItem(`inventario-tour-seen-v1:${USER}`, "1")
    const { result } = renderHook(() => useOnboardingTour(USER))
    expect(result.current.isOpen).toBe(false)
  })

  it("does not auto-launch when userId is null (no authenticated user)", () => {
    const { result } = renderHook(() => useOnboardingTour(null))
    expect(result.current.isOpen).toBe(false)
  })

  it("skip() persists the seen flag and closes the tour", () => {
    const { result } = renderHook(() => useOnboardingTour(USER))
    expect(result.current.isOpen).toBe(true)
    act(() => result.current.skip())
    expect(result.current.isOpen).toBe(false)
    expect(window.localStorage.getItem(`inventario-tour-seen-v1:${USER}`)).toBe("1")
  })

  it("finish() persists the seen flag and closes the tour", () => {
    const { result } = renderHook(() => useOnboardingTour(USER))
    act(() => result.current.finish())
    expect(result.current.isOpen).toBe(false)
    expect(window.localStorage.getItem(`inventario-tour-seen-v1:${USER}`)).toBe("1")
  })

  it("next/prev clamps within [0, totalSteps - 1]", () => {
    const { result } = renderHook(() => useOnboardingTour(USER))
    expect(result.current.step).toBe(0)
    act(() => result.current.prev())
    expect(result.current.step).toBe(0)
    for (let i = 0; i < TOUR_STEPS.length; i += 1) {
      act(() => result.current.next())
    }
    expect(result.current.step).toBe(TOUR_STEPS.length - 1)
  })

  it("restart() clears the seen flag and re-opens at step 0", () => {
    const { result } = renderHook(() => useOnboardingTour(USER))
    act(() => result.current.skip())
    expect(result.current.isOpen).toBe(false)
    act(() => result.current.restart())
    expect(result.current.isOpen).toBe(true)
    expect(result.current.step).toBe(0)
    expect(window.localStorage.getItem(`inventario-tour-seen-v1:${USER}`)).toBeNull()
  })

  it("autoLaunch=false suppresses the auto-launch even for a fresh user", () => {
    const { result } = renderHook(() => useOnboardingTour(USER, { autoLaunch: false }))
    expect(result.current.isOpen).toBe(false)
    act(() => result.current.open())
    expect(result.current.isOpen).toBe(true)
  })
})
