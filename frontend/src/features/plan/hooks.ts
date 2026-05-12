import { useQuery } from "@tanstack/react-query"

import { getGroupPlan, type GroupPlanResult } from "./api"
import { planKeys } from "./keys"

// useGroupPlan fetches the active subscription plan + per-group usage
// for the GroupSettings Plan & quota card (#1389). The query is keyed
// by group slug — the BE resolves the (tenant, group) pair from the
// rewritten URL `/g/{slug}/plan`, so a slug change means a different
// dataset and a clean cache miss.
//
// `enabled` gates on a truthy slug so the card can render placeholder
// content while the parent page is still resolving the group resource
// (the slug only lands after `useGroup(groupId)` returns).
export function useGroupPlan(groupSlug: string | undefined | null) {
  return useQuery<GroupPlanResult>({
    queryKey: planKeys.group(groupSlug ?? ""),
    queryFn: ({ signal }) => getGroupPlan(signal),
    enabled: !!groupSlug,
  })
}
