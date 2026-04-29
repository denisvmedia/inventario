import type { ReactNode } from "react"
import { Navigate, useLocation } from "react-router-dom"

import { useAuth } from "@/features/auth/AuthContext"

interface ProtectedRouteProps {
  children: ReactNode
  // Shown while the boot probe is in flight. Lets callers swap in a sidebar-
  // shaped skeleton later; defaults to nothing so the route renders empty
  // for a few hundred ms instead of flashing the login page.
  fallback?: ReactNode
}

// Bounces unauthenticated users to /login while preserving where they were
// trying to go (so post-login navigation can return them — handled in the
// Auth pages issue, #1407). Renders nothing while the initial /auth/me probe
// is in flight, otherwise the user would briefly see the protected page
// before being redirected.
export function ProtectedRoute({ children, fallback = null }: ProtectedRouteProps) {
  const { isAuthenticated, isInitialized } = useAuth()
  const location = useLocation()
  if (!isInitialized) return <>{fallback}</>
  if (!isAuthenticated) {
    const redirect = location.pathname + location.search
    return <Navigate to={`/login?redirect=${encodeURIComponent(redirect)}`} replace />
  }
  return <>{children}</>
}
