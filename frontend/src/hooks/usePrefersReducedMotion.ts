import { useEffect, useState } from "react"

const QUERY = "(prefers-reduced-motion: reduce)"

// Read the preference synchronously on first render so the very first
// paint already reflects it — no desktop→reduced flash. SSR-safe and
// degrades to "motion allowed" when matchMedia is unavailable (older
// jsdom in unit tests stubs a static `matches: false`).
function getInitialPrefersReducedMotion(): boolean {
  if (typeof window === "undefined" || !window.matchMedia) return false
  return window.matchMedia(QUERY).matches
}

// usePrefersReducedMotion reports whether the user asked the OS to
// minimise non-essential motion. Class-based fades honour this directly
// through Tailwind's `motion-reduce` variant; this hook exists for the
// cases that must decide in JS — e.g. the fullscreen image viewer, whose
// opacity transition is set via inline style (composed with a transform
// transition) and so can't be gated by a CSS media query.
export function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState<boolean>(getInitialPrefersReducedMotion)

  useEffect(() => {
    if (!window.matchMedia) return
    const mql = window.matchMedia(QUERY)
    const onChange = () => setReduced(mql.matches)
    // Sync once on mount in case the preference flipped between the
    // synchronous init read and the effect running.
    // eslint-disable-next-line react-hooks/set-state-in-effect -- subscribing to an external API (matchMedia)
    setReduced(mql.matches)
    mql.addEventListener("change", onChange)
    return () => mql.removeEventListener("change", onChange)
  }, [])

  return reduced
}
