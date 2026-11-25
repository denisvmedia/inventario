import { afterEach, describe, expect, it, beforeEach } from "vitest"
import { act, renderHook } from "@testing-library/react"

import { TOUR_STEPS } from "@/components/OnboardingTour"
import { useOnboardingTour, __resetTourSeenForTests } from "@/hooks/useOnboardingTour"

const USER = "user-abc"
const SEEN_KEY = `inventario-tour-seen-v1:${USER}`

beforeEach(() => {
  window.localStorage.clear()
})

afterEach(() => {
  __resetTourSeenForTests(USER)
})

describe("useOnboardingTour", () => {
  it("auto-launches for a brand-new user with an active group", () => {
    const { result } = renderHook(() => useOnboardingTour(USER, { hasGroup: true }))
    expect(result.current.isOpen).toBe(true)
    expect(result.current.step).toBe(0)
    expect(result.current.totalSteps).toBe(TOUR_STEPS.length)
  })

  it("does not auto-launch without a group and leaves the seen flag alone (#2787)", () => {
    const { result } = renderHook(() => useOnboardingTour(USER, { hasGroup: false }))
    expect(result.current.isOpen).toBe(false)
    expect(window.localStorage.getItem(SEEN_KEY)).toBeNull()
  })

  it("launches the first time a group becomes active (#2787)", () => {
    const { result, rerender } = renderHook(
      ({ hasGroup }) => useOnboardingTour(USER, { hasGroup }),
      { initialProps: { hasGroup: false } }
    )
    expect(result.current.isOpen).toBe(false)
    rerender({ hasGroup: true })
    expect(result.current.isOpen).toBe(true)
    expect(result.current.step).toBe(0)
  })

  it("does not launch once a group is active if the user has already seen the tour", () => {
    window.localStorage.setItem(SEEN_KEY, "1")
    const { result, rerender } = renderHook(
      ({ hasGroup }) => useOnboardingTour(USER, { hasGroup }),
      { initialProps: { hasGroup: false } }
    )
    rerender({ hasGroup: true })
    expect(result.current.isOpen).toBe(false)
  })

  it("does not auto-launch when the user has already seen the tour", () => {
    window.localStorage.setItem(SEEN_KEY, "1")
    const { result } = renderHook(() => useOnboardingTour(USER, { hasGroup: true }))
    expect(result.current.isOpen).toBe(false)
  })

  it("does not auto-launch when userId is null (no authenticated user)", () => {
    const { result } = renderHook(() => useOnboardingTour(null, { hasGroup: true }))
    expect(result.current.isOpen).toBe(false)
  })

  it("skip() persists the seen flag and closes the tour", () => {
    const { result } = renderHook(() => useOnboardingTour(USER, { hasGroup: true }))
    expect(result.current.isOpen).toBe(true)
    act(() => result.current.skip())
    expect(result.current.isOpen).toBe(false)
    expect(window.localStorage.getItem(SEEN_KEY)).toBe("1")
  })

  it("finish() persists the seen flag and closes the tour", () => {
    const { result } = renderHook(() => useOnboardingTour(USER, { hasGroup: true }))
    act(() => result.current.finish())
    expect(result.current.isOpen).toBe(false)
    expect(window.localStorage.getItem(SEEN_KEY)).toBe("1")
  })

  it("close() closes the tour without marking it as seen", () => {
    // OnboardingTour calls close() when no step has a target on the page;
    // the user must still get the tour once it can show something.
    const { result } = renderHook(() => useOnboardingTour(USER, { hasGroup: true }))
    act(() => result.current.close())
    expect(result.current.isOpen).toBe(false)
    expect(window.localStorage.getItem(SEEN_KEY)).toBeNull()
  })

  it("next/prev clamps within [0, totalSteps - 1]", () => {
    const { result } = renderHook(() => useOnboardingTour(USER, { hasGroup: true }))
    expect(result.current.step).toBe(0)
    act(() => result.current.prev())
    expect(result.current.step).toBe(0)
    for (let i = 0; i < TOUR_STEPS.length; i += 1) {
      act(() => result.current.next())
    }
    expect(result.current.step).toBe(TOUR_STEPS.length - 1)
  })

  it("restart() clears the seen flag and re-opens at step 0", () => {
    const { result } = renderHook(() => useOnboardingTour(USER, { hasGroup: true }))
    act(() => result.current.next())
    act(() => result.current.skip())
    expect(result.current.isOpen).toBe(false)
    expect(window.localStorage.getItem(SEEN_KEY)).toBe("1")
    act(() => result.current.restart())
    expect(result.current.isOpen).toBe(true)
    expect(result.current.step).toBe(0)
    expect(window.localStorage.getItem(SEEN_KEY)).toBeNull()
  })

  it("keeps the step of an open tour when autoLaunch turns off and on again", () => {
    // Shell does this when the window crosses the mobile breakpoint for a
    // moment; the tour must not jump back to its first step.
    const { result, rerender } = renderHook(
      ({ autoLaunch }) => useOnboardingTour(USER, { hasGroup: true, autoLaunch }),
      { initialProps: { autoLaunch: true } }
    )
    act(() => result.current.next())
    act(() => result.current.next())
    rerender({ autoLaunch: false })
    rerender({ autoLaunch: true })
    expect(result.current.isOpen).toBe(true)
    expect(result.current.step).toBe(2)
  })

  it("autoLaunch=false suppresses the auto-launch even for a fresh user", () => {
    const { result } = renderHook(() =>
      useOnboardingTour(USER, { hasGroup: true, autoLaunch: false })
    )
    expect(result.current.isOpen).toBe(false)
    act(() => result.current.open())
    expect(result.current.isOpen).toBe(true)
  })

  it("does not auto-launch in WebDriver sessions (Playwright e2e carve-out)", () => {
    // Existing e2e specs log in fresh, and the auto-launched tour's
    // full-screen overlay would intercept their sidebar clicks. Detecting
    // navigator.webdriver short-circuits the launch — restart() still
    // works imperatively if a test wants to exercise the tour. #1543.
    Object.defineProperty(window.navigator, "webdriver", {
      configurable: true,
      value: true,
    })
    try {
      const { result } = renderHook(() => useOnboardingTour(USER, { hasGroup: true }))
      expect(result.current.isOpen).toBe(false)
      // restart() still works regardless of webdriver state.
      act(() => result.current.restart())
      expect(result.current.isOpen).toBe(true)
    } finally {
      Object.defineProperty(window.navigator, "webdriver", {
        configurable: true,
        value: false,
      })
    }
  })
})
