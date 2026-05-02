import type { ReactNode } from "react"
import { Navigate, useLocation } from "react-router-dom"

import { useAuth } from "@/features/auth/AuthContext"

interface ProtectedRouteProps {
  children: ReactNode
  // Shown while the boot probe is in flight or while the backend is in a
  // transient error state. Lets callers swap in a sidebar-shaped skeleton
  // later; defaults to nothing so the route renders empty for a few hundred
  // ms instead of flashing the login page.
  fallback?: ReactNode
}

// Bounces unauthenticated users to /login while preserving where they were
// trying to go (so post-login navigation can return them — handled in the
// Auth pages issue, #1407). Renders the fallback while the initial /auth/me
// probe is in flight or in an unknown state (transient backend error),
// otherwise the user would briefly see the protected page or be bounced to
// /login on a non-auth-related blip.
//
// Tri-state branching on `user`:
//   - undefined → still resolving (or transient backend error). Render fallback.
//   - null      → definitively logged out (no token, or 401). Redirect to /login.
//   - object    → render children.
export function ProtectedRoute({ children, fallback = null }: ProtectedRouteProps) {
  const { user, isInitialized } = useAuth()
  const location = useLocation()
  if (!isInitialized) return <>{fallback}</>
  if (user === undefined) return <>{fallback}</>
  if (user === null) {
    const redirect = location.pathname + location.search
    const params = new URLSearchParams({ redirect, reason: "auth_required" })
    return <Navigate to={`/login?${params.toString()}`} replace />
  }
  return <>{children}</>
}
