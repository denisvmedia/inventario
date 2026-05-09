import { Navigate } from "react-router-dom"

import { useAuth } from "@/features/auth/AuthContext"
import { useCurrentGroup } from "@/features/group/GroupContext"

// Legacy `/system` URL → real GroupSettingsPage at `/groups/:id/settings`.
//
// Up to #1612 `/system` resolved (via UngroupedRedirect) to a placeholder
// page mounted at `/g/:slug/system`. That placeholder is gone — the real
// group settings live at `/groups/:groupId/settings` (issue #1413, on
// `GroupSettingsPage.tsx`). Bookmarks, sidebar muscle memory, and the
// e2e navigation smoke test still reach for `/system`, so this component
// keeps the legacy alias alive by redirecting to the active group's
// settings page using the same default-group-or-first-slug fallback as
// `RootRedirect` / `UngroupedRedirect`.
//
//   - auth/groups loading       → render nothing (spinner upstream).
//   - groups errored / empty    → /no-group (onboarding landing — same as
//                                 UngroupedRedirect for zero-group users).
//   - groups available          → /groups/<active-id>/settings, where
//                                 <active-id> is the user's
//                                 default_group_id when resolved, otherwise
//                                 the first group's id.
export function SystemRedirect() {
  const { user } = useAuth()
  const { groups, isLoading, isError } = useCurrentGroup()

  if (isLoading) return null
  if (isError || !groups || groups.length === 0) {
    return <Navigate to="/no-group" replace />
  }

  const preferredId = user?.default_group_id
  const preferred = preferredId && groups.find((g) => g.id === preferredId)
  const target = preferred || groups[0]
  if (!target?.id) {
    return <Navigate to="/no-group" replace />
  }
  return <Navigate to={`/groups/${encodeURIComponent(target.id)}/settings`} replace />
}
