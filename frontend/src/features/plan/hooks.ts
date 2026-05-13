import { useQuery } from "@tanstack/react-query"

import { getGroupPlan, type GroupPlanResult } from "./api"
import { planKeys } from "./keys"

// useGroupPlan fetches the active subscription plan + per-group usage
// for the GroupSettings Plan & quota card (#1389). The query is keyed
// by group slug + the queryFn threads the slug through to the API call,
// so cache identity and the actual request URL stay in lock-step (this
// matters because GroupSettings is a non-group route, so the http
// client's automatic /g/{slug}/ rewriter won't help us here — see
// `getGroupPlan` for the rationale).
//
// `enabled` gates on a truthy slug so the card can render placeholder
// content while the parent page is still resolving the group resource
// (the slug only lands after `useGroup(groupId)` returns).
export function useGroupPlan(groupSlug: string | undefined | null) {
  return useQuery<GroupPlanResult>({
    queryKey: planKeys.group(groupSlug ?? ""),
    queryFn: ({ signal }) => getGroupPlan(groupSlug!, signal),
    enabled: !!groupSlug,
  })
}
