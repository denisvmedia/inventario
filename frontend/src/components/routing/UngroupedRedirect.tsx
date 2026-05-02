import { Navigate, useLocation } from "react-router-dom"

import { useAuth } from "@/features/auth/AuthContext"
import { useCurrentGroup } from "@/features/group/GroupContext"

// UngroupedRedirect handles legacy / hardcoded URLs that don't carry a group
// slug — for instance /files, /locations, /commodities, /exports, /system.
//
// The Vue frontend mounted those at the top level; the React port (#1404)
// pushed every group-scoped resource under /g/:slug/<path>. Some external
// links (and a chunk of the e2e suite) still point at the unprefixed paths,
// so this component preserves that contract:
//
//   - groups loading           → render nothing (let the spinner upstream
//                                  decide; we don't want to flash a redirect
//                                  through the fallback).
//   - groups errored / empty   → /no-group (the onboarding landing).
//   - groups available         → /g/<active-slug>/<original-path>, where
//                                <active-slug> is the user's
//                                default_group_id when it resolves to a
//                                slug, otherwise the first usable slug.
//
// The original `pathname` (minus the leading slash) is appended as-is, so
// /files becomes /g/<slug>/files, /commodities/abc/edit becomes
// /g/<slug>/commodities/abc/edit. Search/hash are preserved so links like
// /files?category=photos survive the bounce.
export function UngroupedRedirect() {
  const { user } = useAuth()
  const { groups, isLoading, isError } = useCurrentGroup()
  const location = useLocation()

  if (isLoading) return null
  if (isError || !groups || groups.length === 0) {
    return <Navigate to="/no-group" replace />
  }

  const preferredId = user?.default_group_id
  const preferred = preferredId && groups.find((g) => g.id === preferredId && !!g.slug)
  const target = preferred || groups.find((g) => !!g.slug)
  if (!target || !target.slug) {
    return <Navigate to="/no-group" replace />
  }

  // Preserve trailing path verbatim — only the slug is injected before it.
  const stripped = location.pathname.replace(/^\/+/, "")
  const dest = `/g/${encodeURIComponent(target.slug)}/${stripped}${location.search}${location.hash}`
  return <Navigate to={dest} replace />
}
