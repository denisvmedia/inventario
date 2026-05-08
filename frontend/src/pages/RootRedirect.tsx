import { Navigate } from "react-router-dom"

import { useAuth } from "@/features/auth/AuthContext"
import { useCurrentGroup } from "@/features/group/GroupContext"

// RootRedirect handles the "/" landing for an authenticated user. The legacy
// frontend's "/" was a redirect sentinel: the dashboard cannot render there
// because every dashboard widget hits a /api/v1/g/{slug}/* endpoint, so
// without a group the page would 404 in pieces. We mirror that here.
//
// Priority chain (per #1592):
//   1. user.default_group_id → group with that id, if user is still a member
//      AND that group has a slug. The backend's EnsureDefaultGroup invariant
//      guarantees this is set whenever the user has ≥1 membership.
//   2. /no-group → onboarding-friendly empty state.
//
// The legacy "first group with a slug" fallback (#1263) is gone: under the
// invariant, default_group_id is non-NULL whenever there's anywhere to land,
// so a missing/stale preference is a real "let's get the user oriented"
// signal instead of something to paper over with arbitrary pick.
//
// Loading and error fall through to "no-group" / null so a transient blip
// never sits the user on a half-rendered "/". Slug is checked explicitly
// because the generated LocationGroup type marks it as optional — building
// "/g/" with an empty slug would drop into the 404.
export function RootRedirect() {
  const { user } = useAuth()
  const { groups, isLoading, isError } = useCurrentGroup()
  if (isLoading) return null
  if (isError || !groups || groups.length === 0) {
    return <Navigate to="/no-group" replace />
  }
  const preferredId = user?.default_group_id
  const preferred = preferredId && groups.find((g) => g.id === preferredId && !!g.slug)
  if (!preferred || !preferred.slug) {
    return <Navigate to="/no-group" replace />
  }
  return <Navigate to={`/g/${encodeURIComponent(preferred.slug)}`} replace />
}
