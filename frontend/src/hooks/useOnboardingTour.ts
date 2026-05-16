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

export interface UseOnboardingTour {
  // Is the tour currently visible.
  isOpen: boolean
  // 0-based step index, only meaningful when isOpen is true.
  step: number
  // Total steps in TOUR_STEPS — convenient for component consumers.
  totalSteps: number
  // Imperative open / close handlers passed to the component.
  open: () => void
  next: () => void
  prev: () => void
  finish: () => void
  skip: () => void
  // Manual restart from a "Tour" button. Clears the "seen" flag and
  // opens at step 0 — same as a fresh user's first visit.
  restart: () => void
}

/**
 * Drives the OnboardingTour overlay. Auto-launches once per user on first
 * authenticated render (when `userId` flips from null to a real value),
 * unless the user has already finished or skipped the tour. The user-id
 * key isolates per-account state on shared browsers.
 *
 * Pass `autoLaunch={false}` for callers that just want imperative control
 * (e.g. a Storybook demo or the dev-only `/_dev/ui-showcase` route).
 */
export function useOnboardingTour(
  userId: string | null | undefined,
  options: { autoLaunch?: boolean } = {}
): UseOnboardingTour {
  const autoLaunch = options.autoLaunch ?? true
  const totalSteps = TOUR_STEPS.length

  const [isOpen, setIsOpen] = useState(false)
  const [step, setStep] = useState(0)

  // Auto-launch on first authenticated render. Skipped when the user has
  // already finished or skipped. Re-runs only when userId changes — a
  // user logging out and a different user logging in will trigger the
  // probe again for the new user. setState-in-effect is intentional
  // here: the launch is a side-effect of an external value (userId)
  // becoming known, which is exactly when this rule's escape hatch
  // applies. Same pattern as SearchPage / GroupSettingsPage.
  useEffect(() => {
    if (!autoLaunch) return
    if (!userId) return
    const seen = readSeen(userId)
    if (!seen) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setStep(0)
      setIsOpen(true)
    }
  }, [autoLaunch, userId])

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

  const close = useCallback(
    (markSeen: boolean) => {
      setIsOpen(false)
      setStep(0)
      if (markSeen) writeSeen(userId, true)
    },
    [userId]
  )

  const finish = useCallback(() => close(true), [close])
  const skip = useCallback(() => close(true), [close])

  const restart = useCallback(() => {
    writeSeen(userId, false)
    setStep(0)
    setIsOpen(true)
  }, [userId])

  return useMemo(
    () => ({ isOpen, step, totalSteps, open, next, prev, finish, skip, restart }),
    [isOpen, step, totalSteps, open, next, prev, finish, skip, restart]
  )
}

// Test-only helper: clear the persisted "seen" flag for a given user.
export function __resetTourSeenForTests(userId: string): void {
  writeSeen(userId, false)
}
