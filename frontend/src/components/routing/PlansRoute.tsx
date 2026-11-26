import { Navigate } from "react-router-dom"

import { ComingSoonPage } from "@/components/coming-soon"
import { useFeatureFlags } from "@/features/feature-flags/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"

// PlansRoute owns /plans (#1389). The plans feature is frozen and shows
// only when the deployment turns the `plans` flag on; then the route is
// the "coming soon" page it always was. With the flag off nothing links
// here, so a visit comes from an old link or a bookmark, and the route
// sends it to the group home: /g/<slug> for the group named by ?g=, or "/"
// otherwise, which resolves the user's group (or /no-group).
//
// It renders nothing until the flags and the groups have loaded, so a
// deployment with plans on never bounces off the page while they load,
// and the ?g= group is known before the redirect picks a target. A
// failed flags request counts as off.
export function PlansRoute() {
  const flagsQuery = useFeatureFlags()
  const { currentGroup, isLoading: groupsLoading } = useCurrentGroup()

  if (flagsQuery.isPending || groupsLoading) return null
  if (flagsQuery.data?.plans === true) return <ComingSoonPage surface="plans" />

  const home = currentGroup?.slug ? `/g/${encodeURIComponent(currentGroup.slug)}` : "/"
  return <Navigate to={home} replace />
}
