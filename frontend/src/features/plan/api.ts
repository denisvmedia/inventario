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

// The plain JSON response (the endpoint is render.JSON, not JSON:API)
// already matches `GroupPlanResult`, so no envelope unwrap is needed.
export async function getGroupPlan(signal?: AbortSignal): Promise<GroupPlanResult> {
  // `/plan` is rewritten to `/g/{slug}/plan` by the group-scoped URL
  // helper (`http.ts` GROUP_SCOPED_PREFIXES). Callers don't need to
  // thread the slug here — `withGroupQuery` / context handle it.
  return http.get<GroupPlanResult>("/plan", { signal })
}
