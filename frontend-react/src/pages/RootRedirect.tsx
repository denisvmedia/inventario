import { Navigate } from "react-router-dom"

import { useAuth } from "@/features/auth/AuthContext"
import { useCurrentGroup } from "@/features/group/GroupContext"

// RootRedirect handles the "/" landing for an authenticated user. The legacy
// frontend's "/" was a redirect sentinel: the dashboard cannot render there
// because every dashboard widget hits a /api/v1/g/{slug}/* endpoint, so
// without a group the page would 404 in pieces. We mirror that here.
//
// Priority chain (per #1404 + #1263):
//   1. user.default_group_id → group with that id, if user is still a member
//   2. groups[0]              → first group the user has
//   3. /no-group              → onboarding-friendly empty state
//
// Loading and error fall through to "no-group" / null so a transient blip
// never sits the user on a half-rendered "/".
export function RootRedirect() {
  const { user } = useAuth()
  const { groups, isLoading, isError } = useCurrentGroup()
  if (isLoading) return null
  if (isError || !groups || groups.length === 0) {
    return <Navigate to="/no-group" replace />
  }
  const preferredId = user?.default_group_id
  const preferred = preferredId ? groups.find((g) => g.id === preferredId) : undefined
  const target = preferred ?? groups[0]
  return <Navigate to={`/g/${encodeURIComponent(target.slug ?? "")}`} replace />
}
