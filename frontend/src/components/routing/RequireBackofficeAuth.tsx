import type { ReactNode } from "react"
import { Navigate, useLocation } from "react-router-dom"

import { useBackofficeAuth } from "@/features/backoffice/auth/context"

interface RequireBackofficeAuthProps {
  children: ReactNode
  // Rendered while the boot probe is still resolving who the operator
  // is. Defaults to nothing so the guard renders empty for a few
  // hundred ms rather than flashing the login page at an operator
  // who's already signed in.
  fallback?: ReactNode
}

// Gates the /admin/* subtree on a valid back-office session (#1785
// Phase 6). Bounces unauthenticated operators to /backoffice/login while
// preserving where they were trying to go (so post-login navigation can
// return them). Replaces the previous RequireSystemAdmin guard which
// branched on the tenant token's is_system_admin claim — Phase 3 hardened
// /admin/* to require a back-office plane token, so the FE guard MUST
// follow suit. A tenant user with is_system_admin=true is no longer the
// signal; the signal is "the back-office plane returned a user".
//
// Mirrors ProtectedRoute's tri-state branching on `user`:
//   - undefined → still resolving. Render fallback.
//   - null      → definitively logged out of the back-office plane.
//                 Redirect to /backoffice/login.
//   - object    → render children.
export function RequireBackofficeAuth({ children, fallback = null }: RequireBackofficeAuthProps) {
  const { user, isInitialized } = useBackofficeAuth()
  const location = useLocation()
  if (!isInitialized) return <>{fallback}</>
  if (user === undefined) return <>{fallback}</>
  if (user === null) {
    const redirect = location.pathname + location.search
    const params = new URLSearchParams({ redirect, reason: "auth_required" })
    return <Navigate to={`/backoffice/login?${params.toString()}`} replace />
  }
  return <>{children}</>
}
