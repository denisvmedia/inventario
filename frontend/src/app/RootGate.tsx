import { lazy } from "react"

import { useAuth } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { RootRedirect } from "@/pages/RootRedirect"

// LandingPage is the public, unauthenticated "/" surface (#1988). Lazy
// so its chunk (incl. the anonymous create dialog + AI scan step) never
// weighs on an authenticated user's entry bundle.
const LandingPage = lazy(() =>
  import("@/pages/LandingPage").then((m) => ({ default: m.LandingPage }))
)

// RootGate owns "/" for BOTH planes (#1988). Before #1988, "/" sat
// inside the ProtectedRoute → GroupProvider → GroupRequiredRoute tree and
// always resolved via RootRedirect. Now "/" is a PUBLIC route mounted
// above ProtectedRoute, and this gate decides what it shows:
//
//   - boot (!isInitialized, or user still resolving)  → null
//     Mirror ProtectedRoute's tri-state boot guard so a logged-in user
//     who refreshes on "/" never flashes the anonymous landing before
//     the /auth/me probe settles.
//   - logged out (user === null)                      → <LandingPage />
//     The anonymous "add your first item before login" CTA.
//   - logged in (user object)                         → RootRedirect
//     Wrapped in its own GroupProvider — RootRedirect calls
//     useCurrentGroup(), which the public "/" route would otherwise have
//     no provider for (the authenticated subtree's GroupProvider is a
//     sibling, not an ancestor of this gate). Mounting a provider here
//     resolves the slug and redirects to /g/<slug> or /no-group exactly
//     as before.
export function RootGate() {
  const { user, isInitialized } = useAuth()
  // Boot guard: identical shape to ProtectedRoute so the two never
  // disagree about when the auth state is "known".
  if (!isInitialized) return null
  if (user === undefined) return null
  if (user === null) return <LandingPage />
  return (
    <GroupProvider>
      <RootRedirect />
    </GroupProvider>
  )
}
