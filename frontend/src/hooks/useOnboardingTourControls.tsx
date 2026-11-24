import { createContext, useContext, type ReactNode } from "react"

// Why the product tour cannot start right now:
//   - "noGroup": the steps point at a group's sidebar entries, which render
//     only with an active group (#2787).
//   - "narrowScreen": below the mobile breakpoint the sidebar is a closed
//     sheet, so nothing the tour points at is on screen.
export type TourBlocker = "noGroup" | "narrowScreen"

export interface OnboardingTourControls {
  // Clears the "seen" flag and opens the tour at its first step.
  restart: () => void
  // null when the tour can start. Entry points use it to disable themselves
  // and say why, instead of offering a button that does nothing.
  blocker: TourBlocker | null
}

const Context = createContext<OnboardingTourControls | undefined>(undefined)

interface OnboardingTourControlsProviderProps {
  value: OnboardingTourControls
  children: ReactNode
}

// Shell owns the tour state (useOnboardingTour) and provides its controls
// here, so pages rendered in the Shell outlet (Settings → Help) can offer
// "Restart product tour" next to the user-menu entry.
export function OnboardingTourControlsProvider({
  value,
  children,
}: OnboardingTourControlsProviderProps) {
  return <Context.Provider value={value}>{children}</Context.Provider>
}

// Throws outside the provider so a page that lost its Shell fails loudly
// instead of hiding the entry point.
export function useOnboardingTourControls(): OnboardingTourControls {
  const ctx = useContext(Context)
  if (!ctx) {
    throw new Error(
      "useOnboardingTourControls must be used inside <OnboardingTourControlsProvider>"
    )
  }
  return ctx
}
