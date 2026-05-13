// Data-layer for the subscription-plan slice. Hooks live in `./hooks.ts`.
//
// In v1 there is a single endpoint — `GET /g/{slug}/plan` — returning the
// active plan + per-group usage in one payload. The plan catalogue + the
// limits-aware enforcement layer (`plan.Enforce`) from #1389 are deferred
// to follow-up iterations.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type Plan = Schema<"models.Plan">
export type PlanUsage = Schema<"models.PlanUsage">
export type GroupPlanResult = Schema<"models.GroupPlanResult">

// getGroupPlan takes the group slug explicitly rather than relying on the
// http client's group-scoped URL rewriter: the Plan card is mounted on
// GroupSettings (`/groups/:groupId/settings`), which is a non-group
// route — there's no `currentGroupSlug` for the rewriter to splice in,
// so a bare `/plan` would 404. Building the full `/g/{slug}/plan` path
// here keeps the call working regardless of the parent route.
export async function getGroupPlan(
  groupSlug: string,
  signal?: AbortSignal
): Promise<GroupPlanResult> {
  return http.get<GroupPlanResult>(`/g/${encodeURIComponent(groupSlug)}/plan`, { signal })
}
