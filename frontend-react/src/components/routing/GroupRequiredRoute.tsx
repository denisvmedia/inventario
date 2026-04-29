import type { ReactNode } from "react"
import { Navigate } from "react-router-dom"

import { useCurrentGroup } from "@/features/group/GroupContext"

interface GroupRequiredRouteProps {
  children: ReactNode
  fallback?: ReactNode
}

// Bounces a logged-in user with zero groups to /no-group (the guided
// first-run page). Mirrors the legacy Vue guard's behaviour for routes
// outside GROUP_EXEMPT_ROUTE_NAMES — those onboarding-friendly routes
// (login/register/forgot/profile/groups-create/no-group/invite/verify)
// don't wrap their children in this component.
//
// Renders the fallback while groups are still loading so we don't flicker
// to /no-group on the first paint of an authenticated session.
export function GroupRequiredRoute({ children, fallback = null }: GroupRequiredRouteProps) {
  const { groups, isLoading, isError } = useCurrentGroup()
  if (isLoading) return <>{fallback}</>
  // If the groups query errored we let the page render its own error state
  // rather than redirect-loop the user — same fail-open stance as the
  // legacy guard.
  if (isError) return <>{children}</>
  if (!groups || groups.length === 0) return <Navigate to="/no-group" replace />
  return <>{children}</>
}
