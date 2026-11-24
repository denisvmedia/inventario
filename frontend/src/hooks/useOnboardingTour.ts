import { useCallback, useEffect, useMemo, useState } from "react"

import { TOUR_STEPS } from "@/components/OnboardingTour"

// Per-user "tour completed" persistence in localStorage. Keyed by user id
// so a shared browser doesn't see somebody else's "skipped" flag on first
// login. The "v1" suffix lets us invalidate every prior completion if the
// step list changes meaningfully — bump to v2 and old skips become opens
// again, surfacing the refreshed tour. (Not a big-bang reset: a returning
// user just sees the new tour once.)
//
// We never persist the *current step* — the tour always restarts from
// step 0 on (re-)open. Tracking mid-flight position adds storage churn
// and the recovery story for stale targets ("step 4 of 7 but the row
// no longer exists") is worse than restarting.
const STORAGE_KEY_PREFIX = "inventario-tour-seen-v1:"

function storageKey(userId: string | null | undefined): string | null {
  if (!userId) return null
  return `${STORAGE_KEY_PREFIX}${userId}`
}

function readSeen(userId: string | null | undefined): boolean {
  const key = storageKey(userId)
  if (!key || typeof window === "undefined") return true // unknown user → treat as seen (no auto-launch)
  try {
    return window.localStorage.getItem(key) === "1"
  } catch {
    return true
  }
}

function writeSeen(userId: string | null | undefined, seen: boolean): void {
  const key = storageKey(userId)
  if (!key || typeof window === "undefined") return
  try {
    if (seen) window.localStorage.setItem(key, "1")
    else window.localStorage.removeItem(key)
  } catch {
    /* localStorage unavailable (Safari private mode) — silently fall through */
  }
}

export interface UseOnboardingTourOptions {
  // Whether the user has an active group. The tour's steps point at the
  // group's sidebar entries (Add item, Dashboard, Locations, …), which only
  // render with a group, so the tour does not auto-launch without one. It
  // launches the first time a group becomes active instead (#2787).
  hasGroup: boolean
  // Defaults to true. Pass false for callers that want imperative control
  // only, or while nothing the tour points at is on screen (Shell passes
  // false below the mobile breakpoint, where the sidebar is a closed sheet).
  autoLaunch?: boolean
}

export interface UseOnboardingTour {
  // Is the tour currently visible.
  isOpen: boolean
  // 0-based position among the steps OnboardingTour shows (steps whose
  // target is missing are skipped), only meaningful when isOpen is true.
  step: number
  // Number of steps defined in TOUR_STEPS. OnboardingTour may show fewer.
  totalSteps: number
  // Imperative open / close handlers passed to the component.
  open: () => void
  next: () => void
  prev: () => void
  finish: () => void
  skip: () => void
  // Closes the tour without marking it as seen. OnboardingTour calls it
  // when no step has a target on the page, so the user still gets the
  // tour the next time it can show something.
  close: () => void
  // Manual restart from a "Restart product tour" entry. Clears the "seen"
  // flag and opens at step 0 — same as a fresh user's first visit.
  restart: () => void
}

/**
 * Drives the OnboardingTour overlay. Auto-launches once per user, on the
 * first render where the user is known (`userId` set) AND has an active
 * group (`hasGroup`), unless the user has already finished or skipped the
 * tour. The user-id key isolates per-account state on shared browsers.
 *
 * Without a group the tour stays closed and the "seen" flag is left
 * alone, so the tour starts once the user creates or joins a group.
 */
export function useOnboardingTour(
  userId: string | null | undefined,
  options: UseOnboardingTourOptions
): UseOnboardingTour {
  const { hasGroup } = options
  const autoLaunch = options.autoLaunch ?? true
  const totalSteps = TOUR_STEPS.length

  const [isOpen, setIsOpen] = useState(false)
  const [step, setStep] = useState(0)

  // Auto-launch once the user and an active group are known. Skipped when
  // the user has already finished or skipped. Re-runs when userId or
  // hasGroup changes — a different user logging in, or a user creating
  // their first group, triggers the probe again. setState-in-effect is
  // intentional here: the launch is a side-effect of external values
  // becoming known, which is exactly when this rule's escape hatch
  // applies. Same pattern as SearchPage / GroupSettingsPage.
  //
  // E2E carve-out: Playwright / WebDriver sessions also auto-launch into
  // a fresh user, but the tour's full-screen overlay (#1543, z-9999)
  // would intercept every sidebar click in the existing test suite. We
  // detect navigator.webdriver (set to true by every WebDriver runner
  // including Playwright) and skip the auto-launch in that environment.
  // The user can still call `restart()` imperatively — the test would
  // just have to do that explicitly if it wants to exercise the tour.
  useEffect(() => {
    if (!autoLaunch) return
    if (!userId) return
    if (!hasGroup) return
    if (typeof navigator !== "undefined" && navigator.webdriver) return
    const seen = readSeen(userId)
    if (!seen) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setStep(0)
      setIsOpen(true)
    }
  }, [autoLaunch, userId, hasGroup])

  const open = useCallback(() => {
    setStep(0)
    setIsOpen(true)
  }, [])

  const next = useCallback(() => {
    setStep((s) => Math.min(s + 1, totalSteps - 1))
  }, [totalSteps])

  const prev = useCallback(() => {
    setStep((s) => Math.max(s - 1, 0))
  }, [])

  const closeTour = useCallback(
    (markSeen: boolean) => {
      setIsOpen(false)
      setStep(0)
      if (markSeen) writeSeen(userId, true)
    },
    [userId]
  )

  const finish = useCallback(() => closeTour(true), [closeTour])
  const skip = useCallback(() => closeTour(true), [closeTour])
  const close = useCallback(() => closeTour(false), [closeTour])

  const restart = useCallback(() => {
    writeSeen(userId, false)
    setStep(0)
    setIsOpen(true)
  }, [userId])

  return useMemo(
    () => ({ isOpen, step, totalSteps, open, next, prev, finish, skip, close, restart }),
    [isOpen, step, totalSteps, open, next, prev, finish, skip, close, restart]
  )
}

// Test-only helper: clear the persisted "seen" flag for a given user.
export function __resetTourSeenForTests(userId: string): void {
  writeSeen(userId, false)
}
