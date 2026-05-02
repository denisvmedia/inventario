import * as React from "react"

const MOBILE_BREAKPOINT = 768

// Initial value is computed synchronously from window.matchMedia so the
// first render already knows whether we're on a mobile breakpoint. The
// previous "undefined → effect → setState" pattern caused a single-frame
// flash of the desktop branch on small viewports (the sidebar would mount
// in its desktop state before the effect re-classified). SSR-safe: returns
// `false` when window isn't defined.
function getInitialIsMobile(): boolean {
  if (typeof window === "undefined") return false
  return window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`).matches
}

export function useIsMobile() {
  const [isMobile, setIsMobile] = React.useState<boolean>(getInitialIsMobile)

  React.useEffect(() => {
    const mql = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`)
    const onChange = () => {
      setIsMobile(mql.matches)
    }
    // Sync once on mount in case the viewport changed between the synchronous
    // init read and the effect running (rare, but cheap to cover).
    setIsMobile(mql.matches)
    mql.addEventListener("change", onChange)
    return () => mql.removeEventListener("change", onChange)
  }, [])

  return isMobile
}
